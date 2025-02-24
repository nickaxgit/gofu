package main

//import "crypto/rand"

//import "math/rand"

import "math"

type vert struct {
	p       *Vec3
	n       *Vec3
	uv      Vector
	wl      float64       //water level
	acc     float64       //accumulated water (during a pass)
	touches map[*Tri]bool //the triangles that touch this vertex (whos face normals contribute to the vertex normal)
	//touchCount int
}

type Tri struct {
	depth    int
	vi       []uint16 `json:"vi"`
	children []*Tri   `json:"children"`
	mesh     *mesh    //a reference to the mesh this tri is part of (that the vi's point into v's of)
	normal   *Vec3
	//faceIndex uint16
	//isPatch   bool
}

// keep track of the deepest (up to) 6 triangles touching this vert -- allows us to recalculate normals quickly
func (v *vert) touch(t ...*Tri) {

	for _, t := range t {
		v.touches[t] = true
	}
}

func (vt *vert) updateUV(yMin, yMax float64) {

	vrange := (yMax - yMin)
	v := (vt.p.y - yMin) / vrange

	if v > 1 || v < 0 {
		logit("UV out of range", v)

	}

	vt.uv = Vector{math.Atan2(vt.n.x, vt.n.z) / float64(6.28), v} //v
	//	vt.uv = Vector{1, 1}
}

func newVert(p *Vec3, u, v float64) *vert {
	return &vert{p: p, uv: Vector{u, v}, n: &Vec3{0, 0, 0}, touches: make(map[*Tri]bool, 6)}
}

func (t *Tri) addChild(a, b, c uint16) *Tri {
	child := newTri(t.mesh, []uint16{a, b, c}, t.depth+1) //t.mesh.makeTri(t.depth+1, fi, a, b, c)
	t.children = append(t.children, child)
	return child
}

// func (t *Tri) gather(i []uint16, p *int) {

// 	t.faceIndex = uint16(*p / 3)

// 	if len(t.Children) == 0 {
// 		i[*p] = t.Vi[0]
// 		*p++
// 		i[*p] = t.Vi[1]
// 		*p++
// 		i[*p] = t.Vi[2]
// 		*p++

// 	}

// 	for _, c := range t.Children {
// 		c.gather(i, p)
// 	}

// }

func (m *mesh) vertAtMidPointXZ(a, b uint16) uint16 {
	p := m.verts[a].p.tween(m.verts[b].p, 0.5)

	tiny := 0.0001
	for i, v := range m.verts {
		if v.p.x > p.x-tiny && v.p.x < p.x+tiny {
			if v.p.z > p.z-tiny && v.p.z < p.z+tiny {
				return uint16(i)
			}
		}
	}
	return 65535

}

//for every bottom level triangle, look to see if there is a vertex at the midpoint of each edge (caused by a more divided neighbouring tri)
//if so, split in two to the opposite vertex
func (t *Tri) patch(m *mesh) {

	if len(t.children) == 0 {

		for i := 0; i < 3; i++ {

			ai := t.vi[i]
			bi := t.vi[(i+1)%3]
			ci := t.vi[(i+2)%3]
			mi := m.midpoint(ai, bi) //looks both ways for a midpoint   -- m.vertAtMidPointXZ(ai, bi) //this could be way faster by recording mid verts

			if mi != 65535 {
				t.removeFromTouches() //remove this tri from the list tris touching this vertex
				t.addChild(mi, bi, ci)
				t.addChild(ai, mi, ci)
				break //only one split per tri
			}

		}

	}

	for _, c := range t.children {
		c.patch(m)
	}

}
func (t *Tri) yMin() float64 {
	v := t.mesh.verts
	return math.Min(math.Min(v[t.vi[0]].p.y, v[t.vi[1]].p.y), v[t.vi[2]].p.y)
}

func (t *Tri) yMax() float64 {
	v := t.mesh.verts
	return math.Max(math.Max(v[t.vi[0]].p.y, v[t.vi[1]].p.y), v[t.vi[2]].p.y)
}

func (t *Tri) addToTouches() {

	m := t.mesh
	m.verts[t.vi[0]].touch(t)
	m.verts[t.vi[1]].touch(t)
	m.verts[t.vi[2]].touch(t)

}

