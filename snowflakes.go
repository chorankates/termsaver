package main

import (
	"math/rand"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
)

type Snowflake struct {
	x      float64
	y      float64
	speed  float64
	windX  float64 // Horizontal drift due to wind
	active bool
}

func runSnowflakes(screen tcell.Screen, sigChan chan os.Signal, interactive bool, grayscale bool, windChangeTime float64, windStrength float64) bool {
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

	// Wind simulation - changes over time
	globalWind := 0.0 // Global wind direction (-1 to 1, negative = left, positive = right)
	windChangeTimer := 0
	tickInterval := 100 * time.Millisecond
	windChangeTicks := int(windChangeTime * float64(time.Second) / float64(tickInterval))
	if windChangeTicks < 1 {
		windChangeTicks = 1
	}
	// Clamp wind strength to reasonable range
	if windStrength < 0 {
		windStrength = 0
	}
	if windStrength > 1.0 {
		windStrength = 1.0
	}
	windRange := windStrength * 2.0 // Total range from -windStrength to +windStrength

	ticker := time.NewTicker(tickInterval) // Snow falls slower than matrix
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
			return false
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
					if int(flake.x) >= 0 && int(flake.x) < w && flake.y >= 0 && int(flake.y) < h {
						validSnowflakes = append(validSnowflakes, flake)
					}
				}
				snowflakes = validSnowflakes
				maxSnowflakes = w / 2

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
			// Update wind over time (gradual changes)
			windChangeTimer++
			if windChangeTimer >= windChangeTicks {
				// Wind gradually shifts between -windStrength and +windStrength
				globalWind = (rand.Float64() - 0.5) * windRange
				windChangeTimer = 0
			} else {
				// Smooth wind transitions
				targetWind := (rand.Float64() - 0.5) * windRange
				globalWind = globalWind*0.95 + targetWind*0.05
			}

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
				// Each snowflake has its own wind component (individual variation)
				// plus the global wind effect
				// Individual variation is proportional to wind strength
				individualWind := (rand.Float64() - 0.5) * windStrength * 0.375 // Small individual variation (about 37.5% of baseline)
				totalWind := globalWind + individualWind
				
				snowflakes = append(snowflakes, Snowflake{
					x:      float64(rand.Intn(w)),
					y:      0.0,
					speed:  0.5 + rand.Float64()*0.5, // Speed between 0.5 and 1.0
					windX:  totalWind,
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

				// Apply wind effect (horizontal movement)
				// Wind effect is stronger at higher altitudes (more realistic)
				windStrength := 1.0 - (snowflakes[i].y / float64(h)) // Stronger at top
				snowflakes[i].x += snowflakes[i].windX * windStrength

				// Wrap around screen edges horizontally
				if snowflakes[i].x < 0 {
					snowflakes[i].x += float64(w)
				} else if snowflakes[i].x >= float64(w) {
					snowflakes[i].x -= float64(w)
				}

				// Check if snowflake has reached the ground (or accumulated snow)
				xPos := int(snowflakes[i].x)
				if xPos < 0 {
					xPos = 0
				}
				if xPos >= w {
					xPos = w - 1
				}
				groundY := float64(groundLevel[xPos])
				if snowflakes[i].y >= groundY {
					// Snowflake has landed, accumulate it
					// Raise the ground level for this column (decrement Y to move up)
					if groundLevel[xPos] > 0 {
						groundLevel[xPos]--
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
				if flake.active {
					xPos := int(flake.x)
					if xPos >= 0 && xPos < w {
						yPos := int(flake.y)
						if yPos >= 0 && yPos < h {
							// Only draw if not in accumulated snow area
							if yPos < groundLevel[xPos] {
								char := snowflakeChars[xPos%len(snowflakeChars)]
								screen.SetContent(xPos, yPos, char, nil, snowStyle)
							}
						}
					}
				}
			}

			screen.Show()
		}
	}
}

