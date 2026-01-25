package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
)

type Base struct {
	Pos      Point
	Cooldown int
	LastFire int
}

type Missile struct {
	Pos      Point
	PrevPos  Point
	Velocity Point
	Alive    bool
}

type Projectile struct {
	Pos      Point
	Velocity Point
	Alive    bool
}

type Terrain struct {
	Pos Point
}

type MissileDefender struct {
	bases       []Base
	missiles    []Missile
	projectiles []Projectile
	terrain     []Terrain
	lastRandomize time.Time
	score       int
	missilesDestroyed int
}

func runMissileDefender(screen tcell.Screen, sigChan chan os.Signal, interactive bool, grayscale bool) {
	w, h := screen.Size()
	rand.Seed(time.Now().UnixNano())

	game := &MissileDefender{
		bases:       []Base{},
		missiles:    []Missile{},
		projectiles: []Projectile{},
		terrain:     []Terrain{},
		lastRandomize: time.Now(),
		score:       0,
		missilesDestroyed: 0,
	}

	game.randomizeLayout(w, h)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Event handling for exit
	eventChan := make(chan tcell.Event, 10)
	go func() {
		for {
			eventChan <- screen.PollEvent()
		}
	}()

	frameCount := 0

	for {
		select {
		case <-sigChan:
			return
		case event := <-eventChan:
			switch ev := event.(type) {
			case *tcell.EventKey:
				if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
					return
				}
				// In non-interactive mode, any key exits
				if !interactive {
					return
				}
			case *tcell.EventResize:
				w, h = screen.Size()
				game.randomizeLayout(w, h)
				screen.Sync()
			}
		case <-ticker.C:
			frameCount++

			// Randomize layout every 30-45 seconds
			if time.Since(game.lastRandomize) > time.Duration(30+rand.Intn(16))*time.Second {
				game.randomizeLayout(w, h)
				game.lastRandomize = time.Now()
			}

			// Spawn missiles periodically from the top
			if frameCount%25 == 0 && len(game.missiles) < 8 {
				game.spawnMissile(w, h)
			}

			// Update game state
			game.update(w, h)

			// Draw
			game.draw(screen, w, h, grayscale)
			screen.Show()
		}
	}
}

func (g *MissileDefender) randomizeLayout(w, h int) {
	// Clear existing layout
	g.bases = []Base{}
	g.terrain = []Terrain{}
	g.missiles = []Missile{}
	g.projectiles = []Projectile{}

	// Place 3-5 bases on the bottom row
	numBases := 3 + rand.Intn(3) // 3-5 bases
	baseY := h - 2 // Bottom row (accounting for border)
	
	// Generate base positions with spacing
	spacing := (w - 4) / numBases
	for i := 0; i < numBases; i++ {
		x := 2 + (i * spacing) + rand.Intn(spacing/2) - spacing/4
		// Keep within bounds
		if x < 2 {
			x = 2
		}
		if x >= w-2 {
			x = w - 3
		}
		
		g.bases = append(g.bases, Base{
			Pos:      Point{X: x, Y: baseY},
			Cooldown: 20,
			LastFire: rand.Intn(20), // Randomize initial cooldown
		})
	}

	// Randomize terrain (3-8 terrain pieces, not on bottom row)
	numTerrain := 3 + rand.Intn(6)
	for i := 0; i < numTerrain; i++ {
		terrain := Terrain{
			Pos: Point{
				X: 1 + rand.Intn(w-2),
				Y: 2 + rand.Intn(h-5), // Avoid bottom row and top border
			},
		}
		// Make sure terrain doesn't overlap with bases
		overlap := false
		for _, base := range g.bases {
			if terrain.Pos.X == base.Pos.X && terrain.Pos.Y == base.Pos.Y {
				overlap = true
				break
			}
		}
		// Also check for overlap with existing terrain
		if !overlap {
			for _, existing := range g.terrain {
				if terrain.Pos.X == existing.Pos.X && terrain.Pos.Y == existing.Pos.Y {
					overlap = true
					break
				}
			}
		}
		if !overlap {
			g.terrain = append(g.terrain, terrain)
		}
	}
}

func (g *MissileDefender) spawnMissile(w, h int) {
	// Spawn missiles from the top, falling downward
	pos := Point{
		X: 1 + rand.Intn(w-2),
		Y: 1,
	}
	
	// Velocity: slight horizontal variation, always downward
	velocity := Point{
		X: rand.Intn(3) - 1, // -1, 0, or 1
		Y: 1,                // Always fall down
	}

	g.missiles = append(g.missiles, Missile{
		Pos:      pos,
		PrevPos:  pos,
		Velocity: velocity,
		Alive:    true,
	})
}

