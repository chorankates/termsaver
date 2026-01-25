package main

import (
	"math/rand"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
)

type MatrixColumn struct {
	chars    []rune
	position int
	speed    int
}

func runMatrixRain(screen tcell.Screen, sigChan chan os.Signal, interactive bool, grayscale bool) {
	w, h := screen.Size()
	columns := make([]MatrixColumn, w)

	// Initialize columns
	for i := range columns {
		columns[i] = MatrixColumn{
			chars:    generateMatrixChars(h),
			position: -rand.Intn(h * 2),
			speed:    1 + rand.Intn(2),
		}
	}

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	style := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorGreen, grayscale)).Background(tcell.ColorBlack)
	brightStyle := tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorWhite, grayscale)).Background(tcell.ColorBlack)

	// Event handling for resize
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
			case *tcell.EventResize:
				w, h = screen.Size()
				// Reinitialize columns for new size
				newColumns := make([]MatrixColumn, w)
				for i := range newColumns {
					if i < len(columns) {
						newColumns[i] = columns[i]
					} else {
						newColumns[i] = MatrixColumn{
							chars:    generateMatrixChars(h),
							position: -rand.Intn(h * 2),
							speed:    1 + rand.Intn(2),
						}
					}
				}
				columns = newColumns
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

			for x := 0; x < w; x++ {
				col := &columns[x]
				col.position += col.speed

				for i, char := range col.chars {
					y := col.position - (len(col.chars) - i)
					if y >= 0 && y < h {
						// Fade effect: brighter at head, darker at tail
						charStyle := style
						if i == len(col.chars)-1 {
							charStyle = brightStyle
						} else if i > len(col.chars)-5 {
							charStyle = tcell.StyleDefault.Foreground(toGrayscale(tcell.ColorLime, grayscale)).Background(tcell.ColorBlack)
						}
						screen.SetContent(x, y, char, nil, charStyle)
					}
				}

				// Reset column when it goes off screen
				if col.position > h+len(col.chars) {
					col.position = -len(col.chars)
					col.chars = generateMatrixChars(h)
					col.speed = 1 + rand.Intn(2)
				}
			}

			screen.Show()
		}
	}
}

func generateMatrixChars(length int) []rune {
	chars := make([]rune, length)
	for i := range chars {
		// Mix of katakana, hiragana, and alphanumeric
		matrixChars := []rune{
			'ア', 'イ', 'ウ', 'エ', 'オ', 'カ', 'キ', 'ク', 'ケ', 'コ',
			'サ', 'シ', 'ス', 'セ', 'ソ', 'タ', 'チ', 'ツ', 'テ', 'ト',
			'ナ', 'ニ', 'ヌ', 'ネ', 'ノ', 'ハ', 'ヒ', 'フ', 'ヘ', 'ホ',
			'マ', 'ミ', 'ム', 'メ', 'モ', 'ヤ', 'ユ', 'ヨ', 'ラ', 'リ',
			'ル', 'レ', 'ロ', 'ワ', 'ヲ', 'ン',
			'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
		}
		chars[i] = matrixChars[rand.Intn(len(matrixChars))]
	}
	return chars
}

