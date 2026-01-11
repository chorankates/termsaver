package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gdamore/tcell/v2"
)

func main() {
	var mode = flag.String("mode", "matrix", "Visualization mode: matrix, nyancat, snake, missiledefender, spectrograph, or snowflakes")
	var interactive = flag.Bool("interactive", false, "Enable interactive mode (for snake: use arrow keys to play)")
	var grayscale = flag.Bool("grayscale", false, "Use grayscale colors instead of colors")
	flag.Parse()

	screen, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating screen: %v\n", err)
		os.Exit(1)
	}

	if err := screen.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing screen: %v\n", err)
		os.Exit(1)
	}
	defer screen.Fini()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start the selected visualization
	switch *mode {
	case "matrix":
		runMatrixRain(screen, sigChan, *grayscale)
	case "nyancat":
		runNyancat(screen, sigChan, *grayscale)
	case "snake":
		runSnake(screen, sigChan, *interactive, *grayscale)
	case "missiledefender":
		runMissileDefender(screen, sigChan, *grayscale)
	case "spectrograph":
		runSpectrograph(screen, sigChan, *grayscale)
	case "snowflakes":
		runSnowflakes(screen, sigChan, *grayscale)
	default:
		screen.Fini()
		fmt.Fprintf(os.Stderr, "Unknown mode: %s. Use: matrix, nyancat, snake, missiledefender, spectrograph, or snowflakes\n", *mode)
		os.Exit(1)
	}
}

// toGrayscale converts a color to grayscale if grayscale mode is enabled
func toGrayscale(color tcell.Color, grayscale bool) tcell.Color {
	if !grayscale {
		return color
	}
	
	// Map colors to grayscale equivalents based on typical brightness
	switch color {
	case tcell.ColorWhite:
		return tcell.ColorWhite
	case tcell.ColorBlack:
		return tcell.ColorBlack
	case tcell.ColorYellow, tcell.ColorLime, tcell.ColorOrange:
		return tcell.ColorWhite
	case tcell.ColorGreen, tcell.ColorBlue, tcell.ColorRed, tcell.ColorPurple,
		 tcell.ColorAqua, tcell.ColorFuchsia, tcell.ColorPink:
		return tcell.ColorGray
	case tcell.ColorDarkGray:
		return tcell.ColorDarkGray
	default:
		// For any other colors, use a default gray
		return tcell.ColorGray
	}
}

