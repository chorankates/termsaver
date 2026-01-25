package main

import (
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
)

var rainbow = []tcell.Color{
	tcell.ColorRed,
	tcell.ColorOrange,
	tcell.ColorYellow,
	tcell.ColorGreen,
	tcell.ColorBlue,
	tcell.ColorPurple,
}

func runNyancat(screen tcell.Screen, sigChan chan os.Signal, interactive bool, grayscale bool) bool {
	w, h := screen.Size()

	// Nyancat sprite (simplified ASCII art)
	catSprite := []string{
		"   ,,,",
		"  (*_*)",
		" (    )",
		"  \"  \"",
	}

	catY := h / 2
	x := 0

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Event handling for resize
	eventChan := make(chan tcell.Event, 10)
	go func() {
		for {
			eventChan <- screen.PollEvent()
		}
	}()

	for {
		select {
		case <-sigChan:
			return false
		case event := <-eventChan:
			switch ev := event.(type) {
			case *tcell.EventResize:
				w, h = screen.Size()
				catY = h / 2
				screen.Sync()
			case *tcell.EventKey:
				if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
					return false
				}
				// Space cycles to next mode
				if ev.Rune() == ' ' {
					return true
				}
				// In non-interactive mode, any key exits
				if !interactive {
					return false
				}
			}
		case <-ticker.C:
			screen.Clear()

			// Draw rainbow trail
			for i := 0; i < w; i++ {
				if x-i >= 0 && x-i < len(rainbow)*3 {
					color := rainbow[(x-i)/3%len(rainbow)]
					style := tcell.StyleDefault.Foreground(toGrayscale(color, grayscale)).Background(tcell.ColorBlack)
					for j := 0; j < 3; j++ {
						if catY+j < h && catY+j >= 0 {
							screen.SetContent(i, catY+j, '▔', nil, style)
						}
					}
				}
			}

			// Draw nyancat
			catX := x
			for i, line := range catSprite {
				y := catY + i - len(catSprite)/2
				if y >= 0 && y < h {
					for j, char := range line {
						px := catX + j
						if px >= 0 && px < w {
							style := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorWhite, grayscale)).Background(tcell.ColorBlack)
							screen.SetContent(px, y, char, nil, style)
						}
					}
				}
			}

			// Draw stars in background
			for i := 0; i < 20; i++ {
				sx := (x + i*7) % w
				sy := (i * 3) % h
				style := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorWhite, grayscale)).Background(tcell.ColorBlack)
				screen.SetContent(sx, sy, '*', nil, style)
			}

			x++
			if x >= w+20 {
				x = -20
			}

			screen.Show()
		}
	}
}