func (t *Tri) removeFromTouches() {
	if len(t.children) > 0 {
		panic("Tri has children")
	}
	for _, vi := range t.vi {
		v := t.mesh.verts[vi]
		delete(v.touches, t) //remove this tri from the list tris touching this vertex
		// if len(v.touches) == 0 && vi > 2 { //we always keep the original 3 verts of the land
		// 	logit("Vert", vi, "freed for reuse")
		// 	v.p = newVec3(0, 0, 0)                            //zero out the vertex (prior to reuse (to make accidental use of a reusable vertex obvious)
		// 	t.mesh.reuseVerts = append(t.mesh.reuseVerts, vi) //if there are no longer any traingles touchin gthis vert - we can reuse it

		// 	//remove all references to this vertex in the midpoints map
		// 	for k, v := range t.mesh.midpoints {
		// 		if v == vi {
		// 			delete(t.mesh.midpoints, k) //this is safe in go
		// 		}
		// 	}
		// }
	}
}

func (t *Tri) split(m *mesh, pos *Vec3, maxRdepth int, maxHeight float64) {

	//      0
	//		/\
	//     /  \
	//  5 /____\ 3
	//   / \  / \
	//  /___\/___\
	// 2     4    1

	v0 := t.vi[0]
	v1 := t.vi[1]
	v2 := t.vi[2]

	if t.depth < maxRdepth {

		triCentre := t.centre()
		triCentre.y = 0 //we want to measure distance on the x/z plane (otherwise subdivision is affected by terrain height)
		d := triCentre.distanceFrom(newVec3(pos.x, 0, pos.z))

		d2 := float64(t.depth + 1)

		f := 7000 / ((math.Pow(d2, 2)) * d) //distance as a fraction of the land size (a number rougly between 0 and 1)

		if f > .25 { //should i be split in four ?

			if len(t.children) == 0 {
				Yrange := maxHeight / (float64(t.depth*t.depth*t.depth) + 1) //maximum kink in this edge

				//rnGen = &rand.New(rand.NewPCG(seed+1, seed))
				//rnGen = &rand.New(rand.NewPCG(m.verts[v0].p.x, m.verts[v0].p.y))

				rn1 := 0.0 //math.Sin(math.Round(m.verts[v0].p.x))
				rn2 := 0.0 //math.Sin(math.Round(m.verts[v1].p.z))
				rn3 := 0.0 //math.Sin(math.Round(m.verts[v2].p.z))

				// v3 := m.splitEdge(v0, v1, (rnGen.Float64()-.5)*Yrange)
				// v4 := m.splitEdge(v1, v2, (rnGen.Float64()-.5)*Yrange)
				// v5 := m.splitEdge(v2, v0, (rnGen.Float64()-.5)*Yrange)

				v3 := m.splitEdge(v0, v1, rn1/2*Yrange)
				v4 := m.splitEdge(v1, v2, rn2/2*Yrange)
				v5 := m.splitEdge(v2, v0, rn3/2*Yrange)

				//when we add a child - reuse the parents face index (for the top triangle) - and add 3 more
				t.removeFromTouches()

				t.addChild(v0, v3, v5) //top
				t.addChild(v3, v1, v4) //right
				t.addChild(v5, v4, v2) //left
				t.addChild(v3, v4, v5) //centre

			}

			for _, c := range t.children {
				c.split(m, pos, maxRdepth, maxHeight)
			}

		}
		//if i didn't need splitting ... then my children don't either

	}

}

func (t *Tri) centre() *Vec3 {
	v := t.mesh.verts
	return v[t.vi[0]].p.add(v[t.vi[1]].p).add(v[t.vi[2]].p).multiply(float64(1) / 3)
}

func (t *Tri) contains(pop *Vec3, includeOnVert bool, includeOnEdge bool) bool {

	v := t.mesh.verts //this is a reference not a copy

	a := v[t.vi[0]].p
	b := v[t.vi[1]].p
	c := v[t.vi[2]].p

	return pop.isInsideTri(a, b, c, includeOnEdge, includeOnVert)

}

func (t *Tri) probeLand(p *Vec3) (surfacePoint *Vec3, surfaceNormal *Vec3) {

	found := t.vProbe(p) //recursively find the leaf tri that contains the point

	if found == nil {
		return nil, nil
	}
	//fire a ray through that
	return found.probePlane(newVec3(p.x, -10000, p.z), newVec3(p.x, 10000, p.z)), found.normal
}

func (t *Tri) vProbe(p *Vec3) *Tri {

	if p.y != 0 {
		panic("tri vProbe Y must be 0")
	}
	a := t.mesh.verts[t.vi[0]].p.clone()
	b := t.mesh.verts[t.vi[1]].p.clone()
	c := t.mesh.verts[t.vi[2]].p.clone()

	a.y = 0
	b.y = 0
	c.y = 0

	if p.isInsideTri(a, b, c, true, true) {
		if len(t.children) == 0 {
			return t
		}

		for _, ct := range t.children {
			tt := ct.vProbe(p)
			if tt != nil {
				return tt
			}
		}
	}

	//panic("vProbe failed to find a tri")
	return nil

}

