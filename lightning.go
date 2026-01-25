package main

import (
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
)

type Cloud struct {
	x      float64 // X position (center)
	y      float64 // Y position (top of cloud)
	width  int     // Base cloud width
	height int     // Cloud height (number of layers)
	speed  float64 // Horizontal movement speed
	active bool
	layers []CloudLayer // Stacked rectangles
}

type CloudLayer struct {
	width  int // Width of this layer
	offset int // Horizontal offset from cloud center
}

type Lightning struct {
	x        float64 // X position
	y        float64 // Y position (start)
	age      float64 // Age in frames
	active   bool
	branches []LightningBranch
	cloudIdx int // Index of cloud this lightning came from (-1 if none)
}

type LightningBranch struct {
	startX       float64
	startY       float64
	endX         float64
	endY         float64
	age          float64
	progress     float64 // 0.0 to 1.0, how much of this branch has been drawn
	parentIdx    int     // Index of parent branch (-1 for main branch segments)
	segmentOrder int     // Order in the main path (0 = first, increases down)
}

func runLightning(screen tcell.Screen, sigChan chan os.Signal, grayscale bool) {
	w, h := screen.Size()

	clouds := make([]Cloud, 0)
	maxClouds := 3
	lightnings := make([]Lightning, 0)
	maxLightnings := 2

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	lastCloudSpawn := time.Now().Add(-3000 * time.Millisecond) // Spawn first cloud immediately
	cloudSpawnInterval := 500 * time.Millisecond

	lastLightningTime := time.Now()
	lightningInterval := time.Duration(1500+rand.Intn(2000)) * time.Millisecond // 1.5-3.5 seconds

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
				w, h = screen.Size()
				// Remove clouds/lightnings that are out of bounds
				validClouds := make([]Cloud, 0)
				for _, cloud := range clouds {
					if cloud.x+float64(cloud.width) >= 0 && cloud.x < float64(w) && cloud.y < float64(h) {
						validClouds = append(validClouds, cloud)
					}
				}
				clouds = validClouds
				screen.Sync()
			case *tcell.EventKey:
				if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
					return
				}
			}
		case <-ticker.C:
			// Spawn new clouds
			if len(clouds) < maxClouds && time.Since(lastCloudSpawn) >= cloudSpawnInterval {
				// Spawn clouds at random heights in the upper portion of screen
				cloudY := float64(rand.Intn(h / 4)) // Top quarter of screen
				cloudWidth := 30 + rand.Intn(40)    // 30-70 characters wide (much wider)
				cloudHeight := 4 + rand.Intn(4)     // 4-8 layers
				speed := 0.2 + rand.Float64()*0.3   // 0.2-0.5 speed (slower for bigger clouds)

				// Create stacked layers that increase in width (normal cloud orientation)
				layers := make([]CloudLayer, cloudHeight)
				for i := 0; i < cloudHeight; i++ {
					// Each layer is wider as we go down
					// Top layer (i=0) is narrowest, bottom layer is widest
					layerPercent := 0.5 + (float64(i) / float64(cloudHeight-1) * 0.5) // 50% to 100%
					if cloudHeight == 1 {
						layerPercent = 1.0
					}
					layerWidth := int(float64(cloudWidth) * layerPercent)
					if layerWidth < 5 {
						layerWidth = 5
					}
					// Center the layer
					offset := (cloudWidth - layerWidth) / 2
					layers[i] = CloudLayer{
						width:  layerWidth,
						offset: offset,
					}
				}

				// Spawn from left or right edge
				var startX float64
				if rand.Float64() < 0.5 {
					startX = -float64(cloudWidth) // Start off-screen left
				} else {
					startX = float64(w) // Start off-screen right
					speed = -speed      // Move left
				}

				clouds = append(clouds, Cloud{
					x:      startX,
					y:      cloudY,
					width:  cloudWidth,
					height: cloudHeight,
					speed:  speed,
					active: true,
					layers: layers,
				})
				lastCloudSpawn = time.Now()
				cloudSpawnInterval = time.Duration(2000+rand.Intn(3000)) * time.Millisecond
			}

			// Update clouds
			for i := range clouds {
				if clouds[i].active {
					clouds[i].x += clouds[i].speed
					// Deactivate clouds that have moved off-screen
					// Cloud x is the left edge, width extends to the right
					if clouds[i].x+float64(clouds[i].width) < 0 || clouds[i].x > float64(w) {
						clouds[i].active = false
					}
				}
			}

			// Remove inactive clouds
			activeClouds := make([]Cloud, 0)
			for _, cloud := range clouds {
				if cloud.active {
					activeClouds = append(activeClouds, cloud)
				}
			}
			clouds = activeClouds

			// Spawn lightning from clouds (100% from clouds)
			if len(lightnings) < maxLightnings && time.Since(lastLightningTime) >= lightningInterval && len(clouds) > 0 {
				// Pick a random cloud
				cloudIdx := rand.Intn(len(clouds))
				cloud := clouds[cloudIdx]

				// Lightning emerges from bottom of cloud, random x within cloud width
				lightningX := cloud.x + float64(rand.Intn(cloud.width))
				lightningY := cloud.y + float64(cloud.height) // Bottom of cloud

				// Create lightning with more branches (fractal)
				branches := generateFractalLightning(lightningX, lightningY, w, h, 0)

				lightnings = append(lightnings, Lightning{
					x:        lightningX,
					y:        lightningY,
					age:      0.0,
					active:   true,
					branches: branches,
					cloudIdx: cloudIdx,
				})
				lastLightningTime = time.Now()
				lightningInterval = time.Duration(1500+rand.Intn(2000)) * time.Millisecond
			}

			// Update lightnings
			for i := range lightnings {
				if lightnings[i].active {
					lightnings[i].age += 1.0

					// Update branch progress - lightning travels down at ~15 units per frame
					lightningSpeed := 15.0
					for j := range lightnings[i].branches {
						branch := &lightnings[i].branches[j]

						// Calculate branch length
						dx := branch.endX - branch.startX
						dy := branch.endY - branch.startY
						branchLength := math.Sqrt(dx*dx + dy*dy)

						if branchLength > 0 {
							// Check if parent branch has reached this branch's start point
							canProgress := true
							if branch.parentIdx >= 0 && branch.parentIdx < len(lightnings[i].branches) {
								parent := lightnings[i].branches[branch.parentIdx]
								// Side branch can only start when parent has reached at least 0.8 progress
								// (allowing some overlap)
								if parent.progress < 0.8 {
									canProgress = false
								}
							}

							if canProgress && branch.progress < 1.0 {
								// Increment progress based on speed
								progressIncrement := lightningSpeed / branchLength
								branch.progress += progressIncrement
								if branch.progress > 1.0 {
									branch.progress = 1.0
								}
							}
						}
					}

					// Lightning flashes briefly (2-4 frames)
					if lightnings[i].age > 2.0+rand.Float64()*2.0 {
						lightnings[i].active = false
					}
				}
			}

			// Remove inactive lightnings
			activeLightnings := make([]Lightning, 0)
			for _, lightning := range lightnings {
				if lightning.active {
					activeLightnings = append(activeLightnings, lightning)
				}
			}
			lightnings = activeLightnings

			// Draw
			screen.Clear()

			// Check which clouds have active lightning
			activeLightningClouds := make(map[int]bool)
			for _, lightning := range lightnings {
				if lightning.active && lightning.cloudIdx >= 0 && lightning.cloudIdx < len(clouds) {
					activeLightningClouds[lightning.cloudIdx] = true
				}
			}

			// Draw clouds
			cloudStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorGray, grayscale)).Background(tcell.ColorBlack)
			darkCloudStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorDarkGray, grayscale)).Background(tcell.ColorBlack)
			litCloudStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorWhite, grayscale)).Background(tcell.ColorBlack)
			litCloudMediumStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorLightYellow, grayscale)).Background(tcell.ColorBlack)
			cloudChars := []rune{'█', '▓', '▒'}

			for cloudIdx, cloud := range clouds {
				if !cloud.active {
					continue
				}

				cloudStartX := int(cloud.x)
				cloudStartY := int(cloud.y)
				isLit := activeLightningClouds[cloudIdx]

				// Draw cloud as stacked rectangles
				for layerIdx, layer := range cloud.layers {
					y := cloudStartY + layerIdx
					if y < 0 || y >= h {
						continue
					}

					// Draw this layer
					layerStartX := cloudStartX + layer.offset
					for dx := 0; dx < layer.width; dx++ {
						x := layerStartX + dx
						if x >= 0 && x < w {
							// Add some texture variation
							var char rune
							var style tcell.Style

							randVal := rand.Float64()
							if randVal < 0.6 {
								char = cloudChars[0] // █ (solid)
								if isLit {
									style = litCloudStyle
								} else {
									style = cloudStyle
								}
							} else if randVal < 0.85 {
								char = cloudChars[1] // ▓ (medium)
								if isLit {
									style = litCloudMediumStyle
								} else {
									style = cloudStyle
								}
							} else {
								char = cloudChars[2] // ▒ (light)
								if isLit {
									style = litCloudMediumStyle
								} else {
									style = darkCloudStyle
								}
							}

							// Edges are more wispy
							if dx < 2 || dx >= layer.width-2 {
								if rand.Float64() < 0.5 {
									char = cloudChars[2] // ▒
									if isLit {
										style = litCloudMediumStyle
									} else {
										style = darkCloudStyle
									}
								}
							}

							screen.SetContent(x, y, char, nil, style)
						}
					}
				}
			}

			// Draw lightning with glow
			lightningStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorYellow, grayscale)).Background(tcell.ColorBlack)
			brightLightningStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorWhite, grayscale)).Background(tcell.ColorBlack)
			glowStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorDarkCyan, grayscale)).Background(tcell.ColorBlack)
			glowMediumStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorBlue, grayscale)).Background(tcell.ColorBlack)

			for _, lightning := range lightnings {
				if !lightning.active {
					continue
				}

				// First pass: Draw glow around lightning (only drawn portions)
				for _, branch := range lightning.branches {
					if branch.progress > 0 {
						drawLightningGlow(screen, branch, glowStyle, glowMediumStyle, w, h, grayscale)
					}
				}

				// Second pass: Draw the lightning itself (only drawn portions)
				for _, branch := range lightning.branches {
					if branch.progress > 0 {
						drawLightningBranch(screen, branch, lightningStyle, brightLightningStyle, w, h)
					}
				}
			}

			screen.Show()
		}
	}
}

