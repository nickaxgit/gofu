package main

type face struct {
	m []*mass //mass indexes  a face is planar, and defined by n masses forming a convex poygon
}

func newFace(m []*mass) *face {
	return &face{m: m}
}
func (f *face) normal() *Vec3 {
	a := f.m[0].p
	b := f.m[1].p
	c := f.m[2].p

	ab := b.sub(a)
	ac := c.sub(a)
	return ab.cross(ac).normalise()

}

func (f *face) penetration(m *mass) float64 {
	dist := m.sideOf(f) //distance from plane .. negative means its penetrated
	if dist < m.r {
		return m.fromEdge(f)
	}

	return 0
}

func (m *mass) fromEdge(f *face) float64 {

	//find the closest point on any edge of the face to the point p
	huge := 1000000.0
	smallestOutDist := huge //smallest distance outside an edge
	verts := len(f.m)

	faceNormal := f.normal()
	for mi := 0; mi < verts; mi++ {
		a := f.m[mi].p
		b := f.m[(mi+1)%verts].p

		p := m.p.closestPointOnPlane(a, faceNormal)
		d := p.signedDistanceFromLineSegment(a, b, faceNormal)

		if d > 0 && d < smallestOutDist {
			smallestOutDist = d
		}
	}

	if smallestOutDist == huge {
		//the mass (projected onto the face) is entriely inside the face
		return m.r
	}

	if smallestOutDist > m.r {
		//the mass is well outside the face
		return 0
	}

	//the hard part - outside the face, but inside the radius
	return smallestOutDist //this is an approximation

}