func (t *Tri) calcNormal() *Vec3 {

	v := t.mesh.verts

	numMeshVerts := uint16(len(v))
	if t.vi[0] >= numMeshVerts || t.vi[1] >= numMeshVerts || t.vi[2] >= numMeshVerts {
		panic("index out of range in tri.normal")
	}

	//n1 := v[t.Vi[1]].p.sub(v[t.Vi[0]].p).cross(v[t.Vi[2]].p.sub(v[t.Vi[0]].p)).normalise()
	//logit(n1.X, n1.Y, n1.Z)
	ab := v[t.vi[1]].p.sub(v[t.vi[0]].p) //.normalise()
	ac := v[t.vi[2]].p.sub(v[t.vi[0]].p) //.normalise()
	n2 := (ab.cross(ac)).normalise()
	ln := n2.length()
	if ln < .999 || ln > 1.00001 {
		panic("normal is not unit length")
	}

	t.normal = n2
	return n2
}

//join up the edge penetrations of the tool along the edges of the clay triangle (mutuates segs)

func (ct *Tri) joinEdgePenetrations(segs *segSet, toolMesh *mesh, output *mesh) *segSet {
	//create the outer loop of the clay triangle by walking the edges and their penetrations

	outerSegs := newSegSet()
	//create segments to close the outer ring(s) from the original triangle
	//ctn := ct.normal()

	if len(ct.vi) > 3 {
		panic("not a traingle")
	}

	for i := 0; i < 3; i++ { //range ct.Vi { //for each vertex of the clay triangle

		vt := ct.vi[i]
		vn := ct.vi[(i+1)%3]

		edgePens := segs.getEdgePenetrations(output, vt, vn) //get a sorted (by distance from vt) set of edge penetrations
		if len(edgePens) == 0 {
			outerSegs.add(vt, vn, 0) //this an unpentrated outer edge of the triangle
		} else {
			corner := output.verts[vt].p //the corner of the triangle
			// ep0 := ct.mesh.verts[edgePens[0]].p //the first penetration point
			// cornerIsCut := ep0.sub(corner).dot(segs.normalTo(ct.mesh, edgePens[0], ctn)) > 0
			cornerIsCut := corner.isInside(toolMesh)
			outerSegs.addFromEdgePens(toolMesh, vt, vn, edgePens, cornerIsCut, output) //collect alternating segments
		}

	}

	return outerSegs

}

func (ss *segSet) discardOpenSegments() *segSet {

	keep := newSegSet()
	for _, s := range ss.segs {
		if s.touchesEdges == 0 || s.touchesEdges == 2 { //it's an internal segment (from a pointy triangle penetrating the face somewhere in the middle) - OR it clips off one corner
			keep.add(s.from, s.to, s.touchesEdges)
		} else {
			if s.connectsToEdge(ss) {
				keep.add(s.from, s.to, s.touchesEdges)
			}
		}
	}

	return keep

}

func (s *seg) connectsToEdge(ss *segSet) bool {

	//TODO Implement !
	return true //false
}

//for testing
func (t *Tri) edge0() *Vec3 {
	return t.mesh.verts[t.vi[1]].p.sub(t.mesh.verts[t.vi[0]].p)
}

func (t *Tri) check() {

	m := t.mesh
	panic("triangle has two or more vertices at the same point")
	if m.verts[t.vi[0]].p.equals(m.verts[t.vi[1]].p) || m.verts[t.vi[0]].p.equals(m.verts[t.vi[2]].p) || m.verts[t.vi[1]].p.equals(m.verts[t.vi[2]].p) {
	}
	if m.inAline(t.vi[0], t.vi[1], t.vi[2]) {
		panic("triangle is a line")
	}

}

