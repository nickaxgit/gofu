package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand/v2"
	//"slices"
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
	msgGameId msgEnum = 1
	msgMesh   msgEnum = 4
	//msgMeshPositions    msgEnum = 5
	//msgMeshNormals      msgEnum = 6
	//msgMeshUVs          msgEnum = 7
	//msgMeshFaces        msgEnum = 8
	msgThings     msgEnum = 11
	msgMasses     msgEnum = 12
	msgHighlit    msgEnum = 13
	msgCamera     msgEnum = 14
	msgCursor     msgEnum = 15
	msgMessage    msgEnum = 16
	msgVectors    msgEnum = 17
	msgPlayers    msgEnum = 18
	msgCreateGame msgEnum = 19
	msgJoinGame   msgEnum = 20
	//	msgBoundValues      msgEnum = 21
	msgValueChange       msgEnum = 22
	msgGrid              msgEnum = 23 //send the current grid position and axes to the client
	msgSave              msgEnum = 24
	msgLoad              msgEnum = 25
	msgMode              msgEnum = 26 //send a message of the current (editor) mode - renders on client sceen
	msgLabels            msgEnum = 27 //collection of player/telemetry labels
	msgLabelSet          msgEnum = 28 //set of labels
	msgClear             msgEnum = 29 //clear all spheres/lines
	msgCentreOfMass      msgEnum = 30 //centre of mass
	msgControlToken      msgEnum = 31 //control token (4 digit Pin) for display on the client (as a QR code)
	msgJoinAsController  msgEnum = 32 //join as a controller (not a player)
	msgControlPositions  msgEnum = 33 //receive the control positions (tracked blobs)
	msgSound             msgEnum = 34 //sound message (play a sound)
	msgDetune            msgEnum = 35 //detune (pitch) sound (user for rpm/revs/throttle/ engine sound)
	msgMovement          msgEnum = 36 //moved masses
	msgBindValue         msgEnum = 37
	msgClearContextMenu  msgEnum = 38 //clear the context menu (on the client)
	msgTelemetry         msgEnum = 39 //send telemetry data (to the client)
	msgPositionInstances msgEnum = 40 //send mesh instance positions (to the client) (trees etc)
	//msgWater            msgEnum = 40 //send water levels for land vertices
)

type landMesh struct {
	name  string
	verts []*vert  //{}
	fi    []uint32 //{} //face indices

	midpoints map[uint64]uint32 //compound key of the two endpoints of an edge, map contains the index of its midpoint vertex
	splits    int               //maximum recursion depth
	height    float64           // +/- height(depth) of the land
	size      float64           //+/- land size
	kinks     []float64         //fractions of the land height to pull down by at each level
}

type seg struct {
	from         uint32
	to           uint32
	used         bool
	touchesEdges int
}

func newLandMesh(name string, maxFaces uint16, splits int, height float64, size float64, kinks []float64) *landMesh {

	//we need three entries per face in fis

	//l := int(maxFaces) * 3
	//fis := make([]uint32, l)
	//return &landMesh{name: name, verts: []*vert{}, fi: fis, midpoints: make(map[uint64]uint32, 0), splits: splits, height: height, size: size, kinks: kinks}
	return &landMesh{name: name, verts: []*vert{}, midpoints: make(map[uint64]uint32, 0), splits: splits, height: height, size: size, kinks: kinks}
}

func (m *landMesh) midpoint(v1 uint32, v2 uint32) uint32 {
	//return the index of the midpoint of the edge v1,v2
	key := uint64(v1) + uint64(math.MaxUint32)*uint64(v2)
	mid, present := m.midpoints[key]
	if present {
		return mid
	}

	//look the other way
	key = uint64(v2) + uint64(math.MaxUint32)*uint64(v1)
	mid, present = m.midpoints[key]
	if present {
		return mid
	}

	return math.MaxUint32
}

// func (m *landMesh) neighboursOf(tri *Tri) []*Tri {
// 	//return all the triangles sharing two of tris verts
// 	//(we know which triangles a vert touches already)
// 	neighbours := make([]*Tri, 0, 3)
// 	for i := 0; i < 3; i++ {
// 		i2 := (i + 1) % 3
// 		nextDoor := tri.neighbourSharing(m.verts[tri.vi[i]], m.verts[tri.vi[i2]])
// 		if nextDoor != nil {
// 			neighbours = append(neighbours, nextDoor)
// 		}
// 	}

// 	return neighbours

// }

// func (tri *Tri) neighbourSharing(v1 *vert, v2 *vert) *Tri {

