package main

import (
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
)

type SpectrographBar struct {
	baseFrequency float64
	phase         float64
	amplitude     float64
	color         tcell.Color
}

func runSpectrograph(screen tcell.Screen, sigChan chan os.Signal, interactive bool, grayscale bool) {
	w, h := screen.Size()

	// Define a palette of vibrant colors for the bars
	colors := []tcell.Color{
		tcell.ColorRed,
		tcell.ColorOrange,
		tcell.ColorYellow,
		tcell.ColorLime,
		tcell.ColorGreen,
		tcell.ColorAqua,
		tcell.ColorBlue,
		tcell.ColorPurple,
		tcell.ColorFuchsia,
		tcell.ColorPink,
		tcell.ColorLightCoral,
		tcell.ColorLightGreen,
		tcell.ColorLightBlue,
		tcell.ColorMediumPurple,
	}

	// Variable number of bars - use a reasonable spacing (every 2-3 columns for better visibility)
	barSpacing := 2
	if w > 100 {
		barSpacing = 3
	}
	numBars := (w - 1) / barSpacing
	if numBars < 5 {
		numBars = 5 // Minimum bars
	}

	bars := make([]SpectrographBar, numBars)
	rand.Seed(time.Now().UnixNano())

	// Initialize bars with different frequencies, phases, and colors
	for i := range bars {
				color := colors[i%len(colors)]
				if grayscale {
					color = toGrayscale(color, grayscale)
				}
				bars[i] = SpectrographBar{
					baseFrequency: 0.05 + float64(i)*0.03 + rand.Float64()*0.02,
					phase:         float64(i) * 0.5,
					amplitude:     0.5 + rand.Float64()*0.5,
					color:         color,
				}
	}

	ticker := time.NewTicker(30 * time.Millisecond) // Fast updates for smooth animation
	defer ticker.Stop()

	// Event handling for resize and exit
	eventChan := make(chan tcell.Event, 10)
	go func() {
		for {
			eventChan <- screen.PollEvent()
		}
	}()

	startTime := time.Now()

	for {
		select {
		case <-sigChan:
			return
		case event := <-eventChan:
			switch ev := event.(type) {
			case *tcell.EventResize:
				w, h = screen.Size()
				// Recalculate number of bars
				barSpacing = 2
				if w > 100 {
					barSpacing = 3
				}
				numBars = (w - 1) / barSpacing
				if numBars < 5 {
					numBars = 5
				}
				// Reinitialize bars
				newBars := make([]SpectrographBar, numBars)
				for i := range newBars {
					if i < len(bars) {
						newBars[i] = bars[i]
					} else {
						color := colors[i%len(colors)]
						if grayscale {
							color = toGrayscale(color, grayscale)
						}
						newBars[i] = SpectrographBar{
							baseFrequency: 0.05 + float64(i)*0.03 + rand.Float64()*0.02,
							phase:         float64(i) * 0.5,
							amplitude:     0.5 + rand.Float64()*0.5,
							color:         color,
						}
					}
				}
				bars = newBars
				screen.Sync()
			case *tcell.EventKey:
				if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
					return
				}
				// In non-interactive mode, any key exits
				if !interactive {
					return
				}
			}
		case <-ticker.C:
			screen.Clear()

			// Calculate time since start for animation
			elapsed := time.Since(startTime).Seconds()

			// Draw each bar
			for i, bar := range bars {
				// Calculate bar height using multiple sine waves for more complex motion
				// Combine multiple frequencies for more realistic audio visualization
				value1 := math.Sin(elapsed*bar.baseFrequency + bar.phase)
				value2 := math.Sin(elapsed*bar.baseFrequency*2.3 + bar.phase*1.7)
				value3 := math.Sin(elapsed*bar.baseFrequency*0.7 + bar.phase*0.5)

				// Combine the waves
				combined := (value1 + value2*0.6 + value3*0.3) / 1.9
				normalized := (combined + 1.0) / 2.0 // Normalize to 0-1

				// Calculate bar height (leave some space at bottom and top)
				maxHeight := float64(h) * 0.85 // Use 85% of screen height
				minHeight := float64(h) * 0.05 // Minimum 5% height
				barHeight := minHeight + (maxHeight-minHeight)*normalized*bar.amplitude

				barX := i * barSpacing
				barY := h - 1 // Start from bottom

				// Create style for this bar
				style := tcell.StyleDefault.Foreground(toGrayscale(bar.color, grayscale)).Background(tcell.ColorBlack)

				// Draw the bar upward from the bottom
				heightPixels := int(barHeight)
				if heightPixels > h {
					heightPixels = h
				}

				// Use different block characters for gradient effect
				blockChars := []rune{'█', '▓', '▒', '░'}

				for j := 0; j < heightPixels; j++ {
					y := barY - j
					if y >= 0 && y < h && barX < w {
						// Use brighter blocks at the top (peak), dimmer at bottom
						charIdx := 0
						if heightPixels > 4 {
							if j < heightPixels/4 {
								charIdx = 0 // Full block at peak
							} else if j < heightPixels/2 {
								charIdx = 1 // 3/4 block
							} else if j < heightPixels*3/4 {
								charIdx = 2 // 1/2 block
							} else {
								charIdx = 3 // 1/4 block at base
							}
						}
						screen.SetContent(barX, y, blockChars[charIdx], nil, style)
					}
				}

				// Add subtle variation to surrounding pixels for more movement
				// This ensures we're changing as many pixels as possible
				for offset := -1; offset <= 1; offset++ {
					if offset == 0 {
						continue
					}
					x := barX + offset
					if x >= 0 && x < w {
						// Add some small sparkles/particles that change
						sparkleY := barY - heightPixels + int(math.Sin(elapsed*5.0+float64(i*2))*2)
						if sparkleY >= 0 && sparkleY < h {
							sparkleChar := '·'
							if elapsed*10.0+float64(i) > 0 && int(elapsed*10.0+float64(i))%3 == 0 {
								sparkleChar = '*'
							}
							sparkleStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorWhite, grayscale)).Background(tcell.ColorBlack)
							screen.SetContent(x, sparkleY, sparkleChar, nil, sparkleStyle)
						}
					}
				}
			}

			// Fill remaining pixels with animated background pattern
			// This ensures maximum pixel changes for screensaver purposes
			bgStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorDarkGray, grayscale)).Background(tcell.ColorBlack)
			patternTime := int(elapsed * 10)
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					// Check if this pixel is already drawn (skip bar columns)
					isBarColumn := false
					for i := range bars {
						if x == i*barSpacing {
							isBarColumn = true
							break
						}
					}
					if !isBarColumn {
						// Add subtle animated pattern
						if (x+y+patternTime)%7 == 0 {
							screen.SetContent(x, y, '·', nil, bgStyle)
						}
					}
				}
			}

			screen.Show()
		}
	}
}