func generateFractalLightning(startX, startY float64, w, h int, depth int) []LightningBranch {
	branches := make([]LightningBranch, 0)

	// Maximum recursion depth
	if depth > 3 {
		return branches
	}

	// Main branch goes down
	currentX := startX
	currentY := startY
	targetY := float64(h - 1)

	// Don't go past bottom
	if currentY >= targetY {
		return branches
	}

	// Create jagged path down
	segments := 6 + rand.Intn(6) // 6-12 segments
	remainingDist := targetY - startY
	if remainingDist <= 0 {
		return branches
	}

	segmentLength := remainingDist / float64(segments)
	if segmentLength < 1 {
		segmentLength = 1
	}

	prevX := currentX
	prevY := currentY

	for i := 0; i < segments; i++ {
		// Add horizontal jitter
		jitter := (rand.Float64() - 0.5) * 4.0 // ±2 characters
		currentX += jitter
		currentY += segmentLength

		// Don't go past bottom
		if currentY > targetY {
			currentY = targetY
		}

		// Clamp to screen bounds
		if currentX < 1 {
			currentX = 1
		}
		if currentX >= float64(w-1) {
			currentX = float64(w - 2)
		}

		// Main branch segment - parent is previous segment (or -1 for first)
		parentIdx := -1
		if i > 0 {
			parentIdx = len(branches) - 1
		}

		branches = append(branches, LightningBranch{
			startX:       prevX,
			startY:       prevY,
			endX:         currentX,
			endY:         currentY,
			age:          0.0,
			progress:     0.0,
			parentIdx:    parentIdx,
			segmentOrder: i,
		})

		// Create side branches more frequently (fractal branching)
		if rand.Float64() < 0.6 && i < segments-1 && currentY < targetY-5 {
			// Determine branch direction and length
			branchAngle := (rand.Float64() - 0.5) * 1.5 // -0.75 to 0.75
			branchLength := 5.0 + rand.Float64()*10.0   // 5-15 units

			branchEndX := currentX + branchAngle*branchLength
			branchEndY := currentY + branchLength*0.7 // Go mostly down

			if branchEndX >= 1 && branchEndX < float64(w-1) && branchEndY < float64(h) {
				// Side branch - parent is the current main branch segment
				sideBranchParentIdx := len(branches) - 1

				// Add side branch segment
				branches = append(branches, LightningBranch{
					startX:       currentX,
					startY:       currentY,
					endX:         branchEndX,
					endY:         branchEndY,
					age:          0.0,
					progress:     0.0,
					parentIdx:    sideBranchParentIdx,
					segmentOrder: i, // Same order as parent segment
				})

				// Recursively create sub-branches (fractal)
				if depth < 2 && rand.Float64() < 0.5 {
					sideBranchIdx := len(branches) - 1
					subBranches := generateFractalLightning(branchEndX, branchEndY, w, h, depth+1)
					// Update parent indices for sub-branches - first branch should point to side branch
					for k := range subBranches {
						if subBranches[k].parentIdx == -1 {
							// First branch in recursive call should point to the side branch
							subBranches[k].parentIdx = sideBranchIdx
						} else {
							// Other branches need their parent indices adjusted
							subBranches[k].parentIdx += len(branches)
						}
					}
					branches = append(branches, subBranches...)
				}
			}
		}

		prevX = currentX
		prevY = currentY

		// Stop if we reached the bottom
		if currentY >= targetY {
			break
		}
	}

	return branches
}

