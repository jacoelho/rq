package jsonpath

type pathMatcher struct {
	segs         []segment
	effectivePes []pathElem
	vals         []any
}

// matchPath checks if the current path (pes, vals) matches the compiled segments.
// For path "$", pes and vals should be nil/empty.
// Returns true if the path matches the JSONPath expression represented by segs.
func matchPath(segs []segment, pes []pathElem, vals []any) bool {
	if len(segs) == 0 {
		return len(pes) == 0
	}

	matcher := &pathMatcher{
		segs: segs,
		vals: vals,
	}

	// The path from the streamer contains a dummy root element that the matcher shouldn't see.
	if len(pes) > 0 {
		matcher.effectivePes = pes[1:]
	}

	return matcher.match(0, 0)
}

func (m *pathMatcher) match(segIdx, pathIdx int) bool {
	if segIdx == len(m.segs) {
		return pathIdx == len(m.effectivePes)
	}
	if pathIdx == len(m.effectivePes) {
		return false
	}

	seg := m.segs[segIdx]
	currentPe := m.effectivePes[pathIdx]
	currentVal := m.getValueAt(pathIdx)

	if seg.deep {
		return m.matchDeepSegment(segIdx, pathIdx)
	}

	if selMatch(seg.sels, currentPe, currentVal) {
		return m.match(segIdx+1, pathIdx+1)
	}
	return false
}

func (m *pathMatcher) matchDeepSegment(segIdx, pathIdx int) bool {
	seg := m.segs[segIdx]

	for k := pathIdx; k < len(m.effectivePes); k++ {
		pe := m.effectivePes[k]
		val := m.getValueAt(k)

		if selMatch(seg.sels, pe, val) && m.match(segIdx+1, k+1) {
			return true
		}
	}
	return false
}

func (m *pathMatcher) getValueAt(pathIdx int) any {
	// vals corresponds to original pes, so access with pathIdx+1
	if pathIdx+1 < len(m.vals) {
		return m.vals[pathIdx+1]
	}
	return nil
}

// selMatch checks if any selector in sels matches the given path element and its value.
func selMatch(sels []selector, pe pathElem, val any) bool {
	if len(sels) == 0 {
		return false
	}
	for _, s := range sels {
		if s.match(pe, val) {
			return true
		}
	}
	return false
}

// isFilterMatch determines if there's a filter selector in the segments and returns it.
func isFilterMatch(segs []segment, pes []pathElem, vals []any) (filterSel, bool, int) {
	effectivePes := pes
	if len(pes) > 0 {
		effectivePes = pes[1:]
	}

	var recur func(si, pi int) (filterSel, bool, int)
	recur = func(si, pi int) (filterSel, bool, int) {
		if si == len(segs) {
			return filterSel{}, false, -1
		}
		if pi == len(effectivePes) {
			if segs[si].deep {
				return recur(si+1, pi)
			}
			return filterSel{}, false, -1
		}

		seg := segs[si]
		currentPe := effectivePes[pi]
		var currentVal any
		if pi+1 < len(vals) {
			currentVal = vals[pi+1]
		}
		if len(seg.sels) == 1 {
			if fs, ok := seg.sels[0].(filterSel); ok {
				return fs, true, si
			}
		}

		if seg.deep {
			if fs, ok, segIdx := recur(si+1, pi); ok {
				return fs, true, segIdx
			}
			return recur(si, pi+1)
		}
		if selMatch(seg.sels, currentPe, currentVal) {
			return recur(si+1, pi+1)
		}
		return filterSel{}, false, -1
	}

	fs, ok, segIdx := recur(0, 0)
	return fs, ok, segIdx
}
