package main

//import "crypto/rand"

import "math/rand/v2"

import "math"

type vert struct {
	p       *vec3
	n       *vec3
	uv      vec2
	wl      float64       //water level
	acc     float64       //accumulated water (during a pass)
	incount int           //number of ferts flowing into this vert
	touches map[*tri]bool //the triangles that touch this vertex (whos face normals contribute to the vertex normal)
	// a *vert //the two ends of the edge I was created on
	// b *vert
	// depth int
	//touchCount int
}

type tri struct {
	depth    int
	vi       []uint32
	children []*tri
	mesh     *landMesh //a reference to the mesh this tri is part of (that the vi's point into v's of)
	normal   *vec3
	scorched bool
	cull     bool

	//faceIndex uint16
	//isPatch   bool
}

// keep track of the deepest (up to) 6 triangles touching this vert -- allows us to recalculate normals quickly
func (v *vert) touch(t ...*tri) {

	for _, t := range t {
		v.touches[t] = true
	}
}

func newVert(p *vec3, u, v float64) *vert {
	return &vert{p: p, uv: vec2{u, v}, n: &vec3{0, 0, 0}, touches: make(map[*tri]bool, 6)}
}

// func (ft *fTri) find(p *vec3, fm *fireMesh) *fTri {
// 	if len(ft.children) == 0 {
// 		if ft.contains(p, fm) {
// 			return ft
// 		}
// 		return nil
// 	}
// 	for _, c := range ft.children {
// 		f := c.find(p, fm)
// 		if f != nil {
// 			return f
// 		}
// 	}
// 	logit("warn: fTri find failed to find a tri")
// 	return nil
// }

func (ft *fTri) find(p *vec3, fm *fireMesh) *fTri {

	if ft.fastContains(p, fm) {
		if len(ft.children) == 0 {
			return ft
		}

		scorched := 0
		for _, c := range ft.children {
			if c.flames == -1 {
				scorched++
			}
			f := c.find(p, fm)
			if f != nil {
				return f
			}
		}
		if scorched == len(ft.children) {
			ft.flames = -1 //mark parent as scorched
			ft.children = nil
		}
	} else {
		return nil
	}
	logit("warn: fTri find failed to find a tri")
	return nil
}

func (fm *fireMesh) scorchedAt(t *tri) bool {
	p := t.centre()
	p.y = 0

	leaf := fm.root.find(p, fm) //once leafs have burned out - they can be retracted into a single scorched parent

	if leaf == nil {
		return false //no leaf node here
	}
	if leaf.flames == -1 || leaf.flames > 10 {
		return true
	}

	return false
}