func drawLightningGlow(screen tcell.Screen, branch LightningBranch, glowStyle, glowMediumStyle tcell.Style, w, h int, grayscale bool) {
	// Draw soft glow around the lightning path (only drawn portion)
	dx := branch.endX - branch.startX
	dy := branch.endY - branch.startY
	length := math.Sqrt(dx*dx + dy*dy)

	if length == 0 {
		return
	}

	steps := int(length) + 1
	if steps < 1 {
		steps = 1
	}

	// Only draw glow up to the current progress
	maxStep := int(float64(steps) * branch.progress)

	// Draw glow around each point in the lightning path
	for i := 0; i <= maxStep; i++ {
		t := float64(i) / float64(steps)
		centerX := branch.startX + dx*t
		centerY := branch.startY + dy*t

		// Draw glow in a small radius around this point
		glowRadius := 2
		for gy := -glowRadius; gy <= glowRadius; gy++ {
			for gx := -glowRadius; gx <= glowRadius; gx++ {
				distance := math.Sqrt(float64(gx*gx + gy*gy))
				if distance > float64(glowRadius) {
					continue
				}

				x := int(centerX) + gx
				y := int(centerY) + gy

				if x >= 0 && x < w && y >= 0 && y < h {
					// Don't overwrite if there's already content (like the lightning itself)
					currentRune, _, _, _ := screen.GetContent(x, y)
					if currentRune != ' ' && currentRune != 0 {
						continue
					}

					// Choose glow intensity based on distance
					var glowChar rune
					var style tcell.Style
					if distance < 1.0 {
						glowChar = '·'
						style = glowMediumStyle
					} else if distance < 1.5 {
						glowChar = '.'
						style = glowStyle
					} else {
						glowChar = ' '
						style = glowStyle
					}

					// Randomly skip some glow for organic effect
					if rand.Float64() < 0.6 {
						screen.SetContent(x, y, glowChar, nil, style)
					}
				}
			}
		}
	}
}

