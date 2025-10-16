.PHONY: build clean run install test test-client cross-compile release

# Build the binary with version from version file
build:
	@VERSION=$$(cat version 2>/dev/null || echo "dev"); \
	go build -ldflags="-X main.Version=$$VERSION" -o parallite main.go

# Build with version
build-version:
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION is required. Use: make build-version VERSION=v1.0.0"; \
		exit 1; \
	fi
	go build -ldflags="-X main.Version=$(VERSION)" -o parallite main.go

# Build test client
test-client:
	cd test && go build -o test-client client.go

# Clean build artifacts
clean:
	rm -f parallite parallite.exe parallite-* parallite.sqlite*
	rm -f test/test-client test/test-client.exe

# Run the daemon
run: build
	./parallite

# Install dependencies
install:
	go mod download

# Cross-compile for all platforms with version
cross-compile:
	@VERSION=$$(cat version 2>/dev/null || echo "dev"); \
	GOOS=linux GOARCH=amd64 go build -ldflags="-X main.Version=$$VERSION" -o parallite-linux main.go; \
	GOOS=darwin GOARCH=amd64 go build -ldflags="-X main.Version=$$VERSION" -o parallite-macos main.go; \
	GOOS=windows GOARCH=amd64 go build -ldflags="-X main.Version=$$VERSION" -o parallite.exe main.go

# Interactive release process
release:
	@echo "🚀 Parallite Release Process"
	@echo ""
	@CURRENT_VERSION=$$(cat version 2>/dev/null || echo "unknown"); \
	echo "Current version: $$CURRENT_VERSION"; \
	echo ""; \
	read -p "Enter new version (format: v0.0.0): " VERSION; \
	if [ -z "$$VERSION" ]; then \
		echo "❌ Version cannot be empty"; \
		exit 1; \
	fi; \
	if ! echo "$$VERSION" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$$'; then \
		echo "❌ Invalid version format. Use: v0.0.0"; \
		exit 1; \
	fi; \
	echo ""; \
	echo "📝 Enter release message (press Ctrl+D when done):"; \
	MESSAGE=$$(cat); \
	if [ -z "$$MESSAGE" ]; then \
		MESSAGE="Release $$VERSION"; \
	fi; \
	echo ""; \
	echo "📋 Summary:"; \
	echo "  Version: $$VERSION"; \
	echo "  Message: $$MESSAGE"; \
	echo ""; \
	read -p "Continue with release? [y/N]: " CONFIRM; \
	if [ "$$CONFIRM" != "y" ] && [ "$$CONFIRM" != "Y" ]; then \
		echo "❌ Release cancelled"; \
		exit 1; \
	fi; \
	echo ""; \
	echo "📝 Updating version file..."; \
	echo "$$VERSION" > version; \
	echo "✅ Version updated in version file"; \
	echo ""; \
	echo "🔨 Building with version $$VERSION..."; \
	go build -ldflags="-X main.Version=$$VERSION" -o parallite main.go || exit 1; \
	echo "✅ Build successful"; \
	echo ""; \
	echo "📦 Committing changes..."; \
	git add version; \
	git add -A; \
	if git diff --cached --quiet; then \
		echo "ℹ️  No changes to commit"; \
	else \
		git commit -m "chore: bump version to $$VERSION" || exit 1; \
		echo "✅ Changes committed"; \
	fi; \
	echo ""; \
	echo "🏷️  Creating tag $$VERSION..."; \
	git tag -a "$$VERSION" -m "$$MESSAGE" || exit 1; \
	echo "✅ Tag created"; \
	echo ""; \
	echo "⬆️  Pushing to remote..."; \
	git push origin main || exit 1; \
	git push origin "$$VERSION" || exit 1; \
	echo "✅ Pushed to remote"; \
	echo ""; \
	echo "🎉 Release $$VERSION completed successfully!"; \
	echo ""; \
	echo "GitHub Actions will now build binaries for all platforms."; \
	echo "Check: https://github.com/b7s/parallite/actions"

# Default target
all: build
