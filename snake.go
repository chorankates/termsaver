package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
)

type Point struct {
	X, Y int
}

type Snake struct {
	body      []Point
	direction Point
	alive     bool
}

func runSnake(screen tcell.Screen, sigChan chan os.Signal, interactive bool, grayscale bool, requestedSize int, scale int) {
	termW, termH := screen.Size()

	// Clamp scale to reasonable values
	if scale < 1 {
		scale = 1
	}
	if scale > 4 {
		scale = 4
	}

	// Cell size: each game cell is rendered as cellW x cellH terminal characters
	// Using scale for width and scale/2 (min 1) for height to maintain roughly square cells
	cellW := scale
	cellH := scale / 2
	if cellH < 1 {
		cellH = 1
	}

	// Calculate game grid size (in game cells, not terminal chars)
	// Default to a reasonable size based on terminal
	var gameW, gameH int
	var offsetX, offsetY int

	if !interactive {
		// Non-interactive mode: use entire terminal window
		// Leave 1 row at top for score display
		gameW = termW / cellW
		gameH = (termH - 1) / cellH
		if gameW < 10 {
			gameW = 10
		}
		if gameH < 10 {
			gameH = 10
		}
		offsetX = 0
		offsetY = 1 // Leave room for score at top
	} else {
		// Interactive mode: centered square game area
		gameSize := requestedSize
		if gameSize <= 0 {
			// Auto-size based on terminal, accounting for cell size
			maxW := (termW * 8 / 10) / cellW
			maxH := (termH * 8 / 10) / cellH
			gameSize = maxW
			if maxH < gameSize {
				gameSize = maxH
			}
		}
		if gameSize > 50 {
			gameSize = 50
		}
		if gameSize < 10 {
			gameSize = 10
		}

		gameW = gameSize
		gameH = gameSize

		// Calculate offset to center the game area in terminal
		pixelW := gameW * cellW
		pixelH := gameH * cellH
		offsetX = (termW - pixelW) / 2
		offsetY = (termH - pixelH) / 2
	}

	// Calculate pixel dimensions (used for score positioning and resize)
	pixelW := gameW * cellW
	pixelH := gameH * cellH

	// Helper function to draw a game cell as a block
	drawCell := func(gx, gy int, ch rune, style tcell.Style) {
		px := offsetX + gx*cellW
		py := offsetY + gy*cellH
		for dy := 0; dy < cellH; dy++ {
			for dx := 0; dx < cellW; dx++ {
				screen.SetContent(px+dx, py+dy, ch, nil, style)
			}
		}
	}

	// Initialize snake in center of game area
	centerX := gameW / 2
	centerY := gameH / 2
	snake := Snake{
		body: []Point{
			{centerX, centerY},
			{centerX - 1, centerY},
			{centerX - 2, centerY},
		},
		direction: Point{1, 0},
		alive:     true,
	}

	food := Point{1 + (gameW-2)/4, 1 + (gameH-2)/4}
	score := 0
	var gameOverTime time.Time

	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	// Event handling for input
	eventChan := make(chan tcell.Event, 10)
	go func() {
		for {
			eventChan <- screen.PollEvent()
		}
	}()

	for {
		select {
		case <-sigChan:
			return
		case event := <-eventChan:
			switch ev := event.(type) {
			case *tcell.EventKey:
				// Always handle exit keys, regardless of interactive mode
				if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
					return
				}
				// In non-interactive mode, any key exits
				if !interactive {
					return
				}
				// Handle movement keys only in interactive mode
				if interactive {
					switch ev.Key() {
					case tcell.KeyUp, tcell.KeyCtrlP:
						if snake.direction.Y == 0 {
							snake.direction = Point{0, -1}
						}
					case tcell.KeyDown, tcell.KeyCtrlN:
						if snake.direction.Y == 0 {
							snake.direction = Point{0, 1}
						}
					case tcell.KeyLeft, tcell.KeyCtrlB:
						if snake.direction.X == 0 {
							snake.direction = Point{-1, 0}
						}
					case tcell.KeyRight, tcell.KeyCtrlF:
						if snake.direction.X == 0 {
							snake.direction = Point{1, 0}
						}
					}
				}
			case *tcell.EventResize:
				termW, termH = screen.Size()
				if !interactive {
					// Non-interactive: resize game to fill terminal
					gameW = termW / cellW
					gameH = (termH - 1) / cellH
					if gameW < 10 {
						gameW = 10
					}
					if gameH < 10 {
						gameH = 10
					}
					pixelW = gameW * cellW
					pixelH = gameH * cellH
					offsetX = 0
					offsetY = 1
				} else {
					// Interactive: recalculate offset to keep game centered
					offsetX = (termW - pixelW) / 2
					offsetY = (termH - pixelH) / 2
				}
				screen.Sync()
			}
		case <-ticker.C:
			if !snake.alive {
				// Track when game over started
				if gameOverTime.IsZero() {
					gameOverTime = time.Now()
				}
				
				// Calculate countdown (3, 2, 1, 0)
				elapsed := time.Since(gameOverTime)
				countdown := 3 - int(elapsed.Seconds())
				
				screen.Clear()
				
				if countdown > 0 {
					// Show countdown
					msg1 := fmt.Sprintf("GAME OVER - Score: %d", score)
					countdownMsg := fmt.Sprintf("Restarting in %d...", countdown)
					
					x1 := (termW - len(msg1)) / 2
					if x1 < 0 {
						x1 = 0
					}
					x2 := (termW - len(countdownMsg)) / 2
					if x2 < 0 {
						x2 = 0
					}
					
					style1 := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorRed, grayscale)).Background(tcell.ColorBlack)
					style2 := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorYellow, grayscale)).Background(tcell.ColorBlack)
					
					for i, char := range msg1 {
						if x1+i >= 0 && x1+i < termW {
							screen.SetContent(x1+i, termH/2-1, char, nil, style1)
						}
					}
					for i, char := range countdownMsg {
						if x2+i >= 0 && x2+i < termW {
							screen.SetContent(x2+i, termH/2+1, char, nil, style2)
						}
					}
				} else {
					// Countdown finished - restart the game
					termW, termH = screen.Size()
					if !interactive {
						// Non-interactive: resize game to fill terminal
						gameW = termW / cellW
						gameH = (termH - 1) / cellH
						if gameW < 10 {
							gameW = 10
						}
						if gameH < 10 {
							gameH = 10
						}
						pixelW = gameW * cellW
						pixelH = gameH * cellH
						offsetX = 0
						offsetY = 1
					} else {
						offsetX = (termW - pixelW) / 2
						offsetY = (termH - pixelH) / 2
					}
					// Recalculate center based on current game size
					centerX = gameW / 2
					centerY = gameH / 2
					snake = Snake{
						body: []Point{
							{centerX, centerY},
							{centerX - 1, centerY},
							{centerX - 2, centerY},
						},
						direction: Point{1, 0},
						alive:     true,
					}
					food = Point{1 + (gameW-2)/4, 1 + (gameH-2)/4}
					score = 0
					gameOverTime = time.Time{}
					continue
				}
				
				screen.Show()
				continue
			}

			// Automatic gameplay: calculate optimal direction
			if !interactive {
				snake.direction = findOptimalDirection(snake, food, gameW, gameH)
			}

			// Move snake
			head := snake.body[0]
			newHead := Point{
				X: head.X + snake.direction.X,
				Y: head.Y + snake.direction.Y,
			}

			// Check wall collision (account for border)
			if newHead.X <= 0 || newHead.X >= gameW-1 || newHead.Y <= 0 || newHead.Y >= gameH-1 {
				snake.alive = false
				continue
			}

			// Check self collision
			for _, segment := range snake.body {
				if newHead.X == segment.X && newHead.Y == segment.Y {
					snake.alive = false
					break
				}
			}

			if !snake.alive {
				continue
			}

			snake.body = append([]Point{newHead}, snake.body...)

			// Check food collision
			if newHead.X == food.X && newHead.Y == food.Y {
				score++
				// Generate new food (avoid border area)
				food = Point{1 + rand.Intn(gameW-2), 1 + rand.Intn(gameH-2)}
				// Make sure food is not on snake
				for {
					onSnake := false
					for _, segment := range snake.body {
						if food.X == segment.X && food.Y == segment.Y {
							onSnake = true
							break
						}
					}
					if !onSnake {
						break
					}
					food = Point{1 + rand.Intn(gameW-2), 1 + rand.Intn(gameH-2)}
				}
			} else {
				snake.body = snake.body[:len(snake.body)-1]
			}

			// Draw
			screen.Clear()

			// Draw grid background - checkerboard pattern for visibility
			gridLight := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorDarkGray, grayscale)).Background(tcell.ColorBlack)
			gridDark := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorBlack)
			for y := 1; y < gameH-1; y++ {
				for x := 1; x < gameW-1; x++ {
					if (x+y)%2 == 0 {
						drawCell(x, y, '·', gridLight)
					} else {
						drawCell(x, y, ' ', gridDark)
					}
				}
			}

			// Draw border using block characters
			borderStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorWhite, grayscale)).Background(tcell.ColorBlack)
			// Top and bottom borders
			for x := 0; x < gameW; x++ {
				drawCell(x, 0, '█', borderStyle)
				drawCell(x, gameH-1, '█', borderStyle)
			}
			// Left and right borders
			for y := 1; y < gameH-1; y++ {
				drawCell(0, y, '█', borderStyle)
				drawCell(gameW-1, y, '█', borderStyle)
			}

			// Draw snake (same character for head and body)
			snakeStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorGreen, grayscale)).Background(tcell.ColorBlack)
			for _, segment := range snake.body {
				drawCell(segment.X, segment.Y, '█', snakeStyle)
			}

			// Draw food
			foodStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorRed, grayscale)).Background(tcell.ColorBlack)
			drawCell(food.X, food.Y, '█', foodStyle)

			// Draw score above the game area
			scoreStr := fmt.Sprintf("Score: %d", score)
			scoreStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorYellow, grayscale)).Background(tcell.ColorBlack)
			scoreX := offsetX + (pixelW-len(scoreStr))/2
			scoreY := offsetY - 1
			if scoreY >= 0 {
				for i, char := range scoreStr {
					screen.SetContent(scoreX+i, scoreY, char, nil, scoreStyle)
				}
			}

			screen.Show()
		}
	}
}

