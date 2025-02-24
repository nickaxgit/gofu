package main

import (
	"slices"
	"sort"
)

type segSet struct {
	segs []*seg
}

func newSegSet() *segSet {
	return &segSet{segs: []*seg{}}
}

func (ss *segSet) merge(other *segSet) {
	ss.segs = append(ss.segs, other.segs...)
}

func (ss *segSet) add(from, to uint16, touchesEdges int) {
	if from == to {
		panic("//degenerate segment")
	}
	ss.segs = append(ss.segs, &seg{from, to, false, touchesEdges})
}

func (ss *segSet) findSegTo(to uint16, loop *loop) *seg {
	for _, s := range ss.segs {
		if s.to == to && !slices.Contains(loop.vi, s.to) {
			return s
		}
	}
	return nil // penetrations of single triangles through faces create one segment that does not form a loop
	//panic("seg not found")
}
func (ss *segSet) findSegFrom(from uint16, loop *loop) *seg {
	for _, s := range ss.segs {
		if s.from == from && !slices.Contains(loop.vi, s.from) {
			return s
		}
	}
	return nil // penetrations of single triangles through faces create one segment that does not form a loop
	//panic("seg not found")
}

func (ss *segSet) addFromEdgePens(tm *mesh, vt, vn uint16, edgePens []uint16, cornerIsCut bool, output *mesh) { //collect alternating segments))

	if cornerIsCut {
		if len(edgePens) > 1 { //if the corner is cut and there is only one edge penetration - that penerataton IS that corner cut
			for i := 0; i < len(edgePens)-1; i += 2 {
				ss.add(edgePens[i], edgePens[i+1], 1)
			}
		}
		if len(edgePens)%2 == 1 {
			if output.verts[vn].p.isInside(tm) { //TODO remove
				panic("start corner is cut but opposite corner is inside tool, despite an odd number of edge penetrations")
			}
			ss.add(edgePens[len(edgePens)-1], vn, 1)
		}
	} else {
		ss.add(vt, edgePens[0], 1)
		for i := 1; i < len(edgePens)-1; i += 2 {
			ss.add(edgePens[i], edgePens[i+1], 1)
		}
		if len(edgePens)%2 == 0 {
			if output.verts[vn].p.isInside(tm) { //TODO remove
				logit("edgepens", len(edgePens))
				logit("start corner IS NOT cut but opposite corner is inside tool, despite an EVEN number of edge penetrations")
			}
			ss.add(edgePens[len(edgePens)-1], vn, 1)
		}
	}
}

func (ss *segSet) otherEnd(v uint16) uint16 {
	for _, s := range ss.segs {
		if s.from == v {
			return s.to
		}
		if s.to == v {
			return s.from
		}
	}
	panic("other end not found")
}

// func (ss *segSet) normalTo(m *mesh, v int, ctn *Vec3) *Vec3 {
// 	//return a vector normal to the segment that contains vertex v

// 	v2 := ss.otherEnd(v)

// 	return m.verts[v2].p.sub(m.verts[v].p).cross(ctn)
// }

func (ss *segSet) unused() *seg {

	//ugly - but look for outer segments first
	for _, s := range ss.segs {
		if !s.used && s.touchesEdges > 0 {
			return s
		}
	}

	for _, s := range ss.segs {
		if !s.used {
			return s
		}
	}

	return nil
}

// return a sorted list of vertex indices, from segs that meet the edge (by distance from corner)
func (ss *segSet) getEdgePenetrations(m *mesh, vThis, vNext uint16) []uint16 {

	dm := map[float64]uint16{}

	e0 := m.verts[vThis].p
	e1 := m.verts[vNext].p

	if e0.equals(e1) {
		panic("degenerate edge")
	}

	for _, seg := range ss.segs {
		f := m.verts[seg.from].p
		t := m.verts[seg.to].p

		if f.distanceFromLine(e0, e1) < 0.01 {
			dm[f.distanceFrom(e0)] = seg.from
		}
		if t.distanceFromLine(e0, e1) < 0.01 {
			dm[t.distanceFrom(e0)] = seg.to
		}

	}

	index := make([]float64, len(dm))
	i := int(0)
	for k := range dm {
		index[i] = k
		i++
	}
	sort.Float64s(index)

	sorted := make([]uint16, len(dm))
	for i, p := range index {
		if dm[p] < 3 {
			panic("bad")
		}
		sorted[i] = dm[p]
	}

	return sorted

}

func (ss *segSet) unUsedVert(loops *loopSet) uint16 {
	//return a vertex in the segset that has not been used in a loop
	for _, s := range ss.segs {
		if !loops.contains(s.from) {
			return s.from
		}
		if !loops.contains(s.to) {
			return s.to
		}
	}
	return 65535
}

// find a linked vert not already in the loop
func (ss *segSet) findLinked(vi uint16, l *loop) uint16 {

	for _, s := range ss.segs {
		if s.from == vi && !slices.Contains(l.vi, s.to) {
			return s.to
		}
		if s.to == vi && !slices.Contains(l.vi, s.from) {
			return s.from
		}
	}
	return 65535

}

func (ss *segSet) getLoops(m *mesh) *loopSet {
	//return a list of loops - each loop is a closed list of segments

	loopSet := newLoopSet()

	for {
		at := ss.unUsedVert(loopSet) //find a vert in the segset, that has not been used in a loop

		if at == 65535 {
			return loopSet
		} //no more unused verts

		loop := NewLoop()

		for {
			loop.addVert(m, at)
			at = ss.findLinked(at, loop)
			if at == 65535 {
				break
			}

		}
		loopSet.append(loop)
	}

}