func (g *MissileDefender) update(w, h int) {
	// Update missiles - move them downward
	for i := range g.missiles {
		if !g.missiles[i].Alive {
			continue
		}
		// Store previous position for line drawing
		g.missiles[i].PrevPos = g.missiles[i].Pos
		g.missiles[i].Pos.X += g.missiles[i].Velocity.X
		g.missiles[i].Pos.Y += g.missiles[i].Velocity.Y

		// Remove missiles that go off screen (especially bottom)
		if g.missiles[i].Pos.Y >= h-1 || g.missiles[i].Pos.X < 1 || g.missiles[i].Pos.X >= w-1 {
			g.missiles[i].Alive = false
			continue
		}

		// Check collision with terrain
		for _, t := range g.terrain {
			if g.missiles[i].Pos.X == t.Pos.X && g.missiles[i].Pos.Y == t.Pos.Y {
				g.missiles[i].Alive = false
				break
			}
		}

		// Check collision with bases
		for _, base := range g.bases {
			if g.missiles[i].Pos.X == base.Pos.X && g.missiles[i].Pos.Y == base.Pos.Y {
				g.missiles[i].Alive = false
				break
			}
		}
	}

	// Update bases - fire projectiles upward at missiles
	for i := range g.bases {
		if g.bases[i].LastFire > 0 {
			g.bases[i].LastFire--
		}

		if g.bases[i].LastFire == 0 {
			// Find closest missile above this base
			var target *Missile
			minDist := 10000 // Large initial distance
			
			for j := range g.missiles {
				if !g.missiles[j].Alive {
					continue
				}
				// Only target missiles above the base
				if g.missiles[j].Pos.Y < g.bases[i].Pos.Y {
					dx := g.missiles[j].Pos.X - g.bases[i].Pos.X
					dy := g.missiles[j].Pos.Y - g.bases[i].Pos.Y
					dist := dx*dx + dy*dy
					if dist < minDist {
						minDist = dist
						target = &g.missiles[j]
					}
				}
			}

			if target != nil {
				// Fire projectile upward toward target
				dx := target.Pos.X - g.bases[i].Pos.X
				dy := target.Pos.Y - g.bases[i].Pos.Y
				dist := math.Sqrt(float64(dx*dx + dy*dy))
				if dist > 0 {
					// Projectile moves upward (negative Y) toward target
					velX := int(float64(dx) / dist * 2)
					velY := int(float64(dy) / dist * 2)
					
					// Ensure projectile moves upward
					if velY > 0 {
						velY = -velY
					}
					if velY == 0 {
						velY = -2 // Default upward velocity
					}
					
					// Clamp velocities
					if velX > 2 {
						velX = 2
					}
					if velX < -2 {
						velX = -2
					}
					if velY > -1 {
						velY = -2
					}

					g.projectiles = append(g.projectiles, Projectile{
						Pos:      Point{X: g.bases[i].Pos.X, Y: g.bases[i].Pos.Y - 1},
						Velocity: Point{X: velX, Y: velY},
						Alive:    true,
					})
					g.bases[i].LastFire = g.bases[i].Cooldown
				}
			}
		}
	}

	// Update projectiles - move them
	for i := range g.projectiles {
		if !g.projectiles[i].Alive {
			continue
		}
		g.projectiles[i].Pos.X += g.projectiles[i].Velocity.X
		g.projectiles[i].Pos.Y += g.projectiles[i].Velocity.Y

		// Remove projectiles that go off screen (especially top)
		if g.projectiles[i].Pos.Y < 1 || g.projectiles[i].Pos.X < 1 || g.projectiles[i].Pos.X >= w-1 {
			g.projectiles[i].Alive = false
			continue
		}

		// Check collision with missiles
		for j := range g.missiles {
			if !g.missiles[j].Alive {
				continue
			}
			// Check if projectile is close to missile (within 1 cell)
			if abs(g.projectiles[i].Pos.X-g.missiles[j].Pos.X) <= 1 &&
				abs(g.projectiles[i].Pos.Y-g.missiles[j].Pos.Y) <= 1 {
				g.missiles[j].Alive = false
				g.projectiles[i].Alive = false
				g.missilesDestroyed++
				g.score += 10
				break
			}
		}
	}

	// Clean up dead missiles and projectiles
	newMissiles := []Missile{}
	for _, m := range g.missiles {
		if m.Alive {
			newMissiles = append(newMissiles, m)
		}
	}
	g.missiles = newMissiles

	newProjectiles := []Projectile{}
	for _, p := range g.projectiles {
		if p.Alive {
			newProjectiles = append(newProjectiles, p)
		}
	}
	g.projectiles = newProjectiles
}

