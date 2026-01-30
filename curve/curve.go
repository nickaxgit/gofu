package curve

import (
	"github.com/nickax/gofu/tcs"
	"github.com/nickax/gofu/vec"
)

type Curve struct {
	Name   string
	Bounds tcs.Tcs
	Points []vec.V2 //x is 0-1 along the length of the segment, y is 0-1 along the radius of the segment
}

func (c *Curve) addPoint(x float64, y float64) {
	c.Points = append(c.Points, vec.NewVec2(x, y))
}

func (c *Curve) GetX(y float64) float64 {

	if y <= c.Points[0].Y {
		return c.Points[0].X
	}
	if y >= c.Points[len(c.Points)-1].Y {
		return c.Points[len(c.Points)-1].X
	}

	for i, v := range c.Points {
		if v.Y > y {
			prv := c.Points[i-1]
			f := (y - prv.Y) / (v.Y - prv.Y)
			return prv.Tween(v, f).X
		}
	}

	panic("getX: y value out of range")
}

func (c *Curve) GetY(x float64) float64 {

	if x >= c.Points[len(c.Points)-1].X {
		return c.Points[len(c.Points)-1].Y
	}
	if x <= c.Points[0].X {
		return c.Points[0].Y
	}

	for i, v := range c.Points {
		if v.X > x {
			prv := c.Points[i-1]
			f := (x - prv.X) / (v.X - prv.X)
			return prv.Tween(v, f).Y
		}
	}

	return 1
}

func New(name string, bounds tcs.Tcs, values ...float64) *Curve {
	c := Curve{Name: name, Bounds: bounds, Points: make([]vec.V2, 0)}
	for i := 0; i < len(values); i += 2 {
		c.addPoint(values[i], values[i+1])
	}

	return &c
}
