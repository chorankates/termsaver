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
	hmClockTick  = 100
	hmPollRate   = 1 * time.Second
	hmLerpFactor = 0.08
	hmMaxProcs   = 48
	// Terminal cells are visually ~2× taller than wide; correct for aspect ratio
	hmCharAspect = 2.1
)

type hmProcSnap struct {
	pid      int
	name     string
	cpuTicks uint64
	memKB    int64
	ioBytes  uint64
}

type hmEntry struct {
	name       string
	memKB      int64
	heat       float64 // animated [0,1]: driven by CPU or disk IO, whichever is hotter
	heatTarget float64
}

type tmRect struct {
	x, y, w, h int
	e          *hmEntry
}

// ── Sampling ──────────────────────────────────────────────────────────────────

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

	var ioBytes uint64
	if f, err := os.Open(fmt.Sprintf("/proc/%d/io", pid)); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "read_bytes:") || strings.HasPrefix(line, "write_bytes:") {
				if parts := strings.Fields(line); len(parts) >= 2 {
					v, _ := strconv.ParseUint(parts[1], 10, 64)
					ioBytes += v
				}
			}
		}
		f.Close()
	}

	return hmProcSnap{pid, strings.TrimSpace(string(nameData)), utime + stime, memKB, ioBytes}, true
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

