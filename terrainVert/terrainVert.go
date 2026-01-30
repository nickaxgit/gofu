package terrainVert

import (
	"github.com/nickax/gofu/curve"
	"github.com/nickax/gofu/tcs"
	"github.com/nickax/gofu/vec"
)

//mud,sand, grass, rock, snow
var Contours = curve.New("Contours", tcs.NewTcs(0, -1000, 5, 1000),
	0, -1000, //begin mud
	1, -10, //begin sand
	2, 50, // begin grass
	3, 100, //begin rock
	4, 400, //begin snow
	5, 700, //end snow
)

//at each interface (e.g. mud-sand) - how strongly and in which direction to adjust texture coords based on steepness
var Steeps = curve.New("Steeps", tcs.NewTcs(0, -5, 5, 5),
	0, 0,
	1, 0,
	2, 0,
	3, 0,
	4, 0,
	5, 0,
)

type Vert struct {
	P vec.V3
	//n  vec.V3 - now in "touchedverts"
	Uv vec.V2
	Wl float64 //water level
}

func New(p vec.V3, uv vec.V2, wl float64) *Vert {
	return &Vert{P: p, Wl: wl, Uv: uv}
}

func (a *Vert) Flow(b *Vert) {

	//if a.wl > -.1 && b.wl > -.1 {

	if a.Wl > 0 || b.Wl > 0 {
		diff := (a.P.Y + a.Wl) - (b.P.Y + b.Wl) //uses the absolute water level

		a.Wl -= diff * .25
		b.Wl += diff * .25
	}

	//erode the land
	//a.p.Y -= diff * .01
	//b.p.Y -= diff * .01

	//}

}
