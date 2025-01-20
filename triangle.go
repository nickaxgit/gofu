package main

//import "crypto/rand"

//import "math/rand/v2"

type vert struct {
	p   *Vec3
	n   *Vec3
	uv  Vector
	wl  float64 //water level
	acc float64 //accumulated water (during a pass)
}

type Tri struct {
	depth    int
	Vi       []int  `json:"vi"`
	Children []*Tri `json:"children"`
	mesh     *mesh  //a reference to the mesh this tri is part of (that the vi's point into v's of)
}

//creates a new vertex halfway between the indexed verts a and b and returns the index of the new vertex
func (m *mesh) splitEdge(a int, b int, dy float64) int {

	p := m.verts[a].p.tween(m.verts[b].p, 0.5)
	p.Y += dy
	return m.addOrReuseVertAtXZ(p)
}

func (t *Tri) addChild(a, b, c int) {
	t.Children = append(t.Children, t.mesh.makeTri(t.depth+1, a, b, c))
}

func (t *Tri) gather(depth int, i []int, p *int) {

	if t.depth == depth {
		i[*p] = t.Vi[0]
		*p++
		i[*p] = t.Vi[1]
		*p++
		i[*p] = t.Vi[2]
		*p++

	}

	for _, c := range t.Children {
		c.gather(depth, i, p)
	}

}
func (t *Tri) split(m *mesh, maxRdepth int, maxHeight float64) {

	//      0
	//		/\
	//     /  \
	//  5 /____\ 3
	//   / \  / \
	//  /___\/___\
	// 2     4    1

	v0 := t.Vi[0]
	v1 := t.Vi[1]
	v2 := t.Vi[2]

	Yrange := maxHeight / float64(t.depth+1) //maximum kink in this edge

	v3 := m.splitEdge(v0, v1, (rnGen.Float64()-.5)*Yrange)
	v4 := m.splitEdge(v1, v2, (rnGen.Float64()-.5)*Yrange)
	v5 := m.splitEdge(v2, v0, (rnGen.Float64()-.5)*Yrange)

	t.addChild(v0, v3, v5) //top
	t.addChild(v3, v1, v4) //right
	t.addChild(v5, v4, v2) //left
	t.addChild(v3, v4, v5) //centre

	for _, c := range t.Children {
		if c.depth < maxRdepth {
			c.split(m, maxRdepth, maxHeight)
		}
	}

}

func (t *Tri) distanceFrom(p *Vec3) float64 {

	v := t.mesh.verts
	a := v[t.Vi[0]].p
	b := v[t.Vi[1]].p
	c := v[t.Vi[2]].p

	return p.distanceFromTriPlane(a, b, c)

}

func (t *Tri) contains(pop *Vec3, includeOnVert bool, includeOnEdge bool) bool {

	v := t.mesh.verts //this is a reference not a copy

	a := v[t.Vi[0]].p
	b := v[t.Vi[1]].p
	c := v[t.Vi[2]].p

	if a.equals(pop) || b.equals(pop) || c.equals(pop) {
		return includeOnVert // return true or false, depending on the value of includeOnVert
	}

	return pop.isInsideTri(a, b, c, includeOnEdge)

}

func (t *Tri) normal() *Vec3 {
	v := t.mesh.verts

	l := len(v)
	if t.Vi[0] >= l || t.Vi[1] >= l || t.Vi[2] >= l {
		panic("index out of range in tri.normal")
	}

	//n1 := v[t.Vi[1]].p.sub(v[t.Vi[0]].p).cross(v[t.Vi[2]].p.sub(v[t.Vi[0]].p)).normalise()
	//logit(n1.X, n1.Y, n1.Z)
	ab := v[t.Vi[1]].p.sub(v[t.Vi[0]].p) //.normalise()
	ac := v[t.Vi[2]].p.sub(v[t.Vi[0]].p) //.normalise()
	n2 := (ab.cross(ac)).normalise()
	ln := n2.length()
	if ln < .999 || ln > 1.00001 {
		panic("normal is not unit length")
	}
	return n2
}

//join up the edge penetrations of the tool along the edges of the clay triangle (mutuates segs)

func (ct *Tri) joinEdgePenetrations(segs *segSet, toolMesh *mesh, output *mesh) *segSet {
	//create the outer loop of the clay triangle by walking the edges and their penetrations

	outerSegs := newSegSet()
	//create segments to close the outer ring(s) from the original triangle
	//ctn := ct.normal()

	if len(ct.Vi) > 3 {
		panic("not a traingle")
	}

	for i := 0; i < 3; i++ { //range ct.Vi { //for each vertex of the clay triangle

		vt := ct.Vi[i]
		vn := ct.Vi[(i+1)%3]

		edgePens := segs.getEdgePenetrations(output, vt, vn) //get a sorted (by distance from vt) set of edge penetrations
		if len(edgePens) == 0 {
			outerSegs.add(vt, vn, 0) //this an unpentrated outer edge if the triangle
		} else {
			corner := output.verts[vt].p //the corner of the triangle
			// ep0 := ct.mesh.verts[edgePens[0]].p //the first penetration point
			// cornerIsCut := ep0.sub(corner).dot(segs.normalTo(ct.mesh, edgePens[0], ctn)) > 0
			cornerIsCut := corner.isInside(toolMesh)
			outerSegs.addFromEdgePens(ct, toolMesh, vt, vn, edgePens, cornerIsCut, output) //collect alternating segments
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
	return t.mesh.verts[t.Vi[1]].p.sub(t.mesh.verts[t.Vi[0]].p)
}

func (t *Tri) check() {

	m := t.mesh
	if m.verts[t.Vi[0]].p.equals(m.verts[t.Vi[1]].p) || m.verts[t.Vi[0]].p.equals(m.verts[t.Vi[2]].p) || m.verts[t.Vi[1]].p.equals(m.verts[t.Vi[2]].p) {
		panic("triangle has two or more vertices at the same point")
	}
	if m.inAline(t.Vi[0], t.Vi[1], t.Vi[2]) {
		panic("triangle is a line")
	}

}

func (ct *Tri) internalSegmentsFromPenetrationsOf(tool *mesh, output *mesh) *segSet {

	segs := newSegSet()

	if ct.mesh == tool {
		panic("tool and clay are the same mesh")
	}

	ctn := ct.normal()

	ct.check()

	for j := 0; j < len(tool.fi); j += 3 {
		tt := tool.triangleFrom(j)
		ttn := tt.normal()

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
