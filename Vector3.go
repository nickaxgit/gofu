package main

import (
	"math"
)

type Vec3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

func newVec3(x, y, z float64) Vec3 {
	return Vec3{x, y, z}
}

// func (a *Vec3) add(b Vec3) Vec3 {
// 	return newVec3(a.X+b.X, a.Y+b.Y, a.Z+b.Z)
// }

func hypo3(a, b, c float64) float64 {
	return math.Sqrt(float64(a*a + b*b + c*c))
}

func (a Vec3) distanceFrom(b *Vec3) float64 {
	return hypo3(a.X-b.X, a.Y-b.Y, a.Z-b.Z)
}

// func (a *Vec3) lengthSq3() float64 {
// 	return a.X*a.X + a.Y*a.Y + a.Z*a.Z
// }

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

// func (a *Vec3) multiply(f float64) Vec3 {
// 	return newVec3(a.X*f, a.Y*f, a.Z*f)
// }

func (a *Vec3) tween(b *Vec3, f float64) Vec3 {
	return newVec3(a.X+(b.X-a.X)*f, a.Y+(b.Y-a.Y)*f, a.Z+(b.Z-a.Z)*f)
}

// func (a *Vec3) subtract(b *Vec3) Vec3 {
// 	return newVec3(a.X-b.X, a.Y-b.Y, a.Z-b.Z)
// }

// func (a *Vec3) normalise() Vector {
// 	l := a.length()
// 	return Vector{X: a.X / l, Y: a.Y / l}
// }

// func (a *Vec3) length() float64 {
// 	return hypo3(a.X, a.Y, a.Z)
// }

func (a *Vec3) Equals(b *Vec3) bool {
	return a.X == b.X && a.Y == b.Y && a.Z == b.Z
}

// func (p Vector) rotate(angle float64) Vector {
// 	x := p.X*math.Cos(angle) - p.Y*math.Sin(angle)
// 	y := p.X*math.Sin(angle) + p.Y*math.Cos(angle)
// 	return newVector(x, y)
// }

// func (a Vector) dot(b *Vector) float64 {
// 	return a.X*b.X + a.Y*b.Y
// }

// func (a Vector) cross(b Vector) float64 {
// 	return (a.X * b.Y) - (a.Y * b.X)
// }

// func (p Vector) closestPointOnLine(a, b *Vector) Vector {
// 	ab := b.subtract(a)
// 	abn := ab.normalise()
// 	dp := p.subtract(a).dot(&abn)
// 	return a.add(abn.multiply(dp))
// }

// func (p *Vector) distanceFromLine(a, b *Vector) float64 {
// 	return p.closestPointOnLine(a, b).distanceFrom(p)
// }

// func (p *Vector) liesBetween(a *Vector, b *Vector) bool {

// 	v1 := p.subtract(a) //vector from a to p
// 	v2 := p.subtract(b) //vector from a to p

// 	//if the dot product is negative, then the vectors (from the point to the endpoints) are pointing in opposite directions - and the point lies between A-B
// 	return v1.dot(&v2) < 0

// }
