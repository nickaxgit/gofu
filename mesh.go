package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"time"
)

//	type edge struct {
//		v1  uint16
//		v2  uint16
//		mid uint16
//		t1  *Tri
//		t2  *Tri
//	}

type msgEnum byte

const (
	msgGameId        msgEnum = 1
	msgMesh          msgEnum = 4
	msgMeshPositions msgEnum = 5
	msgMeshNormals   msgEnum = 6
	msgMeshUVs       msgEnum = 7
	msgMeshFaces     msgEnum = 8
	msgThings        msgEnum = 11
	msgMasses        msgEnum = 12
	msgHighlit       msgEnum = 13
	msgCamera        msgEnum = 14
	msgCursor        msgEnum = 15
	msgMessage       msgEnum = 16
	msgVectors       msgEnum = 17
	msgPlayers       msgEnum = 18
	msgCreateGame    msgEnum = 19
	msgJoinGame      msgEnum = 20
	msgBoundValues   msgEnum = 21
	msgValueChange   msgEnum = 22
	msgGrid          msgEnum = 23 //send the current grid position and axes to the client
	msgSave          msgEnum = 24
	msgLoad          msgEnum = 25
	msgMode          msgEnum = 26 //send a message of the current (edior) mode - renders on client sceen
	msgLabels        msgEnum = 27 //collection of player/telemetry labels
	msgLabelSet      msgEnum = 28 //set of labels
	msgClear         msgEnum = 29 //clear all spheres/lines
	msgCentreOfMass  msgEnum = 30 //centre of mass

)

type mesh struct {
	name      string
	verts     []*vert  //{}
	fi        []uint16 //{} //face indices
	faceCount uint16
	yMax      float64
	yMin      float64
	midpoints map[uint32]uint16 //compound key of the two endpoints of an edge, map contains the index of its midpoint vertex
	//reuseVerts []uint16

	//faceNormals []*Vec3
}

type seg struct {
	from         uint16
	to           uint16
	used         bool
	touchesEdges int
}

func NewMesh(name string, maxFaces uint16) *mesh {

	//we need three entries per face in fis

	l := int(maxFaces) * 3
	fis := make([]uint16, l)
	return &mesh{name: name, verts: []*vert{}, fi: fis, midpoints: make(map[uint32]uint16, 0)}
}

func (m *mesh) midpoint(v1 uint16, v2 uint16) uint16 {
	//return the index of the midpoint of the edge v1,v2
	key := uint32(v1) + uint32(65536)*uint32(v2)
	mid, present := m.midpoints[key]
	if present {
		return mid
	}

	//look the other way
	key = uint32(v2) + uint32(65536)*uint32(v1)
	mid, present = m.midpoints[key]
	if present {
		return mid
	}

	return 65535
}

func (m *mesh) neighboursOf(tri *Tri) []*Tri {
	//return all the triangles sharing two of tris verts
	//(we know which triangles a vert touches already)
	neighbours := make([]*Tri, 0, 3)
	for i := 0; i < 3; i++ {
		i2 := (i + 1) % 3
		nextDoor := tri.neighbourSharing(m.verts[tri.vi[i]], m.verts[tri.vi[i2]])
		if nextDoor != nil {
			neighbours = append(neighbours, nextDoor)
		}
	}

	return neighbours

}

func (tri *Tri) neighbourSharing(v1 *vert, v2 *vert) *Tri {

	for t := range v1.touches {
		if t != tri {
			_, present := v2.touches[t]
			if present {
				return t
			}
		}
	}
	return nil

}

// creates a new vertex halfway between the indexed verts a and b and returns the index of the new vertex
func (m *mesh) splitEdge(a, b uint16, dy float64) uint16 {

	pa := m.verts[a].p
	pb := m.verts[b].p

	p := pa.tween(pb, 0.5)

	p.y += dy

	//maintain a map of edges to midpoints
	// key := uint32(a) + uint32(65536)*uint32(b)
	// existingMid, present := m.midpoints[key]

	vi := m.midpoint(a, b) //will look (in a map) for an existing midpoints a->b or b->a
	if vi != 65535 {
		if vi == 0 {
			logit("zero midpoint")
		}
		return vi
	}

	// if len(m.reuseVerts) > 0 {
	// 	vi = m.reuseVerts[0]
	// 	logit("reusing vert", vi)
	// 	m.verts[vi].p = p
	// 	m.reuseVerts = m.reuseVerts[1:]
	// } else {
	vi = m.addVert(p, false, 0, 0) //OrReuseVertAtXZ(p)
	//}

	//add it to the index of midpoints
	key := uint32(a) + uint32(65536)*uint32(b)
	m.midpoints[key] = vi

	return vi
}

