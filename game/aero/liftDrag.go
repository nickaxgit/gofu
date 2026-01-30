package aero

import (
	"github.com/nickax/gofu/curve"
	"github.com/nickax/gofu/tcs"
	"github.com/nickax/gofu/vec"
)

var Rho = 1.225                                                     //kg/ m3 //atmospheric air density at sea level
var TestFlight = vec.NewVec3(0, -.1, -1).Normalised().Multiply(.33) //50 m/s

type Section byte //do vstab - also give masses mass -
const (
	Cambered   Section = 0
	Symetrical Section = 1
)

var SectionNames = map[Section]string{Cambered: "Cambered", Symetrical: "Symetrical"}

// looseley basedon  https://aerospaceweb.org/question/airfoils/q0150b.shtml (for high alpha values)

var liftBounds = tcs.NewTcs(-90, -3, 90, 3)

var LiftCurves = []*curve.Curve{ //cambered lift Alpha (degrees), Cl

	curve.New("Cambered lift", liftBounds, //cambered lift
		-90, 0,
		-45, -1.5,
		-5, 0,
		10, 1.5,
		15, 1.75,
		20, 1.5,
		50, 1.7,
		90, 0),
	curve.New("Sym Lift", liftBounds, //symetrical lift
		-90, 0,
		-45, -1.5,
		0, 0,
		45, 1.5,
		90, 0),
}

var dragBounds = tcs.NewTcs(-90, 0, 90, 1.0)
var DragCurves = []*curve.Curve{
	curve.New("Cambered drag", dragBounds,
		-90, 1,
		-45, 0.5,
		-20, 0.1,
		-0, 0.01,
		20, 0.1,
		45, 0.5,
		90, 1),
	curve.New("Symetrical drag", dragBounds,
		-90, 1,
		-45, 0.5,
		-5, .1,
		0, .01,
		5, .1,
		45, 0.5,
		90, 1,
	)}
