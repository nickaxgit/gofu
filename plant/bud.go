package plant

import (
	"github.com/nickax/gofu/curve"
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/mesh"
	"github.com/nickax/gofu/vec"
	"math"
	"math/rand/v2"
)

// buds are a template for adding segments to the tree - they are the DNA .. not the wood
type bud struct {
	//position  vec3    //in segment space
	twist     float64 //radians
	turn      float64
	grows     []*segmentType //what this bud is likely to sprout as it ages
	sproutAge float64        //how old the segment is before its buds generate new segments
}

type segmentType struct {
	name          string
	buds          []*bud
	tcs           mesh.Tcs
	sides         int     //number of sides when making a tubular mesh
	twistsFlat    float64 //tendacncy to twist such that the z axis is down (leaves)
	droopTo       float64 //0 - angle to which sement want to droop 0 is horizontal -pi is weepeing, pi is pointing directly upwards (mares tails)
	droopStrength float64 //how strongly the segment tends to its droop angle
	girthAtDay    *curve.Curve
	lengthAtDay   *curve.Curve
}

type segment struct { //of a plant/tree
	p     vec.V3
	xAxis vec.V3 //direction of (accumulated) twist
	//yAxis *vec3 //direction of growth
	//zAxis *vec3 //ortho to x and y (local down)
	//length float64
	radius  float64
	parent  *segment
	segType *segmentType
	//age      float64
	children []*segment
	//vis      []int
}

// func squareSimpleMesh(id uint16, materialName string) *simpleMesh {

// 	sm := newSimpleMesh(id, materialName, 4, 2)
// 	tcs := &tcs{left: 0, top: 0, right: 1, bottom: 1}                                    //texture atlas coordinates
// 	sm.billboard(NewVec3(0, 0, 0), NewVec3(0, 1, 0), NewVec3(0, 0, -1), 1, 1, 1, 4, tcs) //just to set up the arrays
// 	return sm

// }
func GrowTree(meshId uint16) *mesh.SimpleMesh {

	// segmentTypes:=make(map[string]*segmentType, 0)

	leafCurve := curve.New(0, 0, 10, 1)               //grow to 30cm over 20 days
	stalkLength := curve.New(0, 0, 20, 1.6, 100, 2)   //grow to 1 metre over 100 days
	stalkGirth := curve.New(0, .001, 70, .2, 100, .5) //rapid initial girth growth, then slow down

	stalk := NewSegmentType("stalk", mesh.NewTcs(0, 0, 1, 0.5), 8, 0, .25, 1, stalkGirth, stalkLength) //, 2, -math.Pi/4, .8, 1, nil, nil)
	leaf := NewSegmentType("leaf", mesh.NewTcs(0, .5, 1, 1), 2, -math.Pi/2*1.2, .9, 0.0, leafCurve, leafCurve)

	stalk.addBud(&bud{twist: math.Pi / 5, turn: math.Pi / 6, grows: []*segmentType{leaf, leaf, leaf, stalk, stalk, leaf, stalk, stalk, stalk}, sproutAge: 11})

	stalk.addBud(&bud{twist: -.1, turn: -math.Pi / 5, grows: []*segmentType{leaf, leaf, leaf, stalk, leaf, stalk, stalk}, sproutAge: 9})

	//stalk.addBud(&bud{twist: .1, direction: NewVec3(0, 1, -1).normalise(), grows: stalk, sproutAge: 9})
	// trunk:=segment{segType:segmentType{name:"stalk"}}

	trunk := NewSegment(nil, vec.NewVec3(0, 0, 0), vec.NewVec3(1, 0, 0), stalk, stalkGirth.GetY(100)) //segment{segType: stalk, age: 0, children: make([]*segment, 0)}

	sprouts := 0
	trunk.grow(120, &sprouts, 0)
	log.Logit("sprouts", sprouts)

	m := mesh.New(meshId, "atlas", 10000, 3000) //newLandMesh("tree", 20000, 10, 100, 100, kinks)
	//trunk.getBillBoardedMesh(m)                             //uses the meshes internal vert and face write pointer (vwp,fwp)
	trunk.getTubularMesh(m)

	return m

}

func pickSegmentType(grows []*segmentType, age float64) *segmentType {
	// Bias factor: higher age favors later entries
	//bias := float64(age) / 10.0 // tweak denominator for curve steepness

	// Random index with bias toward later entries
	//r := rand.Float64()
	//index := int(float64(len(grows)) * math.Pow(r, 1.0-bias))
	index := int(float64(len(grows)) * age / 100) //math.Pow(r, 1.0-bias))
	index += int(rand.Float64()*2.0 - 1.0)
	if index < 0 {
		index = 0
	}
	if index >= len(grows) {
		index = len(grows) - 1
	}
	return grows[index]
}

