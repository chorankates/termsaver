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
			// Full color cycle every 90 seconds
			colorPhase := elapsed / 90.0
			// Subtle breathing: lightness oscillates gently
			breathe := 0.05 * math.Sin(elapsed*0.4)

			refreshBgChars(bgChars, w, h)

			phrase := fuzzyTimePhrase(now)
			modLine, hourLine := splitFuzzyPhrase(phrase)

			// Draw dim background fill (no screen.Clear needed — every cell is set)
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					bgPhase := colorPhase + float64(x)*0.005 + float64(y)*0.011
					bgColor := fuzzyHSLColor(bgPhase, 0.25, 0.17+breathe*0.3, grayscale)
					screen.SetContent(x, y, bgChars[y][x], nil, tcell.StyleDefault.Foreground(bgColor))
				}
			}

			centerY := h / 2

			if hourLine == "" {
				// Single-word phrase: "noon" or "midnight"
				drawFuzzyLine(screen, modLine, w, centerY, colorPhase, 0.0, 0.80+breathe, 1.0, grayscale)
			} else {
				drawFuzzyLine(screen, modLine, w, centerY-1, colorPhase, 0.0, 0.78+breathe, 1.0, grayscale)
				drawFuzzyLine(screen, hourLine, w, centerY+1, colorPhase, 0.08, 0.82+breathe, 1.0, grayscale)
			}

			// Seconds progress bar
			drawFuzzySecondsBar(screen, now, w, h, colorPhase, grayscale)

			// Actual time, dim, top-right corner
			actualTime := now.Format("15:04:05")
			dimColor := fuzzyHSLColor(colorPhase, 0.4, 0.38, grayscale)
			dimStyle := tcell.StyleDefault.Foreground(dimColor)
			for i, ch := range actualTime {
				screen.SetContent(w-len(actualTime)+i, 0, ch, nil, dimStyle)
			}

			screen.Show()
		}
	}
}

func drawFuzzyLine(screen tcell.Screen, text string, w, y int, phase, phaseOffset, lightness, saturation float64, grayscale bool) {
	runeSlice := []rune(text)
	x := (w - len(runeSlice)) / 2
	if x < 0 {
		x = 0
	}
	for i, ch := range runeSlice {
		charPhase := phase + phaseOffset + float64(i)*0.008
		color := fuzzyHSLColor(charPhase, saturation, lightness, grayscale)
		style := tcell.StyleDefault.Foreground(color).Bold(true)
		screen.SetContent(x+i, y, ch, nil, style)
	}
}

func drawFuzzySecondsBar(screen tcell.Screen, now time.Time, w, h int, colorPhase float64, grayscale bool) {
	sec := now.Second()
	barWidth := w - 4
	if barWidth <= 0 {
		return
	}
	filled := (sec * barWidth) / 60
	barY := h - 2
	for x := 0; x < barWidth; x++ {
		var ch rune
		var color tcell.Color
		if x < filled {
			ch = '━'
			color = fuzzyHSLColor(colorPhase+0.5, 0.9, 0.65, grayscale)
		} else {
			ch = '─'
			color = fuzzyHSLColor(colorPhase+0.5, 0.2, 0.22, grayscale)
		}
		screen.SetContent(x+2, barY, ch, nil, tcell.StyleDefault.Foreground(color))
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