func (m *mesh) addVert(p *vec3, reUseVert bool, u float64, v float64) uint16 {

	if reUseVert {
		for i, v := range m.verts {
			if v.p.equals(p) {
				return uint16(i)
			}
		}
	}

	m.verts = append(m.verts, newVert(p, u, v))
	if p.y > m.yMax {
		m.yMax = p.y
	} else if p.y < m.yMin {
		m.yMin = p.y
	}

	if len(m.verts) > 65500 {
		panic("Big mesh")
	}

	return uint16(len(m.verts) - 1)
}

func (m *mesh) addFi(vi ...uint16) {
	m.fi = append(m.fi, vi...)
}

func (m *mesh) rain(amount float64) {

	//pour an amount on every vertex proportional to altitude
	for _, v := range m.verts {

		distFromMid := (v.p.y - (m.yMax / 2)) / m.yMax
		if distFromMid < 0 {
			distFromMid = 0
		}
		//rainfall := (1 - distFromMid) * 1 //rainfall is proportional to altitude - the midground is wettest
		rainfall := float64(amount)
		v.wl += rainfall
	}

}

func (m *mesh) flowWater() {

	//flow the water (only along the deepest faces)
	//for iw := 0; iw < 10; iw++ { //iteration of water
	for i := 0; i < len(m.fi); i += 3 {
		a := m.verts[m.fi[i]]
		b := m.verts[m.fi[i+1]]
		c := m.verts[m.fi[i+2]]
		flow(a, b)
		flow(b, c)
		flow(c, a)
	}
	//update the water levels (from the accumulators)
	for v := range m.verts {
		m.verts[v].wl += m.verts[v].acc
		m.verts[v].acc = 0
	}
	//}

}

func (m *mesh) getUVs(vc uint32) []float32 {

	uvs := make([]float32, vc*2)
	for i, v := range m.verts {
		v.updateUV(m.yMin, m.yMax)
		uvs[i*2] = float32(v.uv.X)
		uvs[i*2+1] = float32(v.uv.Y)
	}

	return uvs

}

func (m *mesh) getPositions(vc uint32) []float32 {

	p := make([]float32, vc*3) //position x,y,z

	for i, v := range m.verts {
		p[i*3+0] = float32(v.p.x)
		p[i*3+1] = float32(v.p.y)
		p[i*3+2] = float32(v.p.z)
	}

	return p

}

// take a list of face indices - and return the index and 3 vertex indices for each face
func (t *Tri) getFacesInto(fis []uint16, p *uint32) {

	if len(t.children) == 0 {
		fis[*p] = t.vi[0]
		fis[*p+1] = t.vi[1]
		fis[*p+2] = t.vi[2]
		*p += 3
	}

	for _, c := range t.children {
		c.getFacesInto(fis, p)
	}

}

// return the normals of the verts specified in vis (vertices we've added)
func (m *mesh) getNormals(vc uint32) []float32 {

	//for every new vertex, reset the normal to zero, then add the normals of the faces it touches
	n := make([]float32, vc*3) //position x,y,z
	for i, v := range m.verts {

		if len(v.touches) > 0 {
			v.n = &vec3{0, 0, 0}
			//usually 6 0- can be 5 - or even 3 at edges and 1 in conrners
			for t := range v.touches { //for every face this vertex touches
				if len(t.children) == 0 { //ony include bottom level traingles in the normal calculation
					v.n.addIn(t.normal)
				}
			}

			v.n.normalise() //renormalise the normal
			n[i*3+0] = float32(v.n.x)
			n[i*3+1] = float32(v.n.y)
			n[i*3+2] = float32(v.n.z)
		} else {
			logit(v, " touches no tris")
		}
	}

	return n

}

func (m *mesh) sendWater(state *state) {

	wl := make([]int, len(m.verts)) //water level

	for i, p := range m.verts {
		wl[i] = int(p.wl * 10)
	}

	state.send(nil, &reply{Cmd: "water", Payload: wl})

}

