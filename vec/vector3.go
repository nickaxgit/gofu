package vec

import (
	"bytes"
	"encoding/binary"
	"math"
	//"github.com/nickax/gofu/geom"
	//	"strconv"
)

var Up = NewVec3(0, 1, 0)
var NoWhereSpecial = NewVec3(10, 10, 10)

type V3 struct {
	X float64
	Y float64
	Z float64
}

// func (p *V3) GetY() float64 {
// 	return p.Y
// }
// func (p *V3) GetX() float64 {
// 	return p.X
// }

// func (p *V3) GetZ() float64 {
// 	return p.Z
// }

// func (p *V3) SetX(x float64) *V3 {
// 	p.X = x
// 	return p //allows method chaining
// }

// func (p *V3) SetZ(z float64) *V3 {
// 	p.Z = z
// 	return p //allows method chaining
// }
// func (p *V3) SetY(y float64) *V3 {
// 	p.Y = y
// 	return p //allows method chaining
// }

func (p *V3) AsFloat32s() []float32 {
	return []float32{float32(p.X), float32(p.Y), float32(p.Z)}
}

func (dst *V3) TweenInto(a, b *V3, f float64) *V3 {

	dst.X = a.X + (b.X-a.X)*f
	dst.Y = a.Y + (b.Y-a.Y)*f
	dst.Z = a.Z + (b.Z-a.Z)*f
	return dst //dest is mutated - but returning it too allows method chaining
}
func (dst *V3) SetFrom(src *V3) *V3 {
	dst.X = src.X
	dst.Y = src.Y
	dst.Z = src.Z
	return dst
}
func (dst *V3) Set(x, y, z float64) *V3 {
	// dst = (x,y,z)
	dst.X = x
	dst.Y = y
	dst.Z = z
	return dst
}

// AddInto mutates dst to be the sum of the components
func (dst *V3) AddInto(components ...*V3) *V3 {
	// dst = a + b
	for _, b := range components {
		dst.X += b.X
		dst.Y += b.Y
		dst.Z += b.Z
	}

	return dst
}

// SubInto mutates dst to be a-b
func (dst *V3) SubInto(a, b *V3) *V3 {
	// dst = a - b
	dst.X = a.X - b.X
	dst.Y = a.Y - b.Y
	dst.Z = a.Z - b.Z
	return dst
}

// MulInto mutates dst to be a*s
func (dst *V3) MulInto(a *V3, s float64) *V3 {
	// dst = a * s
	dst.X = a.X * s
	dst.Y = a.Y * s
	dst.Z = a.Z * s
	return dst
}

// distanceBetweenLines returns the shortest distance between two lines (not line segments)
func DistanceBetweenLines(a1, a2, b1, b2 *V3) float64 {
	//from http://geomalgorithms.com/a07-_distance.html
	u := a2.Sub(a1)
	v := b2.Sub(b1)
	w := a1.Sub(b1)

	a := u.Dot(u)
	b := u.Dot(v)
	c := v.Dot(v)
	d := u.Dot(w)
	e := v.Dot(w)

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

	return a1.Add(u.Multiply(sc)).Sub(b1.Add(v.Multiply(tc))).Length()

	//return a1.add(u.multiply(sc))

}

// returns a copy of the vector with y set to 0
func (p *V3) Y0() *V3 {
	return &V3{p.X, 0, p.Z}
}

func (p *V3) ReflectInPlane(pop *V3, normal *V3) *V3 {
	return p.Sub(normal.Multiply(2 * (p.Dot(normal) - pop.Dot(normal))))
}

// func (p *V3) SignedDistanceFrom(plane *geom.Plane) float64 {
// 	return p.Sub(plane.point).Dot(plane.normal)
// }

// construct a new vector - use sparingly!
func NewVec3(x, y, z float64) *V3 {
	return &V3{x, y, z}
}

// add - adds vectors - prefer using AddInto to avoid allocations
func (a *V3) Add(b *V3) *V3 {
	return NewVec3(a.X+b.X, a.Y+b.Y, a.Z+b.Z)
}

// hypo3 - pythagorean in 3D - helped for dist
func hypo3(a, b, c float64) float64 {
	return math.Sqrt(float64(a*a + b*b + c*c))
}
func (p *V3) WriteTo(buff *bytes.Buffer) {

	le := binary.LittleEndian
	binary.Write(buff, le, float32(p.X)) //write x
	binary.Write(buff, le, float32(p.Y)) //write y
	binary.Write(buff, le, float32(p.Z)) //write z
}

func (a *V3) AddIn(b *V3) {
	a.X += b.X
	a.Y += b.Y
	a.Z += b.Z
}

func (a *V3) SubIn(b *V3) {
	a.X -= b.X
	a.Y -= b.Y
	a.Z -= b.Z
}

// func (p *V3) ProjectOntoPlane(plane *geom.Plane) *V3 {
// 	return plane.ClosestPointOnPlane(p)

// }

func (a *V3) LengthSq() float64 {
	return a.X*a.X + a.Y*a.Y + a.Z*a.Z
}

func (p *V3) RotateAbout(axis *V3, angle float64) *V3 {
	al := axis.Length()
	if al < .999 || al > 1.001 {
		panic("axis is not unit length")
	}
	if axis.Dot(p) == 1 {
		return p
	}
	if axis.Dot(p) == -1 {
		return p
	}

	// Rodrigues' rotation formula
	k := axis
	v := p
	cosTheta := math.Cos(angle)
	sinTheta := math.Sin(angle)

	term1 := v.Multiply(cosTheta)
	term2 := k.Cross(v).Multiply(sinTheta)
	term3 := k.Multiply(k.Dot(v) * (1 - cosTheta))

	return term1.Add(term2).Add(term3)

}

