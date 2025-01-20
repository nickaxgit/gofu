package main

import (
	"math"
	"math/rand/v2"
	"slices"
	"sort"
	"strconv"
	"time"
)

type loop struct {
	vi []int //an ordered list of vertex indices - that form a closed loop
}

func NewLoop() *loop {
	return &loop{vi: []int{}}
}

func (l *loop) mesh(lm *mesh, ctn *Vec3, m *mesh) *mesh {

	for li := range l.vi { //index

		vi := l.vi[li]

		p := lm.verts[vi].p
		np := lm.verts[l.vi[(li+1)%len(l.vi)]].p
		t1 := m.addVert(p, false, 0, 0) //note addvert will combine verts at the same position

		dir := np.sub(p).normalise()
		t2 := m.addVert(p.sub(dir.cross(ctn).multiply(10)), false, 0, .5)
		t3 := m.addVert(np, false, 1, 0)

		m.addFi(t2, t1, t3)

	}

	return m
}

func (t *loop) merge(s *loop) {
	//merge the vertices of l into this loop (t becomes a degenerate loop)
	//last := t.vi[len(t.vi)-1]
	t.vi = append(t.vi, t.vi[0]) //close the first loop (add the 0th vert)
	t.vi = append(t.vi, s.vi...)
	t.vi = append(t.vi, s.vi[0])
	//t.vi = append(t.vi, t.vi[last)
}

func (loop *loop) triangulate(dbm *mesh, m *mesh, ctn *Vec3) []int {
	//triangulate this loop using the ear cutting algorithm
	//return a list of faces (triples of vertex indices)
	//the loop is assumed to be closed - vert indices may apear more than once as there may be bridges to inner holes

	facelist := []int{}

	ll := len(loop.vi)
	for {
		//find an ear
		facelist = append(facelist, loop.findAndRemoveEar(m, ctn)...) //remove the ear and add it to the face list (mutates l.vi)

		if len(loop.vi) < 3 {
			break
		}
	}
	logit("loop of ", ll, "verts triangulated into ", len(facelist)/3, " faces")

	return facelist
}

