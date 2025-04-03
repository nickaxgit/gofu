package main

import (
	"math"
)

type vec2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func newVector(x, y float64) *vec2 {
	return &vec2{x, y}
}

func (a *vec2) add(b *vec2) *vec2 {
	return newVector(a.X+b.X, a.Y+b.Y)
}

func hypo(adjacent, opposite float64) float64 {
	return math.Sqrt(float64(adjacent*adjacent + opposite*opposite))
}
func (a vec2) distanceFrom(b *vec2) float64 {
	return hypo(a.X-b.X, a.Y-b.Y)
}

func (a *vec2) lengthSq() float64 {
	return a.X*a.X + a.Y*a.Y
}

func (a *vec2) subIn(b vec2) {
	a.X -= b.X
	a.Y -= b.Y
}

func (a *vec2) addIn(b vec2) {
	a.X += b.X
	a.Y += b.Y
}

func (a *vec2) multiply(f float64) *vec2 {
	return newVector(a.X*f, a.Y*f)
}

func (a *vec2) subtract(b *vec2) *vec2 {
	return newVector(a.X-b.X, a.Y-b.Y)
}

func (a *vec2) normalise() *vec2 {
	l := a.length()
	return &vec2{X: a.X / l, Y: a.Y / l}
}

func (a *vec2) length() float64 {
	return hypo(a.X, a.Y)
}

func (a *vec2) Equals(b *vec2) bool {
	return a.X == b.X && a.Y == b.Y
}

func (p vec2) rotate(angle float64) *vec2 {
	x := p.X*math.Cos(angle) - p.Y*math.Sin(angle)
	y := p.X*math.Sin(angle) + p.Y*math.Cos(angle)
	return newVector(x, y)
}

func (a *vec2) dot(b *vec2) float64 {
	return a.X*b.X + a.Y*b.Y
}

func (a vec2) cross(b vec2) float64 {
	return (a.X * b.Y) - (a.Y * b.X)
}

func (p vec2) closestPointOnLine(a, b *vec2) *vec2 {
	ab := b.subtract(a)
	abn := ab.normalise()
	dp := p.subtract(a).dot(abn)
	return a.add(abn.multiply(dp))
}

func (p *vec2) distanceFromLine(a, b *vec2) float64 {
	return p.closestPointOnLine(a, b).distanceFrom(p)
}

func (p *vec2) liesBetween(a *vec2, b *vec2) bool {

	v1 := p.subtract(a) //vector from a to p
	v2 := p.subtract(b) //vector from a to p

	//if the dot product is negative, then the vectors (from the point to the endpoints) are pointing in opposite directions - and the point lies between A-B
	return v1.dot(v2) < 0

}
