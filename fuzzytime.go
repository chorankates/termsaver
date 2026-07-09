package main

import (
	"math"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

var bgAlphabet = []rune("abcdefghijklmnopqrstuvwxyz")

// Block font: 4 pixels wide × 5 pixels tall per glyph.
// Each pixel renders as 1 terminal column × 2 terminal rows.
const (
	bigFontW   = 4 // pixels wide per glyph
	bigFontH   = 5 // pixels tall per glyph
	bigScaleX  = 1 // terminal columns per pixel
	bigScaleY  = 2 // terminal rows per pixel
	bigCharGap = 1 // terminal columns between glyphs
	bigWordGap = 3 // terminal columns between words
)

// Each entry is 5 rows; each row is 4 bits (bit 3 = leftmost column, bit 0 = rightmost).
var blockFont = map[rune][bigFontH]uint8{
	'a': {0x6, 0x9, 0xF, 0x9, 0x9}, // .XX. / X..X / XXXX / X..X / X..X
	'b': {0x8, 0xE, 0x9, 0x9, 0xE}, // X... / XXX. / X..X / X..X / XXX.
	'c': {0x7, 0x8, 0x8, 0x8, 0x7}, // .XXX / X... / X... / X... / .XXX
	'd': {0x1, 0x7, 0x9, 0x9, 0x7}, // ...X / .XXX / X..X / X..X / .XXX
	'e': {0x6, 0x9, 0xF, 0x8, 0x7}, // .XX. / X..X / XXXX / X... / .XXX
	'f': {0x7, 0x8, 0xE, 0x8, 0x8}, // .XXX / X... / XXX. / X... / X...
	'g': {0x7, 0x8, 0xB, 0x9, 0x7}, // .XXX / X... / X.XX / X..X / .XXX
	'h': {0x8, 0x8, 0xE, 0x9, 0x9}, // X... / X... / XXX. / X..X / X..X
	'i': {0xE, 0x4, 0x4, 0x4, 0xE}, // XXX. / .X.. / .X.. / .X.. / XXX.
	'j': {0x7, 0x1, 0x1, 0x9, 0x6}, // .XXX / ...X / ...X / X..X / .XX.
	'k': {0x9, 0xA, 0xC, 0xA, 0x9}, // X..X / X.X. / XX.. / X.X. / X..X
	'l': {0xC, 0x4, 0x4, 0x4, 0x7}, // XX.. / .X.. / .X.. / .X.. / .XXX
	'm': {0x9, 0xF, 0x9, 0x9, 0x9}, // X..X / XXXX / X..X / X..X / X..X
	'n': {0x8, 0xE, 0x9, 0x9, 0x9}, // X... / XXX. / X..X / X..X / X..X
	'o': {0x6, 0x9, 0x9, 0x9, 0x6}, // .XX. / X..X / X..X / X..X / .XX.
	'p': {0xE, 0x9, 0xE, 0x8, 0x8}, // XXX. / X..X / XXX. / X... / X...
	'q': {0x7, 0x9, 0x7, 0x1, 0x1}, // .XXX / X..X / .XXX / ...X / ...X
	'r': {0x7, 0x8, 0x8, 0x8, 0x8}, // .XXX / X... / X... / X... / X...
	's': {0x7, 0x8, 0x6, 0x1, 0xE}, // .XXX / X... / .XX. / ...X / XXX.
	't': {0xF, 0x4, 0x4, 0x4, 0x3}, // XXXX / .X.. / .X.. / .X.. / ..XX
	'u': {0x9, 0x9, 0x9, 0x9, 0x7}, // X..X / X..X / X..X / X..X / .XXX
	'v': {0x9, 0x9, 0x9, 0x6, 0x6}, // X..X / X..X / X..X / .XX. / .XX.
	'w': {0x9, 0x9, 0x9, 0xF, 0x6}, // X..X / X..X / X..X / XXXX / .XX.
	'x': {0x9, 0x9, 0x6, 0x9, 0x9}, // X..X / X..X / .XX. / X..X / X..X
	'y': {0x9, 0x6, 0x6, 0x4, 0x4}, // X..X / .XX. / .XX. / .X.. / .X..
	'z': {0xF, 0x2, 0x4, 0x8, 0xF}, // XXXX / ..X. / .X.. / X... / XXXX
	'-': {0x0, 0x0, 0x6, 0x0, 0x0}, // .... / .... / .XX. / .... / ....
	'\'': {0x6, 0x4, 0x0, 0x0, 0x0}, // .XX. / .X.. / .... / .... / ....
}

// bigLineWidth returns the terminal-column width of text rendered in block font.
func bigLineWidth(text string) int {
	words := strings.Fields(text)
	total := 0
	for wi, word := range words {
		if wi > 0 {
			total += bigWordGap
		}
		for ci := range word {
			if ci > 0 {
				total += bigCharGap
			}
			total += bigFontW * bigScaleX
		}
	}
	return total
}

// drawBigLine renders text at (startX, startY) using the block font.
// On-pixels draw '█' with fg; off-pixels draw the background char with bg.
func drawBigLine(screen tcell.Screen, text string, startX, startY int, fg, bg tcell.Color, bgGrid [][]rune, w, h int) {
	fgStyle := tcell.StyleDefault.Foreground(fg)
	bgStyle := tcell.StyleDefault.Foreground(bg)
	x := startX
	for wi, word := range strings.Fields(text) {
		if wi > 0 {
			x += bigWordGap
		}
		for ci, ch := range word {
			if ci > 0 {
				x += bigCharGap
			}
			rows, ok := blockFont[ch]
			if !ok {
				x += bigFontW * bigScaleX
				continue
			}
			for py := 0; py < bigFontH; py++ {
				for sy := 0; sy < bigScaleY; sy++ {
					ty := startY + py*bigScaleY + sy
					if ty < 0 || ty >= h {
						continue
					}
					for px := 0; px < bigFontW; px++ {
						for sx := 0; sx < bigScaleX; sx++ {
							tx := x + px*bigScaleX + sx
							if tx < 0 || tx >= w {
								continue
							}
							if (rows[py]>>uint(bigFontW-1-px))&1 == 1 {
								screen.SetContent(tx, ty, '█', nil, fgStyle)
							} else {
								var bgRune rune = ' '
								if ty < len(bgGrid) && tx < len(bgGrid[ty]) {
									bgRune = bgGrid[ty][tx]
								}
								screen.SetContent(tx, ty, bgRune, nil, bgStyle)
							}
						}
					}
				}
			}
			x += bigFontW * bigScaleX
		}
	}
}

func makeBgGrid(w, h int) [][]rune {
	grid := make([][]rune, h)
	for y := range grid {
		grid[y] = make([]rune, w)
		for x := range grid[y] {
			grid[y][x] = bgAlphabet[rand.Intn(len(bgAlphabet))]
		}
	}
	return grid
}

func refreshBgChars(grid [][]rune, w, h int) {
	n := (w * h) / 80
	if n < 1 {
		n = 1
	}
	for i := 0; i < n; i++ {
		x := rand.Intn(w)
		y := rand.Intn(h)
		grid[y][x] = bgAlphabet[rand.Intn(len(bgAlphabet))]
	}
}

func runFuzzyTime(screen tcell.Screen, sigChan chan os.Signal, interactive bool, grayscale bool) bool {
	w, h := screen.Size()
	bgChars := makeBgGrid(w, h)

	eventChan := make(chan tcell.Event, 10)
	go func() {
		for {
			eventChan <- screen.PollEvent()
		}
	}()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	startTime := time.Now()
	lastPhrase := ""
	phraseHue := rand.Float64()

	for {
		select {
		case <-sigChan:
			return false
		case event := <-eventChan:
			switch ev := event.(type) {
			case *tcell.EventKey:
				if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
					return false
				}
				if ev.Rune() == ' ' {
					return true
				}
			case *tcell.EventResize:
				w, h = screen.Size()
				screen.Sync()
				bgChars = makeBgGrid(w, h)
			}
		case <-ticker.C:
			now := time.Now()
			elapsed := now.Sub(startTime).Seconds()
			colorPhase := elapsed / 90.0
			breathe := 0.05 * math.Sin(elapsed*0.4)

			refreshBgChars(bgChars, w, h)

			phrase := fuzzyTimePhrase(now)
			if phrase != lastPhrase {
				phraseHue = rand.Float64()
				lastPhrase = phrase
			}
			modLine, hourLine := splitFuzzyPhrase(phrase)

			phraseColor := fuzzyHSLColor(phraseHue, 1.0, 0.58+breathe, grayscale)
			bgDrawColor := tcell.NewRGBColor(56, 56, 56)

			// Render full background grid
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					screen.SetContent(x, y, bgChars[y][x], nil, tcell.StyleDefault.Foreground(bgDrawColor))
				}
			}

			// Overlay block-font phrase; each glyph line is bigFontH*bigScaleY terminal rows tall
			centerY := h / 2
			charH := bigFontH * bigScaleY
			if hourLine == "" {
				lineY := centerY - charH/2
				lineX := (w - bigLineWidth(modLine)) / 2
				if lineX < 0 {
					lineX = 0
				}
				drawBigLine(screen, modLine, lineX, lineY, phraseColor, bgDrawColor, bgChars, w, h)
			} else {
				totalH := charH*2 + 2
				topY := centerY - totalH/2
				modX := (w - bigLineWidth(modLine)) / 2
				if modX < 0 {
					modX = 0
				}
				hourX := (w - bigLineWidth(hourLine)) / 2
				if hourX < 0 {
					hourX = 0
				}
				drawBigLine(screen, modLine, modX, topY, phraseColor, bgDrawColor, bgChars, w, h)
				drawBigLine(screen, hourLine, hourX, topY+charH+2, phraseColor, bgDrawColor, bgChars, w, h)
			}

			// Exact time, dim, top-right corner
			actualTime := now.Format("15:04:05")
			dimColor := fuzzyHSLColor(colorPhase, 0.5, 0.50, grayscale)
			dimStyle := tcell.StyleDefault.Foreground(dimColor)
			for i, ch := range actualTime {
				screen.SetContent(w-len(actualTime)+i, 0, ch, nil, dimStyle)
			}

			screen.Show()
		}
	}
}

