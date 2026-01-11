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
	var mode = flag.String("mode", "matrix", "Visualization mode: matrix, nyancat, or snake")
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
		runMatrixRain(screen, sigChan)
	case "nyancat":
		runNyancat(screen, sigChan)
	case "snake":
		runSnake(screen, sigChan)
	default:
		screen.Fini()
		fmt.Fprintf(os.Stderr, "Unknown mode: %s. Use: matrix, nyancat, or snake\n", *mode)
		os.Exit(1)
	}
}

