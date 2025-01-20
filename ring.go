package main

import "slices"
import "math"

//records a set of vertex indices within two arms of a sector (used during triangulation)
type sectorPoints struct {
	p []*angleIndex
}

type angleIndex struct {
	angle float64
	vi    int
	d     float64 //distance from the hinge of the sector
}

//keep only the sector point with the smallest distance d (from the hinge of the sector)
func (s *sectorPoints) update(angle float64, vi int, d float64) {
	for _, p := range s.p {
		if angle == p.angle {
			if d < p.d {
				p.d = d
				p.vi = vi
				return
			}
		}
	}
	s.p = append(s.p, &angleIndex{angle, vi, d})
}

func NewSectorPoints() *sectorPoints {
	return &sectorPoints{p: []*angleIndex{}}
}

type Ring struct {
	vi       []int
	children []*Ring
}

func newRing(vi ...int) *Ring {
	return &Ring{vi: vi, children: []*Ring{}}
}

func (ring *Ring) addChild(vi ...int) {
	ring.children = append(ring.children, newRing(vi...))
}

func (ring *Ring) addClosedLoops(Pens *penSet, clay *mesh) {
	pen := Pens.unused() //select an arbitrary first unused penetration

	if pen == nil {
		return
	} //no closed loop penetrations

	if pen.isBoundary {
		panic("Boundary penetration after boundary processing")
	}
	for {
		childRing := newRing([]int{}...)
		for {
			childRing.append(clay.addVert(pen.p, false, 0, 0))
			pen.markUsed()

			//find an 'opposite' face penetration, at the same position (to determine the adjoining face)
			op := Pens.oppositePenByEdge(pen)
			if op == nil {
				panic("no opposite face penetration")
			} else {
				pen = Pens.otherPenByToolFace(op)
				if pen == nil {
					panic("no otherPenByFace")
				}
			}

			if pen.used {
				break //loop complete
			}
		}

		ring.addChild(childRing.vi...)

		if Pens.usedCount == len(Pens.pens) { //we have used all penetrations
			break
		}

		pen = Pens.unused() //find an unused pen
		if pen == nil {
			panic("no unused penetration after count match")
		}
	}
}
func (ring *Ring) append(vi int) {
	if slices.Contains(ring.vi, vi) {
		panic("duplicate vertex")
	}
	ring.vi = append(ring.vi, vi)
}

func (ps *penSet) getPenetration(penetrator *Tri) *pen {

	for _, p := range ps.pens {

		if p.toolTri == penetrator {
			return p
		}

	}
	panic("penetration not found")

}

func (ps *penSet) penAt(p *Vec3) *pen {
	for _, pen := range ps.pens {
		if pen.p.equals(p) {
			return pen
		}
	}

	return nil
}

func (ring *Ring) addOpenLoops(pens *penSet, clay *mesh) {
	//deal with the edge-to-edge open loops - which may cleave this triangle into polygon (and edge intrusions)

	//start on the outer edge (clay triangle) at the vertex farthest (signed) from the tool face

	//walk the edges, and into any edge intrusions

	//splice the open loops into the polygon of the clay triangle

	// vi= clayTri.vertFarthestFrom(tooltri)

	// nr:=newRing([]int{}...)
	// nr.append(vi)
	// for{
	//  	//the clay triangles edges are penetrated by the tool face
	// 		for p:= range penset.EdgePens(thisV,nextV) {
	// 			if p!=nil &&p!=nr.lastPoint(){
	// 				nr.addOpenLoopStartingAt(p) //pppend(p.vi)
	// 			}
	// 		}
	// }
	// 				//p=penset.getPenetration(p.clayTri,p) //get the other}
	// 	if !p==nil{
	// 		vi=p.vi
	// 	}

	// }

	// add the holes
	//do the ear clipping triangulation

	// for { //there may be many open loops
	// 	pen := pens.unusedBoundaryPen() //select an arbitrary first unused edge penetration
	// 	startPen := pen
	// 	if pen == nil {
	// 		break
	// 	} //we're done
	// 	childRing := newRing([]int{}...)
	// 	for {
	// 		pen.markUsed()
	// 		childRing.append(clay.addVert(pen.p))
	// 		//pen = Pens.otherPenByToolFace(pen) //there will always be two penetraions (one, or both, may be boundary penetrations)

	// 		if pen.isBoundary { //we start at a boundary
	// 			if pen != startPen {
	// 				break //we are done (we've gone boundary to boundary)
	// 			}
	// 			pen = pens.getPenetration(pen.clayTri, pen)
	// 		} else {

	// 			outpen := pens.oppositePenByEdge(pen)
	// 			if outpen == nil {
	// 				//this was an intrusion accross a face that never exited - we are done
	// 				break
	// 			}
	// 			outpen.markUsed()
	// 			//pen = pens.otherPenByToolFace(outpen)
	// 			pen = pens.getPenetration(outpen.toolTri, outpen)
	// 		}
	// 	}
	// 	ring.addChild(childRing.vi...)

	// }

}

