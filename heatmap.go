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
	// Each terminal char is visually ~2× taller than wide.
	// Visual pixels: width = w*1, height = h*hmCharAspect.
	hmCharAspect = 2.0
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
	heat       float64
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

// ── Treemap — recursive binary split ─────────────────────────────────────────

// hmAspect returns the visual aspect ratio of a w×h character rectangle.
// Visual pixels: w wide, h*hmCharAspect tall. Perfect square = 1.0.
func hmAspect(w, h int) float64 {
	if w == 0 || h == 0 {
		return math.MaxFloat64
	}
	vw := float64(w)
	vh := float64(h) * hmCharAspect
	return math.Max(vw/vh, vh/vw)
}

// layoutRecursive splits the region choosing whichever cut direction (vertical
// or horizontal) gives the most square children, so cells grow in both x and y.
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

	// Find the split closest to 50/50 by area
	half := total / 2
	pivot := 1
	var cum int64
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
	frac := float64(leftMem) / float64(total)

	// Vertical cut (left | right)
	lw := int(math.Round(float64(w) * frac))
	lw = clampInt(lw, 1, w-1)
	worstV := math.Max(hmAspect(lw, h), hmAspect(w-lw, h))

	// Horizontal cut (top / bottom)
	th := int(math.Round(float64(h) * frac))
	th = clampInt(th, 1, h-1)
	worstH := math.Max(hmAspect(w, th), hmAspect(w, h-th))

	if worstV <= worstH {
		layoutRecursive(entries[:pivot], x, y, lw, h, out)
		layoutRecursive(entries[pivot:], x+lw, y, w-lw, h, out)
	} else {
		layoutRecursive(entries[:pivot], x, y, w, th, out)
		layoutRecursive(entries[pivot:], x, y+th, w, h-th, out)
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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

	rawHeat := make([]float64, w*h)
	bloomedHeat := make([]float64, w*h)

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
				sort.Slice(rawList, func(i, j int) bool { return rawList[i].memKB > rawList[j].memKB })
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
					cpuN := r.cpuRate / maxCPU
					ioN := r.ioRate / maxIO
					target := 1 - (1-cpuN)*(1-ioN)
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
				rawHeat = make([]float64, w*h)
				bloomedHeat = make([]float64, w*h)
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

			// ── Layout ───────────────────────────────────────────────────────
			rects := make([]tmRect, 0, len(entries))
			if len(entries) > 0 {
				layoutRecursive(entries, 0, 0, w, h, &rects)
			}

			// ── Raw heat grid ────────────────────────────────────────────────
			for i := range rawHeat {
				rawHeat[i] = 0.04 // dim baseline when idle
			}
			for _, r := range rects {
				phase := (float64(r.x)*1.618 + float64(r.y)*0.927) * 0.5
				pulsed := r.e.heat + r.e.heat*r.e.heat*0.08*
					math.Sin(elapsed*(0.7+r.e.heat*2.8)*math.Pi*2+phase)
				pulsed = math.Max(0, math.Min(1, pulsed))
				for row := r.y; row < r.y+r.h; row++ {
					for col := r.x; col < r.x+r.w; col++ {
						rawHeat[row*w+col] = pulsed
					}
				}
			}

			// ── Bloom: hot cells glow into neighbours ────────────────────────
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					base := rawHeat[y*w+x]
					glow := 0.0
					for dy := -2; dy <= 2; dy++ {
						for dx := -2; dx <= 2; dx++ {
							if dy == 0 && dx == 0 {
								continue
							}
							ny, nx := y+dy, x+dx
							if ny < 0 || ny >= h || nx < 0 || nx >= w {
								continue
							}
							n := rawHeat[ny*w+nx]
							if n > base+0.01 {
								// distance in visual pixels
								vdx := float64(dx)
								vdy := float64(dy) * hmCharAspect
								dist := math.Sqrt(vdx*vdx + vdy*vdy)
								g := (n - base) * 0.5 / (1.0 + dist)
								if g > glow {
									glow = g
								}
							}
						}
					}
					bloomedHeat[y*w+x] = math.Min(1, base+glow)
				}
			}

			// ── Render ───────────────────────────────────────────────────────
			// ▀ splits each char cell into a top and bottom visual half-pixel,
			// doubling effective vertical resolution for the colour gradient.
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					bh := bloomedHeat[y*w+x]
					top := hmHeatColor(math.Min(1, bh+0.04), grayscale)
					bot := hmHeatColor(math.Max(0, bh-0.04), grayscale)
					screen.SetContent(x, y, '▀', nil,
						tcell.StyleDefault.Foreground(top).Background(bot))
				}
			}

			// ── Labels ───────────────────────────────────────────────────────
			for _, r := range rects {
				if r.w < 8 || r.h < 2 {
					continue
				}
				name := r.e.name
				if runes := []rune(name); len(runes) > r.w-2 {
					name = string(runes[:r.w-2])
				}
				runes := []rune(name)
				lx := r.x + (r.w-len(runes))/2
				ly := r.y + r.h/2
				heat := r.e.heat
				var fg tcell.Color
				if heat > 0.58 {
					fg = tcell.NewRGBColor(15, 10, 5)
				} else {
					fg = tcell.NewRGBColor(200, 210, 240)
				}
				if grayscale {
					if heat > 0.55 {
						fg = tcell.NewRGBColor(10, 10, 10)
					} else {
						fg = tcell.NewRGBColor(200, 200, 200)
					}
				}
				bg := hmHeatColor(bloomedHeat[ly*w+lx], grayscale)
				style := tcell.StyleDefault.Foreground(fg).Background(bg)
				for i, ch := range runes {
					if lx+i >= r.x && lx+i < r.x+r.w {
						screen.SetContent(lx+i, ly, ch, nil, style)
					}
				}
			}

			screen.Show()
		}
	}
}
