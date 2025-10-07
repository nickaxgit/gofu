package main

import (
	"math"
)

type vec2 struct {
	x float64
	y float64
}

func newVector(x, y float64) *vec2 {
	return &vec2{x, y}
}

func (a *vec2) add(b *vec2) *vec2 {
	return newVector(a.x+b.x, a.y+b.y)
}

func hypo(adjacent, opposite float64) float64 {
	return math.Sqrt(float64(adjacent*adjacent + opposite*opposite))
}
func (a vec2) distanceFrom(b *vec2) float64 {
	return hypo(a.x-b.x, a.y-b.y)
}

func (a *vec2) lengthSq() float64 {
	return a.x*a.x + a.y*a.y
}

func (a *vec2) subIn(b vec2) {
	a.x -= b.x
	a.y -= b.y
}

func (a *vec2) addIn(b vec2) {
	a.x += b.x
	a.y += b.y
}

func (a *vec2) multiply(f float64) *vec2 {
	return newVector(a.x*f, a.y*f)
}

func (a *vec2) subtract(b *vec2) *vec2 {
	return newVector(a.x-b.x, a.y-b.y)
}

func (a *vec2) normalise() *vec2 {
	l := a.length()
	return &vec2{x: a.x / l, y: a.y / l}
}

func (a *vec2) length() float64 {
	return hypo(a.x, a.y)
}

func (a *vec2) Equals(b *vec2) bool {
	return a.x == b.x && a.y == b.y
}

func (p vec2) rotate(angle float64) *vec2 {
	x := p.x*math.Cos(angle) - p.y*math.Sin(angle)
	y := p.x*math.Sin(angle) + p.y*math.Cos(angle)
	return newVector(x, y)
}

func (a *vec2) dot(b *vec2) float64 {
	return a.x*b.x + a.y*b.y
}

func (a vec2) cross(b vec2) float64 {
	return (a.x * b.y) - (a.y * b.x)
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