// func encodeFloat64toFloat32Bytes(ar []float64) []byte {

// 	newar := make([]float32, len(ar))
// 	var v float64
// 	var i int // delcared outside the range for perofrmance (alegedly)
// 	for i, v = range ar {
// 		newar[i] = float32(v)
// 	}

// 	unsafeBytes := *(*[]byte)(unsafe.Pointer(&newar)) //spicy
// 	return unsafeBytes

// }

// func (m *mesh) sendToAll(name string, state *state) {

// 	vc := uint16(len(m.verts))

// 	if vc > 65535 {
// 		panic("too many verts")
// 	}

// 	vc2 := vc * 2
// 	vc3 := vc * 3

// 	v := make([]float32, vc3)  //position x,y,z
// 	n := make([]float32, vc3)  //normal x,y,z triples
// 	uv := make([]float32, vc2) //u,v pairs

// 	for i, p := range m.verts {
// 		v[i*3+0] = float32(p.p.x)
// 		v[i*3+1] = float32(p.p.y)
// 		v[i*3+2] = float32(p.p.z)
// 		//v[i*4+3] = float32(p.wl * 10) //water l
// 		//wl[i] = float32(p.wl )

// 		n[i*3+0] = float32(p.n.x)
// 		n[i*3+1] = float32(p.n.y)
// 		n[i*3+2] = float32(p.n.z)

// 		uv[i*2+0] = float32(p.uv.X)
// 		uv[i*2+1] = float32(p.uv.Y)

// 	}

// 	state.sendMakeMesh(name)
// 	state.sendData(5, vc, v, 0)                         //positions
// 	state.sendData(6, vc, n, 0)                         //normals
// 	state.sendData(7, vc, uv, 0)                        //uvs
// 	state.sendData(8, m.faceCount, m.getFaces(m.fi), 0) //faces (client will then create mesh)

// }

//see ToByteBuffer in Vector3.go
// func writeVec3Binary(b *bytes.Buffer, e binary.ByteOrder, v *Vec3) {
// 	binary.Write(b, e, float32(v.x))
// 	binary.Write(b, e, float32(v.y))
// 	binary.Write(b, e, float32(v.z))
// }

func (p *player) sendThings(things []*thing) {
	if things[0] == nil {
		logit("Things is nil", p.currentThing)
		return
	}
	p.sendBytes(thingsToBytes(things))
}

// func (p *player) sendMassDetail(m *mass) {
// 	buff := new(bytes.Buffer)
// 	e := binary.LittleEndian
// 	binary.Write(buff, e, byte(msgMassDetail)) //Mass
// 	binary.Write(buff, e, uint32(m.index))
// 	m.writeBinary(e, buff, true)

// 	p.sendBinary(buff.Bytes())
// }

func thingsToBytes(things []*thing) []byte {

	buff := new(bytes.Buffer)

	binary.Write(buff, le, byte(msgThings))
	binary.Write(buff, le, uint32(len(things))) //number of things (in this messsage)
	for _, t := range things {
		t.toByteBuffer(buff)
	}

	return buff.Bytes()

}

func (player *player) makeLand(y0pos *vec3, splits int, maxHeight float64, size float64, runwayStart *vec3, runwayEnd *vec3) *vec3 {

	player.landMesh = NewMesh("land", 65000) //&state.land    //get a reference to state.land (saves a lot of typing)
	land := player.landMesh
	player.lastLandPos = y0pos

	land.verts = []*vert{} //clear the verts

	seed := uint64(0) //uint64(time.Now().Nanosecond())
	logit("seed:" + strconv.Itoa(int(seed)))

	rnGen = rand.New(rand.NewPCG(seed+1, seed))

	//(rnGen.Float64()-.5)*maxHeight
	land.addVert(newVec3(0, 0, size), false, 0, 0)
	land.addVert(newVec3(size, 0, -size), false, 0, 0)
	land.addVert(newVec3(-size, 0, -size), false, 0, 0)

	t := newTri(land, []uint16{0, 1, 2}, 0)
	player.landTri = t

	ts := time.Now()

	t.split(y0pos, splits, maxHeight) //split the triangle into 4 recursively
	t.patch()

	if runwayStart != nil {
		//runwayStart, _ := t.probeLand(pos)
		//runwayEnd := runwayStart.add(newVec3(0, 0, -600))
		t.plough(runwayStart, runwayEnd) //recurse down through and plough a runway
	}

	logit("splitting took", time.Since(ts).Milliseconds())

	player.sendLand("land", true)

	pos, _ := t.probeLand(y0pos)

	return pos

}

