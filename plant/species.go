package plant

import (
	"math"

	"github.com/nickax/gofu/curve"
	"github.com/nickax/gofu/tcs"
)

type Species struct {
	name     string
	startSeg string
	segments map[string]*segmentType //segments contan buds
}

func NewSpecies(name string, startSeg string, segmentTypes ...*segmentType) *Species {

	sts := make(map[string]*segmentType)
	for _, st := range segmentTypes {
		sts[st.name] = st
	}

	return &Species{
		name:     name,
		startSeg: startSeg,
		segments: sts,
	}

}

//note segmentTypes have an array of buds (possible child segment types/probablities)
func (species *Species) AddSegmentType(segType *segmentType) {
	species.segments[segType.name] = segType
}

func ExampleSpecies() *Species {
	//plant.NewSpecies("example", "stalk")

	pi := math.Pi

	leafSize := curve.New("leafSize", tcs.NewTcs(0, 0, 100, 1), 0, 0, 10, .2)                     //grow to 20cm over 10 days
	stalkLength := curve.New("stalkLength", tcs.NewTcs(0, 0, 100, 3), 0, 0, 20, .4, 100, 1)       //grow to 1 metre over 100 days
	stalkRadius := curve.New("stalkRadius", tcs.NewTcs(0, 0, 100, .5), 0, .001, 70, .1, 100, .15) //rapid initial girth growth, then slow down
	droopToward := curve.New("droopTowards", tcs.NewTcs(0, -pi/2, 0, pi), 0, pi, 100, -pi/2)
	droopStrength := curve.New("droopStrength", tcs.NewTcs(0, 0, 100, 1), 0, .5, 100, .2)
	twistsFlat := curve.New("twistsFlat", tcs.NewTcs(0, 0, 100, 1), 0, 0, 10, .9)
	noTwistFlat := curve.New("noTwistFlat", tcs.NewTcs(0, 0, 100, 1), 0, 0, 10, 0)

	stalk := NewSegmentType("stalk", tcs.NewTcs(0, 0, .5, 0.5), 8, droopToward, droopStrength, noTwistFlat, stalkRadius, stalkLength) //, 2, -math.Pi/4, .8, 1, nil, nil)
	leaf := NewSegmentType("leaf", tcs.NewTcs(0, .5, .5, 1), 2, droopToward, droopStrength, twistsFlat, leafSize, leafSize)

	stalk.addBud(&bud{twist: math.Pi / 5, turn: math.Pi / 6,
		grows: []*segmentType{leaf, leaf, leaf, stalk, stalk, leaf, stalk, stalk, stalk}, sproutAge: 11})

	stalk.addBud(&bud{twist: -.1, turn: -math.Pi / 5, grows: []*segmentType{leaf, leaf, stalk, leaf, stalk, leaf, stalk, stalk}, sproutAge: 9})

	//stalk.addBud(&bud{twist: .1, direction: NewVec3(0, 1, -1).normalise(), grows: stalk, sproutAge: 9})
	// trunk:=segment{segType:segmentType{name:"stalk"}}

	return NewSpecies("example", "stalk", stalk, leaf)

}

var Distributions = []*Distribution{
	NewDistribution("pine", 103),
	NewDistribution("grass", 101),
	NewDistribution("oak", 0),
	NewDistribution("willow", 0),
	NewDistribution("maple", 0),
	NewDistribution("knotweed", 0),
	NewDistribution("bamboo", 0),
	NewDistribution("rock", 1050),
}

type Distribution struct {
	Name        string       //oak, grass, rock, boulder etc.
	meshId      uint16       //mesh to use for this distribution
	Altitude    *curve.Curve //altitude distribution curve
	Slope       *curve.Curve //slope distribution curve
	Direction   *curve.Curve //aspect distribution curve
	Density     float64      //how many division (per level 8 triangle) (is an int really beut we bind to it)
	BbDistance  float64      //distance after which client should render billboards
	MaxDistance float64      //maximum distance at which plant is visible (as a billboard or model)
}

func NewDistribution(name string, meshId uint16) *Distribution {
	//likeleyhood that a plant will be found at a given altitude/slope/direction

	altCurve := curve.New("altitude", tcs.NewTcs(0, 0, 600, 1), 0, 0, 100, .1, 200, .5, 300, .2, 400, .6, 600, 0)
	slopeCurve := curve.New("slope", tcs.NewTcs(0, 0, 90, 1), 0, .1, 25, .5, 45, .1, 90, 0)
	directionCurve := curve.New("direction", tcs.NewTcs(0, 0, 360, 1), 0, 1, 360, 1)
	return &Distribution{Name: name, meshId: meshId, Altitude: altCurve, Slope: slopeCurve, Direction: directionCurve, Density: 4}
}

//this structure is as light as possible because there will be millions of plants/rocks
type Instance struct { //instances live on triangles in a slice of slices, indexed by distribution
	//distribution     uint16
	BcU         byte //barycentric U
	BcV         byte //barycentric V
	Rotation    byte
	Scale       byte //encoded/stored transportedorted as a single byte (0-255) representing 0.01 to 100.0 scale (logarithmically)
	Ycorrection int8 //as the terrain is split further - the plant may need to be moved up or down to sit on the surface
}

//helper to get the max distance squared for a species
func MaxDistSq(di int) float64 {
	md := Distributions[di].MaxDistance
	return md * md
}

func New(u, v float64, scale float64, rotation byte) Instance {
	return Instance{
		BcU:      byte(u * 255),
		BcV:      byte(v * 255),
		Scale:    EncodeScale(scale), //store scale in a singe byte (logarithmically) 1.0 maps to 1.0 range 0.01 to 100.0
		Rotation: rotation,
	}
}

func EncodeScale(scale float64) byte {

	s := math.Min(100, math.Max(0.01, float64(scale)))
	v := math.Log10(s)               // [-2, 2]
	b := math.Round(128 + 127*(v/2)) // center 128 at v=0
	// Keep within 1..255 to avoid slight overshoot at extreme rounding
	return byte(math.Max(1, math.Min(255, b)))
}

func DecodeScale(b byte) float64 {
	v := (float64(b) - 128) * (2.0 / 127.0) // [-2, 2]
	s := math.Pow(10, v)
	return s
}
