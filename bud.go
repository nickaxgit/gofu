package main

//buds are a template for adding segments to the tree - they are the DNA .. not the wood
type bud struct {
	//position  vec3    //in segment space
	twist     float64 //radians
	direction *vec3
	grows     *segmentType //what this bud is likely to sprout as it ages
	sproutAge int          //how old the segment is before its buds generate new segments
}

type segmentType struct {
	name string
	buds []*bud
}

type segment struct { //of a plant/tree
	p     *vec3
	xAxis *vec3 //direction of (accumulated) twist
	yAxis *vec3 //direction of growth
	zAxis *vec3 //ortho to x and y

	segType  *segmentType
	age      int
	children []*segment
}

func squareSimpleMesh(id uint16, materialName string) *simpleMesh {

	sm := newSimpleMesh(id, materialName, 4, 2)
	sm.billboard(newVec3(0, 0, 0), newVec3(0, 1, 0), newVec3(0, 0, -1), 1, 1, 1, 4) //just to set up the arrays
	return sm

}
func growTree() *simpleMesh {

	// segmentTypes:=make(map[string]*segmentType, 0)

	stalk := NewSegmentType("stalk")

	stalk.addBud(&bud{twist: .3, direction: newVec3(1, 1, .5).normalise(), grows: stalk, sproutAge: 10})
	stalk.addBud(&bud{twist: .1, direction: newVec3(-1, 1, 1).normalise(), grows: stalk, sproutAge: 9})
	//stalk.addBud(&bud{twist: .1, direction: newVec3(0, 1, -1).normalise(), grows: stalk, sproutAge: 9})

	// trunk:=segment{segType:segmentType{name:"stalk"}}

	trunk := NewSegment(newVec3(0, 0, 0), newVec3(0, 1, 0), newVec3(1, 0, 0), stalk, 0) //segment{segType: stalk, age: 0, children: make([]*segment, 0)}

	sprouts := 0
	trunk.grow(100, &sprouts)
	logit("sprouts", sprouts)

	m := newSimpleMesh(100, "bark", 3000, 1000) //newLandMesh("tree", 20000, 10, 100, 100, kinks)
	trunk.getMesh(m)                            //uses the meshes internal vert and face write pointer (vwp,fwp)

	return m

}

func (st *segmentType) addBud(b *bud) {
	st.buds = append(st.buds, b)
}

func (seg *segment) grow(age int, sprouts *int) {

	seg.age = age

	for _, bud := range seg.segType.buds {
		if seg.age > bud.sproutAge {
			//logit("sprouted", seg.age)
			*sprouts++

			up := newVec3(0, 1, 0)

			sproutYAxis := seg.yAxis

			if !bud.direction.equals(up) {

				axis := bud.direction.cross(up)
				droop := bud.direction.SignedAngleFrom(up, axis) //find the elevation	 of the bud
				sproutYAxis = seg.yAxis.rotateAbout(seg.xAxis, droop)
			}

			rotation := float64(0)
			right := newVec3(1, 0, 0)
			if !bud.direction.equals(right) {
				rotation = bud.direction.SignedAngleFrom(right, up)
			}

			sproutYAxis = sproutYAxis.rotateAbout(seg.yAxis, rotation).normalise()
			sproutXAxis := seg.xAxis.rotateAbout(seg.yAxis, rotation+bud.twist).normalise()
			//sproutZAxis:= sproutYAxis.cross(sproutXAxis)

			p := seg.p.add(seg.yAxis.multiply(float64(seg.age) * .05))
			newSegment := NewSegment(p, sproutYAxis, sproutXAxis, bud.grows, age-seg.age)
			seg.children = append(seg.children, newSegment)
			newSegment.grow(age-bud.sproutAge, sprouts)
		}
	}
}

func (seg *segment) getMesh(m *simpleMesh) {

	//uv := newVec2(0, 0)
	//po := m.addVert(seg.p, seg.xAxis, uv) //parent origin
	//n:=newVec3(0,0,-1)

	campos := newVec3(0, 0, -10) //turn billboards to a virtual camera 10metres away
	for _, child := range seg.children {

		//co := m.addVert(child.p, child.xAxis, uv)
		//sw := m.addVert(child.p.add(seg.xAxis.multiply(1)), seg.zAxis, uv)
		//m.addFace(po, co, sw)

		m.billboard(seg.p, child.p.sub(seg.p), campos, float64(seg.age)/100, float64(child.age)/100, 1, 4) //NB: width and heigs are determined by the non-normalised 'up' vector

		child.getMesh(m) //recurse
	}

}

func NewSegment(p *vec3, yAxis *vec3, xAxis *vec3, segType *segmentType, age int) *segment {

	yal := yAxis.length()
	if yal < .999 || yal > 1.0001 {
		panic("yAxis is not unit length")
	}
	xal := xAxis.length()
	if xal < .999 || xal > 1.0001 {
		panic("xAxis is not unit length")
	}

	return &segment{p: p, xAxis: xAxis, yAxis: yAxis, zAxis: yAxis.cross(xAxis), segType: segType, age: age}
}

func NewSegmentType(name string) *segmentType {
	// segmentTypes[name]=
	return &segmentType{name: name}
}
