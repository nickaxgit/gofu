package main

import (
	"math"
	"strconv"
)

type Vec3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

func newVec3(x, y, z float64) *Vec3 {
	return &Vec3{x, y, z}
}

func (a *Vec3) add(b *Vec3) *Vec3 {
	return newVec3(a.X+b.X, a.Y+b.Y, a.Z+b.Z)
}

func hypo3(a, b, c float64) float64 {
	return math.Sqrt(float64(a*a + b*b + c*c))
}

func (p *Vec3) projectOntoPlane(pop, normal *Vec3) *Vec3 {
	return p.sub(normal.multiply(p.dot(normal) - pop.dot(normal)))

}

// func (a *Vec3) distanceFrom(b *Vec3) float64 {
// 	return hypo3(a.X-b.X, a.Y-b.Y, a.Z-b.Z)
// }

func (a *Vec3) lengthSq() float64 {
	return a.X*a.X + a.Y*a.Y + a.Z*a.Z
}

func (p *Vec3) rotateAbout(axis *Vec3, angle float64) *Vec3 {
	al := axis.length()
	if al < .999 || al > 1.001 {
		panic("axis is not unit length")
	}
	if axis.dot(p) == 1 {
		return p
	}
	if axis.dot(p) == -1 {
		return p
	}

	// Rodrigues' rotation formula
	k := axis
	v := p
	cosTheta := math.Cos(angle)
	sinTheta := math.Sin(angle)

	term1 := v.multiply(cosTheta)
	term2 := k.cross(v).multiply(sinTheta)
	term3 := k.multiply(k.dot(v) * (1 - cosTheta))

	return term1.add(term2).add(term3)

}

// func (a *Vec3) subIn(b Vec3) {
// 	a.X -= b.X
// 	a.Y -= b.Y
// 	a.Z -= b.Z
// }

// func (a *Vec3) addIn(b Vec3) {
// 	a.X += b.X
// 	a.Y += b.Y
// 	a.Z += b.Z
// }

func (a *Vec3) multiply(f float64) *Vec3 {
	return newVec3(a.X*f, a.Y*f, a.Z*f)
}

func (a *Vec3) tween(b *Vec3, f float64) *Vec3 {
	return newVec3(a.X+(b.X-a.X)*f, a.Y+(b.Y-a.Y)*f, a.Z+(b.Z-a.Z)*f)
}

func (a *Vec3) sub(b *Vec3) *Vec3 {
	return newVec3(a.X-b.X, a.Y-b.Y, a.Z-b.Z)
}

func (a *Vec3) normalise() *Vec3 {
	l := a.length()
	if l == 0 {
		return a //panic("can't normalise 0 vector")
	}
	return &Vec3{X: a.X / l, Y: a.Y / l, Z: a.Z / l}
}

func (a *Vec3) length() float64 {
	return hypo3(a.X, a.Y, a.Z)
}

func (a *Vec3) equals(b *Vec3) bool {
	if a.X == b.X && a.Y == b.Y && a.Z == b.Z {
		return true
	}
	if a.distanceFrom(b) < 0.0001 {
		logit("distance is close enough")
		return true
	}
	return false

}

// func (p Vector) rotate(angle float64) Vector {
// 	x := p.X*math.Cos(angle) - p.Y*math.Sin(angle)
// 	y := p.X*math.Sin(angle) + p.Y*math.Cos(angle)
// 	return newVector(x, y)
// }

func (a *Vec3) dot(b *Vec3) float64 {
	return a.X*b.X + a.Y*b.Y + a.Z*b.Z
}

func (a *Vec3) cross(b *Vec3) *Vec3 {
	if (a.length() == 0) || (b.length() == 0) {
		panic("can't cross 0 vector")
	}
	return &Vec3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}

func (p Vec3) closestPointOnLine(a, b *Vec3) *Vec3 {
	ab := b.sub(a)
	abn := ab.normalise()
	dp := p.sub(a).dot(abn)
	return a.add(abn.multiply(dp))
}

