package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
)

type Tower struct {
	pos    Point
	range_ int
	damage int
	cooldown int
	lastShot int
}

type Enemy struct {
	pos      Point
	health   int
	maxHealth int
	pathIndex int
	alive    bool
}

type Terrain struct {
	pos Point
	blocking bool
}

func runTowerDefense(screen tcell.Screen, sigChan chan os.Signal) {
	w, h := screen.Size()
	
	// Game state
	towers := []Tower{}
	enemies := []Enemy{}
	terrain := []Terrain{}
	path := []Point{}
	wave := 0
	score := 0
	enemiesKilled := 0
	
	// Initialize path (simple zigzag path)
	path = generatePath(w, h)
	
	// Initialize terrain and towers
	towers, terrain = generateLayout(w, h, path)
	
	// Last randomization time
	lastRandomize := time.Now()
	randomizeInterval := time.Duration(30+rand.Intn(16)) * time.Second
	
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	// Event handling
	eventChan := make(chan tcell.Event, 10)
	go func() {
		for {
			eventChan <- screen.PollEvent()
		}
	}()
	
	enemySpawnTimer := 0
	enemySpawnDelay := 50 // frames
	
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
			case *tcell.EventResize:
				w, h = screen.Size()
				path = generatePath(w, h)
				towers, terrain = generateLayout(w, h, path)
				screen.Sync()
			}
		case <-ticker.C:
			// Randomize layout every 30-45 seconds
			if time.Since(lastRandomize) >= randomizeInterval {
				towers, terrain = generateLayout(w, h, path)
				lastRandomize = time.Now()
				randomizeInterval = time.Duration(30+rand.Intn(16)) * time.Second
				// Clear existing enemies when layout changes
				enemies = []Enemy{}
				wave = 0
			}
			
			// Spawn enemies
			enemySpawnTimer++
			if enemySpawnTimer >= enemySpawnDelay && len(enemies) < 10 {
				if len(path) > 0 {
					enemies = append(enemies, Enemy{
						pos:      path[0],
						health:   10 + wave*2,
						maxHealth: 10 + wave*2,
						pathIndex: 0,
						alive:    true,
					})
					enemySpawnTimer = 0
					if len(enemies)%5 == 0 {
						wave++
					}
				}
			}
			
			// Move enemies along path
			for i := range enemies {
				if !enemies[i].alive {
					continue
				}
				
				enemies[i].pathIndex++
				if enemies[i].pathIndex >= len(path) {
					// Enemy reached the end - remove it
					enemies[i].alive = false
					continue
				}
				enemies[i].pos = path[enemies[i].pathIndex]
			}
			
			// Towers shoot at enemies
			for i := range towers {
				towers[i].lastShot++
				if towers[i].lastShot < towers[i].cooldown {
					continue
				}
				
				// Find closest enemy in range
				closestEnemy := -1
				closestDist := towers[i].range_ + 1
				
				for j := range enemies {
					if !enemies[j].alive {
						continue
					}
					
					dist := abs(towers[i].pos.X-enemies[j].pos.X) + abs(towers[i].pos.Y-enemies[j].pos.Y)
					if dist <= towers[i].range_ && dist < closestDist {
						closestDist = dist
						closestEnemy = j
					}
				}
				
				// Shoot at closest enemy
				if closestEnemy >= 0 {
					enemies[closestEnemy].health -= towers[i].damage
					towers[i].lastShot = 0
					
					if enemies[closestEnemy].health <= 0 {
						enemies[closestEnemy].alive = false
						enemiesKilled++
						score += 10
					}
				}
			}
			
			// Remove dead enemies
			aliveEnemies := []Enemy{}
			for i := range enemies {
				if enemies[i].alive {
					aliveEnemies = append(aliveEnemies, enemies[i])
				}
			}
			enemies = aliveEnemies
			
			// Draw
			screen.Clear()
			
			// Draw terrain
			terrainStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray).Background(tcell.ColorBlack)
			for _, t := range terrain {
				if t.blocking {
					screen.SetContent(t.pos.X, t.pos.Y, '▓', nil, terrainStyle)
				} else {
					screen.SetContent(t.pos.X, t.pos.Y, '·', nil, terrainStyle)
				}
			}
			
			// Draw path
			pathStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow).Background(tcell.ColorBlack)
			for _, p := range path {
				if p.X >= 0 && p.X < w && p.Y >= 0 && p.Y < h {
					screen.SetContent(p.X, p.Y, '·', nil, pathStyle)
				}
			}
			
			// Draw towers
			towerStyle := tcell.StyleDefault.Foreground(tcell.ColorBlue).Background(tcell.ColorBlack)
			for _, tower := range towers {
				if tower.pos.X >= 0 && tower.pos.X < w && tower.pos.Y >= 0 && tower.pos.Y < h {
					screen.SetContent(tower.pos.X, tower.pos.Y, '▲', nil, towerStyle)
				}
			}
			
			// Draw enemies
			for _, enemy := range enemies {
				if enemy.pos.X >= 0 && enemy.pos.X < w && enemy.pos.Y >= 0 && enemy.pos.Y < h {
					enemyStyle := tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack)
					healthPercent := float64(enemy.health) / float64(enemy.maxHealth)
					var char rune
					if healthPercent > 0.75 {
						char = '█'
					} else if healthPercent > 0.5 {
						char = '▓'
					} else if healthPercent > 0.25 {
						char = '▒'
					} else {
						char = '░'
					}
					screen.SetContent(enemy.pos.X, enemy.pos.Y, char, nil, enemyStyle)
				}
			}
			
			// Draw projectiles (visual effect - bullets from towers to enemies)
			for _, tower := range towers {
				if tower.lastShot > tower.cooldown-5 && tower.lastShot < tower.cooldown {
					// Find target
					closestEnemy := -1
					closestDist := tower.range_ + 1
					
					for j := range enemies {
						if !enemies[j].alive {
							continue
						}
						dist := abs(tower.pos.X-enemies[j].pos.X) + abs(tower.pos.Y-enemies[j].pos.Y)
						if dist <= tower.range_ && dist < closestDist {
							closestDist = dist
							closestEnemy = j
						}
					}
					
					if closestEnemy >= 0 {
						drawLine(screen, tower.pos, enemies[closestEnemy].pos, tcell.ColorWhite)
					}
				}
			}
			
			// Draw UI
			scoreStr := fmt.Sprintf("Wave: %d | Killed: %d | Score: %d", wave, enemiesKilled, score)
			uiStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)
			for i, char := range scoreStr {
				if i < w {
					screen.SetContent(i, 0, char, nil, uiStyle)
				}
			}
			
			// Draw next randomization countdown
			timeLeft := int(randomizeInterval.Seconds() - time.Since(lastRandomize).Seconds())
			if timeLeft > 0 {
				countdownStr := fmt.Sprintf("Next layout: %ds", timeLeft)
				for i, char := range countdownStr {
					if w-len(countdownStr)+i >= 0 && w-len(countdownStr)+i < w {
						screen.SetContent(w-len(countdownStr)+i, h-1, char, nil, uiStyle)
					}
				}
			}
			
			screen.Show()
		}
	}
}