func fuzzyTimePhrase(t time.Time) string {
	h := t.Hour()
	m := t.Minute()

	rounded := ((m + 2) / 5) * 5
	if rounded >= 60 {
		rounded = 0
		h = (h + 1) % 24
	}

	if rounded == 0 {
		suffix := " o'clock"
		if h == 0 || h == 12 {
			suffix = ""
		}
		return fuzzyHourWord(h) + suffix
	}

	if rounded <= 30 {
		return fuzzyMinuteWord(rounded) + " past " + fuzzyHourWord(h)
	}
	return fuzzyMinuteWord(60-rounded) + " to " + fuzzyHourWord((h+1)%24)
}

func fuzzyHourWord(h int) string {
	words := []string{
		"midnight", "one", "two", "three", "four", "five",
		"six", "seven", "eight", "nine", "ten", "eleven",
		"noon", "one", "two", "three", "four", "five",
		"six", "seven", "eight", "nine", "ten", "eleven",
	}
	return words[h]
}

func fuzzyMinuteWord(m int) string {
	switch m {
	case 5:
		return "five"
	case 10:
		return "ten"
	case 15:
		return "quarter"
	case 20:
		return "twenty"
	case 25:
		return "twenty-five"
	case 30:
		return "half"
	}
	return ""
}