func drawLightningBranch(screen tcell.Screen, branch LightningBranch, style, brightStyle tcell.Style, w, h int) {
	// Draw line from start to end (only drawn portion)
	dx := branch.endX - branch.startX
	dy := branch.endY - branch.startY
	length := math.Sqrt(dx*dx + dy*dy)

	if length == 0 {
		return
	}

	steps := int(length) + 1
	if steps < 1 {
		steps = 1
	}

	// Only draw up to the current progress
	maxStep := int(float64(steps) * branch.progress)

	lightningChars := []rune{'|', '/', '\\', '╲', '╱'}

	for i := 0; i <= maxStep; i++ {
		t := float64(i) / float64(steps)
		x := branch.startX + dx*t
		y := branch.startY + dy*t

		xInt := int(x)
		yInt := int(y)

		if xInt >= 0 && xInt < w && yInt >= 0 && yInt < h {
			// Choose character based on direction
			var char rune
			if math.Abs(dx) < 0.1 {
				char = lightningChars[0] // |
			} else if dx > 0 && dy > 0 {
				char = lightningChars[1] // /
			} else if dx < 0 && dy > 0 {
				char = lightningChars[2] // \
			} else if dx > 0 {
				char = lightningChars[3] // ╲
			} else {
				char = lightningChars[4] // ╱
			}

			// Alternate between bright and normal for flicker effect
			if rand.Float64() < 0.3 {
				screen.SetContent(xInt, yInt, char, nil, brightStyle)
			} else {
				screen.SetContent(xInt, yInt, char, nil, style)
			}
		}
	}
}
