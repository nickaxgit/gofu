package plane

import (
	"math"

	"github.com/nickax/gofu/ray"
	"github.com/nickax/gofu/vec"
)

type Plane struct {
	normal   vec.V3 //normal
	distance float64
	//point    *vec.V3 //a point on the plane
}

func NewFromPoints(a, b, c vec.V3) Plane {
	ab := b.Sub(a)
	ac := c.Sub(a)
	n := ab.Cross(ac).Normalised()

	return Plane{normal: n, distance: n.Dot(a)}
}

func (plane *Plane) GetNormal() vec.V3 {
	return plane.normal
}

func NewFromNormalAndPoint(n vec.V3, p vec.V3) Plane {

	l := n.LengthSq()
	if l < .999 || l > 1.001 {
		panic("plane normal not normalised")
	}

	return Plane{normal: n, distance: n.Dot(p)}
}

func (plane *Plane) DistanceFrom(p vec.V3) float64 {

	//return p.Sub(plane.point).Dot(plane.normal)
	return plane.normal.Dot(p) - plane.distance
}

func (plane Plane) ClosestPointOnPlane(p vec.V3) vec.V3 {
	dist := plane.DistanceFrom(p)
	return p.Sub(plane.normal.Multiply(dist))
}

func (plane Plane) ProbeLine(ray ray.Ray) (bool, vec.V3) {

	denom := plane.normal.Dot(ray.GetDirection())
	// if denom > 1e6 {
	// 	return false, vec.V3{} //ray could only peirce backface
	// }

	if math.Abs(denom) < 1e-9 { //ray glancing the plane - DBZ - intersect would be at infinity
		return false, vec.V3{}
	}

	t := (plane.distance - plane.normal.Dot(ray.Origin)) / denom

	if t < 0 || t > 1 {
		return false, vec.V3{}
	}

	return true, ray.Origin.Tween(ray.End, t)
}
