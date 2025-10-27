package main

import (
	"context"
	"errors"
	"runtime"
	"sync"
)

type blockingJob func(context.Context) error

type BlockingPool struct {
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	once     sync.Once
	capacity int
	jobs     chan blockingJob
}

func NewBlockingPool(parent context.Context, capacity int) *BlockingPool {
	if capacity <= 0 {
		capacity = runtime.NumCPU()
		if capacity < 2 {
			capacity = 2
		}
	}
	ctx, cancel := context.WithCancel(parent)
	pool := &BlockingPool{
		ctx:      ctx,
		cancel:   cancel,
		capacity: capacity,
		jobs:     make(chan blockingJob, capacity*2),
	}

	pool.start()
	return pool
}

func (p *BlockingPool) start() {
	for i := 0; i < p.capacity; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

func (p *BlockingPool) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case job, ok := <-p.jobs:
			if !ok {
				return
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						logError("panic recovered in blocking pool: %v", r)
					}
				}()
				job(p.ctx)
			}()
		}
	}
}

func (p *BlockingPool) Submit(fn blockingJob) error {
	if fn == nil {
		return errors.New("blocking job cannot be nil")
	}
	select {
	case <-p.ctx.Done():
		return context.Canceled
	case p.jobs <- fn:
		return nil
	}
}

func (p *BlockingPool) Stop() {
	p.once.Do(func() {
		p.cancel()
		close(p.jobs)
	})
	p.wg.Wait()
}