func (ss *segSet) addFromEdgePens(ct *Tri, tm *mesh, vt, vn int, edgePens []int, cornerIsCut bool, output *mesh) { //collect alternating segments))

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

func (ss *segSet) otherEnd(v int) int {
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

func (ss *segSet) normalTo(m *mesh, v int, ctn *Vec3) *Vec3 {
	//return a vector normal to the segment that contains vertex v

	v2 := ss.otherEnd(v)

	return m.verts[v2].p.sub(m.verts[v].p).cross(ctn)
}

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

func (ss *segSet) getLoops(ct *Tri, m *mesh) []*loop {
	//return a list of loops - each loop is a closed list of segments

	loops := []*loop{}

	for {

		start := ss.unused()
		if start == nil {
			break
		} //no more unused segments
		at := start

		loop := NewLoop()
		for {
			loop.addVert(m, at.from)
			at.used = true
			at = ss.findSegFrom(at.to)
			if at == nil {
				break //open loop do not add (single triangle penetration)
			}

			if at.to == start.from {
				at.used = true
				loop.addVert(m, at.from) //todo - refactor
				loops = append(loops, loop)
				break
			}
		}

		logit("loop", loop.vi)

	}

	return loops
}

func (loop *loop) addVert(m *mesh, vi int) {
	loop.vi = append(loop.vi, vi)

	//check for verts in a line (can go eventually)
	if len(loop.vi) > 2 {
		l := len(loop.vi) - 1
		if m.inAline(loop.vi[l-2], loop.vi[l-1], loop.vi[l]) {
			panic("verts in a line")
		}
	}
}

func (l *loop) reverse() {
	//reverse the order of the verts in the loop
	//this is used to ensure the loop is wound in the correct direction for the ear cutting algorithm
	//the loop is mutated
	ll := len(l.vi)
	n := make([]int, ll)
	for i := 0; i < len(l.vi); i++ {
		n[i] = l.vi[(ll-i)-1]
	}
	l.vi = n
}

// check if the ear is empty (no other verts inside the triangle)
func (l *loop) isEarEmpty(bi int, m *mesh) bool {

	ll := len(l.vi) //length of the loop
	ai := (bi + ll - 1) % ll
	a := m.verts[l.vi[ai]].p
	b := m.verts[l.vi[bi]].p
	c := m.verts[l.vi[(bi+1)%ll]].p

	if m.inAline(l.vi[ai], l.vi[bi], l.vi[(bi+1)%ll]) {
		return false //triangle is degenerate and not one we want to keep
	}

	if a.equals(b) || a.equals(c) || b.equals(c) {
		panic("degenerate triangle")
	}

	for j := (bi + 2) % ll; j%ll != ai; j++ {
		v := m.verts[l.vi[j%ll]]

		if v.p.equals(a) || v.p.equals(b) || v.p.equals(c) {
			//panic("on ear vert") = can happen when there are hols
			return true //false
		}

		if v.p.isInsideTri(a, b, c, false) {
			return false
		}
	}
	return true

}

func (l *loop) findAndRemoveEar(m *mesh, ctn *Vec3) (face []int) {
	//find an ear and remove it from the loop
	//return the indices of the three verts that make up the ear
	//the loop is mutated and a vertex is removed

	biggestAngle := float64(0)
	ear := []int{0, 0, 0}
	ei := int(-1)
	logit("loop contains", len(l.vi))
	for i := 0; i < len(l.vi); i++ {

		a := l.vi[i]
		bi := (i + 1) % len(l.vi)
		b := l.vi[bi]
		c := l.vi[(i+2)%len(l.vi)]

		ap := m.verts[a].p
		bp := m.verts[b].p
		cp := m.verts[c].p

		ab := bp.sub(ap)
		bc := cp.sub(bp)

		if ab.length() < 0.01 || bc.length() < 0.01 {
			panic("degenerate triangle")
		}

		turnAngle := -bc.SignedAngleFrom(ab, ctn) //math.Acos((cp.sub(bp)).normalise().dot((bp.sub(ap)).normalise()))

		if turnAngle == 0 {
			//	panic("zero turn angle") //can happen when ears are cut off
		}

		if turnAngle > 0 { // angle is postive for a left hand turn,
			logit("acute angle", turnAngle)
			if turnAngle > biggestAngle {
				if l.isEarEmpty(bi, m) {
					biggestAngle = turnAngle
					ear = []int{a, b, c}
					ei = (i + 1) % len(l.vi) //remove the middle 'B' vertex
				}
			}
		} else {
			logit("reflex angle (right turn)", turnAngle)
		}
	}

	if ei == -1 {
		panic("no acute angle (or empty ear) found")
	}

	logit("Removing", ei, l.vi[ei])

	l.vi = append(l.vi[:ei], l.vi[ei+1:]...)

	if m.verts[ear[0]].p.equals(m.verts[ear[1]].p) || m.verts[ear[0]].p.equals(m.verts[ear[2]].p) || m.verts[ear[1]].p.equals(m.verts[ear[2]].p) {
		panic("degenerate ear by area")
	}
	if ear[0] == ear[1] || ear[0] == ear[2] || ear[1] == ear[2] {
		panic("degenerate ear")
	}

	return ear
}

// return a sorted list of vertex indices, from segs that meet the edge (by distance from corner)
func (ss *segSet) getEdgePenetrations(m *mesh, vThis, vNext int) []int {

	dm := map[float64]int{}

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

	sorted := make([]int, len(dm))
	for i, p := range index {
		if dm[p] < 3 {
			panic("bad")
		}
		sorted[i] = dm[p]
	}

	return sorted

}

type mesh struct {
	name  string
	verts []*vert //{}
	fi    []int   //{} //face indices
	yMax  float64 //= 0
}

type seg struct {
	from         int
	to           int
	used         bool
	touchesEdges int
}

type segSet struct {
	segs []*seg
}

func (ss *segSet) merge(other *segSet) {
	ss.segs = append(ss.segs, other.segs...)
}

func (ss *segSet) add(from, to int, touchesEdges int) {
	if from == to {
		panic("//degenerate segment")
	}
	ss.segs = append(ss.segs, &seg{from, to, false, touchesEdges})
}

func (ss *segSet) findSegFrom(from int) *seg {
	for _, s := range ss.segs {
		if s.from == from {
			return s
		}
	}
	return nil // penetrations of single triangles through faces create one segment that does not form a loop
	//panic("seg not found")
}

func newSegSet() *segSet {
	return &segSet{segs: []*seg{}}
}

func NewMesh(name string) *mesh {
	return &mesh{name: name, verts: []*vert{}, fi: []int{}}
}

func (m *mesh) addVert(p *Vec3, reUseVert bool, u float64, v float64) int {

	if reUseVert {
		for i, v := range m.verts {
			if v.p.equals(p) {
				return i //found an existing vert at this position - return its index
			}
		}
	}

	m.verts = append(m.verts, &vert{p: p, uv: Vector{u, v}, n: &Vec3{0, 0, 0}})
	return len(m.verts) - 1
}

func (m *mesh) addFi(vi ...int) {
	m.fi = append(m.fi, vi...)
}

func (m *mesh) rain(amount float64) {

	//pour an amount on every vertex proportional to altitude
	for _, v := range m.verts {

		distFromMid := (v.p.Y - (m.yMax / 2)) / m.yMax
		if distFromMid < 0 {
			distFromMid = 0
		}
		//rainfall := (1 - distFromMid) * 1 //rainfall is proportional to altitude - the midground is wettest
		rainfall := float64(amount)
		v.wl += rainfall
	}

}

func (m *mesh) flowWater() {

	//flow the water (only along the deepest faces)
	//for iw := 0; iw < 10; iw++ { //iteration of water
	for i := 0; i < len(m.fi); i += 3 {
		a := m.verts[m.fi[i]]
		b := m.verts[m.fi[i+1]]
		c := m.verts[m.fi[i+2]]
		flow(a, b)
		flow(b, c)
		flow(c, a)
	}
	//update the water levels (from the accumulators)
	for v := range m.verts {
		m.verts[v].wl += m.verts[v].acc
		m.verts[v].acc = 0
	}
	//}

}

func (m *mesh) sendWater(state *State) {

	wl := make([]int, len(m.verts)) //water level

	for i, p := range m.verts {
		wl[i] = int(p.wl * 10)
	}

	state.sendToAll(&reply{Cmd: "water", Payload: wl})

}

func (m *mesh) sendToAll(name string, state *State) {

	vc := len(m.verts)
	vc2 := vc * 2
	vc3 := vc * 3

	v := make([]int, vc3)  //position x,y,z
	n := make([]int, vc3)  //normal x,y,z triples
	uv := make([]int, vc2) //u,v pairs

	for i, p := range m.verts {
		v[i*3+0] = int(p.p.X * 10)
		v[i*3+1] = int(p.p.Y * 10)
		v[i*3+2] = int(p.p.Z * 10)
		//v[i*4+3] = int(p.wl * 10) //water level
		//wl[i] = int(p.wl * 10)

		n[i*3+0] = int(p.n.X * 100)
		n[i*3+1] = int(p.n.Y * 100)
		n[i*3+2] = int(p.n.Z * 100)

		uv[i*2+0] = int(p.uv.X * 1000)
		uv[i*2+1] = int(p.uv.Y * 1000)

	}

	meshPayload := meshPayload{Name: name, Verts: v, Faces: m.fi, Norms: n, UV: uv}
	state.sendToAll(&reply{Cmd: "mesh", Payload: meshPayload})

}

func makeLand(splits int, maxHeight float64, dist float64) *mesh {

	land := NewMesh("land") //&state.land    //get a reference to state.land (saves a lot of typing)
	land.verts = []*vert{}  //clear the verts

	seed := uint64(time.Now().Nanosecond())
	logit("seed:" + strconv.Itoa(int(seed)))

	rnGen = rand.New(rand.NewPCG(seed+1, seed))

	land.addOrReuseVertAtXZ(newVec3(0, 0, dist))
	land.addOrReuseVertAtXZ(newVec3(dist, 0, -dist))
	land.addOrReuseVertAtXZ(newVec3(-dist, 0, -dist))

	// hdist := dist / 2
	// lnd.addOrReuseVertAtXZ(newVec3(0, 0, hdist))
	// lnd.addOrReuseVertAtXZ(newVec3(hdist, 0, -hdist))
	// lnd.addOrReuseVertAtXZ(newVec3(-hdist, 0, -hdist))

	t := land.makeTri(0, 0, 1, 2) //make a depth 0 triangle within the mesh

	// r := newRing(0, 1, 2)
	// r.children = append(r.children, newRing(3, 4, 5))

	// r.triangulate(t)

	// //Recursively split landscape
	t.split(land, splits, maxHeight) //spring the triangle 4 times recursively
	numFaces := 1 << (2 * splits)    //left shift 2*splits - 4 splits = 16 faces

	land.fi = make([]int, numFaces*3) //face vert indices (three per triangle)

	p := int(0)
	t.gather(splits, land.fi, &p) //get all the indices of the verts at depth 4

	land.fi = land.fi[:p] //truncate (actually redundant)

	land.generateNormals()

	// //cull every trianlge below the sea
	// nfi := make([]int, len(fi))

	// o := 0
	// for i := 0; i < len(fi); i += 3 {

	// 	a := verts[fi[i]].p
	// 	b := verts[fi[i+1]].p
	// 	c := verts[fi[i+2]].p

	// 	if a.Y > 0 || b.Y > 0 || c.Y > 0 {
	// 		nfi[o] = fi[i]
	// 		nfi[o+1] = fi[i+1]
	// 		nfi[o+2] = fi[i+2]
	// 		o += 3
	// 	}
	// }

	// fi = nfi[:o] //keep the shortened face list

	//move every undewater vertex to the surface
	land.yMax = float64(0)
	for i := 0; i < len(land.verts); i++ {
		// 	if verts[i].p.Y < 0 {
		// 		verts[i].p.Y = 0
		// 	}
		// 	// verts[i].p.X += (rnGen.Float64() - float64(.5)) * 100
		// 	// verts[i].p.Z += (rnGen.Float64() - float64(.5)) * 100
		if land.verts[i].p.Y > land.yMax {
			land.yMax = land.verts[i].p.Y
		}

	}

	//generate UVs
	for _, v := range land.verts {
		//v := &lnd.verts[i] //DONT use range value here - we need to modify the actual vert (not a copy!)
		v.n = v.n.normalise()
		//use the angle of the normal projected onto x/y as the u component
		//TODO incororate slope of the terrain - if the terrain is flatter..
		//that the v component from lower down - this should put now on flat mountaintops
		//similarly north facing slopes should get their v component from higher in the map
		v.uv = Vector{math.Atan2(v.n.X, v.n.Z) / float64(6.28), v.p.Y / land.yMax}
	}

	//randomize Ys (AFTER) generating TC's
	for _, v := range land.verts {
		v.p.Y += (rnGen.Float64() - float64(.5)) * 100
		//if v.p.Y < 0 {
		//	v.p.Y = 0
		//}
	}

	y := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	t.getY(300, 200, y)

	logit(y)

	logit(p)
	logit(len(land.verts))

	return land

}

func testFloat(name string, f func() float64, expect float64, failMsg string) {

	v := f()
	if v != expect {
		logit("FAILED", name, failMsg, "expected:", expect, "got:", v)
	} else {
		logit("passed", name)
	}

}

func testBool(name string, f func() bool, expect bool, failMsg string) {

	v := f()
	if v != expect {
		logit("FAILED", name, failMsg, "expected:", expect, "got:", v)
	} else {
		logit("passed", name)
	}
}

func tests() {

	s := float64(100)
	t1 := NewMesh("t1")
	t1.addVert(newVec3(0, 0, s), false, 0, 0)   //far
	t1.addVert(newVec3(s, 0, -s), false, 0, 0)  //right
	t1.addVert(newVec3(-s, 0, -s), false, 0, 0) //left

	t1.fi = []int{0, 1, 2}

	tt := t1.triangleFrom(0)

	testFloat("Triangle, distance from point/plane (negative)", func() float64 { return tt.distanceFrom(newVec3(0, -50, 0)) }, -50, "distanceFrom wrong")
	testFloat("Triangle, distance from point/plane (positive)", func() float64 { return tt.distanceFrom(newVec3(0, 50, 0)) }, 50, "distanceFrom wrong")

	testBool("Triangle contains, (exclude verts and edges) - point inside",
		func() bool { return tt.contains(newVec3(0, 0, 0), false, false) }, true, "contains wrong")
	testBool("Triangle contains, (exclude verts and edges) - point outside", func() bool { return tt.contains(newVec3(100, 0, 0), false, false) }, false, "contains wrong")
	testBool("Triangle contains (point is vert) - include verts", func() bool { return tt.contains(newVec3(0, 0, 100), true, true) }, true, "contains wrong")
	testBool("Triangle contains (point is vert) - dont include verts", func() bool { return tt.contains(newVec3(0, 0, 100), false, false) }, false, "contains wrong")

	testBool("Triangle contains - on vertex - true", func() bool { return tt.contains(newVec3(0, 0, 100), false, false) }, false, "contains (on vertex)wrong")
	testBool("Triangle contains - on edge - true ", func() bool { return tt.contains(newVec3(0, 0, 100), true, true) }, true, "contains (on vertex) wrong")

	testFloat("Normal and edge are orthogonal ", func() float64 { return tt.normal().dot(tt.edge0()) }, 0, "normal and edge are not orthogonal")

	a := &Vec3{0, 2, 0}
	b := &Vec3{1.1, 0, 0}
	axis := &Vec3{0, 0, 1}
	testFloat("Positive angle", func() float64 { return b.SignedAngleFrom(a, axis) }, math.Pi/2, "angle wrong")
	testFloat("Negative angle", func() float64 { return a.SignedAngleFrom(b, axis) }, -math.Pi/2, "angle wrong")

	testFloat("Cross product orthogonal", func() float64 { return a.cross(b).dot(a) }, 0, "cross product not orthogonal")

	clay := NewMesh("clay")
	clay.addVert(newVec3(0, 100, 0), false, .5, 0)
	clay.addVert(newVec3(100, 0, -100), false, 1, 1)
	clay.addVert(newVec3(-100, 0, -100), false, 0, 1)
	clay.fi = []int{0, 1, 2}
	ct := clay.triangleFrom(0)

	tool := NewMesh("tool")
	tool.addVert(newVec3(0, 200, 0), false, 0, 0)
	tool.addVert(newVec3(100, -50, -100), false, 0, 0)
	tool.addVert(newVec3(-100, -50, -100), false, 0, 0)
	tool.fi = []int{0, 1, 2}
	tt = tool.triangleFrom(0)

	facePens := ct.penetrationsByEdgesOf(tt, false)

	logit(len(facePens.pens))

}

func tetra() *mesh {

	s := float64(100)

	// t0 := NewMesh("t0")
	// t0.addVert(newVec3(0, 0, s))
	// t0.addVert(newVec3(s, 0, -s))
	// t0.addVert(newVec3(-s, 0, -s))

	// t0.fi = []int{0, 1, 2}

	tests()

	//tree := growTree()

	// t1.fi = []int{0, 1, 2}

	// t1.cut(t0)

	tetra := NewMesh("clay")
	far := tetra.addVert(newVec3(0, 0, s), false, 0.5, 0)
	right := tetra.addVert(newVec3(s, 0, -s), false, 1, 1)
	left := tetra.addVert(newVec3(-s, 0, -s), false, 0, 1)
	top := tetra.addVert(newVec3(0, s, 0), false, 0.5, 0)

	tetra.fi = []int{
		far, left, right, //bottom
		top, right, left, //near/front face
		top, far, right, //right face
		top, left, far, //left face
	}

	//inside := newVec3(0, 1, 0)
	//outside := newVec3(-100, 100, 0)

	// testBool(func() bool { return inside.isInside(tetra) }, true, "inside is Outside tetra")
	// testBool(func() bool { return outside.isInside(tetra) }, false, "outside is inside tetra")

	t2 := tetra.clone()
	t2.name = "tool"
	t2.verts[3].p.Y = 200 //make a tall and pointy tool
	t2.verts[3].p.Z = 0   //make a tall and pointy tool
	t2.verts[3].p.X = 0   //make a tall and pointy tool

	t2.offset(newVec3(0, -50, 0)) //move it down 50 (and for towards the cam)
	t2.rotateAbout(newVec3(0, 1, 0), math.Pi/6)

	debugMesh := NewMesh("loop")
	result := t2.cut(tetra, debugMesh)

	result.generateNormals()

	//tree.generateNormals()
	//return tree //tetra //tetra //return the mutated tetra

	debugMesh.generateNormals()
	return debugMesh //result

}

func (m *mesh) rotateAbout(axis *Vec3, angle float64) {

	for _, v := range m.verts {
		v.p = v.p.rotateAbout(axis, angle)
	}

}
func (m *mesh) clone() *mesh {
	//return a copy of this mesh
	//the new mesh will have new verts, but the same faces
	//the new mesh will have the same yMax as the old mesh
	newMesh := NewMesh("clone of " + m.name)
	for _, v := range m.verts {
		newMesh.addVert(v.p.clone(), false, v.uv.X, v.uv.Y) //newVec3(v.p.X, v.p.Y, v.p.Z))
	}
	newMesh.fi = slices.Clone(m.fi) //important we clone (othewise we end up with a reference to the old fi slice)
	newMesh.yMax = m.yMax
	return newMesh
}

func (m *mesh) offset(offset *Vec3) {
	for _, v := range m.verts {
		v.p = v.p.add(offset)
	}
}

func (m *mesh) generateNormals() {
	//generate normals
	for i := 0; i < len(m.fi); i += 3 {

		a := m.verts[m.fi[i]]   //& lets us manipulate the verts by reference
		b := m.verts[m.fi[i+1]] //.p
		c := m.verts[m.fi[i+2]] //.p

		ab := b.p.sub(a.p)
		ac := c.p.sub(a.p)

		n := ab.cross(ac).normalise()

		if n.Y < 0 {
			//panic("faces down")
			n = n.multiply(-1)
		}

		a.n = a.n.add(n) //add the face normal to each vertex
		b.n = b.n.add(n)
		c.n = c.n.add(n)
	}

}

func (m *mesh) addOrReuseVertAtXZ(p *Vec3) int {

	for i, v := range m.verts {
		if v.p.X > p.X-0.01 && v.p.X < p.X+0.01 {
			if v.p.Z > p.Z-0.01 && v.p.Z < p.Z+0.01 {
				return i
			}
		}
	}

	return m.addVert(p, true, 0, 0)
}

// creates a triangle (which is NOT a face) from the indices of the verts
// holds a pointer to this mesh for access to its verts
// note - it does not add vertices or face indices to the mesh
func (m *mesh) makeTri(depth int, vi ...int) *Tri {
	if vi[0] == vi[1] || vi[0] == vi[2] || vi[1] == vi[2] {
		panic("degenerate triangle")
	}

	p0 := m.verts[vi[0]].p
	p1 := m.verts[vi[1]].p
	p2 := m.verts[vi[2]].p

	if p0.equals(p1) || p0.equals(p2) || p1.equals(p2) {
		panic("infinitely thin triangle")
	}

	return &Tri{depth: depth, Vi: vi, Children: []*Tri{}, mesh: m}
}

func (m *mesh) inAline(a, b, c int) bool {

	ap := m.verts[a].p
	bp := m.verts[b].p
	cp := m.verts[c].p

	//if any two points are the same - all three are in a line
	if ap.equals(bp) || ap.equals(cp) || bp.equals(cp) {
		return true
	}

	if ap.distanceFromLine(bp, cp) < 0.001 {
		return true
	}
	return false

}

func (m *mesh) triangleFrom(fi int) *Tri {

	if fi%3 != 0 {
		panic("panic - not a face index")
	}

	vi := m.fi[fi : fi+3] //slice three vert indices from the face list
	p0 := m.verts[vi[0]].p
	p1 := m.verts[vi[1]].p
	p2 := m.verts[vi[2]].p

	if p0.equals(p1) || p0.equals(p2) || p1.equals(p2) {
		panic("infinitely thin triangle")
	}

	return &Tri{depth: 0, Vi: vi, Children: []*Tri{}, mesh: m}
}

// func (ps *penSet) setNeighbours() {

// 	for _, p:= range ps.pens{
// 		if p.isBoundary{
// 			//you can have a negbouring outer vertex, edge penetration or face penetration
// 			nfp:= ps.getPenetration(p.clayTri)
// 			nep:= p.getnextEdgePenetration() //find the next edge penetration on the same edge as this
// 			nv:= p.getNextVertOutervert() //find the next vert penetration on the same edge as this

// 		}
// 	}

// }

func (tool *mesh) cut(clay *mesh, debugMesh *mesh) *mesh {
	//the tool is unharmed (for now)
	//the clay has its faces subdivided (and vertices added) where the tool penetrates
	//its facelist is replaced

	if tool == clay {
		panic("tool and clay are the same mesh")
	}

	facelist := []int{}

	//for each face of the clay - DONT include the ones we add !!

	//DO NOT Mess with the original mesh - clone it
	output := clay.clone()

	for i := 0; i < len(clay.fi); i += 3 {

		ct := clay.triangleFrom(i)
		ctn := ct.normal()

		//for each triangle face of the tool mesh
		logit("before", len(output.verts))
		segs := ct.internalSegmentsFromPenetrationsOf(tool, output)
		logit("after", len(output.verts))
		//segs = segs.discardOpenSegments() //segments that touch an edge only once are discarded (they are flat faces (or chains therof) intruding)
		edgeSegs := ct.joinEdgePenetrations(segs, tool, output)
		if len(edgeSegs.segs) < 3 {
			panic("not enough edge segs")
		}
		segs.merge(edgeSegs) //add segments to close the outer ring(s) from the original triangle

		//we now have many directed segments - that should form closed loops, none of which should intersect
		//some of them may entirely contain others

		loops := segs.getLoops(ct, output)

		for l := 1; l < len(loops); l++ {
			//loops[l].reverse()
			loops[0].merge(loops[l])
		}

		loops[0].mesh(output, ctn, debugMesh)

		//trianglulate loop 0 (which contains all bridged/merged loops)
		if len(loops) == 0 {
			logit("no loops !!")
		} else {
			facelist = append(facelist, loops[0].triangulate(debugMesh, output, ctn)...) //triangulates around the child holes held in the ring (at ct.depth+1)
		}

	}

	output.fi = facelist
	output.generateNormals()

	debugMesh.generateNormals()
	return output //debugmesh

}
