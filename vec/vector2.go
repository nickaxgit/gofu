package vec

import (
	"math"
)

type V2 struct {
	X float64
	Y float64
}

func (uv *V2) UpdateUVxFromNormal(n V3) {
	uv.X = math.Atan2(n.X, n.Z) / float64(6.28)
}

func (p *V2) AsFloat32s() []float32 {
	return []float32{float32(p.X), float32(p.Y)}
}

func (a *V2) Tween(b *V2, f float64) *V2 {
	return NewVec2(a.X+(b.X-a.X)*f, a.Y+(b.Y-a.Y)*f)
}

func NewVec2(x, y float64) *V2 {
	return &V2{x, y}
}

func (a *V2) Add(b *V2) *V2 {
	return NewVec2(a.X+b.X, a.Y+b.Y)
}

func hypo(adjacent, opposite float64) float64 {
	return math.Sqrt(float64(adjacent*adjacent + opposite*opposite))
}
func (a V2) DistanceFrom(b *V2) float64 {
	return hypo(a.X-b.X, a.Y-b.Y)
}

func (a *V2) LengthSq() float64 {
	return a.X*a.X + a.Y*a.Y
}

func (a *V2) SubIn(b V2) {
	a.X -= b.X
	a.Y -= b.Y
}

func (a *V2) AddIn(b V2) {
	a.X += b.X
	a.Y += b.Y
}

func (a *V2) Mul(f float64) *V2 {
	return NewVec2(a.X*f, a.Y*f)
}

func (a *V2) Sub(b *V2) *V2 {
	return NewVec2(a.X-b.X, a.Y-b.Y)
}

func (a *V2) Normalise() *V2 {
	l := a.Length()
	return &V2{X: a.X / l, Y: a.Y / l}
}

func (a *V2) Length() float64 {
	return hypo(a.X, a.Y)
}

func (a *V2) Equals(b *V2) bool {
	return a.X == b.X && a.Y == b.Y
}

func (p V2) Rotate(angle float64) *V2 {
	x := p.X*math.Cos(angle) - p.Y*math.Sin(angle)
	y := p.X*math.Sin(angle) + p.Y*math.Cos(angle)
	return NewVec2(x, y)
}

func (a *V2) Dot(b *V2) float64 {
	return a.X*b.X + a.Y*b.Y
}

func (a V2) Cross(b *V2) float64 {
	return (a.X * b.Y) - (a.Y * b.X)
}

func (p *V2) Clone() *V2 {
	return &V2{X: p.X, Y: p.Y}
}

func (p V2) closestPointOnLine(a, b *V2) *V2 {
	ab := b.Sub(a)
	abn := ab.Normalise()
	dp := p.Sub(a).Dot(abn)
	return a.Add(abn.Mul(dp))
}

func (p *V2) DistanceFromLine(a, b *V2) float64 {
	return p.closestPointOnLine(a, b).DistanceFrom(p)
}

func (p *V2) LiesBetween(a *V2, b *V2) bool {

	v1 := p.Sub(a) //vector from a to p
	v2 := p.Sub(b) //vector from a to p

	//if the dot product is negative, then the vectors (from the point to the endpoints) are pointing in opposite directions - and the point lies between A-B
	return v1.Dot(v2) < 0

}