func (t *tri) addChild(vi ...uint32) *tri {
	child := newTri(t.mesh, t.depth+1, vi...) //t.mesh.makeTri(t.depth+1, fi, a, b, c)
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

// func (m *mesh) vertAtMidPointXZ(a, b uint16) uint16 {
// 	p := m.verts[a].p.tween(m.verts[b].p, 0.5)

// 	tiny := 0.0001
// 	for i, v := range m.verts {
// 		if v.p.x > p.x-tiny && v.p.x < p.x+tiny {
// 			if v.p.z > p.z-tiny && v.p.z < p.z+tiny {
// 				return uint16(i)
// 			}
// 		}
// 	}
// 	return 65535

// }

//for every bottom level triangle, look to see if there is a vertex at the midpoint of each edge (caused by a more divided neighbouring tri)
//if so, split in two to the opposite vertex
func (t *tri) patch() {

	m := t.mesh
	if len(t.children) == 0 && !t.cull {

		for i := 0; i < 3; i++ {

			ai := t.vi[i]
			bi := t.vi[(i+1)%3]
			ci := t.vi[(i+2)%3]
			//mi := m.midpoint(ai, bi) //looks both ways for a midpoint   -- m.vertAtMidPointXZ(ai, bi) //this could be way faster by recording mid verts
			mi := m.midpoint(bi, ci) //looks both ways for a midpoint   -- m.vertAtMidPointXZ(ai, bi) //this could be way faster by recording mid verts

			if mi != math.MaxUint32 { //is there a midpoint ?

				//if t.aspect() < 2 { //is it 'fat'
				t.splitIn2(ai, bi, ci, mi)
				break //only one edge of this tri (becuase it is now multiple child tris)
				//} else {
				//	t.split()
				//}

			}

		}

	}

	for _, c := range t.children {
		c.patch()
	}

}
func (t *tri) yMin() float64 {
	v := t.mesh.verts
	return math.Min(math.Min(v[t.vi[0]].p.y, v[t.vi[1]].p.y), v[t.vi[2]].p.y)
}

func (t *tri) yMax() float64 {
	v := t.mesh.verts
	return math.Max(math.Max(v[t.vi[0]].p.y, v[t.vi[1]].p.y), v[t.vi[2]].p.y)
}

func (t *tri) addToTouches() {

	m := t.mesh
	m.verts[t.vi[0]].touch(t)
	m.verts[t.vi[1]].touch(t)
	m.verts[t.vi[2]].touch(t)

}

func (t *tri) removeFromTouches() {
	if len(t.children) > 0 {
		panic("Tri has children")
	}
	for _, vi := range t.vi {
		v := t.mesh.verts[vi]
		delete(v.touches, t) //remove this tri from the list of tris touching this vertex
	}
}

func (t *tri) facesTowards(direction *vec3) bool {
	//the extra -.1 is to account for traingles facing away at less than half the camera vertical FOV
	return t.normal.dot(direction) < -.1 //is the traingle forward facing ? (relative to the camera)

}

func (t *tri) allVertsLeftOrRightOfFov(pos *vec3, focus *vec3, fov float64) bool {

	camDir := focus.sub(pos).normalise()
	onLeft := 0
	for _, vi := range t.vi {
		cam2vert := t.mesh.verts[vi].p.sub(pos).normalise()
		if camDir.dot(cam2vert) > fov {
			return false //a vertex is within the FOV
		}
		//we're outside the FOV
		cp := cam2vert.cross(camDir)
		if cp.y < 0 {
			onLeft++
		}
	}

	if onLeft == 3 {
		return true //all vertices are outside and on the same side of the FOV

	}
	if onLeft == 0 {
		return true
	}

	return false //vertices straddle the viewing frustum

}

func (t *tri) hasVertexWithinFov(pos *vec3, focus *vec3, fov float64) bool {

	camDir := focus.sub(pos).normalise()
	for _, vi := range t.vi {
		cam2vert := t.mesh.verts[vi].p.sub(pos).normalise()
		if camDir.dot(cam2vert) > fov {
			return true //a vertex is within the FOV
		}
	}

	return false //no vertices are within the FOV

}

func (t *tri) countChildren(count *int) {

	*count += len(t.children)

	for _, c := range t.children {
		c.countChildren(count)
	}

}

//get an on screen area
func (t *tri) pixels(camPos *vec3) float64 {

	a := t.mesh.verts[t.vi[0]].p
	b := t.mesh.verts[t.vi[1]].p
	c := t.mesh.verts[t.vi[2]].p

	cama := a.sub(camPos).normalise()
	camb := b.sub(camPos).normalise()
	camc := c.sub(camPos).normalise()

	ab := cama.dot(camb)
	if ab < 0 {
		return 100
	} //this triange has vertices in front of and behind the camera
	bc := camb.dot(camc)
	if bc < 0 {
		return 100
	} //this triangle has vertices in front of and behind the camera
	ca := camc.dot(cama)
	if ca < 0 {
		return 100
	} //this triange has vertices in front of and behind the camera

	ab = 1 - ab
	bc = 1 - bc
	ca = 1 - ca

	s := (ab + bc + ca) / 2 //half the perimeter

	return s

	//A = √[s(s-a)(s-b)(s-c)]
	p := s * (s - ab) * (s - bc) * (s - ca)

	if p < 0 {
		return 0
		logit("negative area", s, ab, bc, ca)

	}
	area := math.Sqrt(p) //area of the triangle (Herons formula)

	return area

}
func (t *tri) splitIfNeeded(pos *vec3, focus *vec3) {

	//      0
	//		/\
	//     /  \
	//  5 /____\ 3
	//   / \  / \
	//  /___\/___\
	// 2     4    1
	if t.depth >= len(t.mesh.kinks) {
		return
	}

	//triCentre := t.centre()

	//if t.hasVertexInFrontOf(pos,focus){
	//if t.facesTowards(focus.sub(pos)) { //is the traingle forward facing ? (relative to the camera)
	camDir := focus.sub(pos).normalise()
	inFov := !t.allVertsLeftOrRightOfFov(pos, focus, .4)
	if t.depth < 5 || inFov { //high numbers here gives a narrow field of view getting split
		//if t.normal.dot(camDir) < -0.1 { //is the traingle forward facing ? (relative to the camera)
		if t.normal.dot((t.centre().sub(pos)).normalise()) < 0.3 { //lower number here cull more backfacing tris

			dist := pos.distanceFrom(t.centre())
			//apud := (2 * t.area()) / (dist * dist)
			apud := (2 * t.area()) / (0.0005 * (dist * dist))

			dp := camDir.dot(t.centre().sub(pos).normalise())
			//at a value of 1 (area per unit distance), a notional 100 square metre square, would require splitting when it was 10 metres away
			if apud > 8-(dp*4) || t.depth < 5 { //.001 is a about 1cm triangles at the horizon

				//logit("splitting", t.depth, apud, dist, t.area())
				t.split()
				for _, c := range t.children {
					c.splitIfNeeded(pos, focus) //recurse
				}
			}

		} //else {
		//		t.cull = true
		//		}
		//}
	}

	//}

	if len(t.children) == 0 && t.depth > 5 && t.normal.dot((t.centre().sub(pos)).normalise()) > 0.2 {
		t.cull = true
		//final triangle is backfacing - cull it
	}

}

//}

func (t *tri) splitDownTo(level int) {

	if t.depth < level {
		t.split()
		for _, c := range t.children {
			c.splitDownTo(level) //recurse
		}
	}

}

//extract children of T within a FOV, to a level based on distance from the camera and being in view
//populate the mapDown list with the used vertex indices
func (t *tri) extract(pos *vec3, focus *vec3, into *tri, mapDown map[uint32]uint32) {

	//if t.depth < t.mesh.splits {

	//was 0.7
	if !t.allVertsLeftOrRightOfFov(pos, focus, 0.7) { //if the triangle straddles the viewing frustum
		pixels := t.pixels(pos) //returns half the permiter of the traingle in units of dot product
		if pixels > 0.001 {     //smaller number here gives more detail 0.0001 is a lot of detail
			// 	if pixels < 100 {
			// 		//logit("split ", t.depth, pixels)
			// 	}

			//map the vertices from the giant map into a vertex list for this mesh
			vt := make([]uint32, 3)
			for i, bvi := range t.vi { //for each vertex of this triangle
				svi, present := mapDown[bvi]
				if !present {
					svi = uint32(len(mapDown))
					mapDown[bvi] = svi
				}
				vt[i] = svi
			}

			//n := into.addChild(vt...) - we don't want to addchild, because we dont wat to calc normals, or add midpoints, or add to touches

			n := tri{depth: into.depth + 1, vi: vt, children: []*tri{}, mesh: nil}
			//into.children = append(t.children, &n)
			into.children = append(into.children, &n)
			if len(into.children) > 4 {
				//	logit("more than 4 children in a triangle", len(into.children), into.depth, into.aspect(), pixels)
			}

			for _, c := range t.children {
				c.extract(pos, focus, &n, mapDown) //recurse
			}
			//} else {
			//logit("not splitting", t.depth, t.aspect(), pixels)
			//}

		}

	}

}

func (t *tri) splitIn2(a, b, c, m uint32) {

	t.removeFromTouches() //remove this tri from the list tris touching this vertex
	t.addChild(a, m, c)   //left (clockwise wound)
	t.addChild(a, b, m)   //right

}

//returns the longest edge of the triangle divided by the shortest edge - so a big number is a 'slinny triangle (and no triangle can be 'fatter' than 0.5)
func (t *tri) aspect() float64 {

	v := t.mesh.verts
	a := v[t.vi[0]].p
	b := v[t.vi[1]].p
	c := v[t.vi[2]].p

	ab := a.sub(b).length()
	ac := a.sub(c).length()
	bc := b.sub(c).length()

	return max(ab, ac, bc) / min(ab, ac, bc)

}

func (t *tri) split() {
	if len(t.children) == 0 {

		//	kink := (t.mesh.height/(float64(t.depth*t.depth)+1) - 1) * .5 //maximum kink in this edge
		//kink := (t.mesh.height/(float64(t.depth*5)+1) - 1) * .5 //maximum kink in this edge
		//kink := -t.mesh.height / math.Pow(2, float64(t.depth))
		kink := t.mesh.kinks[t.depth] * t.mesh.height //maximum kink in this edge

		m := t.mesh
		v0 := t.vi[0]
		v1 := t.vi[1]
		v2 := t.vi[2]

		seed := uint64(m.verts[v1].p.y)
		rnGen = rand.New(rand.NewPCG(seed, seed+1))
		//rnGe§n = &rand.New(rand.NewPCG(m.verts[v0].p.x, m.verts[v0].p.y))

		rn1 := rnGen.NormFloat64() //random number between -1 and 1
		rn2 := rnGen.NormFloat64()
		rn3 := rnGen.NormFloat64()

		v3 := m.splitEdge(v0, v1, rn1*kink, t.depth)
		v4 := m.splitEdge(v1, v2, rn2*kink, t.depth)
		v5 := m.splitEdge(v2, v0, rn3*kink, t.depth)

		t.removeFromTouches() //the list of tirangles touching a vertex is used for normal calculation

		t.addChild(v0, v3, v5) //top
		t.addChild(v3, v1, v4) //right
		t.addChild(v5, v4, v2) //left
		t.addChild(v3, v4, v5) //centre

	} else {
		logit("splitting a triangle that already has children ??")
	}
}

// //plough a runway between p1 and p2 flattening all points with 10 metres
// func (m *mesh) plough(p1 *vec3, p2 *vec3, vis ...uint16) {
// 	for _, vi := range vis {
// 		p := m.verts[vi].p
// 		if p.distanceFromLineSegment(p1, p2) < 40 {
// 			p.y = p.closestPointOnLineSegment(p1, p2).y
// 		}
// 	}
// }

func (t *tri) centre() *vec3 {
	v := t.mesh.verts
	return v[t.vi[0]].p.add(v[t.vi[1]].p).add(v[t.vi[2]].p).multiply(float64(1) / 3)
}

func (t *tri) area() float64 {
	v := t.mesh.verts
	a := v[t.vi[0]].p
	b := v[t.vi[1]].p
	c := v[t.vi[2]].p
	return a.sub(b).cross(a.sub(c)).length() / 2
}

func (t *tri) contains(pop *vec3, includeOnVert bool, includeOnEdge bool) bool {

	v := t.mesh.verts //this is a reference not a copy

	a := v[t.vi[0]].p
	b := v[t.vi[1]].p
	c := v[t.vi[2]].p

	return pop.isInsideTri(a, b, c, t.normal, includeOnEdge, includeOnVert)

}

//returns the deepest (i.e. childless) triangle intersected by the ray from p0 to p1
//maintaining a count, and populating the slice of penetrations by refererence is easier to get your head around than appending slices (possibly faster too)
func (t *tri) probe(p0 *vec3, p1 *vec3, pens []*vec3, penCount *int) []*vec3 {

	pop := t.probePlane(p0, p1) //fire a 10km ray
	if pop != nil {

		if len(t.children) == 0 {
			if t.contains(pop, true, true) {

				pens[*penCount] = pop
				*penCount++

			}

		}
	}

	for _, ct := range t.children {
		ct.probe(p0, p1, pens, penCount)

	}

	return pens

}

func (t *tri) probeLand(p *vec3) (surfacePoint *vec3, surfaceTri *tri) {

	pc := p.clone()
	pc.y = 0
	found := t.vProbe(pc) //recursively find the leaf tri that contains the point

	if found == nil {
		return nil, nil
	}
	//fire a ray through that plane
	return found.probePlane(newVec3(p.x, -100000, p.z), newVec3(p.x, 100000, p.z)), found
}

func (t *tri) vProbe(p *vec3) *tri {

	if p.y != 0 {
		logit("warn: tri vProbe Y is not 0")
	}
	a := t.mesh.verts[t.vi[0]].p.clone()
	b := t.mesh.verts[t.vi[1]].p.clone()
	c := t.mesh.verts[t.vi[2]].p.clone()

	a.y = 0
	b.y = 0
	c.y = 0

	up := &vec3{0, 1, 0}
	if p.isInsideTri(a, b, c, up, true, true) {
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

func (t *tri) calcNormal() *vec3 {

	v := t.mesh.verts

	numMeshVerts := uint32(len(v))
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

// func (ct *tri) joinEdgePenetrations(segs *segSet, toolMesh *landMesh, output *landMesh) *segSet {
// 	//create the outer loop of the clay triangle by walking the edges and their penetrations

// 	outerSegs := newSegSet()
// 	//create segments to close the outer ring(s) from the original triangle
// 	//ctn := ct.normal()

// 	if len(ct.vi) > 3 {
// 		panic("not a traingle")
// 	}

// 	for i := 0; i < 3; i++ { //range ct.Vi { //for each vertex of the clay triangle

// 		vt := ct.vi[i]
// 		vn := ct.vi[(i+1)%3]

// 		edgePens := segs.getEdgePenetrations(output, vt, vn) //get a sorted (by distance from vt) set of edge penetrations
// 		if len(edgePens) == 0 {
// 			outerSegs.add(vt, vn, 0) //this an unpentrated outer edge of the triangle
// 		} else {
// 			corner := output.verts[vt].p //the corner of the triangle
// 			// ep0 := ct.mesh.verts[edgePens[0]].p //the first penetration point
// 			// cornerIsCut := ep0.sub(corner).dot(segs.normalTo(ct.mesh, edgePens[0], ctn)) > 0
// 			cornerIsCut := corner.isInside(toolMesh)
// 			outerSegs.addFromEdgePens(toolMesh, vt, vn, edgePens, cornerIsCut, output) //collect alternating segments
// 		}

// 	}

// 	return outerSegs

// }

// func (ss *segSet) discardOpenSegments() *segSet {

// 	keep := newSegSet()
// 	for _, s := range ss.segs {
// 		if s.touchesEdges == 0 || s.touchesEdges == 2 { //it's an internal segment (from a pointy triangle penetrating the face somewhere in the middle) - OR it clips off one corner
// 			keep.add(s.from, s.to, s.touchesEdges)
// 		} else {
// 			if s.connectsToEdge(ss) {
// 				keep.add(s.from, s.to, s.touchesEdges)
// 			}
// 		}
// 	}

// 	return keep

// }

// return the point of intersection of a ray with the triangle
func (tri *tri) probePlane(p0 *vec3, p1 *vec3) *vec3 {

	d := p1.sub(p0) //the direction of the ray
	if p0.equals(p1) {
		panic("Degenerate probing ray")
	}

	if tri.normal.length() < 0.999 {
		panic("normal not normalised")
	}

	if tri.normal.dot(d) == 0 {
		return nil //this is legit, consider a traingle on the x/y plane and an edge of a triangle else where that is paralell with the X/Y plane
		//panic("ray is parallel to the plane of the triangle")
	}

	if tri.mesh.verts[tri.vi[0]].p.equals(p0) || tri.mesh.verts[tri.vi[1]].p.equals(p0) || tri.mesh.verts[tri.vi[2]].p.equals(p0) {
		//panic("p0 is a vertex of the triangle being probed")
		return p0
	}

	if tri.mesh.verts[tri.vi[0]].p.equals(p1) || tri.mesh.verts[tri.vi[1]].p.equals(p1) || tri.mesh.verts[tri.vi[2]].p.equals(p1) {
		//panic("p1 is a vertex of the triangle being probed")
		return p1
	}

	d0 := p0.signedDistanceFromPlaneOf(tri) //))tri.distanceFrom(p0, true)
	d1 := p1.signedDistanceFromPlaneOf(tri)

	// if d0 == 0 || d1 == 0 {
	// 	logit("probe on the plane")
	// 	return nil
	// }

	//if the distances have the same sign, the ray doesnt cross the plane
	if d0 > 0 && d1 > 0 || d0 < 0 && d1 < 0 {
		return nil
	}

	t := abs(d0) / (abs(d0) + abs(d1))

	if t > 1 {
		panic("t>1")
	}

	if t == 0 || t == 1 {
		logit("probe touches plane")
	}

	if t >= 0 && t <= 1 { //this is significant includes ray ends touching planes
		pop := p0.tween(p1, t)
		pd := pop.signedDistanceFromPlaneOf(tri)
		if pd > 0.05 || pd < -0.05 {
			logit("tween is not on the plane")
			pop.signedDistanceFromPlaneOf(tri)
		}
		return pop
	}
	//	panic("Probe failed")

	return nil
}

func abs(a float64) float64 {
	if a < 0 {
		return -a
	}
	return a
}

// func (s *seg) connectsToEdge(ss *segSet) bool {

// 	//TODO Implement !
// 	return true //false
// }

//for testing
func (t *tri) edge0() *vec3 {
	return t.mesh.verts[t.vi[1]].p.sub(t.mesh.verts[t.vi[0]].p)
}

func (t *tri) check() {

	m := t.mesh

	if m.verts[t.vi[0]].p.equals(m.verts[t.vi[1]].p) || m.verts[t.vi[0]].p.equals(m.verts[t.vi[2]].p) || m.verts[t.vi[1]].p.equals(m.verts[t.vi[2]].p) {
		panic("triangle has two or more vertices at the same point")
	}
	if m.inAline(t.vi[0], t.vi[1], t.vi[2]) {
		panic("triangle is a line")
	}

}

//return 0, 1 or 2 points of intersection of the edges of the tool triangle, with the plane of triangle a
func (clayTri *tri) penetrationsByEdgesOf(toolTri *tri, isEdgePen bool) *penSet {

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

func newTri(m *landMesh, depth int, vi ...uint32) *tri {

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
	t := tri{depth: depth, vi: vi, children: []*tri{}, mesh: m}

	//add this traingle to its verts list of triangles
	t.addToTouches()

	// m.fi[fi*3] = vi[0]
	// m.fi[fi*3+1] = vi[1]
	// m.fi[fi*3+2] = vi[2]

	t.calcNormal()

	return &t
}

// func (ct *tri) internalSegmentsFromPenetrationsOf(tool *landMesh, output *landMesh) *segSet {

// 	segs := newSegSet()

// 	if ct.mesh == tool {
// 		panic("tool and clay are the same mesh")
// 	}

// 	ctn := ct.normal

// 	for j := 0; j < len(tool.fi); j += 3 {
// 		tt := tool.triangleFrom(j)
// 		ttn := tt.normal

// 		tt.check()

// 		//each pen has a toolTri, a claytri, v1, v2 and a point
// 		//the v1 and v2 are always the indices of the penetrating edge

// 		//facePens := ct.penetrationsByEdgesOf(tt, false) //they *may* be on the edge of the clay triangle - or not
// 		t := tt.penetrationsByEdgesOf(ct, true)
// 		touchesEdges := len(t.pens)
// 		if len(t.pens) == 2 {
// 			logit("two edge penetrations")
// 		}
// 		if len(t.pens) > 2 {
// 			panic("More than 2 edge penetrations")
// 		}

// 		if len(t.pens) != 2 {

// 			facePens := ct.penetrationsByEdgesOf(tt, false) //they *may* be on the edge of the clay triangle - or not

// 			for _, fp := range facePens.pens {
// 				if t.penAt(fp.p) == nil {
// 					t.pens = append(t.pens, fp)
// 				}
// 			}
// 		}

// 		//we want the union of these - but edgepens should overried facepens at the same point

// 		// penPair := NewPenSet()
// 		// for p := range facePens.pens {
// 		// 	oep := edgePens.penAt(p.p) //is there an overring edge penetration at this point?
// 		// 	if oep != nil {
// 		// 		penPair.pens = append(penPair.pens, p, oep)
// 		// 	} else {
// 		// 		penPair.pens = append(penPair.pens, p)
// 		// 	}
// 		// }

// 		if len(t.pens) > 0 {
// 			if len(t.pens) != 2 {
// 				e := ct.penetrationsByEdgesOf(tt, false)
// 				f := tt.penetrationsByEdgesOf(ct, true)
// 				logit(len(e.pens), len(f.pens))

// 				logit("unmatched pen")
// 				continue //break
// 			}
// 			//pens.append(t)

// 			v0 := output.addVert(t.pens[0].p, true, 0, 0) //add a vertex at this point (or reuse an existing one)
// 			v1 := output.addVert(t.pens[1].p, true, 0, 0) //add a vertex at this point (or reuse an existing one) TODO TC's

// 			v := t.pens[1].p.sub(t.pens[0].p) //vector between the pair of penetrations points (of this tool face on this clay face)

// 			dir := v.cross(ttn).dot(ctn)
// 			if dir == 0 {
// 				panic("zero cross product dotted")
// 			}
// 			if dir > 0 { //add a directed segment (this is very significant and affects the winding of holes)
// 				segs.add(v0, v1, touchesEdges)
// 			} else {
// 				segs.add(v1, v0, touchesEdges)
// 			}
// 		}

// 	}

// 	return segs

// }
