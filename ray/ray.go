package ray

import (
	"github.com/nickax/gofu/vec"
)

type Ray struct {
	Origin    vec.V3
	End       vec.V3
	direction vec.V3 //NOT normalised (end-origin)
	//Intersect vec.V3 //scratchpad /probe result
}

// func (ray *Ray) GetIntersect() *vec.V3 {
// 	if ray.Intersect == nil {
// 		panic("ray.intersect is nil - did you forget to probe the ray first?")
// 	}
// 	return ray.Intersect
// }

func New(origin, end vec.V3) Ray {
	if origin.Equals(end) {
		panic("degenerate ray")
	}
	direction := end.Sub(origin)

	return Ray{Origin: origin, End: end, direction: direction}
}

// GetDirection returns the (not normalised) direction vector of the ray (end - origin)
func (ray Ray) GetDirection() vec.V3 {
	return ray.direction
}

// PointAt - mutates and and direction, it's faster to repoint an existing ray than make a new one
func (ray *Ray) PointAt(p vec.V3) {
	ray.End = p

	//ray.direction = ray.End.Sub(ray.Origin)
	ray.direction = ray.End.Sub(ray.Origin)
}

// distanceFromLineSegment calculates the shortest distance from the ray to the line segment ab
// TODO - TEST
func (ray Ray) DistanceFromLineSegment(a, b vec.V3) float64 {

	ab := b.Sub(a)
	ac := ray.Origin.Sub(a)
	t := ac.Dot(ab) / ab.Dot(ab)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return ac.Sub(ab.Multiply(t)).Length()
}