// findOptimalDirection uses BFS pathfinding to find the best direction to the food
func findOptimalDirection(snake Snake, food Point, w, h int) Point {
	head := snake.body[0]

	// Create a set of occupied cells (snake body)
	occupied := make(map[Point]bool)
	for _, segment := range snake.body {
		occupied[segment] = true
	}

	// BFS to find shortest path to food
	directions := []Point{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
	type node struct {
		pos       Point
		firstDir  Point
	}

	queue := []node{{head, Point{}}}
	visited := make(map[Point]bool)
	visited[head] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.pos.X == food.X && current.pos.Y == food.Y {
			// Found path to food, return first direction
			if current.firstDir.X != 0 || current.firstDir.Y != 0 {
				return current.firstDir
			}
			// If we're already at food, just continue in current direction
			return snake.direction
		}

		for _, dir := range directions {
			next := Point{
				X: current.pos.X + dir.X,
				Y: current.pos.Y + dir.Y,
			}

			// Check bounds (account for border)
			if next.X <= 0 || next.X >= w-1 || next.Y <= 0 || next.Y >= h-1 {
				continue
			}

			// Check if visited or occupied (except tail, which will move)
			tail := snake.body[len(snake.body)-1]
			if visited[next] || (occupied[next] && next != tail) {
				continue
			}

			// Don't reverse direction (only for first step from head)
			if current.pos == head {
				if snake.direction.X != 0 && dir.X == -snake.direction.X {
					continue
				}
				if snake.direction.Y != 0 && dir.Y == -snake.direction.Y {
					continue
				}
			}

			visited[next] = true
			firstDir := current.firstDir
			if firstDir.X == 0 && firstDir.Y == 0 {
				firstDir = dir
			}
			queue = append(queue, node{next, firstDir})
		}
	}

	// If no path to food found, use a safe movement strategy
	// Try to avoid walls and self
	for _, dir := range directions {
		next := Point{
			X: head.X + dir.X,
			Y: head.Y + dir.Y,
		}

		// Check bounds
		if next.X <= 0 || next.X >= w-1 || next.Y <= 0 || next.Y >= h-1 {
			continue
		}

		// Check if occupied (except tail)
		tail := snake.body[len(snake.body)-1]
		if occupied[next] && next != tail {
			continue
		}

		// Don't reverse
		if snake.direction.X != 0 && dir.X == -snake.direction.X {
			continue
		}
		if snake.direction.Y != 0 && dir.Y == -snake.direction.Y {
			continue
		}

		return dir
	}

	// Fallback: continue in current direction
	return snake.direction
}

