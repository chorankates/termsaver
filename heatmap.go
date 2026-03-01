package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

const (
	hmClockTick  = 100             // Linux CLK_TCK (jiffies/sec)
	hmPollRate   = 1 * time.Second // process stat sample interval
	hmLerpFactor = 0.10            // heat animation smoothing
	hmMaxProcs   = 48              // max processes in the treemap
)

// ── Data types ────────────────────────────────────────────────────────────────

type hmProcSnap struct {
	pid      int
	name     string
	cpuTicks uint64
	memKB    int64
}

type hmEntry struct {
	name      string
	memKB     int64
	cpuRaw    float64 // ticks/sec (for label text)
	cpuHeat   float64 // current animated value [0,1]
	cpuTarget float64 // target [0,1]
}

type tmRect struct {
	x, y, w, h int
	e          *hmEntry
}

// ── Process sampling ──────────────────────────────────────────────────────────

func hmReadSnap(pid int) (hmProcSnap, bool) {
	nameData, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return hmProcSnap{}, false
	}
	statData, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return hmProcSnap{}, false
	}
	s := string(statData)
	end := strings.LastIndex(s, ")")
	if end < 0 {
		return hmProcSnap{}, false
	}
	fields := strings.Fields(s[end+2:])
	if len(fields) < 13 {
		return hmProcSnap{}, false
	}
	utime, _ := strconv.ParseUint(fields[11], 10, 64)
	stime, _ := strconv.ParseUint(fields[12], 10, 64)

	var memKB int64
	if f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid)); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if line := sc.Text(); strings.HasPrefix(line, "VmRSS:") {
				if parts := strings.Fields(line); len(parts) >= 2 {
					memKB, _ = strconv.ParseInt(parts[1], 10, 64)
				}
				break
			}
		}
		f.Close()
	}
	return hmProcSnap{pid, strings.TrimSpace(string(nameData)), utime + stime, memKB}, true
}

func hmSample() map[int]hmProcSnap {
	matches, _ := filepath.Glob("/proc/[0-9]*/comm")
	out := make(map[int]hmProcSnap, len(matches))
	for _, m := range matches {
		parts := strings.Split(m, "/")
		if len(parts) >= 3 {
			if pid, err := strconv.Atoi(parts[2]); err == nil {
				if snap, ok := hmReadSnap(pid); ok {
					out[pid] = snap
				}
			}
		}
	}
	return out
}

// ── Color ─────────────────────────────────────────────────────────────────────

