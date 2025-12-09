package cam

import (
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/vec"
)

type Camera struct {
	Position  vec.V3
	Direction vec.V3 //normalised
	Up        vec.V3
	FarPos    vec.V3
}

func (c *Camera) Clone() *Camera {
	return &Camera{Position: c.Position, Direction: c.Direction, Up: c.Up, FarPos: c.FarPos}

}

func (c *Camera) WriteTo(msg *msg.Msg) {
	msg.Write(c.Position, c.Direction, c.Up)
}

func New(position, direction, up vec.V3) *Camera {
	return &Camera{Position: position, Direction: direction, Up: up, FarPos: vec.NewVec3(0, 0, 0)}
}