func splitFuzzyPhrase(phrase string) (string, string) {
	words := strings.Fields(phrase)
	if len(words) <= 2 {
		return phrase, ""
	}
	pivot := len(words) - 1
	return strings.Join(words[:pivot], " "), words[pivot]
}

func fuzzyHSLColor(h, s, l float64, grayscale bool) tcell.Color {
	h -= math.Floor(h)
	r, g, b := fuzzyHSLtoRGB(h, s, l)
	if grayscale {
		lum := uint8(float64(r)*0.299 + float64(g)*0.587 + float64(b)*0.114)
		return tcell.NewRGBColor(int32(lum), int32(lum), int32(lum))
	}
	return tcell.NewRGBColor(int32(r), int32(g), int32(b))
}

func fuzzyHSLtoRGB(h, s, l float64) (uint8, uint8, uint8) {
	if s == 0 {
		v := uint8(l * 255)
		return v, v, v
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	r := fuzzyHueComponent(p, q, h+1.0/3.0)
	g := fuzzyHueComponent(p, q, h)
	b := fuzzyHueComponent(p, q, h-1.0/3.0)
	return uint8(r * 255), uint8(g * 255), uint8(b * 255)
}

func fuzzyHueComponent(p, q, t float64) float64 {
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	switch {
	case t < 1.0/6.0:
		return p + (q-p)*6*t
	case t < 0.5:
		return q
	case t < 2.0/3.0:
		return p + (q-p)*(2.0/3.0-t)*6
	default:
		return p
	}
}