// 	for t := range v1.touches {
// 		if t != tri {
// 			_, present := v2.touches[t]
// 			if present {
// 				return t
// 			}
// 		}
// 	}
// 	return nil

// }

// creates a new vertex halfway between the indexed verts a and b and returns the index of the new vertex
// depth is used to determin whether to calculate UVs from alitude, or halfway between the two parent verts UVs
// such that geoloiical strata are decided 'early' and later splits distort them - this produces more natural strata, and also beans finer geometry does not have texture coords popping (snow patches appearing out of nowhere for example)
func (m *landMesh) splitEdge(a, b uint32, dy float64, depth int) uint32 {

	pa := m.verts[a].p
	pb := m.verts[b].p

	p := pa.tween(pb, 0.5)

	p.y += dy

	//maintain a map of edges to midpoints
	// key := uint32(a) + uint32(65536)*uint32(b)
	// existingMid, present := m.midpoints[key]

	vi := m.midpoint(a, b) //will look (in a map) for an existing midpoints a->b or b->a
	if vi != math.MaxUint32 {
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

	//for the first few levels of splitting, calculate UVs from altitude (so large scale features are consistent)
	uvy := (p.y + m.height) / (2 * m.height)
	if depth > 6 {
		//later splits, preserve ealier UVs (so fine geometry detail does not cause texture popping)
		uvy = (m.verts[a].uv.y + m.verts[b].uv.y) / 2
	}

	//we don't have normals yet - so must calc UVX's later
	vi = m.addVert(p, false, 0, uvy) //OrReuseVertAtXZ(p)

	//}

	//add it to the index of midpoints
	key := uint64(a) + uint64(math.MaxUint32)*uint64(b)
	m.midpoints[key] = vi

	return vi
}

func (m *landMesh) addVert(p *vec3, reUseVert bool, u float64, v float64) uint32 {

	if reUseVert {
		for i, v := range m.verts {
			if v.p.equals(p) {
				return uint32(i)
			}
		}
	}

	m.verts = append(m.verts, newVert(p, u, v))

	return uint32(len(m.verts) - 1)
}

// func (m *landMesh) addFi(vi ...uint32) {
// 	m.fi = append(m.fi, vi...)
// }

// func (m *landMesh) rain(amount float64) {

// 	//pour an amount on every vertex proportional to altitude
// 	for _, v := range m.verts {

// 		distFromMid := (v.p.y - (m.height / 2)) / m.height
// 		if distFromMid < 0 {
// 			distFromMid = 0
// 		}
// 		//rainfall := (1 - distFromMid) * 1 //rainfall is proportional to altitude - the midground is wettest
// 		rainfall := float64(amount)
// 		v.wl = v.p.y + rainfall
// 	}

// }

// create a shore by dropping the land vertex to the level of
func (m *landMesh) flowWater(fis []uint16) {

	//flow the water (only along the deepest/visible faces)
	//for iw := 0; iw < 10; iw++ { //iteration of water
	for i := 0; i < len(fis); i += 3 {
		a := m.verts[fis[i]]
		b := m.verts[fis[i+1]]
		c := m.verts[fis[i+2]]
		flow(a, b) //update the accumulators at each vertex
		flow(b, c)
		flow(c, a)
	}
	//update the water levels (from the accumulators)
	for _, v := range m.verts { //note m.verts is a slice of pointers to verts - so we *can* mutate them

		v.wl += v.acc / float64(v.incount)
		v.acc = 0
		v.incount = 0
	}

}

func (m *landMesh) updateUVxsFromNormals() {
	for _, v := range m.verts {
		v.uv.x = math.Atan2(v.n.x, v.n.z) / float64(6.28)

	}

}

func (m *landMesh) getUVs() []float32 {

	vc := uint32(len(m.verts))
	uvs := make([]float32, vc*2)

	// yMax := -10000.0
	// yMin := 10000.0
	// for _, v := range m.verts {

	// 	if v.p.y > yMax {
	// 		yMax = v.p.y
	// 	} else if v.p.y < yMin {
	// 		yMin = v.p.y
	// 	}
	// }

	//big vertex index (in the huge mesh) to small vertex index (in the new sub mesh)
	for i, v := range m.verts {

		//	v.updateUV(yMin, yMax)
		uvs[i*2] = float32(v.uv.x)
		uvs[i*2+1] = float32(v.uv.y)
	}

	return uvs

}

func (m *landMesh) getPositions(asWater bool) []float32 {

	vc := uint32(len(m.verts))
	p := make([]float32, vc*3) //position x,y,z

	for i, j := range m.verts {

		p[i*3+0] = float32(j.p.x)
		if asWater {
			p[i*3+1] = float32(j.wl) //y + j.wl*10) //water level * 10
		} else {
			p[i*3+1] = float32(j.p.y)
		}
		p[i*3+2] = float32(j.p.z)
	}
	return p

}

func (t *tri) isUnderwater() bool {

	m := t.mesh
	a := m.verts[t.vi[0]]
	b := m.verts[t.vi[1]]
	c := m.verts[t.vi[2]]

	if a.wl >= a.p.y && b.wl >= b.p.y && c.wl >= c.p.y { //if all verts are below water
		return true
	}
	return false
}

// func (t *tri) getScorchedFacesInto(fis []uint16, p *uint32) {
// 	if len(t.children) == 0 {
// 		j := *p
// 		if t.scorched {
// 			fis[j] = uint16(t.vi[0])
// 			fis[j+1] = uint16(t.vi[1])
// 			fis[j+2] = uint16(t.vi[2])
// 			*p += 3
// 		}
// 	}
// 	for _, c := range t.children {
// 		c.getScorchedFacesInto(fis, p)
// 	}
// }

func (t *tri) getFacesInto(fis []uint16, p *uint32, fm *fireMesh, test func(*tri, *fireMesh) bool) {
	if len(t.children) == 0 {
		j := *p
		if test(t, fm) {
			fis[j] = uint16(t.vi[0])
			fis[j+1] = uint16(t.vi[1])
			fis[j+2] = uint16(t.vi[2])
			*p += 3
		}
	}
	for _, c := range t.children {
		c.getFacesInto(fis, p, fm, test)
	}
}

// func (t *tri) getWetFacesInto(fis []uint16, p *uint32) {

// 	if len(t.children) == 0 {
// 		j := *p

// 		if t.isUnderwater() {
// 			//if a.wl == b.wl && b.wl == c.wl {
// 			fis[j] = uint16(t.vi[0])
// 			fis[j+1] = uint16(t.vi[1])
// 			fis[j+2] = uint16(t.vi[2])
// 			*p += 3
// 		}
// 		//}
// 	}

// 	for _, c := range t.children {
// 		c.getWetFacesInto(fis, p)
// 	}

// }

// construct a list of vertex index triples (faces) for bottom level triangles
// func (t *tri) getDryFacesInto(fis []uint16, p *uint32) {

// 	if len(t.children) == 0 {

// 		v := t.mesh.verts
// 		a := v[t.vi[0]]
// 		b := v[t.vi[1]]
// 		c := v[t.vi[2]]

// 		//only include faces that have at least one vertex above water level
// 		if a.p.y >= a.wl || b.p.y >= b.wl || c.p.y >= c.wl {
// 			j := *p

// 			fis[j] = uint16(t.vi[0])
// 			fis[j+1] = uint16(t.vi[1])
// 			fis[j+2] = uint16(t.vi[2])

// 			*p += 3
// 		}
// 	}

// 	for _, c := range t.children {
// 		c.getDryFacesInto(fis, p)
// 	}

// }

func (m *landMesh) getDepths() []float32 {
	vc := uint32(len(m.verts))
	d := make([]float32, vc) //depths
	for i, j := range m.verts {
		d[i] = float32(j.wl * 10) //water level * 10
	}
	return d
}

// return the normals of the verts specified in vis (vertices we've added)
func (m *landMesh) getNormals(asWater bool) []float32 {

	vc := uint32(len(m.verts))
	//for every new vertex, reset the normal to zero, then add the normals of the faces it touches
	n := make([]float32, vc*3) //position x,y,z

	if asWater {
		for i, _ := range m.verts {
			n[i*3+0] = 0
			n[i*3+1] = 1
			n[i*3+2] = 0
		}
		return n

	}
	//big vertex index (in the huge mesh) to small vertex index (in the new sub mesh)
	for i, v := range m.verts {
		//for i, v := range m.verts {

		if len(v.touches) > 0 {
			v.n = &vec3{0, 0, 0}
			//usually 6 0- can be 5 - or even 3 at edges and 1 in conrners
			for t := range v.touches { //for every face this vertex touches
				if len(t.children) == 0 { //ony include bottom level traingles in the normal calculation
					v.n.addIn(t.normal)
				}
			}

			v.n.normalise()
			n[i*3+0] = float32(v.n.x)
			n[i*3+1] = float32(v.n.y)
			n[i*3+2] = float32(v.n.z)
		} else {
			logit(v, " touches no tris")
		}
	}

	return n

}

// func (m *landMesh) sendWater(state *state) {

// 	wl := make([]int, len(m.verts)) //water level

// 	for i, p := range m.verts {
// 		wl[i] = int(p.wl * 10)
// 	}

// 	state.send(nil, &reply{Cmd: "water", Payload: wl})

// }

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

func (p *player) clearContextMenu() {
	//clear the context menu (on the client)
	buff := new(bytes.Buffer)
	e := binary.LittleEndian
	binary.Write(buff, e, byte(msgClearContextMenu)) //clear the context menu
	p.sendBytes(buff.Bytes())
}

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

func (t *tri) scorchedAt(p *vec3) bool {
	if len(t.children) == 0 {
		return t.scorched
	} else {
		for _, c := range t.children {
			if c.contains(p, true, true) {
				return c.scorchedAt(p)
			}
		}
		return false
	}
}

func (t *tri) fetchTrees(depth int, positions *[]float32, bbm *simpleMesh, wp *int, landTri *tri, camPos *vec3, hidden *int) {

	if *wp >= len(*positions)-3 {
		return
	}

	mid := t.centre()
	up := newVec3(0, 1, 0)

	if t.depth == depth {
		if !t.scorchedAt(mid) {
			a := t.mesh.verts[t.vi[0]]
			b := t.mesh.verts[t.vi[1]]
			c := t.mesh.verts[t.vi[2]]
			if a.p.y <= a.wl || b.p.y <= b.wl || c.p.y <= c.wl {
				//one of the verts is underwater - no trees here
				return
			}
			//place a tree here

			if mid.distanceFrom(camPos) < 100 {
				treeTop := mid.clone()
				treeTop.y += 3

				pop := t.probePlane(treeTop, camPos) //fire a 10km ray
				if pop != nil {
					if t.contains(pop, true, true) {
						*hidden++
						return
					}
				}

				//pens := make([]*vec3, 100)
				//count := int(0)
				//landTri.probe(treeTop, camPos, pens, &count) //can we see the camera from the tree top

				(*positions)[*wp] = float32(mid.x)
				(*positions)[*wp+1] = float32(mid.y)
				(*positions)[*wp+2] = float32(mid.z)
				*wp += 3
			} else {
				bbm.billboard(mid, up, camPos, 20, 20, 20, 4) //billboard tree

			}
		}
	} else {
		for _, c := range t.children {
			c.fetchTrees(depth, positions, bbm, wp, landTri, camPos, hidden)
		}
	}

}

func (player *player) makeLand(position *vec3, focus *vec3, splits int, maxHeight float64, size float64, sendIt bool) *vec3 {

	kinks := []float64{1.0, 0.8, 1.0, 0.5, 0.25, 0.125, 1.0 / 16, 1.0 / 32, 1.0 / 64, 1.0 / 128, 1.0 / 256, 1.0 / 200, 1.0 / 300}
	//player.landMesh = newLandMesh("land", 65000, splits, maxHeight, size, kinks) //&state.land    //get a reference to state.land (saves a lot of typing)
	//land := player.landMesh
	land := newLandMesh("land", 65000, splits, maxHeight, size, kinks) //&state.land    //get a reference to state.land (saves a lot of typing)
	player.lastLandPos = position.clone()
	player.lastCamDir = focus.sub(position).normalise()

	land.verts = []*vert{} //clear the verts

	seed := uint64(0) //uint64(time.Now().Nanosecond())
	logit("seed:" + strconv.Itoa(int(seed)))

	rnGen = rand.New(rand.NewPCG(seed+1, seed))

	//(rnGen.Float64()-.5)*maxHeight
	land.addVert(newVec3(0, .1, size), false, 0, 0) //the height of the first vertex is the initial seed for the entire land
	land.addVert(newVec3(size, 0, -size), false, 0, 0)
	land.addVert(newVec3(-size, 0, -size), false, 0, 0)

	landTri := newTri(land, 0, 0, 1, 2)

	ts := time.Now()

	//note - runway is projected onto y=0 up to here

	rs := player.state.runwayStart
	re := player.state.runwayEnd
	rs.y = 0
	re.y = 0

	//just an even split - no proximity to focus
	//t.splitDownTo(splits) //approx 131k verts

	logit(len(landTri.mesh.verts), " verts")

	landTri.splitIfNeeded(position, focus) //split the triangle into 4 recursively
	landTri.patch()

	//patch convert and send

	s := player.state
	//if s.runwayStart != nil {
	//project the runway up onto the land
	s.runwayStart, _ = landTri.probeLand(s.runwayStart)
	//s.runwayEnd, _ = t.probeLand(s.runwayEnd)
	s.runwayEnd.y = s.runwayStart.y //keep the runway level with the start point

	landTri.plough(s.runwayStart, s.runwayEnd, s.runwayWidth) //recurse down through and plough a runway

	//probe the land at the four corners of the runway and add two triangles
	cross := s.runwayStart.sub(s.runwayEnd).normalise().cross(newVec3(0, 1, 0)).multiply(s.runwayWidth / 2)
	bl := s.runwayStart.sub(cross)
	br := s.runwayStart.add(cross)
	tl := s.runwayEnd.sub(cross)
	tr := s.runwayEnd.add(cross)
	//var n *vec3
	bl, _ = landTri.probeLand(bl) //find the ground surface
	tl, _ = landTri.probeLand(tl) //find the ground surface
	br, _ = landTri.probeLand(br) //find the ground surface
	tr, _ = landTri.probeLand(tr) //find the ground surface

	//if tl.y != br.y || n.y != 1 {
	//		logit("runway is not level")
	//	}

	bl.y += 0.05
	tl.y += 0.05
	br.y += 0.05
	tr.y += 0.05

	runwayMesh := newSimpleMesh(3, "runway", 4, 2) //2 faces
	up := newVec3(0, 1, 0)
	bli := runwayMesh.addVert(bl, up, &vec2{0, 1})
	bri := runwayMesh.addVert(br, up, &vec2{1, 1})
	tli := runwayMesh.addVert(tl, up, &vec2{0, 0})
	trix := runwayMesh.addVert(tr, up, &vec2{1, 0})

	runwayMesh.addFace(tli, bri, bli)
	runwayMesh.addFace(tli, trix, bri)

	logit("splitting took", time.Since(ts).Milliseconds())
	//}

	waterlines := []float64{maxHeight * 0.71, maxHeight * 0.41, 0.1, -maxHeight * 0.52}

	landTri.shoreLines(waterlines) //snaps the lowest vert of triangles spanning the waterline(s) to the waterline

	for i, wl := range waterlines {
		land.flood(wl) //set the waterlevel of all land below this waterline

		if i != len(waterlines)-1 {
			for lake := 0; lake < 10; lake++ {
				drained := false
				for _, v := range land.verts {
					if v.wl > v.p.y+300 {
						count := 0
						land.drain(v, wl, &count)

						logit("drained", count)
						drained = true

						break
					}
				}
				if !drained {
					logit("no more lakes to drain")
					break
				}
			}
		}

		funcIsUnderwater := func(t *tri, fm *fireMesh) bool { return t.isUnderwater() }
		waterMesh := landTri.toSimpleMesh(uint16(4+i), land, nil, "water", true, funcIsUnderwater)

		if sendIt {
			waterMesh.sendTo(player, 1)
		}
	}

	if sendIt {

		runwayMesh.sendTo(player, 1)

		isLand := func(t *tri, fm *fireMesh) bool {
			if t.isUnderwater() {
				return false
			}
			if fm.scorchedAt(t) {
				t.scorched = true
			}
			return !t.scorched
		}
		smallLandMesh := landTri.toSimpleMesh(2, land, player.state.fire, "land", false, isLand)

		funcIsScorchedLand := func(t *tri, fm *fireMesh) bool { return t.scorched } //fm.scorchedAt(t) }
		scorchedLand := landTri.toSimpleMesh(56, land, player.state.fire, "scorched", false, funcIsScorchedLand)

		smallLandMesh.sendTo(player, 1) //send them together - or transient gaps can appear
		scorchedLand.sendTo(player, 1)

		// wires := landTri.toSimpleMesh(32, land, "whiteWires", false,isLand)
		// wires.sendTo(player, 1)

		//need to do trees after waterlines so we don't get trees underwater
		treePositions := make([]float32, 100000) ///3000 xyz floats = 1000 trees
		wp := 0
		hidden := 0

		//TODO only reposition/resend trees in new positions (most trees do not need resending)
		treeBillboards := newSimpleMesh(105, "tree", 4000, 1000)
		landTri.fetchTrees(9, &treePositions, treeBillboards, &wp, landTri, position, &hidden)
		sendInstancePositions(player, 100, treePositions[:wp])
		treeBillboards.sendTo(player, 1)

		logit("sent", wp/3, "trees (hid", hidden, ")")

	}
	pos, _ := landTri.probeLand(focus)

	player.landTri = landTri
	return pos

}

func (t *tri) shoreLines(levels []float64) {
	//snap the lowest vert of triangles spanning the waterline(s) to the waterline
	if len(t.children) == 0 {
		a := t.mesh.verts[t.vi[0]]
		b := t.mesh.verts[t.vi[1]]
		c := t.mesh.verts[t.vi[2]]

		yh := a
		if b.p.y > yh.p.y {
			yh = b
		}
		if c.p.y > yh.p.y {
			yh = c
		}

		yl := a
		if b.p.y < yl.p.y {
			yl = b
		}
		if c.p.y < yl.p.y {
			yl = c
		}

		for _, wl := range levels {
			if yl.p.y < wl && yh.p.y > wl {
				yl.p.y = wl
				break
			}
		}

	} else {
		for _, c := range t.children {
			c.shoreLines(levels)
		}
	}

}

func (m *landMesh) flood(wl float64) {
	for _, v := range m.verts {
		if v.p.y <= wl {
			v.wl = wl
		} else {
			v.wl = -100000
		}
	}

}

func (m *landMesh) drain(v *vert, wl float64, count *int) {

	deep := -1000000.0
	for f := range v.touches {

		if len(f.children) == 0 {
			a := m.verts[f.vi[0]]
			b := m.verts[f.vi[1]]
			c := m.verts[f.vi[2]]

			if a != v && a.wl == wl {
				a.wl = deep
				*count++
				m.drain(a, wl, count)
			}
			if b != v && b.wl == wl {
				b.wl = deep
				*count++
				m.drain(b, wl, count)
			}
			if c != v && c.wl == wl {
				c.wl = deep
				*count++
				m.drain(c, wl, count)
			}
		}

	}
}

func (tri *tri) toSimpleMesh(id uint16, lm *landMesh, fm *fireMesh, material string, asWater bool, faceTest func(*tri, *fireMesh) bool) *simpleMesh {

	//vc := uint32(len(lm.verts)) //vertex count
	vc := uint32(len(lm.verts))  //vertex count
	fis := make([]uint16, vc*10) //there will actually be many less faces than verts - but we need 3 uints per face

	wp := uint32(0)

	tri.getFacesInto(fis, &wp, fm, faceTest) //populate Fis (recursivley from the root triangle)

	fis = fis[:wp] //truncate at the write pointer

	normals := lm.getNormals(asWater) //we must get normals (becuase it calculates them) before updating UVx's
	lm.updateUVxsFromNormals()
	return newFilledSimpleMesh(id, lm.getPositions(asWater), normals, lm.getUVs(), fis, material)

}

// func (lm *landMesh) toSimpleMesh(id uint16, fromTri *tri, material string, mapDown map[uint32]uint32) *simpleMesh {

// 	//vc := uint32(len(lm.verts)) //vertex count
// 	vc := uint32(len(mapDown))   //vertex count
// 	fis := make([]uint16, vc*10) //there will actually be many less faces than verts - but we need 3 uints per face

// 	wp := uint32(0)
// 	fromTri.getFacesInto(fis, &wp, mapDown) //populate Fis (recursivley from the root triangle)
// 	fis = fis[:wp]                          //truncate at the write pointer

// 	return newSimpleMesh(id, lm.getPositions(mapDown), lm.getNormals(mapDown), lm.getUVs(mapDown), fis, material)

// }

func (t *tri) plough(runwayStart *vec3, runwayEnd *vec3, runwayWidth float64) {

	m := t.mesh
	for _, vi := range t.vi {
		pp := m.verts[vi].p.clone()
		pp.y = runwayStart.y //move the vertex to the runway height
		if pp.distanceFromLineSegment(runwayStart, runwayEnd) < runwayWidth*2 {
			//cp := p.closestPointOnLineSegment(runwayStart, runwayEnd)
			m.verts[vi].p.y = runwayStart.y
		}
	}
	t.calcNormal() //SUPER important !

	for _, ct := range t.children {
		ct.plough(runwayStart, runwayEnd, runwayWidth)
	}

}

func (m *landMesh) slowProbe(p0, p1 *vec3) *vec3 {

	sd := 100000000.0
	var wp *vec3 = nil
	for i := 0; i < len(m.fi); i += 3 {
		t := m.triangleFrom(i)
		pop := t.probePlane(p0, p1)

		if pop != nil {
			d := pop.distanceFrom(p0)
			if d < sd {
				if t.contains(pop, true, true) {
					wp = pop
					sd = d
				}
			}
		}
	}
	return wp

}

func (t *tri) bltCount(count *int) {

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

	kinks := []float64{1.0, 0.8, 1.0, 0.5, 0.25, 0.125, 1.0 / 16, 1.0 / 32, 1.0 / 64, 1.0 / 128, 1.0 / 256, 1.0 / 200, 1.0 / 300}

	s := float64(100)
	t1 := newLandMesh("t1", 4, 4, 100, 100, kinks)
	t1.addVert(newVec3(0, 0, s), false, 0, 0)   //far
	t1.addVert(newVec3(s, 0, -s), false, 0, 0)  //right
	t1.addVert(newVec3(-s, 0, -s), false, 0, 0) //left

	// t1.fi = []uint32{0, 1, 2}

	tt := t1.triangleFrom(0)

	testFloat("Triangle, distance from point/plane (negative)", func() float64 { return newVec3(0, -50, 0).signedDistanceFromPlaneOf(tt) }, -50, "distanceFrom wrong")
	testFloat("Triangle, distance from point/plane (positive)", func() float64 { return newVec3(0, 50, 0).signedDistanceFromPlaneOf(tt) }, 50, "distanceFrom wrong")

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

	//kinks := []float64{1.0, 0.8, 1.0, 0.5, 0.25, 0.125, 1 / 16, 1 / 32, 1 / 64, 1 / 128, 1 / 256, 1 / 200, 1 / 300}

	// clay := newLandMesh("clay", 4, 100, 100, 100, kinks)
	// clay.addVert(newVec3(0, 100, 0), false, .5, 0)
	// clay.addVert(newVec3(100, 0, -100), false, 1, 1)
	// clay.addVert(newVec3(-100, 0, -100), false, 0, 1)
	// clay.fi = []uint32{0, 1, 2}
	// ct := clay.triangleFrom(0)

	// tool := newLandMesh("tool", 4, 100, 100, 100, kinks)
	// tool.addVert(newVec3(0, 200, 0), false, 0, 0)
	// tool.addVert(newVec3(100, -50, -100), false, 0, 0)
	// tool.addVert(newVec3(-100, -50, -100), false, 0, 0)
	// tool.fi = []uint32{0, 1, 2}
	// tt = tool.triangleFrom(0)

	// facePens := ct.penetrationsByEdgesOf(tt, false)

	//logit(len(facePens.pens))

}

// func tetra() *landMesh {

// 	s := float64(100)

// 	// t0 := NewMesh("t0")
// 	// t0.addVert(newVec3(0, 0, s))
// 	// t0.addVert(newVec3(s, 0, -s))
// 	// t0.addVert(newVec3(-s, 0, -s))

// 	// t0.fi = []int{0, 1, 2}

// 	//tree := growTree()

// 	// t1.fi = []int{0, 1, 2}

// 	// t1.cut(t0)
// 	//kinks := []float64{1.0, 0.8, 1.0, 0.5, 0.25, 0.125, 1.0 / 16, 1.0 / 32, 1.0 / 64, 1.0 / 128, 1.0 / 256, 1.0 / 200, 1.0 / 300}

// 	// tetra := newLandMesh("clay", 4, 100, 100, 100, kinks)
// 	// far := tetra.addVert(newVec3(0, 0, s), false, 0.5, 0)
// 	// right := tetra.addVert(newVec3(-s, 0, -s), false, 1, 1)
// 	// left := tetra.addVert(newVec3(s, 0, -s), false, 0, 1)
// 	// top := tetra.addVert(newVec3(0, s, 0), false, 0.5, 0)

// 	// tetra.fi = []uint32{
// 	// 	far, left, right, //bottom
// 	// 	top, right, left, //near/front face
// 	// 	top, far, right, //right face
// 	// 	top, left, far, //left face
// 	// }

// 	//inside := newVec3(0, 1, 0)
// 	//outside := newVec3(-100, 100, 0)

// 	// testBool(func() bool { return inside.isInside(tetra) }, true, "inside is Outside tetra")
// 	// testBool(func() bool { return outside.isInside(tetra) }, false, "outside is inside tetra")

// 	t2 := tetra.clone()
// 	t2.name = "tool"
// 	t2.verts[3].p.y = 200 //make a tall and pointy tool
// 	//t2.verts[3].p.Z = 0   //make a tall and pointy tool
// 	//t2.verts[3].p.X = 0   //make a tall and pointy tool

// 	t2.offset(newVec3(0, 0, 0)) //move it down 50 (and for towards the cam)
// 	//t2.rotateAbout(newVec3(0, 1, 0), math.Pi)

// 	//debugMesh := NewMesh("loop")
// 	//result := t2.cut(tetra, debugMesh)

// 	//result.generateNormals()

// 	//tree.generateNormals()
// 	//return tree //tetra //tetra //return the mutated tetra

// 	//debugMesh.generateNormals()

// 	//t2.generateNormals()

// 	return t2 //result

// }

func (m *landMesh) rotateAbout(axis *vec3, angle float64) {

	for _, v := range m.verts {
		v.p = v.p.rotateAbout(axis, angle)
	}

}

// func (m *landMesh) clone() *landMesh {
// 	//return a copy of this mesh
// 	//the new mesh will have new verts, but the same faces
// 	//the new mesh will have the same yMax as the old mesh
// 	newMesh := newLandMesh("clone of "+m.name, uint16(len(m.fi)/3), m.splits, m.height, m.size, m.kinks)
// 	for _, v := range m.verts {
// 		newMesh.addVert(v.p.clone(), false, v.uv.x, v.uv.y) //newVec3(v.p.X, v.p.Y, v.p.Z))
// 	}
// 	newMesh.fi = slices.Clone(m.fi) //important we clone (othewise we end up with a reference to the old fi slice)

// 	return newMesh
// }

// func (m *landMesh) generateNormals() {
// 	//generate normals

// 	j := 0
// 	for i := 0; i < len(m.fi); i += 3 {

// 		a := m.verts[m.fi[i]]
// 		b := m.verts[m.fi[i+1]]
// 		c := m.verts[m.fi[i+2]]

// 		ab := b.p.sub(a.p)
// 		ac := c.p.sub(a.p)

// 		n := (ab.cross(ac)).normalise()

// 		//m.faceNormals[j] = n
// 		j++

// 		if n.y < 0 {
// 			//panic("faces down")
// 			//	n = n.multiply(-1)
// 		}

// 		a.n = a.n.add(n) //add the face normal to each vertex
// 		b.n = b.n.add(n)
// 		c.n = c.n.add(n)
// 	}

// }

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

func (m *landMesh) inAline(a, b, c uint32) bool {

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

func (m *landMesh) triangleFrom(fi int) *tri {

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

	return newTri(m, 0, vi...)
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

// func (tool *landMesh) cut(clay *landMesh, debugMesh *landMesh) *landMesh {
// 	//the tool is unharmed (for now)
// 	//the clay has its faces subdivided (and vertices added) where the tool penetrates
// 	//its facelist is replaced

// 	if tool == clay {
// 		panic("tool and clay are the same mesh")
// 	}

// 	facelist := []uint32{}

// 	//for each face of the clay - DONT include the ones we add !!

// 	//DO NOT Mess with the original mesh - clone it
// 	output := clay.clone()

// 	for i := 0; i < len(clay.fi); i += 3 {

// 		ct := clay.triangleFrom(i)
// 		ct.check()

// 		ctn := ct.normal

// 		//for each triangle face of the tool mesh
// 		logit("before", len(output.verts))
// 		segs := ct.internalSegmentsFromPenetrationsOf(tool, output)
// 		logit("after", len(output.verts))
// 		//segs = segs.discardOpenSegments() //segments that touch an edge only once are discarded (they are flat faces (or chains therof) intruding)
// 		edgeSegs := ct.joinEdgePenetrations(segs, tool, output)
// 		if len(edgeSegs.segs) < 3 {
// 			panic("not enough edge segs")
// 		}
// 		segs.merge(edgeSegs) //add segments to close the outer ring(s) from the original triangle

// 		//we now have many directed segments - that should form closed loops, none of which should intersect
// 		//some of them may entirely contain others

// 		loopSet := segs.getLoops(output)
// 		if len(loopSet.loops) == 0 {
// 			panic("no loops")
// 		}

// 		compoundLoop := loopSet.merge()

// 		compoundLoop.debugMesh(output, ctn, debugMesh)

// 		facelist = append(facelist, compoundLoop.triangulate(debugMesh, output, ctn)...) //triangulates around the child holes held in the ring (at ct.depth+1)
// 	}

// 	output.fi = facelist
// 	output.generateNormals()

// 	debugMesh.generateNormals()
// 	return debugMesh

// }

// func (l *loop) clone() *loop {
// 	n := NewLoop()
// 	n.vi = slices.Clone(l.vi)
// 	return n
// }