func hmHeatColor(v float64, grayscale bool) tcell.Color {
	v = math.Max(0, math.Min(1, v))
	if grayscale {
		lum := int32(v * 230)
		return tcell.NewRGBColor(lum, lum, lum)
	}
	type stop struct{ pos float64; r, g, b int32 }
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

// ── Treemap layout — recursive binary split ───────────────────────────────────

// worstAspect returns the worst visual aspect ratio for a w×h terminal rect.
func worstAspect(w, h int) float64 {
	if w == 0 || h == 0 {
		return math.MaxFloat64
	}
	pw := float64(w) * hmCharAspect
	ph := float64(h)
	return math.Max(pw/ph, ph/pw)
}

// layoutRecursive splits entries into a region using binary space partitioning.
// At each level it tries both a vertical and horizontal cut and picks whichever
// gives the better worst-case aspect ratio, so cells vary in both x and y.
func layoutRecursive(entries []hmEntry, x, y, w, h int, out *[]tmRect) {
	if len(entries) == 0 || w <= 0 || h <= 0 {
		return
	}
	if len(entries) == 1 {
		*out = append(*out, tmRect{x, y, w, h, &entries[0]})
		return
	}

	var total int64
	for i := range entries {
		total += entries[i].memKB
	}
	if total == 0 {
		return
	}

	// Find the split index so the left group's area is as close to half as possible
	half := total / 2
	var cum int64
	pivot := 1
	for i := range entries {
		cum += entries[i].memKB
		if cum >= half {
			pivot = i + 1
			break
		}
	}
	if pivot >= len(entries) {
		pivot = len(entries) - 1
	}

	var leftMem int64
	for i := 0; i < pivot; i++ {
		leftMem += entries[i].memKB
	}
	rightMem := total - leftMem

	// Option A: vertical cut (left | right)
	leftW := int(math.Round(float64(w) * float64(leftMem) / float64(total)))
	if leftW < 1 {
		leftW = 1
	}
	if leftW >= w {
		leftW = w - 1
	}
	aspectV := math.Max(worstAspect(leftW, h), worstAspect(w-leftW, h))

	// Option B: horizontal cut (top / bottom)
	topH := int(math.Round(float64(h) * float64(leftMem) / float64(total)))
	if topH < 1 {
		topH = 1
	}
	if topH >= h {
		topH = h - 1
	}
	aspectH := math.Max(worstAspect(w, topH), worstAspect(w, h-topH))

	// Tiebreak toward the cut that makes the larger group more square
	_ = rightMem
	if aspectV <= aspectH {
		layoutRecursive(entries[:pivot], x, y, leftW, h, out)
		layoutRecursive(entries[pivot:], x+leftW, y, w-leftW, h, out)
	} else {
		layoutRecursive(entries[:pivot], x, y, w, topH, out)
		layoutRecursive(entries[pivot:], x, y+topH, w, h-topH, out)
	}
}

// ── Rendering ─────────────────────────────────────────────────────────────────

func hmRenderRect(screen tcell.Screen, r tmRect, elapsed float64, grayscale bool) {
	heat := r.e.heat
	phase := (float64(r.x)*1.618 + float64(r.y)*0.927) * 0.5

	pulseMag := heat * heat * 0.10
	pulseFreq := 0.6 + heat*3.0
	pulsed := heat + pulseMag*math.Sin(elapsed*pulseFreq*math.Pi*2+phase)
	pulsed = math.Max(0, math.Min(1, pulsed))

	// Fill using ▀ half-blocks so each character row has a top and bottom colour,
	// creating a smooth vertical gradient with twice the effective resolution.
	for row := r.y; row < r.y+r.h; row++ {
		rowFrac := 0.0
		if r.h > 1 {
			rowFrac = float64(row-r.y) / float64(r.h-1)
		}
		glow := (1.0 - math.Abs(rowFrac-0.3)*1.4) * 0.14 * pulsed
		topColor := hmHeatColor(math.Min(1, pulsed+glow), grayscale)
		botColor := hmHeatColor(math.Max(0, pulsed+glow*0.35), grayscale)
		style := tcell.StyleDefault.Foreground(topColor).Background(botColor)
		for col := r.x; col < r.x+r.w; col++ {
			screen.SetContent(col, row, '▀', nil, style)
		}
	}

	// Drifting sparkles on hot cells
	if heat > 0.70 && r.w >= 3 && r.h >= 2 {
		sparks := []rune{'·', '∙', '•', '+', '✦'}
		for s := 0; s < int(heat*4)+1; s++ {
			sf := float64(s)
			sx := r.x + 1 + int((math.Sin(elapsed*4.1+sf*1.9+phase)+1)/2*math.Max(1, float64(r.w-2)))
			sy := r.y + int((math.Cos(elapsed*3.3+sf*2.7+phase)+1)/2*math.Max(1, float64(r.h-1)))
			if sx >= r.x && sx < r.x+r.w && sy >= r.y && sy < r.y+r.h {
				bright := 0.5 + 0.5*math.Sin(elapsed*11+sf*3.1+phase)
				var sparkFg tcell.Color
				if grayscale {
					v := int32(180 + bright*75)
					sparkFg = tcell.NewRGBColor(v, v, v)
				} else {
					sparkFg = tcell.NewRGBColor(int32(215+bright*40), int32(170+bright*85), int32(bright*55))
				}
				ch := sparks[int(elapsed*7+sf*1.3+phase)%len(sparks)]
				screen.SetContent(sx, sy, ch, nil,
					tcell.StyleDefault.Foreground(sparkFg).Background(hmHeatColor(math.Min(1, pulsed+0.05), grayscale)))
			}
		}
	}

	// Name only on cells that have enough room
	if r.w >= 9 && r.h >= 3 {
		name := r.e.name
		if runes := []rune(name); len(runes) > r.w-2 {
			name = string(runes[:r.w-2])
		}
		runes := []rune(name)
		lx := r.x + (r.w-len(runes))/2
		ly := r.y + r.h/2

		var fg tcell.Color
		if grayscale {
			if heat > 0.55 {
				fg = tcell.NewRGBColor(20, 20, 20)
			} else {
				fg = tcell.NewRGBColor(160, 160, 160)
			}
		} else {
			if heat > 0.58 {
				fg = tcell.NewRGBColor(20, 15, 5)
			} else {
				fg = tcell.NewRGBColor(160, 175, 210)
			}
		}
		bg := hmHeatColor(pulsed, grayscale)
		style := tcell.StyleDefault.Foreground(fg).Background(bg)
		for i, ch := range runes {
			if lx+i >= r.x && lx+i < r.x+r.w {
				screen.SetContent(lx+i, ly, ch, nil, style)
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
	startTime := time.Now()

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
					ioRate  float64
					memKB   int64
				}
				rawList := make([]raw, 0, len(snap))
				for pid, curr := range snap {
					p, ok := prev[pid]
					if !ok {
						continue
					}
					var cpuDelta, ioDelta float64
					if curr.cpuTicks >= p.cpuTicks {
						cpuDelta = float64(curr.cpuTicks-p.cpuTicks) / elapsed
					}
					if curr.ioBytes >= p.ioBytes {
						ioDelta = float64(curr.ioBytes-p.ioBytes) / elapsed
					}
					rawList = append(rawList, raw{curr.name, cpuDelta, ioDelta, curr.memKB})
				}

				sort.Slice(rawList, func(i, j int) bool {
					return rawList[i].memKB > rawList[j].memKB
				})
				if len(rawList) > hmMaxProcs {
					rawList = rawList[:hmMaxProcs]
				}

				var maxCPU, maxIO float64
				for _, r := range rawList {
					if r.cpuRate > maxCPU {
						maxCPU = r.cpuRate
					}
					if r.ioRate > maxIO {
						maxIO = r.ioRate
					}
				}
				if maxCPU < 1 {
					maxCPU = 1
				}
				if maxIO < 1 {
					maxIO = 1
				}

				newEntries := make([]hmEntry, len(rawList))
				for i, r := range rawList {
					// Logical OR of intensities: high CPU *or* high disk IO heats the cell
					cpuNorm := r.cpuRate / maxCPU
					ioNorm := r.ioRate / maxIO
					target := 1 - (1-cpuNorm)*(1-ioNorm)
					current, seen := heatByName[r.name]
					if !seen {
						current = target * 0.5
					}
					newEntries[i] = hmEntry{r.name, r.memKB, current, target}
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
			elapsed := time.Since(startTime).Seconds()

			for i := range entries {
				entries[i].heat += (entries[i].heatTarget - entries[i].heat) * hmLerpFactor
				heatByName[entries[i].name] = entries[i].heat
			}

			if len(entries) == 0 {
				breathe := 0.04 + 0.02*math.Sin(elapsed*0.8)
				bg := hmHeatColor(breathe, grayscale)
				style := tcell.StyleDefault.Foreground(bg).Background(tcell.NewRGBColor(0, 0, 0))
				for y := 0; y < h; y++ {
					for x := 0; x < w; x++ {
						screen.SetContent(x, y, '▀', nil, style)
					}
				}
			} else {
				rects := make([]tmRect, 0, len(entries))
				layoutRecursive(entries, 0, 0, w, h, &rects)
				for _, r := range rects {
					hmRenderRect(screen, r, elapsed, grayscale)
				}
			}

			screen.Show()
		}
	}
}