func (a *V3) Divide(f float64) *V3 {
	if f == 0 {
		panic("divide by zero")
	}
	return NewVec3(a.X/f, a.Y/f, a.Z/f)
}

func (a *V3) Multiply(f float64) *V3 {
	return NewVec3(a.X*f, a.Y*f, a.Z*f)
}

func (a *V3) Tween(b *V3, f float64) *V3 {
	return NewVec3(a.X+(b.X-a.X)*f, a.Y+(b.Y-a.Y)*f, a.Z+(b.Z-a.Z)*f)
}

func (a *V3) Sub(b *V3) *V3 {
	return NewVec3(a.X-b.X, a.Y-b.Y, a.Z-b.Z)
}

func (a *V3) Normalise() *V3 {
	l := a.Length()
	if l == 0 {
		panic("can't normalise 0 vector")
		//return a
	}
	return &V3{X: a.X / l, Y: a.Y / l, Z: a.Z / l}
}

// func (p *V3) FromByteBuffer(buff *bytes.Buffer) {

// 	le := binary.LittleEndian
// 	floats := make([]float32, 3) //the components of a vector are float64's but we downscale to 32bits for transmission/storage
// 	binary.Read(buff, le, &floats)
// 	p.X = float64(floats[0])
// 	p.Y = float64(floats[1])
// 	p.Z = float64(floats[2])

// }

func (a *V3) Length() float64 {
	return hypo3(a.X, a.Y, a.Z)
}

func (a *V3) Equals(b *V3) bool {
	if a == b {
		return true //these are two referrences to the same object
	}
	if a.X == b.X && a.Y == b.Y && a.Z == b.Z {
		return true
	}
	return false
}

func (a *V3) AlmostEquals(b *V3) bool {

	tolerance := 0.0001
	if math.Abs(a.X-b.X) < tolerance && math.Abs(a.Y-b.Y) < tolerance && math.Abs(a.Z-b.Z) < tolerance {
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

func (a *V3) Dot(b *V3) float64 {
	return a.X*b.X + a.Y*b.Y + a.Z*b.Z
}

func (a *V3) Cross(b *V3) *V3 {

	return &V3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}

func (d *V3) CrossInto(a, b *V3) *V3 {

	d.X = a.Y*b.Z - a.Z*b.Y
	d.Y = a.Z*b.X - a.X*b.Z
	d.Z = a.X*b.Y - a.Y*b.X

	return d
}

func (p *V3) SignedDistanceFromLineSegment(a, b, n *V3) float64 {

	cp := p.closestPointOnLineSegment(a, b)

	ab := b.Sub(a)
	ap := p.Sub(a)

	d := p.DistanceFrom(cp)

	if ap.Cross(ab).Dot(n) > 0 {
		d = -d
	}

	return d

}

func (p *V3) closestPointOnLineSegment(a, b *V3) *V3 {

	ab := b.Sub(a)
	ap := p.Sub(a)
	bp := p.Sub(b)
	if ap.Dot(ab) < 0 {
		return a //p is outside AB and closest to A
	}
	if bp.Dot(ab) > 0 { // p is outside AB and closest to B
		return b
	}

	return p.ClosestPointOnLine(a, b)

}

func (p *V3) DistanceFromLineSegment(a, b *V3) float64 {
	//TEST this
	ab := b.Sub(a)
	abn := ab.Normalise()
	ap := p.Sub(a)
	bp := p.Sub(b)
	if ap.Dot(abn) < 0 {
		return ap.Length() //p is outside AB and closest to A
	}
	if bp.Dot(abn) > 0 {
		return bp.Length() //p is outside AB and closest to B
	}
	return p.DistanceFromLine(a, b)
}

func (p V3) ClosestPointOnLine(a, b *V3) *V3 {
	ab := b.Sub(a)
	abn := ab.Normalise()
	dp := p.Sub(a).Dot(abn)
	return a.Add(abn.Multiply(dp))
}

func (p *V3) DistanceFromLine(a, b *V3) float64 {
	return p.ClosestPointOnLine(a, b).DistanceFrom(p)
}

func (p *V3) DistanceFrom(b *V3) float64 {
	return hypo3(p.X-b.X, p.Y-b.Y, p.Z-b.Z)
}

func (p *V3) LiesBetween(a *V3, b *V3) bool {

	if a.Equals(b) {
		panic("a and b are the same point")
	}
	if p.Equals(a) || p.Equals(b) {
		return true //if the point is at an endpoint, it doesn't lie *between* the endpoints
	}

	v1 := p.Sub(a) //vector from a to p
	v2 := p.Sub(b) //vector from a to p

	//if the dot product is negative, then the vectors (from the point to the endpoints) are pointing in opposite directions - and the point lies between A-B
	dp := v1.Dot(v2)
	return dp <= 0.00001 //the edge case is 0 when p is at an endpoint (dealt with above)

}

// // can be negative if the point on the side away from the normal
// func (p *vec3) signedDistanceFromTriPlane(a, n *vec3) float64 {
// 	return p.sub(a).dot(n)
// }

func (p *V3) Clone() *V3 {
	return NewVec3(p.X, p.Y, p.Z)
}

func (b *V3) SignedAngleFrom(a *V3, axis *V3) float64 {

	dp := a.Dot(b)
	alXbl := (a.Length() * b.Length())
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

	if b.Cross(a).Dot(axis) < 0 {
		angle = -angle
	}

	return angle

}

func (v *V3) Reflect(n *V3) *V3 {
	return v.Sub(n.Multiply(2 * v.Dot(n)))
}