func (t *Tri) plough(runwayStart *vec3, runwayEnd *vec3) {

	m := t.mesh
	for _, vi := range t.vi {
		p := m.verts[vi].p
		if p.distanceFromLineSegment(runwayStart, runwayEnd) < 40 {
			cp := p.closestPointOnLineSegment(runwayStart, runwayEnd)
			p.y = cp.y
		}
	}

	for _, ct := range t.children {
		ct.plough(runwayStart, runwayEnd)
	}

}

func (m *mesh) slowProbe(p0, p1 *vec3) *vec3 {

	sd := 100000000.0
	var p *vec3 = nil
	for i := 0; i < len(m.fi); i += 3 {
		t := m.triangleFrom(i)
		pop := t.probePlane(p0, p1)

		if pop != nil {
			d := pop.distanceFrom(p0)
			if d < sd {
				if t.contains(pop, true, true) {
					p = pop
					sd = d
				}
			}
		}
	}
	return p

}

func (t *Tri) bltCount(count *int) {

	if len(t.children) == 0 {
		*count++
	}

	for _, c := range t.children {
		c.bltCount(count)
	}

}

// func (m *mesh) makeFaces(t *Tri) {

// 	bltCount := 0
// 	t.bltCount(&bltCount) //count the bottom level triangles

// 	logit("bltCount", bltCount)
// 	logit(len(m.verts), " verts")

// 	m.fi = make([]uint16, bltCount*3) //face vert indices (three per triangle)

// 	p := 0
// 	t.gather(m.fi, &p) //get all the indices of triangles with no children

// 	m.generateNormals() //generates the face normals and avergaes them to all of their verts

// 	// //cull every trianlge below the sea
// 	// nfi := make([]int, len(fi))

// 	// o := 0
// 	// for i := 0; i < len(fi); i += 3 {

// 	// 	a := verts[fi[i]].p
// 	// 	b := verts[fi[i+1]].p
// 	// 	c := verts[fi[i+2]].p

// 	// 	if a.Y > 0 || b.Y > 0 || c.Y > 0 {
// 	// 		nfi[o] = fi[i]
// 	// 		nfi[o+1] = fi[i+1]
// 	// 		nfi[o+2] = fi[i+2]
// 	// 		o += 3
// 	// 	}
// 	// }

// 	// fi = nfi[:o] //keep the shortened face list

// 	//move every undewater vertex to the surface
// 	m.yMax = float64(-1000000)
// 	m.yMin = float64(100000)
// 	for i := 0; i < len(m.verts); i++ {
// 		// 	if verts[i].p.Y < 0 {
// 		// 		verts[i].p.Y = 0
// 		// 	}
// 		// 	// verts[i].p.X += (rnGen.Float64() - float64(.5)) * 100
// 		// 	// verts[i].p.Z += (rnGen.Float64() - float64(.5)) * 100
// 		y := m.verts[i].p.y
// 		if y > m.yMax {
// 			m.yMax = y
// 		}
// 		if y < m.yMin {
// 			m.yMin = y
// 		}

// 	}

// 	//generate UVs
// 	for _, v := range m.verts {
// 		//v := &lnd.verts[i] //DONT use range value here - we need to modify the actual vert (not a copy!)
// 		v.n = v.n.normalise()
// 		//use the angle of the normal projected onto x/y as the u component
// 		//TODO incororate slope of the terrain - if the terrain is flatter..
// 		//that the v component from lower down - this should put now on flat mountaintops
// 		//similarly north facing slopes should get their v component from higher in the map

// 		v.calcUV(m.yMin, m.yMax)
// 		//v.uv = Vector{math.Atan2(v.n.x, v.n.z) / float64(6.28), v.p.y / m.yMax}
// 	}

// 	//randomize Ys (AFTER) generating TC's
// 	// for _, v := range m.verts {
// 	// 	v.p.Y += (rnGen.Float64() - float64(.5)) * 100
// 	// 	//if v.p.Y < 0 {
// 	// 	//	v.p.Y = 0
// 	// 	//}
// 	// }

