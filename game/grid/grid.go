package grid

import "github.com/nickax/gofu/vec"
import "github.com/nickax/gofu/cam"
import "github.com/nickax/gofu/game/msg"
import "github.com/nickax/gofu/plane"

type Grid struct {
	Origin   vec.V3 //defines the plane of this players construction grid
	Xaxis    vec.V3 //defines the plane of this players construction grid
	Yaxis    vec.V3 //defines the plane of this players construction grid
	GridPos  vec.V3
	Zoff     float64 //offset from the grid (along the grid normal)
	SpacePos vec.V3
}

func New(origin, xaxis, yaxis vec.V3) *Grid {
	return &Grid{Origin: origin, Xaxis: xaxis, Yaxis: yaxis, GridPos: vec.V3{}, Zoff: 0, SpacePos: vec.V3{}}
}

func (g *Grid) Plane() plane.Plane {

	return plane.NewFromNormalAndPoint(g.Normal(), g.Origin)
}

func (g *Grid) Normal() vec.V3 {
	return g.Yaxis.Cross(g.Xaxis).Normalise()
}

func (g *Grid) UpdateGridPosAndSpacePos(cam *cam.Camera) {
	gn := g.Normal()

	offsetPlane := plane.NewFromNormalAndPoint(gn, g.Origin.Add(gn.Multiply(g.Zoff)))
	cpop := offsetPlane.ClosestPointOnPlane(cam.Position)
	c2g := cpop.Sub(cam.Position) //vector from the camera pos to the grid
	ttg := cpop.Sub(cam.FarPos)

	if c2g.Dot(ttg) < 0 {
		d0 := c2g.Length()
		d1 := ttg.Length()
		f := d0 / (d0 + d1)
		g.SpacePos = cam.Position.Tween(cam.FarPos, f)
		g.GridPos = g.SpacePos.Sub(gn.Multiply(g.Zoff))
	}

}

// func (g *Grid) fromMsg(msg *msg.Msg) {

// 	hdr := msg.MsgEnum(0)
// 	msg.Read(&hdr)
// 	if hdr != msg.Grid {
// 		panic("grid not next")
// 	}
// 	g := &Grid{}
// 	msg.Read(&g.Origin, &g.Xaxis, &g.Yaxis)

// 	return grid

// }

func (g *Grid) AsMsg() *msg.Msg {

	return msg.NewMsg(msg.Grid, g.Origin, g.Xaxis, g.Yaxis)

}

func (g *Grid) WriteTo(msg *msg.Msg) {
	msg.Write(g.Origin, g.Xaxis, g.Yaxis)
}
