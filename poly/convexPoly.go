package poly

import (
	"github.com/nickax/gofu/plane"
	"github.com/nickax/gofu/ray"
	"github.com/nickax/gofu/vec"
	"math"
	"strconv"
)

type ConvexPoly struct {
	p     []*vec.V3
	Plane *plane.Plane
}

func (poly *ConvexPoly) Penetration(p *vec.V3, r float64) float64 {
	dist := poly.Plane.DistanceFrom(p) //.. negative means its penetrated
	if dist < r {
		return poly.fromEdge(p, r)
	}

	return 0
}

func NewConvexPoly() *ConvexPoly {
	return &ConvexPoly{}
}

func NewConvexPolyFromVecs(pts []*vec.V3) *ConvexPoly {
	poly := ConvexPoly{}
	for _, p := range pts {
		poly.AddPoint(p)

	}

	return &poly
}

func (poly *ConvexPoly) fromEdge(p *vec.V3, r float64) float64 {

	smallestOutDist := math.MaxFloat64 //try and find something smaller

	verts := len(poly.p)
	for mi := 0; mi < verts; mi++ {
		a := poly.p[mi]
		b := poly.p[(mi+1)%verts]

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

func (poly *ConvexPoly) AddPoint(p *vec.V3) {
	poly.p = append(poly.p, p)
	if len(poly.p) == 3 {
		poly.Plane = plane.NewFromPoints(poly.p[0], poly.p[1], poly.p[2])
	}
	if len(poly.p) > 3 {
		cp := poly.p[len(poly.p)-1].Sub(poly.p[len(poly.p)-2]).Cross(poly.p[0].Sub(poly.p[len(poly.p)-2])).Normalise()
		if cp.Dot(poly.Plane.GetNormal()) < .999 {
			panic("warning: convexPoly point added that makes polygon non-convex, or is not coplanar")
		}
	}
}

// probe the convey poly with the ray - *mutates* the ray.intersect
func (poly *ConvexPoly) Probe(ray *ray.Ray) bool {

	if poly.Plane.ProbeLine(ray) {
		if poly.Contains(ray.GetIntersect()) {
			return true
		}
	}
	return false
}

func (poly *ConvexPoly) Contains(p *vec.V3) bool {

	d := poly.Plane.DistanceFrom(p) //expensive TODO remove eventually
	if d > 0.00001 || d < -0.00001 {
		panic("point not coplanar with polygon" + strconv.FormatFloat(d, 'f', 6, 64))
	}

	pointCount := len(poly.p)
	pcp := 0.0

	n := poly.Plane.GetNormal()

	for i, vertex := range poly.p {
		if p.Equals(vertex) {
			return true //on a vertex
		}
		d := p.Sub(vertex)

		nxt := poly.p[(i+1)%pointCount]
		edge := nxt.Sub(vertex)

		cp := edge.Cross(d).Dot(n)
		if cp < 0 && pcp > 0 || cp > 0 && pcp < 0 {
			return false
		}
		pcp = cp //previous cross product
	}

	return true

}
