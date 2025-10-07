package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"strconv"
)

type vec3 struct {
	x float64
	y float64
	z float64
}

func distanceBetweenLines(a1, a2, b1, b2 *vec3) float64 {
	//from http://geomalgorithms.com/a07-_distance.html
	u := a2.sub(a1)
	v := b2.sub(b1)
	w := a1.sub(b1)

	a := u.dot(u)
	b := u.dot(v)
	c := v.dot(v)
	d := u.dot(w)
	e := v.dot(w)

	D := a*c - b*b
	sc := 0.0
	tc := 0.0

	if D < 0.0000001 {
		sc = 0.0
		tc = (e / c)
	} else {
		sc = (b*e - c*d) / D
		tc = (a*e - b*d) / D
	}

	return a1.add(u.multiply(sc)).sub(b1.add(v.multiply(tc))).length()

	//return a1.add(u.multiply(sc))

}
func (p *vec3) reflectInPlane(pop *vec3, normal *vec3) *vec3 {
	return p.sub(normal.multiply(2 * (p.dot(normal) - pop.dot(normal))))
}

func newVec3(x, y, z float64) *vec3 {
	return &vec3{x, y, z}
}

func (a *vec3) add(b *vec3) *vec3 {
	return newVec3(a.x+b.x, a.y+b.y, a.z+b.z)
}

func hypo3(a, b, c float64) float64 {
	return math.Sqrt(float64(a*a + b*b + c*c))
}

func (a *vec3) addIn(b *vec3) {
	a.x += b.x
	a.y += b.y
	a.z += b.z
}

func (a *vec3) subIn(b *vec3) {
	a.x -= b.x
	a.y -= b.y
	a.z -= b.z
}

func (p *vec3) projectOntoPlane(pop, normal *vec3) *vec3 {
	return p.sub(normal.multiply(p.dot(normal) - pop.dot(normal)))

}

// func (a *Vec3) distanceFrom(b *Vec3) float64 {
// 	return hypo3(a.X-b.X, a.Y-b.Y, a.Z-b.Z)
// }

// func (p *vec3) payload() Vec3Payload {
// 	return Vec3Payload{X: p.x, Y: p.y, Z: p.z}
// }

func (a *vec3) lengthSq() float64 {
	return a.x*a.x + a.y*a.y + a.z*a.z
}

