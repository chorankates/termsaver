package main

import (
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
)

type Ripple struct {
	x      float64 // Origin X position
	y      float64 // Origin Y position
	age    float64 // Age of the ripple in frames
	speed  float64 // Expansion speed
	active bool
}

func runWaterRipple(screen tcell.Screen, sigChan chan os.Signal, interactive bool, grayscale bool) bool {
	w, h := screen.Size()

	ripples := make([]Ripple, 0)
	maxRipples := 6 // Fewer ripples for a calmer, more natural feel
	lastRippleTime := time.Now()
	rippleInterval := 1000 * time.Millisecond // More relaxed spawning

	ticker := time.NewTicker(80 * time.Millisecond) // Slower, more natural pace
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
				w, h = screen.Size()
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
			// Spawn new ripple naturally - prefer edges and corners (like drops hitting water)
			if len(ripples) < maxRipples && time.Since(lastRippleTime) >= rippleInterval {
				var rippleX, rippleY float64
				// 60% chance to spawn near edges (more natural)
				if rand.Float64() < 0.6 {
					edge := rand.Intn(4) // 0=top, 1=right, 2=bottom, 3=left
					switch edge {
					case 0: // Top edge
						rippleX = float64(rand.Intn(w))
						rippleY = float64(rand.Intn(h / 4))
					case 1: // Right edge
						rippleX = float64(w - 1 - rand.Intn(w/4))
						rippleY = float64(rand.Intn(h))
					case 2: // Bottom edge
						rippleX = float64(rand.Intn(w))
						rippleY = float64(h - 1 - rand.Intn(h/4))
					case 3: // Left edge
						rippleX = float64(rand.Intn(w / 4))
						rippleY = float64(rand.Intn(h))
					}
				} else {
					// Random position anywhere
					rippleX = float64(rand.Intn(w))
					rippleY = float64(rand.Intn(h))
				}
				ripples = append(ripples, Ripple{
					x:      rippleX,
					y:      rippleY,
					age:    0.0,
					speed:  0.6 + rand.Float64()*0.6, // More varied speed: 0.6 to 1.2
					active: true,
				})
				lastRippleTime = time.Now()
				// More natural timing variation
				rippleInterval = time.Duration(500+rand.Intn(1000)) * time.Millisecond
			}

			// Calculate maximum radius for deactivation (based on screen dimensions)
			maxRadius := math.Sqrt(float64(w*w+h*h)) * 1.5

			// Update ripples
			for i := range ripples {
				if ripples[i].active {
					ripples[i].age += ripples[i].speed
					// Deactivate ripples that have expanded too far
					if ripples[i].age > maxRadius {
						ripples[i].active = false
					}
				}
			}

			// Remove inactive ripples
			activeRipples := make([]Ripple, 0)
			for _, ripple := range ripples {
				if ripple.active {
					activeRipples = append(activeRipples, ripple)
				}
			}
			ripples = activeRipples

			// Draw
			screen.Clear()

			// Natural water characters - simple and organic
			rippleChars := []rune{'o', 'O', '°', ' '}

			// Draw the ripple pattern
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					// Find the maximum intensity from all ripples
					maxIntensity := 0.0
					charIndex := 0

					for _, ripple := range ripples {
						if !ripple.active {
							continue
						}

						// Calculate distance from this ripple's origin
						dx := float64(x) - ripple.x
						dy := float64(y) - ripple.y
						distance := math.Sqrt(dx*dx + dy*dy)

						// Add some natural variation to distance (makes ripples less perfect)
						variation := (rand.Float64() - 0.5) * 0.3
						distance += variation

						// Distance from current ripple wavefront
						rippleDistance := math.Abs(distance - ripple.age)

						// Wider, softer ripples
						rippleWidth := 4.0 + rand.Float64()*2.0 // Vary width naturally
						if rippleDistance < rippleWidth {
							intensity := 1.0 - (rippleDistance / rippleWidth)
							// Softer fade as ripple ages
							ageFade := 1.0 - math.Min(ripple.age/maxRadius, 0.5)
							intensity *= ageFade

							if intensity > maxIntensity {
								maxIntensity = intensity
								// Determine character based on intensity - more gradual transitions
								if intensity > 0.7 {
									charIndex = 0 // o (strongest)
								} else if intensity > 0.4 {
									charIndex = 1 // O
								} else if intensity > 0.15 {
									charIndex = 2 // °
								} else {
									charIndex = 3 // space (subtle)
								}
							}
						}
					}

					// Draw the character if there's any ripple intensity
					if maxIntensity > 0.08 {
						// Softer, more natural colors
						var color tcell.Color
						if maxIntensity > 0.6 {
							color = tcell.ColorLightCyan
						} else if maxIntensity > 0.35 {
							color = tcell.ColorTeal
						} else {
							color = tcell.ColorDarkCyan
						}

						if grayscale {
							color = toGrayscale(color, grayscale)
						}

						style := tcell.StyleDefault.Foreground(color).Background(tcell.ColorBlack)
						screen.SetContent(x, y, rippleChars[charIndex], nil, style)
					}
				}
			}

			screen.Show()
		}
	}
}