//triangulate an arbitrary polygon, which may have holes
func (ringWithChildHoles *Ring) triangulate(mesh *mesh) []int {
	//return a list of triangles that cover the area of the polygon

	if len(ringWithChildHoles.vi) < 3 {
		panic("ring must have at least 3 vertices for triangulation")
	}
	if len(ringWithChildHoles.vi) == 3 && len(ringWithChildHoles.children) == 0 {
		//panic("no need to triangulate a triangle")
		return ringWithChildHoles.vi
	}

	//faces := []Tri{}
	rayList := make([]int, 0) //vert index point pairs of the 'rays' used to triangulate the polygon
	fi := []int{}
	for i, vi := range ringWithChildHoles.vi {

		a := vi //state.land.verts[vi] //for each vertex of the outer ring (typically a triangle)

		//sort ALL the vertices in the sector between i+1,i and i-1 by their angle i,i+1 (clockeise)
		//include the next and previous veritces on the outer ring
		sweep := sortedSectorVerts(mesh, ringWithChildHoles, i) //sweep accross the sector creating triangles

		for j := 0; j < len(sweep.p)-2; j++ {
			b := sweep.p[j].vi
			c := sweep.p[j+1].vi

			if a == b || a == c || b == c {
				panic("degenerate triangle")
			}

			if mesh.inAline(a, b, c) { //check if the triangle would be infinitely thin
				panic("ultra thin triangle")
			}
			//if the potential face does not intersect any of the child holes of the ring
			//and the rays from a to b and c do not intersect any of the rays from the outer ring
			//then add the face to the list
			pFace := mesh.makeTri(0, a, b, c) //potential face

			if !pFace.intersectsOrContainsChildHoleOf(ringWithChildHoles) {
				if (!mesh.rayCross(b, c, rayList)) && (!mesh.rayCross(c, a, rayList)) {
					rayList = append(rayList, b, c)
					rayList = append(rayList, c, a)

					fi = append(fi, a, b, c)
				}
			}

		}

	}

	return fi
}

func (m *mesh) rayCross(a, b int, rvi []int) bool {

	if a == b {
		panic("degenerate ray")
	}

	//rvi
	for i := 0; i < len(rvi); i += 2 {
		if (a == rvi[i] || a == rvi[i+1]) || (b == rvi[i] || b == rvi[i+1]) {
			return false //lines touch but don't 'cross'
		}
		if m.lineSegmentsIntersect(rvi[i], rvi[i+1], a, b) {
			return true
		}
	}
	return false
}

//you have a tool and some clay
//the tool is not effected by the clay  (but the process is symetrical so you can later use the clay as the tool and vice versa)
//find all the double penetrations of the tool into the clay (where 2)
//both penetrations should be in the same direction and are a common edge of two faces
//comparing this direction to the normal of the face will tell you if the tool is entering or exiting the clay
//each triangle of the tool yields a pair of penetrations (although not neccessariy in the same clay triangle)

//aditionally the (bounded) plane of the tool must be intersected with the edges of the clay to produce additiaonl points of penetration

func (tri *Tri) intersectsOrContainsChildHoleOf(ring *Ring) bool {

	//if any of this rings child (hole) rings intersect with, or are completely contained by tri - return true
	for _, child := range ring.children {
		for _, v := range child.vi {
			if tri.contains(tri.mesh.verts[v].p, false, false) {
				return true //early exit - if any vertex of the hole is inside the triangle
			}
		}
		if tri.intersects(child) {
			return true
		}
	}

	return false

}

//do the edges of the triangle intersect any segment of the ring
func (tri *Tri) intersects(ring *Ring) bool {

	for i := 0; i < len(ring.vi); i++ {
		for j := 0; j < 3; j++ {
			if tri.mesh.lineSegmentsIntersect(tri.Vi[j], tri.Vi[(j+1)%3], ring.vi[i], ring.vi[(i+1)%len(ring.vi)]) {
				return true
			}
		}
	}
	return false

}

//do two coplanar line segments intersect ?
func (mesh *mesh) lineSegmentsIntersect(ai, bi, ci, di int) bool {

	if ai == bi || ci == di {
		panic("degenerate line segment")
	}

	a := mesh.verts[ai].p
	b := mesh.verts[bi].p
	c := mesh.verts[ci].p
	d := mesh.verts[di].p
	ab := b.sub(a)
	cd := d.sub(c)

	n := ab.cross(cd)
	if n.lengthSq() < 0.00000001 { //lines are parallel
		return false
	}

	b = b.projectOntoPlane(a, n)
	c = c.projectOntoPlane(a, n)
	d = d.projectOntoPlane(a, n)

	d0 := a.distanceFromLine(c, d)
	d1 := b.distanceFromLine(c, d)

	ip := a.add(ab.multiply(d0 / (d0 + d1)))
	if ip.liesBetween(a, b) && ip.liesBetween(c, d) {
		return true
	}

	return false

}

