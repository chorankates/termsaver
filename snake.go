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

func runSnake(screen tcell.Screen, sigChan chan os.Signal) {
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
				case tcell.KeyEscape, tcell.KeyCtrlC:
					return
				}
			case *tcell.EventResize:
				w, h = screen.Size()
				screen.Sync()
			}
		case <-ticker.C:
			if !snake.alive {
				// Game over - wait for exit
				screen.Clear()
				msg := fmt.Sprintf("GAME OVER - Score: %d", score)
				x := (w - len(msg)) / 2
				if x < 0 {
					x = 0
				}
				y := h / 2
				style := tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack)
				for i, char := range msg {
					if x+i >= 0 && x+i < w {
						screen.SetContent(x+i, y, char, nil, style)
					}
				}
				screen.Show()
				continue
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
			borderStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)
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
			snakeStyle := tcell.StyleDefault.Foreground(tcell.ColorGreen).Background(tcell.ColorBlack)
			headStyle := tcell.StyleDefault.Foreground(tcell.ColorLime).Background(tcell.ColorBlack)
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
			foodStyle := tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack)
			screen.SetContent(food.X, food.Y, '●', nil, foodStyle)

			// Draw score
			scoreStr := fmt.Sprintf("Score: %d", score)
			scoreStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow).Background(tcell.ColorBlack)
			for i, char := range scoreStr {
				if i+1 < w {
					screen.SetContent(i+1, 1, char, nil, scoreStyle)
				}
			}

			screen.Show()
		}
	}
}