//return 0, 1 or 2 points of intersection of the edges of the tool triangle, with the plane of triangle a
func (clayTri *Tri) penetrationsByEdgesOf(toolTri *Tri, isEdgePen bool) *penSet {

	pens := NewPenSet()

	ctn := clayTri.normal
	ttn := toolTri.normal
	dp := ctn.dot(ttn)
	if dp > 0.99999 || dp < -0.99999 {
		return pens //the faces are parallel/coplanar
	}

	//i goes 0,1,2
	for i := 0; i < 3; i++ {
		nxt := (i + 1) % 3

		r0 := toolTri.mesh.verts[toolTri.vi[i]].p
		r1 := toolTri.mesh.verts[toolTri.vi[nxt]].p
		pop := clayTri.probePlane(r0, r1)
		if pop != nil {
			if clayTri.contains(pop, true, true) {
				pens.add(pop, toolTri.vi[i], toolTri.vi[nxt], toolTri, clayTri, isEdgePen)
			}
		}

	}

	return pens

}

func newTri(m *mesh, vi []uint16, depth int) *Tri {

	if vi[0] == vi[1] || vi[0] == vi[2] || vi[1] == vi[2] {
		panic("degenerate triangle")
	}

	p0 := m.verts[vi[0]].p
	p1 := m.verts[vi[1]].p
	p2 := m.verts[vi[2]].p

	if p0.equals(p1) || p0.equals(p2) || p1.equals(p2) {
		panic("infinitely thin triangle")
	}

	// if fi == 65535 {
	// 	if len(m.reuseFaces) > 0 {
	// 		fi = m.reuseFaces[0]
	// 		logit("reusing face", fi)
	// 		m.reuseFaces = m.reuseFaces[1:]
	// 	} else {
	// 		fi = m.faceCount
	// 		m.faceCount++
	// 	}
	// }

	//t := Tri{depth: depth, vi: vi, children: []*Tri{}, mesh: m, faceIndex: fi}
	t := Tri{depth: depth, vi: vi, children: []*Tri{}, mesh: m}

	//add this traingle to its verts list of triangles
	t.addToTouches()

	// m.fi[fi*3] = vi[0]
	// m.fi[fi*3+1] = vi[1]
	// m.fi[fi*3+2] = vi[2]

	t.calcNormal()

	return &t
}
func (ct *Tri) internalSegmentsFromPenetrationsOf(tool *mesh, output *mesh) *segSet {

	segs := newSegSet()

	if ct.mesh == tool {
		panic("tool and clay are the same mesh")
	}

	ctn := ct.normal

	for j := 0; j < len(tool.fi); j += 3 {
		tt := tool.triangleFrom(j)
		ttn := tt.normal

		tt.check()

		//each pen has a toolTri, a claytri, v1, v2 and a point
		//the v1 and v2 are always the indices of the penetrating edge

		//facePens := ct.penetrationsByEdgesOf(tt, false) //they *may* be on the edge of the clay triangle - or not
		t := tt.penetrationsByEdgesOf(ct, true)
		touchesEdges := len(t.pens)
		if len(t.pens) == 2 {
			logit("two edge penetrations")
		}
		if len(t.pens) > 2 {
			panic("More than 2 edge penetrations")
		}

		if len(t.pens) != 2 {

			facePens := ct.penetrationsByEdgesOf(tt, false) //they *may* be on the edge of the clay triangle - or not

			for _, fp := range facePens.pens {
				if t.penAt(fp.p) == nil {
					t.pens = append(t.pens, fp)
				}
			}
		}

		//we want the union of these - but edgepens should overried facepens at the same point

		// penPair := NewPenSet()
		// for p := range facePens.pens {
		// 	oep := edgePens.penAt(p.p) //is there an overring edge penetration at this point?
		// 	if oep != nil {
		// 		penPair.pens = append(penPair.pens, p, oep)
		// 	} else {
		// 		penPair.pens = append(penPair.pens, p)
		// 	}
		// }

		if len(t.pens) > 0 {
			if len(t.pens) != 2 {
				e := ct.penetrationsByEdgesOf(tt, false)
				f := tt.penetrationsByEdgesOf(ct, true)
				logit(len(e.pens), len(f.pens))

				logit("unmatched pen")
				continue //break
			}
			//pens.append(t)

			v0 := output.addVert(t.pens[0].p, true, 0, 0) //add a vertex at this point (or reuse an existing one)
			v1 := output.addVert(t.pens[1].p, true, 0, 0) //add a vertex at this point (or reuse an existing one) TODO TC's

			v := t.pens[1].p.sub(t.pens[0].p) //vector between the pair of penetrations points (of this tool face on this clay face)

			dir := v.cross(ttn).dot(ctn)
			if dir == 0 {
				panic("zero cross product dotted")
			}
			if dir > 0 { //add a directed segment (this is very significant and affects the winding of holes)
				segs.add(v0, v1, touchesEdges)
			} else {
				segs.add(v1, v0, touchesEdges)
			}
		}

	}

	return segs

}
