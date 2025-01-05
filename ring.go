package main

import "slices"
import "math"

type angleIndex struct {
	angle float64
	index int
}

type Ring struct {
	vi       []int
	children []*Ring
}

func newRing(vi ...int) *Ring {
	return &Ring{vi: vi, children: []*Ring{}}
}

//triangulate an arbitrary polygon, which may have holes
func (ringWithChildHoles *Ring) triangulate(parentTriangle *Tri) []Tri {
	//return a list of triangles that cover the area of the polygon

	if len(ringWithChildHoles.vi) < 3 {
		panic("ring must have at least 3 vertices for triangulation")
	}
	if len(ringWithChildHoles.vi) == 3 && len(ringWithChildHoles.children) == 0 {
		panic("no need to triangulate a triangle")
	}

	faces := []Tri{}
	//edgelist := make([][]int,100) //vert index point pairs
	for vi, i := range ringWithChildHoles.vi {

		a := vi //state.land.verts[vi] //for each vertex of the outer ring (typically a triangle)

		//sort ALL the vertices in the sector between i+1,i and i-1 by their angle i,i+1 (clockeise)
		//include the next and previous veritces on the outer ring
		sweep := sortedSectorVerts(parentTriangle.mesh, ringWithChildHoles, i) //sweep accross the sector creating triangles

		for i := 0; i < len(sweep)-1; i++ {
			b := sweep[i].index
			c := sweep[i+1].index
			pFace := parentTriangle.mesh.makeTri(0, a, b, c) //potential face
			if !pFace.intersectsOrContainsChildHoleOf(ringWithChildHoles) {
				// if !crosses(b,c,edgelist){
				// 	edgelist = append(edgelist,[]int{b,c})
				parentTriangle.addChild(a, b, c) //add the triangle to the list
			}
		}

	}

	return faces
}

//you have a tool and some clay
//the tool is not effected by the clay  (but the process is symetrical so you can later use the clay as the tool and vice versa)
//find all the double penetrations of the tool into the clay (where 2)
//both penetrations should be in the same direction and are a common edge of two faces
//comparing this direction to the normal of the face will tell you if the tool is entering or exiting the clay
//each triangle of the tool yields a pair of penetrations (although not neccessariy in the same clay triangle)

//aditionally the (bounded) plane of the tool must be intersected with the edges of the clay to produce additiaonl points of penetration

func (tri *Tri) intersectsOrContainsChildHoleOf(ring *Ring) bool {

	//if any of this rings child (hole) rings interect, or are completely contained by tri - return true
	for _, child := range ring.children {
		for v := range child.vi {
			if tri.contains(tri.mesh.verts[v].p) {
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
			if tri.mesh.linesIntersect(tri.Vi[j], tri.Vi[(j+1)%3], ring.vi[i], ring.vi[(i+1)%len(ring.vi)]) {
				return true
			}
		}
	}
	return false

}

//do two coplanar line segments intersect ?
func (mesh *mesh) linesIntersect(ai, bi, ci, di int) bool {

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

//returns the positive anlge between a line from 0,0 to p and a line from 0,0 to b
//todo - write test
func (p *Vec3) AngleFrom(b *Vec3) float64 {

	a := math.Acos(b.dot(p) / (b.length() * p.length()))
	if a < 0 {
		a += math.Pi * 2
	}

	return a

}

//gathers all points from this ring in the sector a,b,c
func (ring *Ring) gatherAllPointsInsideSector(mesh *mesh, a, b, c *Vec3) []angleIndex {

	points := make([]angleIndex, 0) //a list of vert indexes, and their angle within the sector

	sectorAngle := c.sub(b).AngleFrom(a.sub(b)) + 0.0001

	for _, vi := range ring.vi {
		p := mesh.verts[vi].p
		if b.distanceFrom(p) > 0.001 { //do not include the 'hinge' of the sector

			ia := p.sub(b).AngleFrom(a.sub(b)) //needs testing
			if ia >= 0 && ia <= sectorAngle {
				points = append(points, angleIndex{ia, vi})
			}
		}
	}

	return points
}

func compareAngle(i, j angleIndex) int {
	return int((i.angle - j.angle) * 10000)
}

func sortedSectorVerts(mesh *mesh, ring *Ring, i int) []angleIndex {
	//return the vertices of the ring sorted by their angle with the vertex i
	//clockwise

	a := mesh.verts[ring.vi[(i+1)%len(ring.vi)]].p
	b := mesh.verts[ring.vi[i]].p
	c := mesh.verts[ring.vi[(i-1+len(ring.vi))%len(ring.vi)]].p

	points := ring.gatherAllPointsInsideSector(mesh, a, b, c)

	//get the verts of child holes in the sector
	for _, childRing := range ring.children {
		holepoints := childRing.gatherAllPointsInsideSector(mesh, a, b, c)
		points = append(points, holepoints...)
	}

	slices.SortFunc(points, compareAngle) //sorts in place

	return points
}

// //return 0, 1 or 2 points of intersection of the edges of triangle b, with the plane of triangle a
type pen struct {
	p      *Vec3 //the point of penetration
	v1, v2 int   // the edge of the tool that penetrated - often there will be two penetratins at the same point where an ingoing and outgoing edge (of negbouring faces) pentrate
}

func (clayTri *Tri) penetrationsBy(toolTri *Tri) []pen {

	pens := []pen{}

	dp := clayTri.normal().dot(toolTri.normal())
	if dp > 0.9999 || dp < -0.9999 {
		return pens //the faces are parallel/coplanar
	}

	d := make([]float64, len(toolTri.Vi))
	p := []*Vec3{}
	for i, vi := range toolTri.Vi {
		p[i] = toolTri.mesh.verts[vi].p //colect the position of the vertices of the tool triangle
		d[i] = p[i].distanceFromPlaneOf(clayTri)
	}

	if d[0] < 0 && d[1] < 0 && d[2] < 0 || (d[0] > 0 && d[1] > 0 && d[2] > 0) {
		return pens //all the points are on the same side of the plane
	} //TODO consider touching (d==0) (where a cap is touching a face - we want a hole with no depth

	//i goes 0,1,2
	for i := range toolTri.Vi {
		nxt := (i + 1) % 3
		if oppositeSigns(d[i], d[nxt]) {
			diSquared := d[i] * d[i]
			dnSquared := d[nxt] * d[nxt]
			f := diSquared / (diSquared + dnSquared) //squaring both sides avoids sign issues
			pop := p[i].tween(p[nxt], f)
			if clayTri.contains(pop) {
				pens = append(pens, pen{pop, toolTri.Vi[i], toolTri.Vi[nxt]})
			}
		}
	}

	return pens

}

func oppositeSigns(a, b float64) bool {
	return a*b < 0
}