func (st *segmentType) addBud(b *bud) {
	st.buds = append(st.buds, b)
}

// as we recurse deeper - the age of segments goes down (they are younger)
// i am confusing hte age of the tree and theage of the segments - the age of the *tree* determines what will sprout from the buds
// a very old stalk is sure to sprot stalks
func (seg *segment) grow(age float64, sprouts *int, depth int) {

	//seg.age = age

	yAxis := vec.NewVec3(0, 1, 0)
	if seg.parent != nil {
		yAxis = seg.p.Sub(seg.parent.p).Normalise()
	}

	up := vec.NewVec3(0, 1, 0)

	for _, bud := range seg.segType.buds {
		if age >= bud.sproutAge { //randomise the sprout age a bit

			budAge := age - bud.sproutAge
			sproutType := pickSegmentType(bud.grows, budAge)

			*sprouts++
			xAxis := seg.xAxis.RotateAbout(yAxis, bud.twist).Normalise() //twist the x axis by the bud twist

			sproutYaxis := yAxis.RotateAbout(xAxis, bud.turn) //elevation
			//sproutYAxis = yAxis.rotateAbout(seg.xAxis, droop)
			droopedAxis := up.RotateAbout(vec.NewVec3(xAxis.X, 0, xAxis.Z).Normalise(), sproutType.droopTo) //the 'natural' droop angle for this leaf

			sproutYaxis = sproutYaxis.Tween(droopedAxis, sproutType.droopStrength) //tend towrds the droop angle
			//sproutYAxis = sproutYAxis.rotateAbout(seg.yAxis, rotation+(rand.Float64()-.5)/10).normalise()
			//sproutXAxis := seg.xAxis.rotateAbout(seg.yAxis, rotation+bud.twist+(rand.Float64()-.5)/5).normalise()

			xAxis.Y *= sproutType.twistsFlat //tend to twist the leaves flat
			xAxis = xAxis.Normalise()

			lad := sproutType.lengthAtDay.GetY(budAge)
			p := seg.p.Add(sproutYaxis.Multiply(lad))
			//sproutType := bud.grows[rand.IntN(len(bud.grows))] //randomly pick one of the possible segment types for this bud

			newSegment := NewSegment(seg, p, xAxis, sproutType, 0.5*sproutType.girthAtDay.GetY(budAge))
			seg.children = append(seg.children, newSegment)
			newSegment.grow(budAge, sprouts, depth+1)

		}
	}
}

// func (seg *segment) getBillBoardedMesh(m *mesh.SimpleMesh) {

// 	//uv := newVec2(0, 0)
// 	//po := m.addVert(seg.p, seg.xAxis, uv) //parent origin
// 	//n:=NewVec3(0,0,-1)

// 	campos := vec.NewVec3(0, 0, -10) //turn billboards to a virtual camera 10metres away
// 	for _, child := range seg.children {

// 		//co := m.addVert(child.p, child.xAxis, uv)
// 		//sw := m.addVert(child.p.add(seg.xAxis.multiply(1)), seg.zAxis, uv)
// 		//m.addFace(po, co, sw)

// 		//NB: width and heights are determined by the non-normalised 'up' vector
// 		m.Billboard(seg.p, child.p.Sub(seg.p), campos, seg.radius, child.radius, 1, 4, child.segType.tcs)

// 		child.getBillBoardedMesh(m) //recurse
// 	}

// }

func (seg *segment) getTubularMesh(m *mesh.SimpleMesh) {

	for _, child := range seg.children {

		//m.addTube(seg.p, child.p, seg.xAxis, seg.radius, child.radius, child.segType.tcs, child.segType.sides)
		m.AddTube(seg.p, child.p, child.xAxis, seg.radius, child.radius, child.segType.tcs, child.segType.sides)

		child.getTubularMesh(m) //recurse
	}

}

func NewSegment(ps *segment, p vec.V3, xAxis vec.V3, segType *segmentType, radius float64) *segment {

	return &segment{parent: ps, p: p, xAxis: xAxis, segType: segType, radius: radius}

}

func NewSegmentType(name string, tcs mesh.Tcs, sides int, droopTo, droopStrength, twistsFlat float64, girthAtDays *curve.Curve, lengthAtDays *curve.Curve) *segmentType {
	// segmentTypes[name]=
	return &segmentType{name: name, tcs: tcs, sides: sides, droopTo: droopTo, droopStrength: droopStrength, twistsFlat: twistsFlat, girthAtDay: girthAtDays, lengthAtDay: lengthAtDays, buds: make([]*bud, 0)}
}
