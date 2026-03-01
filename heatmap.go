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
	hmClockTick  = 100             // Linux CLK_TCK (jiffies per second)
	hmPollRate   = 1 * time.Second // How often to sample process stats
	hmLerpFactor = 0.12            // Heat animation smoothing (lower = smoother)
)

type hmProc struct {
	pid      int
	name     string
	cpuTicks uint64
	memKB    int64
	ioBytes  uint64
}

type hmRow struct {
	name      string
	cpuRaw    float64 // delta ticks/sec
	memKB     int64
	diskBps   float64 // delta bytes/sec
	cpuHeat   float64 // current animated value [0,1]
	memHeat   float64
	diskHeat  float64
	cpuTarget float64 // target value [0,1]
	memTarget float64
	diskTarget float64
}

func hmGetPIDs() []int {
	matches, _ := filepath.Glob("/proc/[0-9]*/comm")
	pids := make([]int, 0, len(matches))
	for _, m := range matches {
		parts := strings.Split(m, "/")
		if len(parts) >= 3 {
			if pid, err := strconv.Atoi(parts[2]); err == nil {
				pids = append(pids, pid)
			}
		}
	}
	return pids
}

func hmReadProc(pid int) (hmProc, bool) {
	nameData, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return hmProc{}, false
	}

	statData, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return hmProc{}, false
	}
	s := string(statData)
	end := strings.LastIndex(s, ")")
	if end < 0 {
		return hmProc{}, false
	}
	fields := strings.Fields(s[end+2:])
	if len(fields) < 13 {
		return hmProc{}, false
	}
	utime, _ := strconv.ParseUint(fields[11], 10, 64)
	stime, _ := strconv.ParseUint(fields[12], 10, 64)

	var memKB int64
	if sf, err := os.Open(fmt.Sprintf("/proc/%d/status", pid)); err == nil {
		sc := bufio.NewScanner(sf)
		for sc.Scan() {
			if line := sc.Text(); strings.HasPrefix(line, "VmRSS:") {
				if parts := strings.Fields(line); len(parts) >= 2 {
					memKB, _ = strconv.ParseInt(parts[1], 10, 64)
				}
				break
			}
		}
		sf.Close()
	}

	// /proc/[pid]/io requires matching UID or root; fails silently for others
	var ioBytes uint64
	if iof, err := os.Open(fmt.Sprintf("/proc/%d/io", pid)); err == nil {
		sc := bufio.NewScanner(iof)
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "read_bytes:") || strings.HasPrefix(line, "write_bytes:") {
				if parts := strings.Fields(line); len(parts) >= 2 {
					v, _ := strconv.ParseUint(parts[1], 10, 64)
					ioBytes += v
				}
			}
		}
		iof.Close()
	}

	return hmProc{
		pid:      pid,
		name:     strings.TrimSpace(string(nameData)),
		cpuTicks: utime + stime,
		memKB:    memKB,
		ioBytes:  ioBytes,
	}, true
}

func hmSampleAll() map[int]hmProc {
	pids := hmGetPIDs()
	result := make(map[int]hmProc, len(pids))
	for _, pid := range pids {
		if p, ok := hmReadProc(pid); ok {
			result[pid] = p
		}
	}
	return result
}

