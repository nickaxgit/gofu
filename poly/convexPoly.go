package poly

import (
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/plane"
	"github.com/nickax/gofu/ray"
	"github.com/nickax/gofu/vec"
	"math"
	//	"strconv"
)

type ConvexPoly struct {
	P     []vec.V3
	Plane plane.Plane
	//working variables to avoid allocation when calling contains
	//d      *vec.V3
	//edge   *vec.V3
	//normal *vec.V3
	//cp     *vec.V3 //crossProduct
	//	pcp     float64 //y or plane normal component of previous cross product
	epsilon float64
	//cpn        float64 //length of the normal component of the cross product
	PointCount int
}

func (p *ConvexPoly) Highest() *vec.V3 {
	high := p.P[0]
	for _, v := range p.P {
		if v.Y > high.Y {
			high = v
		}
	}
	return &high
}

// MatchXZ - returns true if the XZ positions of the two polygons match (ignoring Y) and rotational orientation
func (a *ConvexPoly) MatchXZ(b *ConvexPoly) bool {

	if len(a.P) != len(b.P) {
		log.Logit("Poly len mismatch", len(a.P), len(b.P))
		return false
	}

	hits := 0
	for o := 0; o < len(b.P); o++ {
		for _, v := range a.P {

			dx, dy := v.X-b.P[o].X, v.Z-b.P[o].Z
			if math.Abs(dx) < 0.0001 && math.Abs(dy) < 0.0001 {
				hits++
				continue
			}
		}
	}

	if hits == len(a.P) {
		return true
	}

	return false
}

func (poly *ConvexPoly) Penetration(p vec.V3, r float64) float64 {
	dist := poly.Plane.DistanceFrom(p) //.. negative means its penetrated
	if dist < r {
		return poly.fromEdge(p, r)
	}

	return 0
}

func (poly *ConvexPoly) Centre() vec.V3 {
	c := vec.NewVec3(0, 0, 0)

	for _, v := range poly.P {
		//c.AddInto(c, v)
		c.AddIn(v)
	}
	c.DivIn(float64(len(poly.P)))
	return c
}

func NewConvexPoly() *ConvexPoly {
	return &ConvexPoly{P: make([]vec.V3, 0, 4),
		epsilon: 0.0001}
}

func NewConvexPolyFromVecs(pts []vec.V3) *ConvexPoly {
	poly := NewConvexPoly()
	for _, p := range pts {
		poly.AddPoint(p)

	}

	return poly
}

func (poly *ConvexPoly) fromEdge(p vec.V3, r float64) float64 {

	smallestOutDist := math.MaxFloat64 //try and find something smaller

	verts := len(poly.P)
	for mi := 0; mi < verts; mi++ {
		a := poly.P[mi]
		b := poly.P[(mi+1)%verts]

		p := poly.Plane.ClosestPointOnPlane(a)
		d := p.SignedDistanceFromLineSegment(a, b, poly.Plane.GetNormal())

		if d > 0 && d < smallestOutDist {
			smallestOutDist = d
		}
	}

	if smallestOutDist == math.MaxFloat64 {
		//the mass (projected onto the face) is entriely inside the face
		return r
	}

	if smallestOutDist > r {
		//the mass is well outside the face
		return 0
	}

	//the hard part - outside the face, but inside the radius
	return smallestOutDist //this is an approximation

}

func (poly *ConvexPoly) AddPointAt(x, y, z float64) {

	poly.AddPoint(vec.NewVec3(x, y, z))

}

func (poly *ConvexPoly) Cap(a, b, c vec.V3, y float64) {
	a.Y = y
	b.Y = y
	c.Y = y
	poly.AddPoint(a)
	poly.AddPoint(b)
	poly.AddPoint(c)

}

func (poly *ConvexPoly) AddPoint(p vec.V3) {
	if poly.PointCount < len(poly.P) {
		poly.P[poly.PointCount] = p
	} else {
		poly.P = append(poly.P, p)

	}
	poly.PointCount++

	if poly.PointCount > 4 {
		panic("convexPoly beyond a quadrilateral")
	}

	if poly.PointCount == 3 {
		poly.Plane = plane.NewFromPoints(poly.P[0], poly.P[1], poly.P[2])

	}

	// if len(poly.p) > 3 {
	// 	cp := poly.p[len(poly.p)-1].Sub(poly.p[len(poly.p)-2]).Cross(poly.p[0].Sub(poly.p[len(poly.p)-2])).Normalise()
	// 	poly.normal = poly.Plane.GetNormal()
	// 	if cp.Dot(poly.normal) < .999 {
	// 		panic("warning: convexPoly point added that makes polygon non-convex, or is not coplanar")
	// 	}
	// }
}

// probe the convex poly with the ray
func (poly *ConvexPoly) Probe(ray ray.Ray) (bool, vec.V3) {

	hit, where := poly.Plane.ProbeLine(ray)
	if hit { //will return false for backfacing triangles
		//if poly.Contains(ray.GetIntersect()) {

		if poly.Contains3D(where) {
			return true, where
		} else {
			//log.Logit("ray intersect not inside poly")

		}
	}
	return false, vec.NewVec3(0, 0, 0)
}

func (poly *ConvexPoly) Contains2D(p vec.V3) bool {

	for i, vertex := range poly.P {

		d := p.Sub(vertex)

		nxt := poly.P[(i+1)%len(poly.P)]
		edge := nxt.Sub(vertex)

		//flatten
		edge.Y = 0
		d.Y = 0

		if d.X > -poly.epsilon && d.X < poly.epsilon && d.Z > -poly.epsilon && d.Z < poly.epsilon {
			return true //on a vertex
		}

		cp := edge.Cross(d)

		if cp.Y < -poly.epsilon {
			return false
		}

	}

	return true

}

func (poly *ConvexPoly) Contains3D(p vec.V3) bool {

	normal := poly.Plane.GetNormal()

	for i, vertex := range poly.P {

		d := p.Sub(vertex)
		nxt := poly.P[(i+1)%len(poly.P)]
		edge := nxt.Sub(vertex)

		cpn := edge.Cross(d).Dot(normal)

		if cpn < -poly.epsilon {
			return false
		}
	}
	return true
}
