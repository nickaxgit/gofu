package plane

import (
	"github.com/nickax/gofu/ray"
	"github.com/nickax/gofu/vec"
	"math"
)

type Plane struct {
	normal   *vec.V3 //normal
	distance float64
	//point    *vec.V3 //a point on the plane
}

func NewFromPoints(a, b, c *vec.V3) *Plane {
	ab := b.Sub(a)
	ac := c.Sub(a)
	n := ab.Cross(ac).Normalise()

	return &Plane{normal: n, distance: n.Dot(a)}
}

func (plane *Plane) GetNormal() *vec.V3 {
	return plane.normal
}

func NewFromNormalAndPoint(n *vec.V3, p *vec.V3) *Plane {

	l := n.LengthSq()
	if l < .999 || l > 1.001 {
		panic("plane normal not normalised")
	}

	return &Plane{normal: n, distance: n.Dot(p)}
}

func (plane *Plane) DistanceFrom(p *vec.V3) float64 {

	//return p.Sub(plane.point).Dot(plane.normal)
	return plane.normal.Dot(p) - plane.distance
}

func (plane *Plane) ClosestPointOnPlane(p *vec.V3) *vec.V3 {
	dist := plane.DistanceFrom(p)
	return p.Sub(plane.normal.Multiply(dist))
}

func (plane *Plane) ProbeLine(ray *ray.Ray) bool {

	denom := plane.normal.Dot(ray.GetDirection())
	if math.Abs(denom) < 1e-9 {
		return false
	}

	t := (plane.distance - plane.normal.Dot(ray.Origin)) / denom
	if t < 0 || t > 1 {
		return false
	}

	//mutates ray.Intersect (avoiding allocations)
	//ray.GetIntersect().TweenInto(ray.Origin, ray.End, t)
	ray.Intersect.TweenInto(ray.Origin, ray.End, t)

	return true
}
