package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/shirou/gopsutil/v3/cpu"
)

const (
	aurTickMs   = 33
	aurPollRate = time.Second
	aurLerp     = 0.06
)

type aurStar struct {
	cx, cy int     // character-cell position
	phase  float64 // twinkle phase offset
}

type aurState struct {
	ncpu    int
	loads   []float64 // smoothed 0..1
	targets []float64
	stars   []aurStar
	t       float64
}

func runAurora(screen tcell.Screen, sigChan chan os.Signal, interactive bool, grayscale bool) bool {
	w, h := screen.Size()

	// Determine core count. Fall back to 4 on error.
	initial, err := cpu.Percent(100*time.Millisecond, true)
	ncpu := len(initial)
	if err != nil || ncpu == 0 {
		ncpu = 4
	}

	state := &aurState{
		ncpu:    ncpu,
		loads:   make([]float64, ncpu),
		targets: make([]float64, ncpu),
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	state.stars = aurMakeStars(rng, w, h)

	eventChan := make(chan tcell.Event, 10)
	go func() {
		for {
			eventChan <- screen.PollEvent()
		}
	}()

	cpuChan := make(chan []float64, 1)
	go func() {
		for {
			p, err := cpu.Percent(0, true)
			if err == nil && len(p) == ncpu {
				cpuChan <- p
			}
			time.Sleep(aurPollRate)
		}
	}()

	drawTick := time.NewTicker(aurTickMs * time.Millisecond)
	defer drawTick.Stop()
	startTime := time.Now()

	for {
		select {
		case <-sigChan:
			return false

		case p := <-cpuChan:
			for i := range state.targets {
				state.targets[i] = p[i] / 100.0
			}

		case ev := <-eventChan:
			switch e := ev.(type) {
			case *tcell.EventResize:
				w, h = screen.Size()
				state.stars = aurMakeStars(rng, w, h)
				screen.Sync()
			case *tcell.EventKey:
				if e.Key() == tcell.KeyEscape || e.Key() == tcell.KeyCtrlC {
					return false
				}
				if e.Rune() == ' ' {
					return true
				}
				if !interactive {
					return false
				}
			}

		case <-drawTick.C:
			state.t = time.Since(startTime).Seconds()
			for i := range state.loads {
				state.loads[i] += (state.targets[i] - state.loads[i]) * aurLerp
			}
			aurDrawFrame(screen, w, h, state, grayscale)
			screen.Show()
		}
	}
}

func aurMakeStars(rng *rand.Rand, w, h int) []aurStar {
	n := (w * h) / 55
	stars := make([]aurStar, n)
	for i := range stars {
		stars[i] = aurStar{
			cx:    rng.Intn(w),
			cy:    rng.Intn(h),
			phase: rng.Float64() * math.Pi * 2,
		}
	}
	return stars
}

// aurColumnLoad computes the effective CPU load for a given column using
// Gaussian-weighted contributions from each core, so each core has a clear
// horizontal territory with soft boundaries.
func aurColumnLoad(fx float64, loads []float64) float64 {
	n := len(loads)
	sigma := 0.85 / float64(n)
	total, weight := 0.0, 0.0
	for i, l := range loads {
		cx := (float64(i) + 0.5) / float64(n)
		w := math.Exp(-math.Pow((fx-cx)/sigma, 2) * 2)
		total += w * l
		weight += w
	}
	if weight == 0 {
		return 0
	}
	return total / weight
}

// aurIntensity computes the aurora brightness at half-block pixel position (px, py)
// where py runs 0..h*2-1.
//
// Primary information signal: vertical extent of the curtain.
//   - Idle core  → short wisp, stays high in the sky.
//   - Busy core  → long curtain sweeping toward the horizon.
//
// Secondary: overall brightness also scales with load.
func aurIntensity(px, py, w, ph int, t float64, loads []float64) float64 {
	fx := float64(px) / float64(w)
	fy := float64(py) / float64(ph) // 0=top, 1=bottom

	load := aurColumnLoad(fx, loads)

	// Aurora top edge: fixed high in the sky.
	const topAnchor = 0.10
	// Bottom edge: sweeps down with load.
	// 0% load → curtain ends at 0.28; 100% load → curtain ends at 0.82.
	bottomEdge := 0.28 + load*0.54

	// Soft fade-in at top, fade-out at bottom.
	topFade := math.Min(1, (fy-topAnchor)/0.07)
	if topFade < 0 {
		topFade = 0
	}
	bottomFade := math.Min(1, (bottomEdge-fy)/0.09)
	if bottomFade < 0 {
		bottomFade = 0
	}
	vertEnv := topFade * bottomFade

	// Curtain shimmer: pretty but subordinate to the vertical extent signal.
	drift := math.Sin(float64(px)*0.09+t*0.38+math.Sin(t*0.19+float64(px)*0.04)*2.1)*0.5 + 0.5
	ripple := math.Sin(float64(px)*0.14+t*0.73) *
		math.Sin(float64(py)*0.28-t*0.52+math.Sin(t*0.41)*1.6)
	ripple = ripple*0.20 + 0.80 // keep ripple subtle so vertical extent reads clearly

	wave := drift*0.45 + ripple*0.55

	// Brightness floor keeps faint ambient wisps even at 0% load.
	brightness := 0.07 + load*0.93

	return math.Max(0, math.Min(1, brightness*vertEnv*wave))
}

// aurColor maps an intensity 0..1 to an aurora color (teal → green → cyan → violet).
func aurColor(v float64, grayscale bool) tcell.Color {
	if grayscale {
		lum := int32(v * 210)
		return tcell.NewRGBColor(lum, lum, lum)
	}

	type stop struct {
		pos     float64
		r, g, b int32
	}
	stops := []stop{
		{0.00, 2, 4, 14},     // deep night sky
		{0.06, 0, 14, 24},    // faint navy
		{0.15, 0, 42, 45},    // dim teal
		{0.26, 0, 95, 65},    // dark aurora green
		{0.38, 0, 175, 85},   // aurora green
		{0.50, 8, 230, 105},  // vivid green
		{0.62, 35, 240, 145}, // bright green-cyan
		{0.72, 85, 195, 215}, // cyan
		{0.82, 145, 105, 245},// blue-violet
		{0.91, 200, 58, 255}, // purple
		{1.00, 238, 158, 255},// lavender
	}
	for i := 1; i < len(stops); i++ {
		if v <= stops[i].pos {
			t := (v - stops[i-1].pos) / (stops[i].pos - stops[i-1].pos)
			lerp := func(a, b int32) int32 { return int32(float64(a) + t*float64(b-a)) }
			return tcell.NewRGBColor(
				lerp(stops[i-1].r, stops[i].r),
				lerp(stops[i-1].g, stops[i].g),
				lerp(stops[i-1].b, stops[i].b),
			)
		}
	}
	return tcell.NewRGBColor(238, 158, 255)
}

const aurLabelRows = 2 // rows reserved at the bottom for core labels

func aurDrawFrame(screen tcell.Screen, w, h int, state *aurState, grayscale bool) {
	skyH := h - aurLabelRows
	ph := skyH * 2 // half-block pixel height for the aurora region

	// Aurora pass: ▀ gives top-half=fg, bottom-half=bg for double vertical resolution.
	for cy := 0; cy < skyH; cy++ {
		for cx := 0; cx < w; cx++ {
			itop := aurIntensity(cx, cy*2, w, ph, state.t, state.loads)
			ibot := aurIntensity(cx, cy*2+1, w, ph, state.t, state.loads)
			screen.SetContent(cx, cy, '▀', nil,
				tcell.StyleDefault.
					Foreground(aurColor(itop, grayscale)).
					Background(aurColor(ibot, grayscale)))
		}
	}

	// Star pass: only in dark areas where the aurora doesn't wash them out.
	if !grayscale {
		for _, s := range state.stars {
			if s.cx >= w || s.cy >= skyH {
				continue
			}
			aur := aurIntensity(s.cx, s.cy*2, w, ph, state.t, state.loads)
			if aur > 0.22 {
				continue
			}
			brightness := 0.45 + 0.55*math.Sin(state.t*2.3+s.phase)
			lum := int32(brightness*160 + 55)
			sc := tcell.NewRGBColor(lum, lum, lum)
			screen.SetContent(s.cx, s.cy, '·', nil,
				tcell.StyleDefault.Foreground(sc).Background(aurColor(aur, grayscale)))
		}
	}

	aurDrawLabels(screen, w, h, state, grayscale)
}

func aurDrawLabels(screen tcell.Screen, w, h int, state *aurState, grayscale bool) {
	n := state.ncpu
	dark := tcell.NewRGBColor(6, 6, 18)
	sepRow := h - aurLabelRows
	pctRow := h - aurLabelRows + 1

	// Separator line between aurora and labels.
	sepStyle := tcell.StyleDefault.Foreground(tcell.NewRGBColor(30, 30, 55)).Background(dark)
	for cx := 0; cx < w; cx++ {
		screen.SetContent(cx, sepRow, '─', nil, sepStyle)
	}

	// Per-core label + percentage in the bottom row.
	for i := 0; i < n; i++ {
		x0 := i * w / n
		x1 := (i + 1) * w / n
		colW := x1 - x0

		// Fill background for this core's column.
		for cx := x0; cx < x1; cx++ {
			screen.SetContent(cx, pctRow, ' ', nil, tcell.StyleDefault.Background(dark))
		}

		if colW < 5 {
			continue
		}

		pct := int(math.Round(state.loads[i] * 100))

		var fg tcell.Color
		if grayscale {
			lum := int32(90 + state.loads[i]*100)
			fg = tcell.NewRGBColor(lum, lum, lum)
		} else {
			switch {
			case pct >= 75:
				fg = tcell.NewRGBColor(200, 70, 50)
			case pct >= 45:
				fg = tcell.NewRGBColor(190, 145, 45)
			case pct >= 15:
				fg = tcell.NewRGBColor(55, 160, 85)
			default:
				fg = tcell.NewRGBColor(60, 100, 140)
			}
		}

		style := tcell.StyleDefault.Foreground(fg).Background(dark)
		label := fmt.Sprintf("C%-2d %3d%%", i, pct)
		aurDrawCentered(screen, x0, pctRow, colW, label, style)
	}

	// Aggregate average in the top-right corner.
	avg := 0.0
	for _, l := range state.loads {
		avg += l
	}
	avg /= float64(n)
	avgPct := int(math.Round(avg * 100))
	avgLabel := fmt.Sprintf(" avg %d%% ", avgPct)
	avgFg := tcell.NewRGBColor(100, 120, 150)
	if grayscale {
		avgFg = tcell.NewRGBColor(110, 110, 110)
	}
	avgStyle := tcell.StyleDefault.Foreground(avgFg).Background(dark)
	runes := []rune(avgLabel)
	startX := w - len(runes)
	if startX < 0 {
		startX = 0
	}
	for i, r := range runes {
		if startX+i < w {
			screen.SetContent(startX+i, sepRow, r, nil, avgStyle)
		}
	}
}

func aurDrawCentered(screen tcell.Screen, x0, y, colW int, s string, style tcell.Style) {
	runes := []rune(s)
	startX := x0 + (colW-len(runes))/2
	for i, r := range runes {
		cx := startX + i
		if cx >= x0 && cx < x0+colW {
			screen.SetContent(cx, y, r, nil, style)
		}
	}
}
