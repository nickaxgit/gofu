package main

import (
	"math"
	"math/rand/v2"
)

type tcs struct {
	left, top, right, bottom float64
}

// buds are a template for adding segments to the tree - they are the DNA .. not the wood
type bud struct {
	//position  vec3    //in segment space
	twist     float64 //radians
	turn      float64
	grows     []*segmentType //what this bud is likely to sprout as it ages
	sproutAge float64        //how old the segment is before its buds generate new segments
}

type curve struct {
	points []*vec2 //x is 0-1 along the length of the segment, y is 0-1 along the radius of the segment
}

type segmentType struct {
	name          string
	buds          []*bud
	tcs           *tcs
	sides         int     //number of sides when making a tubular mesh
	twistsFlat    float64 //tendacncy to twist such that the z axis is down (leaves)
	droopTo       float64 //0 - angle to which sement want to droop 0 is horizontal -pi is weepeing, pi is pointing directly upwards (mares tails)
	droopStrength float64 //how strongly the segment tends to its droop angle
	girthAtDay    *curve
	lengthAtDay   *curve
}

func (c *curve) addPoint(x float64, y float64) {
	c.points = append(c.points, &vec2{x, y})
}

func (c *curve) getY(x float64) float64 {

	if x >= c.points[len(c.points)-1].x {
		return c.points[len(c.points)-1].y
	}

	for i, v := range c.points {
		if v.x > x {
			prv := c.points[i-1]
			f := (x - prv.x) / (v.x - prv.x)
			return prv.tween(v, f).y
		}
	}

	return 1
}

func newCurve(values ...float64) *curve {
	c := curve{points: make([]*vec2, 0)}
	for i := 0; i < len(values); i += 2 {
		c.addPoint(values[i], values[i+1])
	}

	return &c
}