// 	logit(p, "faces indices", p/3, " faces")

// }

func testFloat(name string, f func() float64, expect float64, failMsg string) {

	v := f()
	if v != expect {
		logit("FAILED", name, failMsg, "expected:", expect, "got:", v)
	} else {
		logit("passed", name)
	}

}

func testBool(name string, f func() bool, expect bool, failMsg string) {

	v := f()
	if v != expect {
		logit("FAILED", name, failMsg, "expected:", expect, "got:", v)
	} else {
		logit("passed", name)
	}
}

func tests() {

	s := float64(100)
	t1 := NewMesh("t1", 4)
	t1.addVert(newVec3(0, 0, s), false, 0, 0)   //far
	t1.addVert(newVec3(s, 0, -s), false, 0, 0)  //right
	t1.addVert(newVec3(-s, 0, -s), false, 0, 0) //left

	t1.fi = []uint16{0, 1, 2}

	tt := t1.triangleFrom(0)

	testFloat("Triangle, distance from point/plane (negative)", func() float64 { return newVec3(0, -50, 0).distanceFromPlaneOf(tt) }, -50, "distanceFrom wrong")
	testFloat("Triangle, distance from point/plane (positive)", func() float64 { return newVec3(0, 50, 0).distanceFromPlaneOf(tt) }, 50, "distanceFrom wrong")

	testBool("Triangle contains, (exclude verts and edges) - point inside",
		func() bool { return tt.contains(newVec3(0, 0, 0), false, false) }, true, "contains wrong")
	testBool("Triangle contains, (exclude verts and edges) - point outside", func() bool { return tt.contains(newVec3(100, 0, 0), false, false) }, false, "contains wrong")
	testBool("Triangle contains (point is vert) - include verts", func() bool { return tt.contains(newVec3(0, 0, 100), true, true) }, true, "contains wrong")
	testBool("Triangle contains (point is vert) - dont include verts", func() bool { return tt.contains(newVec3(0, 0, 100), false, false) }, false, "contains wrong")

	testBool("Triangle contains - on vertex - true", func() bool { return tt.contains(newVec3(0, 0, 100), false, false) }, false, "contains (on vertex)wrong")
	testBool("Triangle contains - on edge - true ", func() bool { return tt.contains(newVec3(0, 0, 100), true, true) }, true, "contains (on vertex) wrong")

	testFloat("Normal and edge are orthogonal ", func() float64 { return tt.normal.dot(tt.edge0()) }, 0, "normal and edge are not orthogonal")

	a := &vec3{0, 2, 0}
	b := &vec3{1.1, 0, 0}
	axis := &vec3{0, 0, 1}
	testFloat("Positive angle", func() float64 { return b.SignedAngleFrom(a, axis) }, math.Pi/2, "angle wrong")
	testFloat("Negative angle", func() float64 { return a.SignedAngleFrom(b, axis) }, -math.Pi/2, "angle wrong")

	testFloat("Cross product orthogonal", func() float64 { return a.cross(b).dot(a) }, 0, "cross product not orthogonal")

	clay := NewMesh("clay", 4)
	clay.addVert(newVec3(0, 100, 0), false, .5, 0)
	clay.addVert(newVec3(100, 0, -100), false, 1, 1)
	clay.addVert(newVec3(-100, 0, -100), false, 0, 1)
	clay.fi = []uint16{0, 1, 2}
	ct := clay.triangleFrom(0)

	tool := NewMesh("tool", 4)
	tool.addVert(newVec3(0, 200, 0), false, 0, 0)
	tool.addVert(newVec3(100, -50, -100), false, 0, 0)
	tool.addVert(newVec3(-100, -50, -100), false, 0, 0)
	tool.fi = []uint16{0, 1, 2}
	tt = tool.triangleFrom(0)

	facePens := ct.penetrationsByEdgesOf(tt, false)

	logit(len(facePens.pens))

}