func generatePath(w, h int) []Point {
	path := []Point{}
	
	// Create a simple zigzag path from left to right
	midY := h / 2
	step := 2
	
	// Start from left edge
	startX := 2
	if startX >= w {
		startX = 1
	}
	
	for x := startX; x < w-2; x += step {
		// Zigzag pattern
		y := midY
		if (x/step)%2 == 1 {
			y = midY - 3
			if y < 2 {
				y = midY
			}
		} else {
			y = midY + 3
			if y >= h-2 {
				y = midY
			}
		}
		path = append(path, Point{X: x, Y: y})
	}
	
	// Ensure we have at least some path
	if len(path) < 5 {
		path = []Point{
			{2, h / 2},
			{w / 4, h / 2},
			{w / 2, h / 2},
			{3 * w / 4, h / 2},
			{w - 3, h / 2},
		}
	}
	
	return path
}

func generateLayout(w, h int, path []Point) ([]Tower, []Terrain) {
	towers := []Tower{}
	terrain := []Terrain{}
	
	// Create a set of path points to avoid placing towers/terrain on path
	pathSet := make(map[Point]bool)
	for _, p := range path {
		pathSet[p] = true
		// Also block adjacent cells
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				pathSet[Point{p.X + dx, p.Y + dy}] = true
			}
		}
	}
	
	// Place some terrain (obstacles)
	numTerrain := (w * h) / 30
	for i := 0; i < numTerrain; i++ {
		attempts := 0
		for attempts < 50 {
			x := 1 + rand.Intn(w-2)
			y := 1 + rand.Intn(h-2)
			pos := Point{x, y}
			
			if !pathSet[pos] {
				terrain = append(terrain, Terrain{
					pos:      pos,
					blocking: rand.Float32() < 0.7, // 70% blocking, 30% decorative
				})
				pathSet[pos] = true
				break
			}
			attempts++
		}
	}
	
	// Place towers (defenders)
	numTowers := 5 + rand.Intn(8) // 5-12 towers
	for i := 0; i < numTowers; i++ {
		attempts := 0
		for attempts < 50 {
			x := 1 + rand.Intn(w-2)
			y := 1 + rand.Intn(h-2)
			pos := Point{x, y}
			
			if !pathSet[pos] {
				towers = append(towers, Tower{
					pos:     pos,
					range_:  5 + rand.Intn(5), // Range 5-9
					damage:  2 + rand.Intn(3), // Damage 2-4
					cooldown: 10 + rand.Intn(10), // Cooldown 10-19 frames
					lastShot: rand.Intn(10),
				})
				pathSet[pos] = true
				break
			}
			attempts++
		}
	}
	
	return towers, terrain
}

func drawLine(screen tcell.Screen, p1, p2 Point, color tcell.Color) {
	w, h := screen.Size()
	dx := abs(p2.X - p1.X)
	dy := abs(p2.Y - p1.Y)
	
	var sx, sy int
	if p1.X < p2.X {
		sx = 1
	} else {
		sx = -1
	}
	if p1.Y < p2.Y {
		sy = 1
	} else {
		sy = -1
	}
	
	err := dx - dy
	x, y := p1.X, p1.Y
	
	for i := 0; i < 10 && (x != p2.X || y != p2.Y); i++ {
		if x >= 0 && x < w && y >= 0 && y < h {
			screen.SetContent(x, y, '•', nil, tcell.StyleDefault.Foreground(color).Background(tcell.ColorBlack))
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

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

