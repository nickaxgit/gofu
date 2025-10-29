package curve

import (
	"github.com/nickax/gofu/vec"
)

type Curve struct {
	points []*vec.V2 //x is 0-1 along the length of the segment, y is 0-1 along the radius of the segment
}

func (c *Curve) addPoint(x float64, y float64) {
	c.points = append(c.points, vec.NewVec2(x, y))
}

func (c *Curve) GetY(x float64) float64 {

	if x >= c.points[len(c.points)-1].GetX() {
		return c.points[len(c.points)-1].GetY()
	}

	for i, v := range c.points {
		if v.GetX() > x {
			prv := c.points[i-1]
			f := (x - prv.GetX()) / (v.GetX() - prv.GetX())
			return prv.Tween(v, f).GetY()
		}
	}

	return 1
}

func New(values ...float64) *Curve {
	c := Curve{points: make([]*vec.V2, 0)}
	for i := 0; i < len(values); i += 2 {
		c.addPoint(values[i], values[i+1])
	}

	return &c
}