// hmHeatColor maps [0,1] → cool-to-hot gradient: navy → blue → cyan → green → yellow → orange → red
func hmHeatColor(v float64, grayscale bool) tcell.Color {
	v = math.Max(0, math.Min(1, v))
	if grayscale {
		lum := int32(v * 230)
		return tcell.NewRGBColor(lum, lum, lum)
	}
	type stop struct {
		pos     float64
		r, g, b int32
	}
	stops := []stop{
		{0.00, 4, 4, 22},
		{0.15, 0, 0, 185},
		{0.30, 0, 115, 255},
		{0.45, 0, 210, 200},
		{0.55, 0, 200, 55},
		{0.65, 175, 215, 0},
		{0.75, 255, 200, 0},
		{0.85, 255, 95, 0},
		{0.95, 255, 18, 0},
		{1.00, 255, 80, 80},
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
	return tcell.NewRGBColor(255, 80, 80)
}

func hmFgColor(heat float64, grayscale bool) tcell.Color {
	if grayscale {
		if heat > 0.55 {
			return tcell.NewRGBColor(0, 0, 0)
		}
		return tcell.NewRGBColor(255, 255, 255)
	}
	if heat > 0.58 {
		return tcell.NewRGBColor(10, 10, 10)
	}
	return tcell.NewRGBColor(235, 240, 255)
}

// ── Treemap layout ────────────────────────────────────────────────────────────

// layoutTreemap arranges entries (sorted by memKB desc) into a squarified strip treemap.
// charAspect corrects for terminal cells being ~2× taller than wide visually.
func layoutTreemap(entries []hmEntry, W, H int) []tmRect {
	var totalMem int64
	for i := range entries {
		totalMem += entries[i].memKB
	}
	if totalMem == 0 || W == 0 || H == 0 {
		return nil
	}

	const charAspect = 2.1 // visual pixel width : pixel height per cell

	var rects []tmRect
	y, i := 0, 0

	for i < len(entries) && y < H {
		remainingH := H - y

		// remaining memory for processes not yet placed
		var remainingMem int64
		for j := i; j < len(entries); j++ {
			remainingMem += entries[j].memKB
		}

		// Squarification: find how many items minimize the worst aspect ratio in this strip
		bestN := 1
		bestWorst := math.MaxFloat64

		for n := 1; n <= len(entries)-i; n++ {
			var stripMem int64
			for j := 0; j < n; j++ {
				stripMem += entries[i+j].memKB
			}
			stripH := float64(remainingH) * float64(stripMem) / float64(remainingMem)

			worst := 0.0
			for j := 0; j < n; j++ {
				cellW := float64(W) * float64(entries[i+j].memKB) / float64(stripMem)
				pixW := cellW * charAspect
				pixH := stripH
				var ar float64
				if pixW > 0 && pixH > 0 {
					ar = math.Max(pixW/pixH, pixH/pixW)
				} else {
					ar = math.MaxFloat64
				}
				if ar > worst {
					worst = ar
				}
			}
			if n == 1 || worst <= bestWorst {
				bestWorst = worst
				bestN = n
			} else {
				break
			}
		}

		var stripMem int64
		for j := 0; j < bestN; j++ {
			stripMem += entries[i+j].memKB
		}
		stripH := int(math.Round(float64(remainingH) * float64(stripMem) / float64(remainingMem)))
		if stripH < 1 {
			stripH = 1
		}
		if y+stripH > H {
			stripH = H - y
		}

		x := 0
		for j := 0; j < bestN; j++ {
			var cellW int
			if j == bestN-1 {
				cellW = W - x
			} else {
				cellW = int(math.Round(float64(W) * float64(entries[i+j].memKB) / float64(stripMem)))
				if cellW < 1 {
					cellW = 1
				}
			}
			if x+cellW > W {
				cellW = W - x
			}
			if cellW > 0 && stripH > 0 {
				rects = append(rects, tmRect{x, y, cellW, stripH, &entries[i+j]})
			}
			x += cellW
		}

		y += stripH
		i += bestN
	}
	return rects
}

// ── Rendering ─────────────────────────────────────────────────────────────────

func hmFmtMem(kb int64) string {
	switch {
	case kb >= 1024*1024:
		return fmt.Sprintf("%.1fGB", float64(kb)/1048576)
	case kb >= 1024:
		return fmt.Sprintf("%.0fMB", float64(kb)/1024)
	default:
		return fmt.Sprintf("%dKB", kb)
	}
}

func hmRenderRect(screen tcell.Screen, r tmRect, grayscale bool) {
	bg := hmHeatColor(r.e.cpuHeat, grayscale)
	fg := hmFgColor(r.e.cpuHeat, grayscale)
	fill := tcell.StyleDefault.Background(bg).Foreground(fg)

	// Fill background
	for row := r.y; row < r.y+r.h; row++ {
		for col := r.x; col < r.x+r.w; col++ {
			screen.SetContent(col, row, ' ', nil, fill)
		}
	}

	// Thin dark border on left and top edges (shared borders save space)
	borderCol := tcell.NewRGBColor(0, 0, 0)
	if grayscale {
		borderCol = tcell.NewRGBColor(8, 8, 8)
	}
	border := tcell.StyleDefault.Background(borderCol)
	if r.x > 0 {
		for row := r.y; row < r.y+r.h; row++ {
			screen.SetContent(r.x, row, ' ', nil, border)
		}
	}
	if r.y > 0 {
		for col := r.x; col < r.x+r.w; col++ {
			screen.SetContent(col, r.y, ' ', nil, border)
		}
	}

	// Interior dimensions (inside the 1-char border where applicable)
	ix := r.x
	iy := r.y
	iw := r.w
	ih := r.h
	if r.x > 0 {
		ix++
		iw--
	}
	if r.y > 0 {
		iy++
		ih--
	}
	if iw < 1 || ih < 1 {
		return
	}

	// Render text labels centered in the interior
	var lines []string
	name := r.e.name
	maxLabelW := iw - 2
	if maxLabelW < 1 {
		return
	}
	if len([]rune(name)) > maxLabelW {
		runes := []rune(name)
		if maxLabelW > 1 {
			name = string(runes[:maxLabelW-1]) + "…"
		} else {
			name = string(runes[:maxLabelW])
		}
	}
	lines = append(lines, name)
	if ih >= 2 && iw >= 7 {
		lines = append(lines, hmFmtMem(r.e.memKB))
	}
	if ih >= 3 && iw >= 9 {
		cpuPct := r.e.cpuRaw / float64(hmClockTick) * 100
		lines = append(lines, fmt.Sprintf("cpu %.1f%%", cpuPct))
	}

	startY := iy + (ih-len(lines))/2
	for li, line := range lines {
		ly := startY + li
		if ly < iy || ly >= iy+ih {
			continue
		}
		runes := []rune(line)
		startX := ix + (iw-len(runes))/2
		for ci, ch := range runes {
			lx := startX + ci
			if lx >= ix && lx < ix+iw {
				screen.SetContent(lx, ly, ch, nil, fill)
			}
		}
	}
}

// ── Main loop ─────────────────────────────────────────────────────────────────

func runHeatmap(screen tcell.Screen, sigChan chan os.Signal, interactive bool, grayscale bool) bool {
	w, h := screen.Size()

	eventChan := make(chan tcell.Event, 10)
	go func() {
		for {
			eventChan <- screen.PollEvent()
		}
	}()

	statsChan := make(chan map[int]hmProcSnap, 1)
	go func() {
		for {
			statsChan <- hmSample()
			time.Sleep(hmPollRate)
		}
	}()

	drawTick := time.NewTicker(50 * time.Millisecond)
	defer drawTick.Stop()

	var prev map[int]hmProcSnap
	var entries []hmEntry
	var lastSample time.Time
	heatByName := make(map[string]float64)

	for {
		select {
		case <-sigChan:
			return false

		case snap := <-statsChan:
			now := time.Now()
			elapsed := now.Sub(lastSample).Seconds()
			if elapsed <= 0 {
				elapsed = 1
			}

			if prev != nil {
				type raw struct {
					name    string
					cpuRate float64
					memKB   int64
				}
				rawList := make([]raw, 0, len(snap))
				for pid, curr := range snap {
					p, ok := prev[pid]
					if !ok {
						continue
					}
					var cpuDelta float64
					if curr.cpuTicks >= p.cpuTicks {
						cpuDelta = float64(curr.cpuTicks-p.cpuTicks) / elapsed
					}
					rawList = append(rawList, raw{curr.name, cpuDelta, curr.memKB})
				}

				// Sort by memory descending, limit count
				sort.Slice(rawList, func(i, j int) bool {
					return rawList[i].memKB > rawList[j].memKB
				})
				if len(rawList) > hmMaxProcs {
					rawList = rawList[:hmMaxProcs]
				}

				var maxCPU float64
				for _, r := range rawList {
					if r.cpuRate > maxCPU {
						maxCPU = r.cpuRate
					}
				}
				if maxCPU < 1 {
					maxCPU = 1
				}

				newEntries := make([]hmEntry, len(rawList))
				for i, r := range rawList {
					target := r.cpuRate / maxCPU
					current, seen := heatByName[r.name]
					if !seen {
						current = target * 0.5
					}
					newEntries[i] = hmEntry{
						name:      r.name,
						memKB:     r.memKB,
						cpuRaw:    r.cpuRate,
						cpuHeat:   current,
						cpuTarget: target,
					}
				}
				entries = newEntries
			}

			prev = snap
			lastSample = now

		case event := <-eventChan:
			switch ev := event.(type) {
			case *tcell.EventResize:
				w, h = screen.Size()
				screen.Sync()
			case *tcell.EventKey:
				if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
					return false
				}
				if ev.Rune() == ' ' {
					return true
				}
				if !interactive {
					return false
				}
			}

		case <-drawTick.C:
			// Animate heat values
			for i := range entries {
				entries[i].cpuHeat += (entries[i].cpuTarget - entries[i].cpuHeat) * hmLerpFactor
				heatByName[entries[i].name] = entries[i].cpuHeat
			}

			screen.Clear()

			// Treemap fills all but the legend row
			treemapH := h - 1

			if len(entries) == 0 {
				bgStyle := tcell.StyleDefault.Background(tcell.NewRGBColor(4, 4, 18))
				for y := 0; y < treemapH; y++ {
					for x := 0; x < w; x++ {
						screen.SetContent(x, y, ' ', nil, bgStyle)
					}
				}
				msgStyle := bgStyle.Foreground(tcell.NewRGBColor(70, 90, 170))
				msg := "Sampling processes…"
				mx := (w - len(msg)) / 2
				if mx < 0 {
					mx = 0
				}
				for i, ch := range msg {
					screen.SetContent(mx+i, treemapH/2, ch, nil, msgStyle)
				}
			} else {
				rects := layoutTreemap(entries, w, treemapH)
				for _, r := range rects {
					hmRenderRect(screen, r, grayscale)
				}
			}

			// Legend bar (bottom row)
			legendBg := tcell.NewRGBColor(0, 0, 0)
			if grayscale {
				legendBg = tcell.NewRGBColor(6, 6, 6)
			}
			legendFg := tcell.NewRGBColor(130, 148, 215)
			if grayscale {
				legendFg = tcell.NewRGBColor(155, 155, 155)
			}
			legStyle := tcell.StyleDefault.Background(legendBg).Foreground(legendFg)
			for x := 0; x < w; x++ {
				screen.SetContent(x, h-1, ' ', nil, legStyle)
			}
			lx := 1
			label := "size=MEM  color=CPU  "
			for _, ch := range label {
				if lx < w {
					screen.SetContent(lx, h-1, ch, nil, legStyle)
					lx++
				}
			}
			gradW := w / 5
			if gradW > 40 {
				gradW = 40
			}
			if gradW < 6 {
				gradW = 6
			}
			for i := 0; i < gradW && lx+i < w; i++ {
				hv := float64(i) / float64(gradW-1)
				col := hmHeatColor(hv, grayscale)
				screen.SetContent(lx+i, h-1, '█', nil,
					tcell.StyleDefault.Foreground(col).Background(legendBg))
			}
			lx += gradW + 1
			suffix := " low→high"
			for _, ch := range suffix {
				if lx < w {
					screen.SetContent(lx, h-1, ch, nil, legStyle)
					lx++
				}
			}

			screen.Show()
		}
	}
}
