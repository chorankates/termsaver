# termsaver

A terminal screensaver with multiple visualizations, written in Go.

## Features

- **Matrix Rain**: Classic falling characters effect with katakana, hiragana, and alphanumeric characters
- **Nyancat**: Animated rainbow-trailing cat flying through space
- **Snake**: Classic Nokia-style snake game (use arrow keys to play)
- **Missile Defender**: Automatic tower defense game where towers defend against incoming missiles (towers and terrain randomize every 30-45 seconds)
- **Spectrograph**: Fake audio spectrograph with animated colored bars that continuously change for screensaver purposes

## Installation

### Build from Source

1. Clone the repository:
```bash
git clone https://github.com/chorankates/termsaver.git
cd termsaver
```

2. Build for your current platform:
```bash
make build
# or
go build -o bin/termsaver .
```

3. Run the screensaver:
```bash
./bin/termsaver -mode matrix
```

### Cross-Platform Builds

Build for all supported platforms:
```bash
make all
```

This will create binaries in the `bin/` directory for:
- `termsaver-darwin-arm64` - macOS on ARM (Apple Silicon)
- `termsaver-linux-arm` - Linux on ARM
- `termsaver-linux-arm64` - Linux on ARM64/AARCH64
- `termsaver-linux-amd64` - Linux on x86_64

Build for a specific platform:
```bash
make darwin-arm64
make linux-arm
make linux-arm64
make linux-amd64
```

Or manually with Go:
```bash
GOOS=darwin GOARCH=arm64 go build -o termsaver-darwin-arm64 .
GOOS=linux GOARCH=arm go build -o termsaver-linux-arm .
GOOS=linux GOARCH=arm64 go build -o termsaver-linux-arm64 .
GOOS=linux GOARCH=amd64 go build -o termsaver-linux-amd64 .
```

## Usage

Run the screensaver with a specific mode:

```bash
./termsaver -mode matrix   # Matrix rain effect
./termsaver -mode nyancat  # Flying rainbow cat
./termsaver -mode snake    # Snake game (automatic by default)
./termsaver -mode snake -interactive  # Snake game with manual control
./termsaver -mode missiledefender  # Tower defense game (fully automatic)
./termsaver -mode spectrograph  # Fake audio spectrograph with animated colored bars
```

### Controls

- **Matrix/Nyancat/Spectrograph**: Press `Esc` or `Ctrl+C` to exit
- **Snake (interactive mode)**: Use arrow keys to control the snake, `Esc` or `Ctrl+C` to exit
- **Snake (automatic mode, default)**: The snake plays perfectly by itself using pathfinding AI
- **Missile Defender**: Fully automatic - the program plays both attack and defense sides. Press `Esc` or `Ctrl+C` to exit

## Requirements

- Go 1.21 or later
- A terminal emulator that supports ANSI colors (most modern terminals)

## Dependencies

- [tcell](https://github.com/gdamore/tcell) - Terminal cell-based UI library

## License

MIT