func (p *vec3) rotateAbout(axis *vec3, angle float64) *vec3 {
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

func (a *vec3) divide(f float64) *vec3 {
	if f == 0 {
		panic("divide by zero")
	}
	return newVec3(a.x/f, a.y/f, a.z/f)
}

func (a *vec3) multiply(f float64) *vec3 {
	return newVec3(a.x*f, a.y*f, a.z*f)
}

func (a *vec3) tween(b *vec3, f float64) *vec3 {
	return newVec3(a.x+(b.x-a.x)*f, a.y+(b.y-a.y)*f, a.z+(b.z-a.z)*f)
}

func (a *vec3) sub(b *vec3) *vec3 {
	return newVec3(a.x-b.x, a.y-b.y, a.z-b.z)
}

func (a *vec3) normalise() *vec3 {
	l := a.length()
	if l == 0 {
		panic("can't normalise 0 vector")
		//return a
	}
	return &vec3{x: a.x / l, y: a.y / l, z: a.z / l}
}

func (p *vec3) toByteBuffer(buff *bytes.Buffer) {
	binary.Write(buff, le, float32(p.x)) //NOTE you CANT write a Vec3 directly as it has float64 components
	binary.Write(buff, le, float32(p.y))
	binary.Write(buff, le, float32(p.z))
}

func (p *vec3) fromByteBuffer(buff *bytes.Buffer) {

	floats := make([]float32, 3) //the components of a vector are float64's but we downscale to 32bits for transmission/storage
	binary.Read(buff, le, &floats)
	p.x = float64(floats[0])
	p.y = float64(floats[1])
	p.z = float64(floats[2])

}

func (a *vec3) length() float64 {
	return hypo3(a.x, a.y, a.z)
}

func (a *vec3) equals(b *vec3) bool {
	if a.x == b.x && a.y == b.y && a.z == b.z {
		return true
	}
	// if a.distanceFrom(b) < 0.0001 {
	// 	//logit("distance is close enough")
	// 	return true
	// }
	return false

}

// func (p Vector) rotate(angle float64) Vector {
// 	x := p.X*math.Cos(angle) - p.Y*math.Sin(angle)
// 	y := p.X*math.Sin(angle) + p.Y*math.Cos(angle)
// 	return newVector(x, y)
// }

func (a *vec3) dot(b *vec3) float64 {
	return a.x*b.x + a.y*b.y + a.z*b.z
}

func (a *vec3) cross(b *vec3) *vec3 {
	if (a.length() == 0) || (b.length() == 0) {
		panic("can't cross 0 vector")
	}
	return &vec3{a.y*b.z - a.z*b.y, a.z*b.x - a.x*b.z, a.x*b.y - a.y*b.x}
}

func (p *vec3) signedDistanceFromLineSegment(a, b, n *vec3) float64 {

	cp := p.closestPointOnLineSegment(a, b)

	ab := b.sub(a)
	ap := p.sub(a)

	d := p.distanceFrom(cp)

	if ap.cross(ab).dot(n) > 0 {
		d = -d
	}

	return d

}

func (p *vec3) closestPointOnLineSegment(a, b *vec3) *vec3 {

	ab := b.sub(a)
	ap := p.sub(a)
	bp := p.sub(b)
	if ap.dot(ab) < 0 {
		return a //p is outside AB and closest to A
	}
	if bp.dot(ab) > 0 { // p is outside AB and closest to B
		return b
	}

	return p.closestPointOnLine(a, b)

}

func (p *vec3) distanceFromLineSegment(a, b *vec3) float64 {
	//TEST this
	ab := b.sub(a)
	abn := ab.normalise()
	ap := p.sub(a)
	bp := p.sub(b)
	if ap.dot(abn) < 0 {
		return ap.length() //p is outside AB and closest to A
	}
	if bp.dot(abn) > 0 {
		return bp.length() //p is outside AB and closest to B
	}
	return p.distanceFromLine(a, b)
}

func (p vec3) closestPointOnLine(a, b *vec3) *vec3 {
	ab := b.sub(a)
	abn := ab.normalise()
	dp := p.sub(a).dot(abn)
	return a.add(abn.multiply(dp))
}

func (p *vec3) distanceFromLine(a, b *vec3) float64 {
	return p.closestPointOnLine(a, b).distanceFrom(p)
}

func (p *vec3) distanceFrom(b *vec3) float64 {
	return hypo3(p.x-b.x, p.y-b.y, p.z-b.z)
}

func (p *vec3) liesBetween(a *vec3, b *vec3) bool {

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

// func (p *Vec3) distanceFromFace(f *face) float64 {
// 	//find the closest point on any edge of the face to the point p
// 	bestDist := 1000000.0
// 	verts := len(f.m)
// 	for mi := 0; mi < verts; mi++ {
// 		a := f.m[mi].p
// 		b := f.m[mi+1%verts].p

// 		d := p.distanceFromLineSegment(a, b)
// 		if d < bestDist {
// 			bestDist = d
// 		}
// 	}

// 	a := f.m[0].p
// 	b := f.m[1].p
// 	c := f.m[2].p

// 	dpop := p.distanceFromTriPlane(a, b, c)

// 	if dpop < bestDist {
// 		bestDist = dpop
// 	}

//		return bestDist
//	}
func (p *vec3) signedDistanceFromPlaneOf(t *tri) float64 {

	v := t.mesh.verts
	a := v[t.vi[0]].p
	//b := v[t.vi[1]].p
	//c := v[t.vi[2]].p

	//return p.signedDistanceFromTriPlane(a, b, c)

	ln := t.normal.length()
	if ln < .99 || ln > 1.01 {
		panic("normal not unit length")
	}

	return p.signedDistanceFromTriPlane(a, t.normal)

}

func (p *vec3) closestPointOnPlane(a, n *vec3) *vec3 {
	//project the point onto the plane
	pop := p.sub(a)
	return p.sub(n.multiply(pop.dot(n)))
}

// func (p *vec3) closestPointOnTriPlane(a, b, c *vec3) *vec3 {

// 	n := b.sub(a).cross(c.sub(a)).normalise() //todo - cache/gen the normals once

// 	return p.sub(n.multiply(p.signedDistanceFromTriPlane(a, b, c)))
// }

// can be negative if the point on the side away from the normal
func (p *vec3) signedDistanceFromTriPlane(a, n *vec3) float64 {

	//get the normal of the triangle
	//n := b.sub(a).cross(c.sub(a)) //todo - cache/gen the normals once
	//n := (b.sub(a).normalise()).cross((c.sub(a)).normalise()).normalise() //todo - cache/gen the normals once

	return p.sub(a).dot(n)

}

// is the point inside the mesh
func (p *vec3) isInside(m *landMesh) bool {

	//todo - add trivial bounds check

	hits := 0
	farFarAway := &vec3{-1000000, 5000, 30}
	for i := 0; i < len(m.fi); i += 3 {
		t := m.triangleFrom(i)
		pop := t.probePlane(p, farFarAway)

		if pop != nil && t.contains(pop, false, false) {
			hits++
		}

	}

	return hits%2 == 1 //if we cross an odd number of faces we are inside

}

func (pop *vec3) isInsideTri(a, b, c *vec3, n *vec3, includeOnEdge bool, includeOnVert bool) bool {
	//get the normal of the triangle

	if a.equals(pop) || b.equals(pop) || c.equals(pop) {
		return includeOnVert // return true or false, depending on the value of includeOnVert
	}

	ab := b.sub(a)
	bc := c.sub(b)
	ca := a.sub(c)

	dp := ab.normalise().dot(bc.normalise())
	if dp < -.999999 || dp > .999999 {
		panic("degenerate triangle (has parallel edges)")
	}
	//cp := ab.cross(bc)
	//n := cp.normalise() //todo - remove ? (normalising is just a loss of precision)

	//project the point onto the plane of the triangle
	//and get the vector from the point to the plane
	//j := pop.sub(a)

	//get the distance from the point to the plane
	d := pop.signedDistanceFromTriPlane(a, n) // b, c)

	if d > 0.1 || d < -0.1 {
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
		return false //	point is on an edge but not between the vertices
		//panic("point is on an edge but not between the vertices")

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

func (p *vec3) clone() *vec3 {

	return newVec3(p.x, p.y, p.z)
}

func (b *vec3) SignedAngleFrom(a *vec3, axis *vec3) float64 {

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

func (v *vec3) reflect(n *vec3) *vec3 {
	return v.sub(n.multiply(2 * v.dot(n)))
}
