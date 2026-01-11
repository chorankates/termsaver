package main

import (
	"math/rand"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
)

type Snowflake struct {
	x      int
	y      float64
	speed  float64
	active bool
}

func runSnowflakes(screen tcell.Screen, sigChan chan os.Signal, grayscale bool) {
	w, h := screen.Size()

	// Track ground level for each column (Y coordinate where next snowflake lands)
	// Y=0 is top of screen, Y=h-1 is bottom
	groundLevel := make([]int, w)
	for i := range groundLevel {
		groundLevel[i] = h - 1 // Start at bottom
	}

	// Snowflakes falling
	snowflakes := make([]Snowflake, 0)
	maxSnowflakes := w / 2 // Limit number of active snowflakes

	ticker := time.NewTicker(100 * time.Millisecond) // Snow falls slower than matrix
	defer ticker.Stop()

	// Event handling for resize and exit
	eventChan := make(chan tcell.Event, 10)
	go func() {
		for {
			eventChan <- screen.PollEvent()
		}
	}()

	rand.Seed(time.Now().UnixNano())

	for {
		select {
		case <-sigChan:
			return
		case event := <-eventChan:
			switch ev := event.(type) {
			case *tcell.EventResize:
				newW, newH := screen.Size()
				// Adjust ground level array
				newGroundLevel := make([]int, newW)
				for i := range newGroundLevel {
					if i < len(groundLevel) {
						// Scale the ground level proportionally
						newGroundLevel[i] = int(float64(groundLevel[i]) * float64(newH-1) / float64(h-1))
					} else {
						newGroundLevel[i] = newH - 1
					}
				}
				groundLevel = newGroundLevel
				w, h = newW, newH

				// Remove snowflakes that are out of bounds
				validSnowflakes := make([]Snowflake, 0)
				for _, flake := range snowflakes {
					if flake.x >= 0 && flake.x < w && flake.y >= 0 && int(flake.y) < h {
						validSnowflakes = append(validSnowflakes, flake)
					}
				}
				snowflakes = validSnowflakes
				maxSnowflakes = w / 2

				screen.Sync()
			case *tcell.EventKey:
				if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
					return
				}
			}
		case <-ticker.C:
			// Check if we should clear accumulated snow
			// Clear if any column has accumulated more than 50% of screen height
			// (ground level less than 50% means more than 50% is filled)
			shouldClear := false
			for _, level := range groundLevel {
				if level < h/2 {
					shouldClear = true
					break
				}
			}

			if shouldClear {
				// Clear all accumulated snow
				for i := range groundLevel {
					groundLevel[i] = h - 1
				}
			}

			// Spawn new snowflakes
			for len(snowflakes) < maxSnowflakes && rand.Float64() < 0.3 {
				snowflakes = append(snowflakes, Snowflake{
					x:      rand.Intn(w),
					y:      0.0,
					speed:  0.5 + rand.Float64()*0.5, // Speed between 0.5 and 1.0
					active: true,
				})
			}

			// Update falling snowflakes
			for i := range snowflakes {
				if !snowflakes[i].active {
					continue
				}

				// Move snowflake down
				snowflakes[i].y += snowflakes[i].speed
				if snowflakes[i].y < 0 {
					snowflakes[i].y = 0
				}

				// Check if snowflake has reached the ground (or accumulated snow)
				groundY := float64(groundLevel[snowflakes[i].x])
				if snowflakes[i].y >= groundY {
					// Snowflake has landed, accumulate it
					// Raise the ground level for this column (decrement Y to move up)
					if groundLevel[snowflakes[i].x] > 0 {
						groundLevel[snowflakes[i].x]--
					}
					// Remove this snowflake
					snowflakes[i].active = false
				}
			}

			// Remove inactive snowflakes
			activeSnowflakes := make([]Snowflake, 0)
			for _, flake := range snowflakes {
				if flake.active {
					activeSnowflakes = append(activeSnowflakes, flake)
				}
			}
			snowflakes = activeSnowflakes

			// Draw
			screen.Clear()

			snowStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorWhite, grayscale)).Background(tcell.ColorBlack)
			accumStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorWhite, grayscale)).Background(tcell.ColorBlack)

			// Draw accumulated snow (solid block from bottom up to ground level)
			for x := 0; x < w; x++ {
				groundY := groundLevel[x]
				// Draw solid block from bottom (h-1) up to ground level
				for y := h - 1; y >= groundY && y >= 0; y-- {
					screen.SetContent(x, y, '█', nil, accumStyle)
				}
			}

			// Draw falling snowflakes
			snowflakeChars := []rune{'*', '.', '+', '·'}
			for _, flake := range snowflakes {
				if flake.active && flake.x >= 0 && flake.x < w {
					yPos := int(flake.y)
					if yPos >= 0 && yPos < h {
						// Only draw if not in accumulated snow area
						if yPos < groundLevel[flake.x] {
							char := snowflakeChars[flake.x%len(snowflakeChars)]
							screen.SetContent(flake.x, yPos, char, nil, snowStyle)
						}
					}
				}
			}

			screen.Show()
		}
	}
}