type segment struct { //of a plant/tree
	p     *vec3
	xAxis *vec3 //direction of (accumulated) twist
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

func (i *tcs) clone() *tcs {
	return &tcs{left: i.left, top: i.top, right: i.right, bottom: i.bottom}
}

func (i *tcs) flipV() *tcs {
	return &tcs{left: i.left, top: i.bottom, right: i.right, bottom: i.top}
}

func (i *tcs) flipH() *tcs {
	return &tcs{left: i.right, top: i.top, right: i.left, bottom: i.bottom}
}

func squareSimpleMesh(id uint16, materialName string) *simpleMesh {

	sm := newSimpleMesh(id, materialName, 4, 2)
	tcs := &tcs{left: 0, top: 0, right: 1, bottom: 1}                                    //texture atlas coordinates
	sm.billboard(newVec3(0, 0, 0), newVec3(0, 1, 0), newVec3(0, 0, -1), 1, 1, 1, 4, tcs) //just to set up the arrays
	return sm

}
func growTree() *simpleMesh {

	// segmentTypes:=make(map[string]*segmentType, 0)

	leafCurve := newCurve(0, 0, 10, 1)               //grow to 30cm over 20 days
	stalkLength := newCurve(0, 0, 20, 1.6, 100, 2)   //grow to 1 metre over 100 days
	stalkGirth := newCurve(0, .001, 70, .2, 100, .5) //rapid initial girth growth, then slow down

	stalk := NewSegmentType("stalk", &tcs{left: 0, top: 0, right: 1, bottom: 0.5}, 8, 0, .25, 1, stalkGirth, stalkLength) //, 2, -math.Pi/4, .8, 1, nil, nil)
	leaf := NewSegmentType("leaf", &tcs{left: 0, top: .5, right: 1, bottom: 1}, 2, -math.Pi/2*1.2, .9, 0.0, leafCurve, leafCurve)

	stalk.addBud(&bud{twist: math.Pi / 5, turn: math.Pi / 6, grows: []*segmentType{leaf, leaf, leaf, stalk, stalk, leaf, stalk, stalk, stalk}, sproutAge: 11})

	stalk.addBud(&bud{twist: -.1, turn: -math.Pi / 5, grows: []*segmentType{leaf, leaf, leaf, stalk, leaf, stalk, stalk}, sproutAge: 9})

	//stalk.addBud(&bud{twist: .1, direction: newVec3(0, 1, -1).normalise(), grows: stalk, sproutAge: 9})

	// trunk:=segment{segType:segmentType{name:"stalk"}}

	trunk := NewSegment(nil, newVec3(0, 0, 0), newVec3(1, 0, 0), stalk, stalkGirth.getY(100)) //segment{segType: stalk, age: 0, children: make([]*segment, 0)}

	sprouts := 0
	trunk.grow(100, &sprouts, 0)
	logit("sprouts", sprouts)

	m := newSimpleMesh(100, "atlas", 3000, 1000) //newLandMesh("tree", 20000, 10, 100, 100, kinks)
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

	yAxis := newVec3(0, 1, 0)
	if seg.parent != nil {
		yAxis = seg.p.sub(seg.parent.p).normalise()
	}

	up := newVec3(0, 1, 0)

	for _, bud := range seg.segType.buds {
		if age >= bud.sproutAge { //randomise the sprout age a bit

			budAge := age - bud.sproutAge
			sproutType := pickSegmentType(bud.grows, budAge)

			*sprouts++
			xAxis := seg.xAxis.rotateAbout(yAxis, bud.twist).normalise() //twist the x axis by the bud twist

			sproutYaxis := yAxis.rotateAbout(xAxis, bud.turn) //elevation
			//sproutYAxis = yAxis.rotateAbout(seg.xAxis, droop)
			droopedAxis := up.rotateAbout(newVec3(xAxis.x, 0, xAxis.z).normalise(), sproutType.droopTo) //the 'natural' droop angle for this leaf

			sproutYaxis = sproutYaxis.tween(droopedAxis, sproutType.droopStrength) //tend towrds the droop angle
			//sproutYAxis = sproutYAxis.rotateAbout(seg.yAxis, rotation+(rand.Float64()-.5)/10).normalise()
			//sproutXAxis := seg.xAxis.rotateAbout(seg.yAxis, rotation+bud.twist+(rand.Float64()-.5)/5).normalise()

			xAxis.y *= sproutType.twistsFlat //tend to twist the leaves flat
			xAxis = xAxis.normalise()

			lad := sproutType.lengthAtDay.getY(budAge)
			p := seg.p.add(sproutYaxis.multiply(lad))
			//sproutType := bud.grows[rand.IntN(len(bud.grows))] //randomly pick one of the possible segment types for this bud

			newSegment := NewSegment(seg, p, xAxis, sproutType, 0.5*sproutType.girthAtDay.getY(budAge))
			seg.children = append(seg.children, newSegment)
			newSegment.grow(budAge, sprouts, depth+1)

		}
	}
}

func (seg *segment) getBillBoardedMesh(m *simpleMesh) {

	//uv := newVec2(0, 0)
	//po := m.addVert(seg.p, seg.xAxis, uv) //parent origin
	//n:=newVec3(0,0,-1)

	campos := newVec3(0, 0, -10) //turn billboards to a virtual camera 10metres away
	for _, child := range seg.children {

		//co := m.addVert(child.p, child.xAxis, uv)
		//sw := m.addVert(child.p.add(seg.xAxis.multiply(1)), seg.zAxis, uv)
		//m.addFace(po, co, sw)

		//NB: width and heights are determined by the non-normalised 'up' vector
		m.billboard(seg.p, child.p.sub(seg.p), campos, seg.radius, child.radius, 1, 4, child.segType.tcs)

		child.getBillBoardedMesh(m) //recurse
	}

}

func (seg *segment) getTubularMesh(m *simpleMesh) {

	for _, child := range seg.children {

		//m.addTube(seg.p, child.p, seg.xAxis, seg.radius, child.radius, child.segType.tcs, child.segType.sides)
		m.addTube(seg.p, child.p, child.xAxis, child.radius, child.radius, child.segType.tcs, child.segType.sides)

		child.getTubularMesh(m) //recurse
	}

}

func NewSegment(ps *segment, p *vec3, xAxis *vec3, segType *segmentType, radius float64) *segment {

	return &segment{parent: ps, p: p, xAxis: xAxis, segType: segType, radius: radius}

}

func NewSegmentType(name string, tcs *tcs, sides int, droopTo, droopStrength, twistsFlat float64, girthAtDays *curve, lengthAtDays *curve) *segmentType {
	// segmentTypes[name]=
	return &segmentType{name: name, tcs: tcs, sides: sides, droopTo: droopTo, droopStrength: droopStrength, twistsFlat: twistsFlat, girthAtDay: girthAtDays, lengthAtDay: lengthAtDays, buds: make([]*bud, 0)}
}