func tetra() *mesh {

	s := float64(100)

	// t0 := NewMesh("t0")
	// t0.addVert(newVec3(0, 0, s))
	// t0.addVert(newVec3(s, 0, -s))
	// t0.addVert(newVec3(-s, 0, -s))

	// t0.fi = []int{0, 1, 2}

	tests()

	//tree := growTree()

	// t1.fi = []int{0, 1, 2}

	// t1.cut(t0)

	tetra := NewMesh("clay", 4)
	far := tetra.addVert(newVec3(0, 0, s), false, 0.5, 0)
	right := tetra.addVert(newVec3(-s, 0, -s), false, 1, 1)
	left := tetra.addVert(newVec3(s, 0, -s), false, 0, 1)
	top := tetra.addVert(newVec3(0, s, 0), false, 0.5, 0)

	tetra.fi = []uint16{
		far, left, right, //bottom
		top, right, left, //near/front face
		top, far, right, //right face
		top, left, far, //left face
	}

	//inside := newVec3(0, 1, 0)
	//outside := newVec3(-100, 100, 0)

	// testBool(func() bool { return inside.isInside(tetra) }, true, "inside is Outside tetra")
	// testBool(func() bool { return outside.isInside(tetra) }, false, "outside is inside tetra")

	t2 := tetra.clone()
	t2.name = "tool"
	t2.verts[3].p.y = 200 //make a tall and pointy tool
	//t2.verts[3].p.Z = 0   //make a tall and pointy tool
	//t2.verts[3].p.X = 0   //make a tall and pointy tool

	t2.offset(newVec3(0, 0, 0)) //move it down 50 (and for towards the cam)
	//t2.rotateAbout(newVec3(0, 1, 0), math.Pi)

	//debugMesh := NewMesh("loop")
	//result := t2.cut(tetra, debugMesh)

	//result.generateNormals()

	//tree.generateNormals()
	//return tree //tetra //tetra //return the mutated tetra

	//debugMesh.generateNormals()

	t2.generateNormals()

	return t2 //result

}

func (m *mesh) rotateAbout(axis *vec3, angle float64) {

	for _, v := range m.verts {
		v.p = v.p.rotateAbout(axis, angle)
	}

}
func (m *mesh) clone() *mesh {
	//return a copy of this mesh
	//the new mesh will have new verts, but the same faces
	//the new mesh will have the same yMax as the old mesh
	newMesh := NewMesh("clone of "+m.name, uint16(len(m.fi)/3))
	for _, v := range m.verts {
		newMesh.addVert(v.p.clone(), false, v.uv.X, v.uv.Y) //newVec3(v.p.X, v.p.Y, v.p.Z))
	}
	newMesh.fi = slices.Clone(m.fi) //important we clone (othewise we end up with a reference to the old fi slice)
	newMesh.yMax = m.yMax
	return newMesh
}

func (m *mesh) offset(offset *vec3) {
	for _, v := range m.verts {
		v.p = v.p.add(offset)
	}
}

func (m *mesh) generateNormals() {
	//generate normals

	j := 0
	for i := 0; i < len(m.fi); i += 3 {

		a := m.verts[m.fi[i]]
		b := m.verts[m.fi[i+1]]
		c := m.verts[m.fi[i+2]]

		ab := b.p.sub(a.p)
		ac := c.p.sub(a.p)

		n := (ab.cross(ac)).normalise()

		//m.faceNormals[j] = n
		j++

		if n.y < 0 {
			//panic("faces down")
			//	n = n.multiply(-1)
		}

		a.n = a.n.add(n) //add the face normal to each vertex
		b.n = b.n.add(n)
		c.n = c.n.add(n)
	}

}

// func (m *mesh) addOrReuseVertAtXZ(p *Vec3) uint16 {

// 	for i, v := range m.verts {
// 		if v.p.x > p.x-0.01 && v.p.x < p.x+0.01 {
// 			if v.p.z > p.z-0.01 && v.p.z < p.z+0.01 {
// 				return uint16(i)
// 			}
// 		}
// 	}

// 	return m.addVert(p, false, 0, 0)
// }

// creates a triangle (which is NOT a face) from the indices of the verts
// holds a pointer to this mesh for access to its verts
// note - it does not add vertices or face indices to the mesh
// func (m *mesh) makeTri(depth int, fi uint16, vi ...uint16) *Tri {
// 	if m == nil {
// 		panic("mesh is nil")
// 	}

// 	if vi[0] == vi[1] || vi[0] == vi[2] || vi[1] == vi[2] {
// 		panic("degenerate triangle")
// 	}

