package motion

import "math"

// scrollShares is how far each scroll phase has traveled from the window it
// left toward the one it arrives at: an ease-out, so most of the distance is
// covered by the first frame and the last one settles.
var scrollShares = [...]float64{0.5, 0.8}

// ScrollStart is the first row a scroll paints in phase: an offset between
// from and to, never past to, truncated toward from so an intermediate frame
// never lands early on a short jump. A phase past the schedule is to.
func ScrollStart(from, to, phase int) int {
	if phase < 0 || phase >= len(scrollShares) {
		return to
	}
	return from + int(float64(to-from)*scrollShares[phase])
}

// RevealHidden withholds the tail of each contiguous run of revealed rows: in
// phase p of n, the first ceil(len*(p+1)/(n+1)) rows of a run are drawn and the
// rest are not yet. A run of one row is always drawn, and so is every row once
// the schedule is spent, which is why the result is nil past it.
func RevealHidden(revealed []bool, phase int) []bool {
	phases := Phases(Reveal)
	if phase < 0 || phase >= phases {
		return nil
	}
	hidden := make([]bool, len(revealed))
	for start := 0; start < len(revealed); {
		if !revealed[start] {
			start++
			continue
		}
		end := start
		for end < len(revealed) && revealed[end] {
			end++
		}
		length := end - start
		shown := (length*(phase+1) + phases) / (phases + 1)
		for index := start + shown; index < end; index++ {
			hidden[index] = true
		}
		start = end
	}
	return hidden
}

// collapseShares is how much of a closed block's height each collapse phase
// still holds open: two thirds, then one third, then none.
var collapseShares = [...]float64{2.0 / 3, 1.0 / 3}

// CollapseRows is the blank rows a gap of rows still holds in phase, rounded
// up so a short block still visibly closes. A phase past the schedule holds
// none.
func CollapseRows(rows, phase int) int {
	if rows <= 0 || phase < 0 || phase >= len(collapseShares) {
		return 0
	}
	return int(math.Ceil(float64(rows) * collapseShares[phase]))
}

// DockRows is how many of a dock's body rows phase draws, top first: a third,
// then two thirds, then all of them. A body of one row is always drawn.
func DockRows(body, phase int) int {
	phases := Phases(Dock)
	if phase < 0 || phase >= phases || body <= 1 {
		return body
	}
	return max((body*(phase+1)+phases)/(phases+1), 1)
}
