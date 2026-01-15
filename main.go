package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"
)

func main() {
	var mode = flag.String("mode", "random", "Visualization mode: matrix, nyancat, snake, missiledefender, spectrograph, snowflakes, waterripple, or random")
	var interactive = flag.Bool("interactive", false, "Enable interactive mode (for snake: use arrow keys to play)")
	var grayscale = flag.Bool("grayscale", false, "Use grayscale colors instead of colors")
	var windChangeTime = flag.Float64("wind-change-time", 3.0, "Time in seconds between wind direction changes (snowflakes mode)")
	var windStrength = flag.Float64("wind-strength", 0.8, "Baseline wind strength (-1.0 to 1.0, snowflakes mode)")
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

	// Handle random mode selection
	selectedMode := *mode
	if selectedMode == "random" {
		modes := []string{"matrix", "nyancat", "snake", "missiledefender", "spectrograph", "snowflakes", "waterripple"}
		rand.Seed(time.Now().UnixNano())
		selectedMode = modes[rand.Intn(len(modes))]
	}

	// Start the selected visualization
	switch selectedMode {
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
		runSnowflakes(screen, sigChan, *grayscale, *windChangeTime, *windStrength)
	case "waterripple":
		runWaterRipple(screen, sigChan, *grayscale)
	default:
		screen.Fini()
		fmt.Fprintf(os.Stderr, "Unknown mode: %s. Use: matrix, nyancat, snake, missiledefender, spectrograph, snowflakes, waterripple, or random\n", *mode)
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