//returns the positive angle (in radians) between a line from 0,0 to p and a line from 0,0 to b
//todo - write test
func (p *Vec3) PositiveAngleFrom(b *Vec3) float64 {

	aa := b.dot(p) / (b.length() * p.length())
	if aa < -1.001 || aa > 1.001 {
		panic("aa out of range")
	}
	if aa < -1 {
		aa = -1
	}
	if aa > 1 {
		aa = 1
	}

	a := math.Acos(aa)
	if a < 0 {
		a += math.Pi * 2
	}

	if math.IsNaN(a) {
		panic("angle is NaN")
	}

	return a

}

func (b *Vec3) SignedAngleFrom(a *Vec3, axis *Vec3) float64 {

	dp := a.dot(b)
	alXbl := (a.length() * b.length())
	aa := dp / alXbl
	if aa < -1.001 || aa > 1.001 {
		panic("aa out of range")
	}
	if aa < -1 {
		aa = -1
	}
	if aa > 1 {
		aa = 1
	}
	angle := math.Acos(aa)

	if math.IsNaN(angle) {
		panic("angle is NaN")
	}

	if b.cross(a).dot(axis) < 0 {
		angle = -angle
	}

	return angle

}

//gathers all points from this ring in the sector a,b,c
func (ring *Ring) gatherAllPointsInsideSector(mesh *mesh, a, b, c *Vec3, points *sectorPoints) {

	sectorAngle := c.sub(b).PositiveAngleFrom(a.sub(b))

	for _, vi := range ring.vi {
		p := mesh.verts[vi].p
		d := b.distanceFrom(p)
		if d > 0.001 { //do not include the 'hinge' of the sector

			ia := p.sub(b).PositiveAngleFrom(a.sub(b)) //needs testing
			if ia >= 0 && ia <= sectorAngle {
				points.update(ia, vi, d)
			}
		}
	}

	//	return points
}

func compareAngle(i, j *angleIndex) int {
	return int((i.angle - j.angle) * 10000)
}

func sortedSectorVerts(mesh *mesh, ring *Ring, i int) *sectorPoints {
	//return the vertices of the ring sorted by their angle with the vertex i
	//clockwise
	//where vertices have a common angle, keep the one closest to the vertex i

	if len(ring.vi) != 3 {
		panic("outer ring does not have 3 verts")
	}

	a := mesh.verts[ring.vi[(i+1)%len(ring.vi)]].p
	b := mesh.verts[ring.vi[i]].p
	c := mesh.verts[ring.vi[(i-1+len(ring.vi))%len(ring.vi)]].p

	points := NewSectorPoints() //a list of vert indexes, and their angle within the sector

	ring.gatherAllPointsInsideSector(mesh, a, b, c, points) //mutates points
	if len(points.p) != 2 {
		panic("outer triangle should have 2 non hinge points")
	}

	//get the verts of child holes in the sector (updating existing points on the same ray with the nearest)
	for _, childRing := range ring.children {
		childRing.gatherAllPointsInsideSector(mesh, a, b, c, points) //mutates points (gathers more)
	}

	slices.SortFunc(points.p, compareAngle) //sorts in place

	return points
}

// //return 0, 1 or 2 points of intersection of the edges of triangle b, with the plane of triangle a
//penetrations will always come in pairs, sometimes two face penetrations, sometimes to edge penetrations, sometimes one of each
type pen struct {
	ps *penSet //hold a reference to the set a belong to
	p  *Vec3   //the point of penetration
	//vi	   	 int       //the new vertex of penetration
	v1, v2     int  // for face penetrations, the edge of the tool that penetrated
	toolTri    *Tri //the penatrator
	clayTri    *Tri //the penetratee
	used       bool //has this been incoroporated into a ring
	isBoundary bool //is on an edge of the clay triangle - v1 and v2 are of the penetrated edge
	next       *pen //penetrations are formed into rings
}

type penSet struct {
	pens      []*pen
	usedCount int
}

func (ps *penSet) append(pens *penSet) {
	if ps.usedCount != 0 || pens.usedCount != 0 {
		panic("cannot append used penSets")
	}
	ps.pens = append(ps.pens, pens.pens...)
}

func (ps *penSet) add(p *Vec3, v1, v2 int, toolTri, clayTri *Tri, isBoundary bool) {
	ps.pens = append(ps.pens, &pen{ps, p, v1, v2, toolTri, clayTri, false, isBoundary, nil})
}

