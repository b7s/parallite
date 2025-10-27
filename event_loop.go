package main

import (
	"context"
	"errors"
	"sync"
	"time"
)

type loopTask struct {
	fn func(context.Context)
}

// EventLoop centralizes orchestrator coordination, ensuring ordering guarantees similar to libuv.
type EventLoop struct {
	ctx        context.Context
	cancel     context.CancelFunc
	workCh     chan loopTask
	stopOnce   sync.Once
	wg         sync.WaitGroup
	timersLock sync.Mutex
	timers     map[*time.Timer]struct{}
	shutdown   chan struct{}
}

func NewEventLoop(parent context.Context, queueSize int) *EventLoop {
	if queueSize <= 0 {
		queueSize = 256
	}
	ctx, cancel := context.WithCancel(parent)
	return &EventLoop{
		ctx:      ctx,
		cancel:   cancel,
		workCh:   make(chan loopTask, queueSize),
		timers:   make(map[*time.Timer]struct{}),
		shutdown: make(chan struct{}),
	}
}

func (l *EventLoop) Start() {
	l.wg.Add(1)
	go l.run()
}

func (l *EventLoop) run() {
	defer l.wg.Done()
	for {
		select {
		case <-l.ctx.Done():
			l.drainTimers()
			close(l.shutdown)
			return
		case task := <-l.workCh:
			func() {
				defer func() {
					if r := recover(); r != nil {
						logError("panic recovered in event loop: %v", r)
					}
				}()
				task.fn(l.ctx)
			}()
		}
	}
}

func (l *EventLoop) Post(fn func(context.Context)) error {
	if fn == nil {
		return errors.New("event loop task cannot be nil")
	}
	select {
	case <-l.ctx.Done():
		return context.Canceled
	case l.workCh <- loopTask{fn: fn}:
		return nil
	}
}

func (l *EventLoop) Context() context.Context {
	return l.ctx
}

func (l *EventLoop) PostDelayed(delay time.Duration, fn func(context.Context)) error {
	if fn == nil {
		return errors.New("event loop delayed task cannot be nil")
	}
	if delay < 0 {
		delay = 0
	}
	timer := time.NewTimer(delay)
	l.timersLock.Lock()
	l.timers[timer] = struct{}{}
	l.timersLock.Unlock()

	go func() {
		defer l.removeTimer(timer)
		select {
		case <-l.ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
			_ = l.Post(fn)
		}
	}()
	return nil
}

func (l *EventLoop) removeTimer(timer *time.Timer) {
	l.timersLock.Lock()
	defer l.timersLock.Unlock()
	delete(l.timers, timer)
}

func (l *EventLoop) drainTimers() {
	l.timersLock.Lock()
	defer l.timersLock.Unlock()
	for timer := range l.timers {
		timer.Stop()
		delete(l.timers, timer)
	}
}

func (l *EventLoop) Stop() {
	l.stopOnce.Do(func() {
		l.cancel()
	})
	<-l.shutdown
	l.wg.Wait()
}