// hmHeatColor maps 0–1 to a cool-to-hot gradient (navy → blue → cyan → green → yellow → orange → red)
func hmHeatColor(v float64, grayscale bool) tcell.Color {
	v = math.Max(0, math.Min(1, v))
	if grayscale {
		lum := int32(v * 230)
		return tcell.NewRGBColor(lum, lum, lum)
	}
	type stop struct{ pos float64; r, g, b int32 }
	stops := []stop{
		{0.00, 4, 4, 22},
		{0.15, 0, 0, 180},
		{0.30, 0, 110, 255},
		{0.45, 0, 210, 200},
		{0.55, 0, 200, 60},
		{0.65, 170, 215, 0},
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

func hmTextColor(heat float64, grayscale bool) tcell.Color {
	if grayscale {
		if heat > 0.55 {
			return tcell.NewRGBColor(0, 0, 0)
		}
		return tcell.NewRGBColor(255, 255, 255)
	}
	// Background turns bright around heat=0.6 (yellow-green); switch to dark text then
	if heat > 0.58 {
		return tcell.NewRGBColor(15, 15, 15)
	}
	return tcell.NewRGBColor(235, 240, 255)
}

// hmFillCell draws a heat-colored cell with centered text
func hmFillCell(screen tcell.Screen, x, y, w int, text string, heat float64, grayscale bool) {
	bg := hmHeatColor(heat, grayscale)
	fg := hmTextColor(heat, grayscale)
	style := tcell.StyleDefault.Background(bg).Foreground(fg)
	for i := 0; i < w; i++ {
		screen.SetContent(x+i, y, ' ', nil, style)
	}
	runes := []rune(text)
	if len(runes) > w {
		runes = runes[:w]
	}
	startX := x + (w-len(runes))/2
	for i, ch := range runes {
		if startX+i >= x && startX+i < x+w {
			screen.SetContent(startX+i, y, ch, nil, style)
		}
	}
}

func hmDrawText(screen tcell.Screen, x, y int, text string, style tcell.Style) {
	for i, ch := range text {
		screen.SetContent(x+i, y, ch, nil, style)
	}
}

func hmCenterPad(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	pad := (width - len(s)) / 2
	out := strings.Repeat(" ", pad) + s
	for len(out) < width {
		out += " "
	}
	return out
}

func hmFmtMem(kb int64) string {
	switch {
	case kb >= 1024*1024:
		return fmt.Sprintf("%.1fGB", float64(kb)/1048576)
	case kb >= 1024:
		return fmt.Sprintf("%.1fMB", float64(kb)/1024)
	default:
		return fmt.Sprintf("%dKB", kb)
	}
}

func hmFmtDisk(bps float64) string {
	switch {
	case bps >= 1e9:
		return fmt.Sprintf("%.1fGB/s", bps/1e9)
	case bps >= 1e6:
		return fmt.Sprintf("%.1fMB/s", bps/1e6)
	case bps >= 1e3:
		return fmt.Sprintf("%.1fKB/s", bps/1e3)
	default:
		return fmt.Sprintf("%.0fB/s", bps)
	}
}

func runHeatmap(screen tcell.Screen, sigChan chan os.Signal, interactive bool, grayscale bool) bool {
	w, h := screen.Size()

	// Calculate column widths from total width
	calcLayout := func(width int) (nameW, cpuW, memW, diskW int) {
		nameW = 20
		if width >= 100 {
			nameW = 26
		} else if width < 60 {
			nameW = 14
		}
		rem := width - nameW - 4 // 4 vertical separators
		if rem < 24 {
			rem = 24
		}
		cpuW = rem / 3
		memW = rem / 3
		diskW = rem - cpuW - memW
		return
	}

	eventChan := make(chan tcell.Event, 10)
	go func() {
		for {
			eventChan <- screen.PollEvent()
		}
	}()

	statsChan := make(chan map[int]hmProc, 1)
	go func() {
		for {
			statsChan <- hmSampleAll()
			time.Sleep(hmPollRate)
		}
	}()

	drawTick := time.NewTicker(50 * time.Millisecond)
	defer drawTick.Stop()

	var prevSample map[int]hmProc
	var rows []hmRow
	var lastSample time.Time
	// Track current heat state by process name for smooth transitions
	heatByName := make(map[string][3]float64) // [cpuHeat, memHeat, diskHeat]

	// Styles (precomputed, refreshed on each draw to support resize)
	mkStyles := func() (title, header, sep, legend tcell.Style) {
		if grayscale {
			title = tcell.StyleDefault.Background(tcell.NewRGBColor(35, 35, 35)).Foreground(tcell.NewRGBColor(240, 240, 240))
			header = tcell.StyleDefault.Background(tcell.NewRGBColor(20, 20, 20)).Foreground(tcell.NewRGBColor(190, 190, 190))
			sep = tcell.StyleDefault.Background(tcell.NewRGBColor(25, 25, 25)).Foreground(tcell.NewRGBColor(110, 110, 110))
			legend = tcell.StyleDefault.Background(tcell.NewRGBColor(10, 10, 10)).Foreground(tcell.NewRGBColor(140, 140, 140))
		} else {
			title = tcell.StyleDefault.Background(tcell.NewRGBColor(12, 12, 42)).Foreground(tcell.NewRGBColor(180, 210, 255))
			header = tcell.StyleDefault.Background(tcell.NewRGBColor(7, 7, 25)).Foreground(tcell.NewRGBColor(120, 158, 238))
			sep = tcell.StyleDefault.Background(tcell.NewRGBColor(18, 18, 52)).Foreground(tcell.NewRGBColor(65, 75, 155))
			legend = tcell.StyleDefault.Background(tcell.NewRGBColor(5, 5, 18)).Foreground(tcell.NewRGBColor(110, 125, 195))
		}
		return
	}

	for {
		select {
		case <-sigChan:
			return false

		case sample := <-statsChan:
			now := time.Now()
			elapsed := now.Sub(lastSample).Seconds()
			if elapsed <= 0 {
				elapsed = 1
			}

			if prevSample != nil {
				type entry struct {
					name    string
					cpuRate float64
					memKB   int64
					diskBps float64
				}
				entries := make([]entry, 0, len(sample))
				for pid, curr := range sample {
					prev, ok := prevSample[pid]
					if !ok {
						continue
					}
					var cpuDelta, diskDelta float64
					if curr.cpuTicks >= prev.cpuTicks {
						cpuDelta = float64(curr.cpuTicks-prev.cpuTicks) / elapsed
					}
					if curr.ioBytes >= prev.ioBytes {
						diskDelta = float64(curr.ioBytes-prev.ioBytes) / elapsed
					}
					entries = append(entries, entry{curr.name, cpuDelta, curr.memKB, diskDelta})
				}

				// Normalization denominators
				var maxCPU, maxDisk float64
				var maxMem int64
				for _, e := range entries {
					if e.cpuRate > maxCPU {
						maxCPU = e.cpuRate
					}
					if e.memKB > maxMem {
						maxMem = e.memKB
					}
					if e.diskBps > maxDisk {
						maxDisk = e.diskBps
					}
				}
				if maxCPU < 1 {
					maxCPU = 1
				}
				if maxMem < 1 {
					maxMem = 1
				}
				if maxDisk < 1 {
					maxDisk = 1
				}

				// Sort hottest first (combined normalized score)
				sort.Slice(entries, func(i, j int) bool {
					si := entries[i].cpuRate/maxCPU + float64(entries[i].memKB)/float64(maxMem) + entries[i].diskBps/maxDisk
					sj := entries[j].cpuRate/maxCPU + float64(entries[j].memKB)/float64(maxMem) + entries[j].diskBps/maxDisk
					return si > sj
				})

				maxRows := h - 5
				if maxRows < 1 {
					maxRows = 1
				}
				if len(entries) > maxRows {
					entries = entries[:maxRows]
				}

				newRows := make([]hmRow, len(entries))
				for i, e := range entries {
					tCPU := e.cpuRate / maxCPU
					tMem := float64(e.memKB) / float64(maxMem)
					tDisk := e.diskBps / maxDisk

					// Inherit current heat values for smooth animation
					cHeat := heatByName[e.name]
					if _, seen := heatByName[e.name]; !seen {
						cHeat = [3]float64{tCPU * 0.5, tMem * 0.5, tDisk * 0.5}
					}

					newRows[i] = hmRow{
						name:       e.name,
						cpuRaw:     e.cpuRate,
						memKB:      e.memKB,
						diskBps:    e.diskBps,
						cpuHeat:    cHeat[0],
						memHeat:    cHeat[1],
						diskHeat:   cHeat[2],
						cpuTarget:  tCPU,
						memTarget:  tMem,
						diskTarget: tDisk,
					}
				}
				rows = newRows
			}

			prevSample = sample
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
			// Animate heat values toward targets and update name cache
			for i := range rows {
				rows[i].cpuHeat += (rows[i].cpuTarget - rows[i].cpuHeat) * hmLerpFactor
				rows[i].memHeat += (rows[i].memTarget - rows[i].memHeat) * hmLerpFactor
				rows[i].diskHeat += (rows[i].diskTarget - rows[i].diskHeat) * hmLerpFactor
				heatByName[rows[i].name] = [3]float64{rows[i].cpuHeat, rows[i].memHeat, rows[i].diskHeat}
			}

			screen.Clear()

			titleStyle, headerStyle, sepStyle, legendStyle := mkStyles()
			nameW, cpuW, memW, diskW := calcLayout(w)

			// Column x-offsets for separator positions
			sep1X := nameW
			sep2X := sep1X + 1 + cpuW
			sep3X := sep2X + 1 + memW

			// ── Title bar ──
			for x := 0; x < w; x++ {
				screen.SetContent(x, 0, ' ', nil, titleStyle)
			}
			titleText := " ◈  PROCESS RESOURCE HEATMAP  ◈ "
			tx := (w - len(titleText)) / 2
			if tx < 0 {
				tx = 0
			}
			hmDrawText(screen, tx, 0, titleText, titleStyle.Bold(true))

			// ── Column headers ──
			for x := 0; x < w; x++ {
				screen.SetContent(x, 1, ' ', nil, headerStyle)
			}
			hmDrawText(screen, 0, 1, hmCenterPad("PROCESS", nameW), headerStyle)
			screen.SetContent(sep1X, 1, '│', nil, sepStyle)
			hmDrawText(screen, sep1X+1, 1, hmCenterPad("CPU %", cpuW), headerStyle)
			screen.SetContent(sep2X, 1, '│', nil, sepStyle)
			hmDrawText(screen, sep2X+1, 1, hmCenterPad("MEMORY", memW), headerStyle)
			screen.SetContent(sep3X, 1, '│', nil, sepStyle)
			hmDrawText(screen, sep3X+1, 1, hmCenterPad("DISK I/O", diskW), headerStyle)

			// ── Header separator ──
			for x := 0; x < w; x++ {
				screen.SetContent(x, 2, '─', nil, sepStyle)
			}
			for _, jx := range []int{sep1X, sep2X, sep3X} {
				if jx < w {
					screen.SetContent(jx, 2, '┬', nil, sepStyle)
				}
			}

			// ── Process rows ──
			if len(rows) == 0 {
				loadStyle := tcell.StyleDefault.Background(tcell.NewRGBColor(5, 5, 18)).Foreground(tcell.NewRGBColor(80, 100, 180))
				if grayscale {
					loadStyle = tcell.StyleDefault.Background(tcell.NewRGBColor(10, 10, 10)).Foreground(tcell.NewRGBColor(130, 130, 130))
				}
				msg := "Sampling processes…"
				mx := (w - len(msg)) / 2
				if mx < 0 {
					mx = 0
				}
				hmDrawText(screen, mx, h/2, msg, loadStyle)
			}

			for ri, row := range rows {
				y := 3 + ri
				if y >= h-2 {
					break
				}

				// Alternating dark name column
				var nameBg tcell.Color
				if ri%2 == 0 {
					if grayscale {
						nameBg = tcell.NewRGBColor(18, 18, 18)
					} else {
						nameBg = tcell.NewRGBColor(10, 10, 26)
					}
				} else {
					if grayscale {
						nameBg = tcell.NewRGBColor(10, 10, 10)
					} else {
						nameBg = tcell.NewRGBColor(6, 6, 17)
					}
				}
				var nameFg tcell.Color
				if grayscale {
					nameFg = tcell.NewRGBColor(205, 205, 205)
				} else {
					nameFg = tcell.NewRGBColor(205, 215, 240)
				}
				nameStyle := tcell.StyleDefault.Background(nameBg).Foreground(nameFg)

				for x := 0; x < nameW; x++ {
					screen.SetContent(x, y, ' ', nil, nameStyle)
				}
				name := " " + row.name
				runes := []rune(name)
				if len(runes) > nameW {
					runes = runes[:nameW-1]
					runes = append(runes, '…')
				}
				for i, ch := range runes {
					if i < nameW {
						screen.SetContent(i, y, ch, nil, nameStyle)
					}
				}

				screen.SetContent(sep1X, y, '│', nil, sepStyle)
				cpuPct := row.cpuRaw / float64(hmClockTick) * 100
				hmFillCell(screen, sep1X+1, y, cpuW, fmt.Sprintf("%.1f%%", cpuPct), row.cpuHeat, grayscale)

				screen.SetContent(sep2X, y, '│', nil, sepStyle)
				hmFillCell(screen, sep2X+1, y, memW, hmFmtMem(row.memKB), row.memHeat, grayscale)

				screen.SetContent(sep3X, y, '│', nil, sepStyle)
				hmFillCell(screen, sep3X+1, y, diskW, hmFmtDisk(row.diskBps), row.diskHeat, grayscale)
			}

			// ── Bottom separator ──
			if h >= 3 {
				for x := 0; x < w; x++ {
					screen.SetContent(x, h-2, '─', nil, sepStyle)
				}
				for _, jx := range []int{sep1X, sep2X, sep3X} {
					if jx < w {
						screen.SetContent(jx, h-2, '┴', nil, sepStyle)
					}
				}
			}

			// ── Legend ──
			for x := 0; x < w; x++ {
				screen.SetContent(x, h-1, ' ', nil, legendStyle)
			}
			hmDrawText(screen, 1, h-1, "HEAT:", legendStyle)
			lx := 7
			gradW := w / 4
			if gradW > 48 {
				gradW = 48
			}
			if gradW < 8 {
				gradW = 8
			}
			for i := 0; i < gradW; i++ {
				hv := float64(i) / float64(gradW-1)
				col := hmHeatColor(hv, grayscale)
				bg := tcell.NewRGBColor(5, 5, 18)
				if grayscale {
					bg = tcell.NewRGBColor(10, 10, 10)
				}
				screen.SetContent(lx+i, h-1, '█', nil, tcell.StyleDefault.Foreground(col).Background(bg))
			}
			lx += gradW + 1
			hmDrawText(screen, lx, h-1, "low → high   [sorted by combined activity]", legendStyle)

			screen.Show()
		}
	}
}
