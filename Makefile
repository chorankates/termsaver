.PHONY: build clean all

# Build all platforms
all: darwin-arm64 linux-arm linux-arm64 linux-amd64

# Build for macOS ARM (Apple Silicon)
darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -o bin/termsaver-darwin-arm64 .

# Build for Linux ARM
linux-arm:
	GOOS=linux GOARCH=arm go build -o bin/termsaver-linux-arm .

# Build for Linux ARM64/AARCH64
linux-arm64:
	GOOS=linux GOARCH=arm64 go build -o bin/termsaver-linux-arm64 .

# Build for Linux x86_64
linux-amd64:
	GOOS=linux GOARCH=amd64 go build -o bin/termsaver-linux-amd64 .

# Build for current platform
build:
	go build -o bin/termsaver .

# Clean build artifacts
clean:
	rm -rf bin/


