package plane

import (
	"github.com/nickax/gofu/ray"
	"github.com/nickax/gofu/vec"
	"math"
)

type Plane struct {
	normal   *vec.V3 //normal
	distance float64
	point    *vec.V3 //a point on the plane
}

func NewFromPoints(a, b, c *vec.V3) *Plane {
	ab := b.Sub(a)
	ac := c.Sub(a)
	n := ab.Cross(ac).Normalise()

	return &Plane{normal: n, distance: n.Dot(a), point: a}
}

func (plane *Plane) GetNormal() *vec.V3 {
	return plane.normal
}

func NewFromNormalAndPoint(n *vec.V3, p *vec.V3) *Plane {
	return &Plane{normal: n.Normalise(), distance: n.Dot(p), point: p}
}

func (plane *Plane) DistanceFrom(p *vec.V3) float64 {

	return p.Sub(plane.point).Dot(plane.normal)
}

func (plane *Plane) ClosestPointOnPlane(p *vec.V3) *vec.V3 {
	//project the point onto the plane
	pop := p.Sub(plane.point)
	return p.Sub(plane.normal.Multiply(pop.Dot(plane.normal)))
}

func (plane *Plane) ProbeLine(ray *ray.Ray) bool {

	denom := plane.normal.Dot(ray.GetDirection())
	if math.Abs(denom) < 1e-6 {
		return false
	}
	t := (plane.distance - plane.normal.Dot(ray.Origin)) / denom
	if t < 0 || t > 1 {
		return false
	}

	//mutates ray.Intersect (avoiding allocations)
	ray.GetIntersect().TweenInto(ray.Origin, ray.End, t)

	return true
}