func NewPenSet() *penSet {
	return &penSet{pens: []*pen{}, usedCount: 0}
}

func (ps *penSet) bind() {
	if len(ps.pens) != 2 {
		panic("can only bind two pens")
	}

	ps.pens[0].next = ps.pens[1]
	ps.pens[1].next = ps.pens[0]

}

func (pen *pen) markUsed() {
	pen.used = true
	pen.ps.usedCount++
}

// func (ps *penSet) penetratorIs(tri *Tri) *pen {
// 	for _, p := range ps.pens {
// 		if p.toolTri == tri {
// 			return p
// 		}
// 	}
// 	panic("penetrator not found")
// }

func (ps *penSet) otherPenByToolFace(p1 *pen) *pen {

	var r *pen //TODO this can be simplified once stable
	for _, p := range ps.pens {
		if (p != p1) && (p.toolTri == p1.toolTri) {
			if r == nil {
				r = p
			} else {
				panic("more than one other penetration by face")
			}
		}
	}
	if r != nil {
		return r
	}
	panic("missing second penetration by tool face")
}

func (ps *penSet) oppositePenByEdge(pen *pen) *pen {
	for _, p := range ps.pens {
		if p.v1 == pen.v2 && p.v2 == pen.v1 {
			if p.p.distanceFrom(pen.p) > 0.001 {
				panic("opposite penetration is not at the same point")
			}
			return p
		}
	}
	return nil
}

func (ps *penSet) unusedBoundaryPen() *pen {
	for _, p := range ps.pens {
		if !p.used && p.isBoundary {
			return p
		}
	}
	return nil
}

func (ps *penSet) unused() *pen {
	for _, p := range ps.pens {
		if !p.used {
			return p
		}
	}
	return nil
}

//return 0, 1 or 2 points of intersection of the edges of the tool triangle, with the plane of triangle a
func (clayTri *Tri) penetrationsByEdgesOf(toolTri *Tri, isEdgePen bool) *penSet {

	pens := NewPenSet()

	ctn := clayTri.normal()
	ttn := toolTri.normal()
	dp := ctn.dot(ttn)
	if dp > 0.99999 || dp < -0.99999 {
		return pens //the faces are parallel/coplanar
	}

	d := make([]float64, 3) //len(toolTri.Vi))
	p := make([]*Vec3, 3)
	for i, vi := range toolTri.Vi {
		p[i] = toolTri.mesh.verts[vi].p //collect the position of the vertices of the tool triangle
		d[i] = p[i].distanceFromPlaneOf(clayTri)
	}

	if toolTri.Vi[0] == 3 && clayTri.Vi[0] == 3 {
		if toolTri.Vi[0] == 0 && clayTri.Vi[0] == 0 {

			logit("rb")
		}

	}

	if (d[0] < 0 && d[1] < 0 && d[2] < 0) || (d[0] > 0 && d[1] > 0 && d[2] > 0) {
		return pens //all the points(of the tool triangle) are on the same side of the plane (of the clay triangle)
	} //TODO consider touching (d==0) (where a cap is touching a face - we want a hole with no depth

	//i goes 0,1,2
	for i := 0; i < 3; i++ {
		nxt := (i + 1) % 3
		//di := d[i]
		//dn := d[nxt]

		r0 := toolTri.mesh.verts[toolTri.Vi[i]].p
		r1 := toolTri.mesh.verts[toolTri.Vi[nxt]].p
		pop := clayTri.probePlane(r0, r1)
		if pop != nil {
			if clayTri.contains(pop, true, true) {
				pens.add(pop, toolTri.Vi[i], toolTri.Vi[nxt], toolTri, clayTri, isEdgePen)
			}
		}

		// if oppositeSigns(di, dn) {

		// 	f := abs(di) / (abs(di) + abs(dn))

		// 	if f<0 || f>1 {panic("f out of range")}

		// 	pop := p[i].tween(p[nxt], f)

		// 	//CHANGED
		// 	if clayTri.contains(pop, false, false) { //tool edge penetration inside clay trianlge

		// 		// if swapToolAndClay {
		// 		// 	pens.add(pop, toolTri.Vi[i], toolTri.Vi[nxt], clayTri, toolTri, true)
		// 		// } else {
		// 		pens.add(pop, toolTri.Vi[i], toolTri.Vi[nxt], toolTri, clayTri, isEdgePen)
		// 		//}
		// 	}
		//}
	}

	return pens

}

func abs(a float64) float64 {
	if a < 0 {
		return -a
	}
	return a
}

//write tests for  distance from planeof (which should be signed)

func oppositeSigns(a, b float64) bool {
	return a*b < 0
}