func (g *MissileDefender) draw(screen tcell.Screen, w, h int, grayscale bool) {
	screen.Clear()

	// Draw border
	borderStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorWhite, grayscale)).Background(tcell.ColorBlack)
	for x := 0; x < w; x++ {
		screen.SetContent(x, 0, '─', nil, borderStyle)
		screen.SetContent(x, h-1, '─', nil, borderStyle)
	}
	for y := 0; y < h; y++ {
		screen.SetContent(0, y, '│', nil, borderStyle)
		screen.SetContent(w-1, y, '│', nil, borderStyle)
	}
	screen.SetContent(0, 0, '┌', nil, borderStyle)
	screen.SetContent(w-1, 0, '┐', nil, borderStyle)
	screen.SetContent(0, h-1, '└', nil, borderStyle)
	screen.SetContent(w-1, h-1, '┘', nil, borderStyle)

	// Draw solid green ground line at the bottom
	groundY := h - 2
	groundStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorGreen, grayscale)).Background(tcell.ColorBlack)
	for x := 1; x < w-1; x++ {
		screen.SetContent(x, groundY, '─', nil, groundStyle)
	}

	// Draw terrain (optional, can be removed or kept for variety)
	terrainStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorYellow, grayscale)).Background(tcell.ColorBlack)
	for _, t := range g.terrain {
		if t.Pos.Y < groundY {
			screen.SetContent(t.Pos.X, t.Pos.Y, '▓', nil, terrainStyle)
		}
	}

	// Draw bases on the bottom (yellow triangles on green ground)
	baseStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorYellow, grayscale)).Background(tcell.ColorBlack)
	for _, base := range g.bases {
		// Draw base on the ground line (one row above the ground line so it sits on top)
		screen.SetContent(base.Pos.X, base.Pos.Y-1, '▲', nil, baseStyle)
	}

	// Draw missiles falling from sky as cyan lines
	missileStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorBlue, grayscale)).Background(tcell.ColorBlack)
	for _, missile := range g.missiles {
		if missile.Alive {
			// Draw line from previous position to current position
			drawLine(screen, missile.PrevPos, missile.Pos, missileStyle)
		}
	}

	// Draw projectiles (defensive shots) - white dots
	projectileStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorWhite, grayscale)).Background(tcell.ColorBlack)
	for _, proj := range g.projectiles {
		if proj.Alive {
			screen.SetContent(proj.Pos.X, proj.Pos.Y, '*', nil, projectileStyle)
		}
	}

	// Draw score in cyan
	scoreStr := fmt.Sprintf("SCORE= %d", g.score)
	scoreStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorBlue, grayscale)).Background(tcell.ColorBlack)
	for i, char := range scoreStr {
		if i+1 < w {
			screen.SetContent(i+1, 1, char, nil, scoreStyle)
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// drawLine draws a line between two points using simple line drawing
func drawLine(screen tcell.Screen, p1, p2 Point, style tcell.Style) {
	dx := abs(p2.X - p1.X)
	dy := abs(p2.Y - p1.Y)
	sx := 1
	if p1.X > p2.X {
		sx = -1
	}
	sy := 1
	if p1.Y > p2.Y {
		sy = -1
	}
	err := dx - dy

	x, y := p1.X, p1.Y
	maxSteps := dx + dy
	if maxSteps == 0 {
		maxSteps = 1
	}
	
	char := '│'
	if dx > dy {
		char = '─'
	} else if (p2.X > p1.X && p2.Y > p1.Y) || (p2.X < p1.X && p2.Y < p1.Y) {
		char = '\\'
	} else {
		char = '/'
	}

	for i := 0; i <= maxSteps; i++ {
		w, h := screen.Size()
		if x >= 1 && x < w-1 && y >= 1 && y < h-1 {
			screen.SetContent(x, y, char, nil, style)
		}
		
		if x == p2.X && y == p2.Y {
			break
		}
		
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x += sx
		}
		if e2 < dx {
			err += dx
			y += sy
		}
	}
}
