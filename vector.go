package main

import (
	"math"
)

type Vector struct {
	x, y float64 //32
}

func newVector(x, y float64) Vector {
	return Vector{x, y}
}

func (a *Vector) add(b Vector) Vector {
	return newVector(a.x+b.x, a.y+b.y)
}

func hypo(adjacent, opposite float64) float64 {
	return math.Sqrt(float64(adjacent*adjacent + opposite*opposite))
}
func (a Vector) distanceFrom(b *Vector) float64 {
	return hypo(a.x-b.x, a.y-b.y)
}

func (a *Vector) lengthSq() float64 {
	return a.x*a.x + a.y*a.y
}

func (a *Vector) subIn(b Vector) {
	a.x -= b.x
	a.y -= b.y
}

func (a *Vector) addIn(b Vector) {
	a.x += b.x
	a.y += b.y
}

func (a *Vector) multiply(f float64) Vector {
	return newVector(a.x*f, a.y*f)
}

func (a *Vector) subtract(b *Vector) Vector {
	return newVector(a.x-b.x, a.y-b.y)
}

func (a *Vector) normalise() Vector {
	l := a.length()
	return Vector{x: a.x / l, y: a.y / l}
}

func (a *Vector) length() float64 {
	return hypo(a.x, a.y)
}

func (a *Vector) Equals(b *Vector) bool {
	return a.x == b.x && a.y == b.y
}

func (p Vector) rotate(angle float64) Vector {
	x := p.x*math.Cos(angle) - p.y*math.Sin(angle)
	y := p.x*math.Sin(angle) + p.y*math.Cos(angle)
	return newVector(x, y)
}

func (a Vector) dot(b *Vector) float64 {
	return a.x*b.x + a.y*b.y
}

func (a Vector) cross(b Vector) float64 {
	return (a.x * b.y) - (a.y * b.x)
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