// 	p0 := m.verts[vi[0]].p
// 	p1 := m.verts[vi[1]].p
// 	p2 := m.verts[vi[2]].p

// 	if p0.equals(p1) || p0.equals(p2) || p1.equals(p2) {
// 		panic("infinitely thin triangle")
// 	}

// 	if fi == 65535 {
// 		if len(m.reuseFaces) > 0 {
// 			fi = m.reuseFaces[0]
// 			logit("reusing face", fi)
// 			m.reuseFaces = m.reuseFaces[1:]
// 		} else {
// 			fi = m.faceCount
// 			m.faceCount++
// 		}
// 	}

// 	return newTri(m, fi, vi, depth) //&Tri{depth: depth, Vi: vi, Children: []*Tri{}, mesh: m}
// }

func (m *mesh) inAline(a, b, c uint16) bool {

	ap := m.verts[a].p
	bp := m.verts[b].p
	cp := m.verts[c].p

	//if any two points are the same - all three are in a line
	if ap.equals(bp) || ap.equals(cp) || bp.equals(cp) {
		return true
	}

	if ap.distanceFromLine(bp, cp) < 0.001 {
		return true
	}
	return false

}

func (m *mesh) triangleFrom(fi int) *Tri {

	if m == nil {
		panic("mesh is nil")
	}
	if fi%3 != 0 {
		panic("panic - not a face index")
	}

	vi := m.fi[fi : fi+3] //slice three vert indices from the face list
	p0 := m.verts[vi[0]].p
	p1 := m.verts[vi[1]].p
	p2 := m.verts[vi[2]].p

	if p0.equals(p1) || p0.equals(p2) || p1.equals(p2) {
		panic("infinitely thin triangle")
	}

	return newTri(m, vi, 0)
	//return &Tri{depth: 0, Vi: vi, Children: []*Tri{}, mesh: m}
}

// func (ps *penSet) setNeighbours() {

// 	for _, p:= range ps.pens{
// 		if p.isBoundary{
// 			//you can have a negbouring outer vertex, edge penetration or face penetration
// 			nfp:= ps.getPenetration(p.clayTri)
// 			nep:= p.getnextEdgePenetration() //find the next edge penetration on the same edge as this
// 			nv:= p.getNextVertOutervert() //find the next vert penetration on the same edge as this

// 		}
// 	}

// }

func (tool *mesh) cut(clay *mesh, debugMesh *mesh) *mesh {
	//the tool is unharmed (for now)
	//the clay has its faces subdivided (and vertices added) where the tool penetrates
	//its facelist is replaced

	if tool == clay {
		panic("tool and clay are the same mesh")
	}

	facelist := []uint16{}

	//for each face of the clay - DONT include the ones we add !!

	//DO NOT Mess with the original mesh - clone it
	output := clay.clone()

	for i := 0; i < len(clay.fi); i += 3 {

		ct := clay.triangleFrom(i)
		ct.check()

		ctn := ct.normal

		//for each triangle face of the tool mesh
		logit("before", len(output.verts))
		segs := ct.internalSegmentsFromPenetrationsOf(tool, output)
		logit("after", len(output.verts))
		//segs = segs.discardOpenSegments() //segments that touch an edge only once are discarded (they are flat faces (or chains therof) intruding)
		edgeSegs := ct.joinEdgePenetrations(segs, tool, output)
		if len(edgeSegs.segs) < 3 {
			panic("not enough edge segs")
		}
		segs.merge(edgeSegs) //add segments to close the outer ring(s) from the original triangle

		//we now have many directed segments - that should form closed loops, none of which should intersect
		//some of them may entirely contain others

		loopSet := segs.getLoops(output)
		if len(loopSet.loops) == 0 {
			panic("no loops")
		}

		compoundLoop := loopSet.merge()

		compoundLoop.debugMesh(output, ctn, debugMesh)

		facelist = append(facelist, compoundLoop.triangulate(debugMesh, output, ctn)...) //triangulates around the child holes held in the ring (at ct.depth+1)
	}

	output.fi = facelist
	output.generateNormals()

	debugMesh.generateNormals()
	return debugMesh

}

func (l *loop) clone() *loop {
	n := NewLoop()
	n.vi = slices.Clone(l.vi)
	return n
}
