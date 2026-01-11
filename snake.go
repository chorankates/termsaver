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

func runSnake(screen tcell.Screen, sigChan chan os.Signal, interactive bool, grayscale bool) {
	w, h := screen.Size()

	// Initialize snake in center
	snake := Snake{
		body: []Point{
			{w / 2, h / 2},
			{w/2 - 1, h / 2},
			{w/2 - 2, h / 2},
		},
		direction: Point{1, 0},
		alive:     true,
	}

	food := Point{1 + (w-2)/4, 1 + (h-2)/4}
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
				w, h = screen.Size()
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
					
					x1 := (w - len(msg1)) / 2
					if x1 < 0 {
						x1 = 0
					}
					x2 := (w - len(countdownMsg)) / 2
					if x2 < 0 {
						x2 = 0
					}
					
					style1 := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorRed, grayscale)).Background(tcell.ColorBlack)
					style2 := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorYellow, grayscale)).Background(tcell.ColorBlack)
					
					for i, char := range msg1 {
						if x1+i >= 0 && x1+i < w {
							screen.SetContent(x1+i, h/2-1, char, nil, style1)
						}
					}
					for i, char := range countdownMsg {
						if x2+i >= 0 && x2+i < w {
							screen.SetContent(x2+i, h/2+1, char, nil, style2)
						}
					}
				} else {
					// Countdown finished - restart the game
					w, h = screen.Size()
					snake = Snake{
						body: []Point{
							{w / 2, h / 2},
							{w/2 - 1, h / 2},
							{w/2 - 2, h / 2},
						},
						direction: Point{1, 0},
						alive:     true,
					}
					food = Point{1 + (w-2)/4, 1 + (h-2)/4}
					score = 0
					gameOverTime = time.Time{}
					continue
				}
				
				screen.Show()
				continue
			}

			// Automatic gameplay: calculate optimal direction
			if !interactive {
				snake.direction = findOptimalDirection(snake, food, w, h)
			}

			// Move snake
			head := snake.body[0]
			newHead := Point{
				X: head.X + snake.direction.X,
				Y: head.Y + snake.direction.Y,
			}

			// Check wall collision (account for border)
			if newHead.X <= 0 || newHead.X >= w-1 || newHead.Y <= 0 || newHead.Y >= h-1 {
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
				food = Point{1 + rand.Intn(w-2), 1 + rand.Intn(h-2)}
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
					food = Point{1 + rand.Intn(w-2), 1 + rand.Intn(h-2)}
				}
			} else {
				snake.body = snake.body[:len(snake.body)-1]
			}

			// Draw
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

			// Draw snake
			snakeStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorGreen, grayscale)).Background(tcell.ColorBlack)
			headStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorLime, grayscale)).Background(tcell.ColorBlack)
			for i, segment := range snake.body {
				char := '█'
				if i == 0 {
					char = '◉'
					screen.SetContent(segment.X, segment.Y, char, nil, headStyle)
				} else {
					screen.SetContent(segment.X, segment.Y, char, nil, snakeStyle)
				}
			}

			// Draw food
			foodStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorRed, grayscale)).Background(tcell.ColorBlack)
			screen.SetContent(food.X, food.Y, '●', nil, foodStyle)

			// Draw score
			scoreStr := fmt.Sprintf("Score: %d", score)
			scoreStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorYellow, grayscale)).Background(tcell.ColorBlack)
			for i, char := range scoreStr {
				if i+1 < w {
					screen.SetContent(i+1, 1, char, nil, scoreStyle)
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