func (p *Vec3) distanceFromLine(a, b *Vec3) float64 {
	return p.closestPointOnLine(a, b).distanceFrom(p)
}

func (p *Vec3) distanceFrom(b *Vec3) float64 {
	return hypo3(p.X-b.X, p.Y-b.Y, p.Z-b.Z)
}

func (p *Vec3) liesBetween(a *Vec3, b *Vec3) bool {

	if a.equals(b) {
		panic("a and b are the same point")
	}
	if p.equals(a) || p.equals(b) {
		return false //if the point is at an endpoint, it doesn't lie *between* the endpoints
	}

	v1 := p.sub(a) //vector from a to p
	v2 := p.sub(b) //vector from a to p

	//if the dot product is negative, then the vectors (from the point to the endpoints) are pointing in opposite directions - and the point lies between A-B
	return v1.dot(v2) < 0 //the edge case is 0 when p is at an endpoint (dealt with above)

}

func (p *Vec3) distanceFromPlaneOf(t *Tri) float64 {
	return t.distanceFrom(p)
}

func (p *Vec3) distanceFromTriPlane(a, b, c *Vec3) float64 {

	//get the normal of the triangle
	n := b.sub(a).cross(c.sub(a)).normalise() //todo - cache/gen the normals once

	pop := p.sub(a)

	//get the distance from the point to the plane
	d := pop.dot(n)

	return d

}

// is the point inside the mesh
func (p *Vec3) isInside(m *mesh) bool {

	//todo - add trivial bounds check

	hits := 0
	farFarAway := &Vec3{-1000000, 5000, 30}
	for i := 0; i < len(m.fi); i += 3 {
		t := m.triangleFrom(i)
		pop := t.probePlane(p, farFarAway)

		if pop != nil && t.contains(pop, false, false) {
			hits++
		}

	}

	return hits%2 == 1 //if we cross an odd number of faces we are inside

}

func (pop *Vec3) isInsideTri(a, b, c *Vec3, includeOnEdge bool) bool {
	//get the normal of the triangle

	ab := b.sub(a)
	bc := c.sub(b)
	ca := a.sub(c)

	dp := ab.normalise().dot(bc.normalise())
	if dp < -.9999 || dp > .9999 {
		panic("degenerate triangle (has parallel edges)")
	}
	cp := ab.cross(bc)
	n := cp //.normalise() //todo - remove ? (normalising is just a loss of precision)

	//project the point onto the plane of the triangle
	//and get the vector from the point to the plane
	//j := pop.sub(a)

	//get the distance from the point to the plane
	d := pop.distanceFromTriPlane(a, b, c)

	if d > 0.001 || d < -0.001 {
		panic("point not on plane " + strconv.Itoa(int(d*1000)))
	}

	//get the vectors from the projected point to the vertices of the triangle
	va := pop.sub(a)
	vb := pop.sub(b)
	vc := pop.sub(c)

	//cross each edge with the point-to-vertex vector

	na := ab.cross(va)
	nb := bc.cross(vb)
	nc := ca.cross(vc)

	tiny := 0.001
	if na.lengthSq() < tiny || nb.lengthSq() < tiny || nc.lengthSq() < tiny {
		//the point is ON an (INFINITE) edge
		if !includeOnEdge {
			return false
		}
		if na.lengthSq() < tiny && pop.liesBetween(a, b) {
			return true
		}
		if nb.lengthSq() < tiny && pop.liesBetween(b, c) {
			return true
		}
		if nc.lengthSq() < tiny && pop.liesBetween(c, a) {
			return true
		}
		panic("point is on an edge but not between the vertices")

	}

	//get the dot products of the normals with the normal of the triangle
	da := na.dot(n)
	db := nb.dot(n)
	dc := nc.dot(n)

	if da == 0 || db == 0 || dc == 0 {
		panic("collapsed normal")
	}
	//if the dot products are all positive, then the point is inside the triangle
	if da > 0 && db > 0 && dc > 0 {
		return true
	}

	return false

}

func (p *Vec3) clone() *Vec3 {

	return newVec3(p.X, p.Y, p.Z)
}
