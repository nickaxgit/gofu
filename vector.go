package main

import (
	"math"
)

type Vector struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func newVector(x, y float64) Vector {
	return Vector{x, y}
}

func (a *Vector) add(b Vector) Vector {
	return newVector(a.X+b.X, a.Y+b.Y)
}

func hypo(adjacent, opposite float64) float64 {
	return math.Sqrt(float64(adjacent*adjacent + opposite*opposite))
}
func (a Vector) distanceFrom(b *Vector) float64 {
	return hypo(a.X-b.X, a.Y-b.Y)
}

func (a *Vector) lengthSq() float64 {
	return a.X*a.X + a.Y*a.Y
}

func (a *Vector) subIn(b Vector) {
	a.X -= b.X
	a.Y -= b.Y
}

func (a *Vector) addIn(b Vector) {
	a.X += b.X
	a.Y += b.Y
}

func (a *Vector) multiply(f float64) Vector {
	return newVector(a.X*f, a.Y*f)
}

func (a *Vector) subtract(b *Vector) Vector {
	return newVector(a.X-b.X, a.Y-b.Y)
}

func (a *Vector) normalise() Vector {
	l := a.length()
	return Vector{X: a.X / l, Y: a.Y / l}
}

func (a *Vector) length() float64 {
	return hypo(a.X, a.Y)
}

func (a *Vector) Equals(b *Vector) bool {
	return a.X == b.X && a.Y == b.Y
}

func (p Vector) rotate(angle float64) Vector {
	x := p.X*math.Cos(angle) - p.Y*math.Sin(angle)
	y := p.X*math.Sin(angle) + p.Y*math.Cos(angle)
	return newVector(x, y)
}

func (a Vector) dot(b *Vector) float64 {
	return a.X*b.X + a.Y*b.Y
}

func (a Vector) cross(b Vector) float64 {
	return (a.X * b.Y) - (a.Y * b.X)
}

func (p Vector) closestPointOnLine(a, b *Vector) Vector {
	ab := b.subtract(a)
	abn := ab.normalise()
	dp := p.subtract(a).dot(&abn)
	return a.add(abn.multiply(dp))
}

func (p *Vector) distanceFromLine(a, b *Vector) float64 {
	return p.closestPointOnLine(a, b).distanceFrom(p)
}

func (p *Vector) liesBetween(a *Vector, b *Vector) bool {

	v1 := p.subtract(a) //vector from a to p
	v2 := p.subtract(b) //vector from a to p

	//if the dot product is negative, then the vectors (from the point to the endpoints) are pointing in opposite directions - and the point lies between A-B
	return v1.dot(&v2) < 0

}
