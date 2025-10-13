package main

//lighteright game state - the objects do not have methods (as they are deserialised from server data)
import (
	"bufio"
	"github.com/gorilla/websocket"
	//	"go.mongodb.org/mongo-driver/bson" //once stuctures are stabilised - can probaly just use bufio direclty
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"strconv"
)

var rnGen *rand.Rand //nd.NewPCG(42, uint64(time.Microsecond)))

// type skinPayload struct {
// 	Ti       int     `json:"ti"`
// 	Scale    Vector  `json:"scale"`
// 	Rotation float64 `json:"rotation"`
// }

type soundPayload struct {
	Sound    string  `json:"sound"`
	Position vec3    `json:"position"`
	Volume   float32 `json:"volume"`
	Label    string  `json:"label"`
	Loop     bool    `json:"loop"`
}

type revsPayload struct {
	Player string  `json:"player"`
	Revs   float32 `json:"revs"`
}

// the client doesn't need to know about springs, faces (except for editing)
// type statePayload struct {
// 	GameId  int             `json:"gameId"`
// 	Players []playerPayload `json:"players"`
// 	Masses  []int           `json:"masses"` //mass info x,y,z,r,fixed,isCoin
// 	Things  []thingPayload  `json:"things"` //thing info
// }

// type playerPayload struct {
// 	Dozer       int32  `json:"dozer"`
// 	Name        string `json:"name"`
// 	Coins       int    `json:"coins"`
// 	Damage      byte   `json:"damage"`
// 	Temperature byte   `json:"temperature"`
// }

// type massPayload struct {
// 	//index: I, mass: state.masses[I]}}) //we receive a new thing sfrom someone -
// 	I      int     `json:"i"`
// 	P      vec3    `json:"p"`
// 	R      float64 `json:"r"`
// 	Fixed  bool
// 	isCoin bool
// }

// type springPayload struct {
// 	Ti     int    `json:"ti"`
// 	Si     int    `json:"si"`
// 	Spring spring `json:"spring"`
// }

// type Vec3Payload struct {
// 	X float64 `json:"x"`
// 	Y float64 `json:"y"`
// 	Z float64 `json:"z"`
// }

// type thingPayload struct {
// 	Ti         int32       `json:"ti"`
// 	MeshName   string      `json:"meshName"`
// 	Omi        int32       `json:"omi"`
// 	Fmi        int32       `json:"fmi"`
// 	Rmi        int32       `json:"rmi"`
// 	Scale      Vec3Payload `json:"scale"`
// 	Offset     Vec3Payload `json:"offset"`
// 	SpringEnds []int       `json:"springEnds"` //m1,m2 index pairs
// }

type meshPayload struct {
	Name  string `json:"name"`
	Verts []int  `json:"verts"`
	Faces []int  `json:"faces"`
	Norms []int  `json:"norms"`
	UV    []int  `json:"uv"`
}

//	type pos struct{
//		fl Vector
//		rl Vector
//		fr Vector
//		rr Vector
//	}
type track struct {
	Pointer int       `json:"pointer"`
	Points  []float64 `json:"points"` //x,y pairs for 4 verts per frame
}

type state struct { //the DATA of a game in progress - it can be entirely replaced at any point by rejoining a game
	filename string
	Sqn      int
	//host      string
	players    map[uint32]*player
	masses     []*mass
	labels     []*label
	things     []*thing
	deathList  []*player
	Tracks     map[string]*track
	Layers     map[string]*Layer
	liftCurves [][]float64 //alternating x,y values
	dragCurves [][]float64 //alternating x,y values
	//controlTokens map[uint32]*player //every login adds a random token to here to allow control by another player/device
	running     bool
	runwayStart *vec3
	runwayEnd   *vec3
	runwayWidth float64
	stretchDir  bool
	zeroG       bool
	sounds      uint16    //next sound handle
	fire        *fireMesh //messh/root triangle of the fire map
}

func (s *state) addLabel(l *label) *label {
	s.labels = append(s.labels, l)
	l.index = uint16(len(s.labels) - 1)
	return l
}

func (s *state) addMass(m *mass) *mass {
	s.masses = append(s.masses, m)
	m.index = int32(len(s.masses) - 1)
	return m
}

func (st *state) mergeThing(t *thing) *thing {

	st.addThing(t)

	for m := range t.uniqueMasses {
		st.addMass(m)
	}
	// for st:= range t.springs {
	// 	st.addSpring(s)
	// }

	return t

}

// func (p Vec3) distanceFrom(t *Tri, v []vert) float64 {
// 	return t.distanceFrom(v, p)
// }

// flow water between v1 and v2 acording to the absolute water level and y-coord of the land
func flow(a *vert, b *vert) {

	//if diff < 0 {
	//rate := float64(1) //free flow - water is above ground at both ends

	//surfaceWater := a.p.wl - m.p.y
	// if a.wl < a.p.y {
	// 	rate -= .3
	// } //ground percolation
	// if b.wl < b.p.y {
	// 	rate -= .3
	// }

	// if b.wl < b.p.Y {
	// 	rate *= .1
	// } //ground percolation

	//use an accumulator per vertex for the in/out flow

	//amount := diff  * rate

	if a.wl > a.p.y || b.wl > b.p.y {
		diff := a.wl - b.wl //uses the absolute water level
		if diff < 0 {
			diff = -diff
		}

		if a.wl >= b.wl {
			//a is high
			a.acc -= diff //* .8
			//b.acc += diff * .2
		} else {
			b.acc -= diff //* 0.8
			//a.acc += diff * 0.2
		}

		a.incount++
		b.incount++
	}

}

type simpleMesh struct {
	id           uint16
	pad          byte      //arays ,must be aligned
	p            []float32 //positions
	n            []float32 //normals
	uv           []float32 //TC's
	fi           []uint16  //faces (3 indeices per triangle)
	materialName string
	vwp          uint16
	fwp          uint16
}

func newSimpleMesh(id uint16, materialName string, numVerts int, numFaces int) *simpleMesh {
	// p []float32, n []float32, uv []float32, fi []uint16
	return &simpleMesh{id: id, pad: 0, p: make([]float32, numVerts*3), n: make([]float32, numVerts*3), uv: make([]float32, numVerts*2), fi: make([]uint16, numFaces*3), materialName: materialName}
}

func (sm *simpleMesh) billboard(p *vec3, up *vec3, camPos *vec3, widthBottom float64, widthTop float64, height float64, shape int) {

	//	up := newVec3(0, 1, 0)
	toCam := camPos.sub(p).normalise()
	right := toCam.cross(up).normalise()
	rightTop := right.multiply(widthTop * .5)
	rightBottom := right.multiply(widthBottom * .5)

	top := p.add(up.multiply(height))

	v0 := sm.addVert(p.sub(rightBottom), toCam, newVec2(0, 0))
	v1 := sm.addVert(p.add(rightBottom), toCam, newVec2(1, 0))

	//triangular billboard (pine trees/flames)
	if shape == 3 {
		v2 := sm.addVert(top, toCam, newVec2(0.5, 1))
		sm.addFace(v0, v1, v2)
		return
	}

	v2 := sm.addVert(top.add(rightTop), toCam, newVec2(1, 1))
	v3 := sm.addVert(top.sub(rightTop), toCam, newVec2(0, 1))

	sm.addFace(v0, v1, v2)
	sm.addFace(v2, v3, v0)

}

func (m *simpleMesh) offset(offset *vec3) *simpleMesh {
	for i := 0; i < len(m.p); i += 3 {
		m.p[i] += float32(offset.x)
		m.p[i+1] += float32(offset.y)
		m.p[i+2] += float32(offset.z)
	}
	return m
}

func newFilledSimpleMesh(id uint16, p []float32, n []float32, uv []float32, fi []uint16, materialName string) *simpleMesh {
	return &simpleMesh{id: id, pad: 0, p: p, n: n, uv: uv, fi: fi, materialName: materialName, vwp: uint16(len(p) / 3), fwp: uint16(len(fi) / 3)}
}

func (sm *simpleMesh) addVert(p *vec3, n *vec3, uv *vec2) uint16 {

	sm.p = append(sm.p, float32(p.x), float32(p.y), float32(p.z))
	sm.n = append(sm.n, float32(n.x), float32(n.y), float32(n.z))
	sm.uv = append(sm.uv, float32(uv.x), float32(uv.y))

	return uint16(len(sm.p)/3) - 1

}

func (sm *simpleMesh) addFace(v1, v2, v3 uint16) {

	sm.fi = append(sm.fi, v1, v2, v3)

}

// func (m *landMesh) sendWater(p *player) {

// 	buff := new(bytes.Buffer)

// 	binary.Write(buff, le, msgWater)
// 	binary.Write(buff, le, uint32(len(m.verts)))
// 	binary.Write(buff, le, m.getDepths())
// 	p.sendBytes(buff.Bytes())

// }

func sendInstancePositions(p *player, meshId uint16, positions []float32) {

	instanceCount := uint16((len(positions) - 1) / 3)
	buff := new(bytes.Buffer)

	binary.Write(buff, le, msgPositionInstances) //0
	binary.Write(buff, le, meshId)               //1
	binary.Write(buff, le, uint16(0))            //uint32 //3
	binary.Write(buff, le, instanceCount)        // to instance (not element) //5
	binary.Write(buff, le, byte(0))              //padding byte (to aligin 32bit vertex data) //7

	// for i := range positions {
	// 	positions[i] = rand.Float32() * 100
	// }

	binary.Write(buff, le, positions) //x,y,z float32 triples //8

	p.sendBytes(buff.Bytes())

}

func (sm *simpleMesh) sendTo(p *player, instances uint16) {

	buff := new(bytes.Buffer)

	binary.Write(buff, le, msgMesh)
	binary.Write(buff, le, sm.id)                //uint32
	binary.Write(buff, le, uint32(len(sm.p)/3))  //num verts
	binary.Write(buff, le, uint32(len(sm.fi)/3)) //num faces
	binary.Write(buff, le, sm.pad)               //padding byte (to aligin 32bit vertex data)

	binary.Write(buff, le, sm.p) //x,y,z float32 triples
	binary.Write(buff, le, sm.n)
	binary.Write(buff, le, sm.uv) //UV float32 pairs
	binary.Write(buff, le, sm.fi) // or uint16 face indices
	writeString(buff, sm.materialName)

	binary.Write(buff, le, instances) //num instances
	p.sendBytes(buff.Bytes())

}

// func (player *player) sendLand(name string, init bool) {

// 	//vi's are vert indices into the mesh - we need to send the actual verts (positions, normals, UVs)
// 	//calc normals and uvs on new face verts

// 	vc := uint32(len(player.landMesh.verts))
// 	fis := make([]uint16, vc*6) //the will actually many less faces than verts - but we need 3 uints per face

// 	p := uint32(0)
// 	player.landTri.getFacesInto(fis, &p)
// 	fis = fis[:p]
// 	fc := uint32(len(fis) / 3)

// 	if init {
// 		player.sendMakeMesh(name, uint32(vc), uint32(fc))
// 	}

// 	player.sendData(name, 5, 0, vc, player.landMesh.getPositions(vc)) //need to send normals and UVs too
// 	player.sendData(name, 6, 0, vc, player.landMesh.getNormals(vc))
// 	player.sendData(name, 7, 0, vc, player.landMesh.getUVs(vc))
// 	player.sendData(name, 8, 0, fc, fis)

// }

// func (s *state) mirrorMass(m *mass, g *grid) *mass {

// 	a := g.origin
// 	b := g.origin.add(g.Xaxis)
// 	c := g.origin.add(g.Yaxis)

// 	d := m.p.distanceFromTriPlane(a, b, c)
// 	if d < -0.01 || d > 0.01 { //not on the mirror - make a new mass
// 		mm := s.addMass(newMass(m.p.reflectInPlane(g.origin, g.normal()), m.r, m.fixed, m.collideable, m.isCoin, m.thing))

// 		return mm
// 	} else {
// 		return m //on the mirror - return the original
// 	}

// }

func (s *state) closestSpringToRay(start *vec3, end *vec3) (*thing, *spring) {

	closestDistance := float64(1000)
	var closestSpring *spring = nil
	var closestThing *thing = nil

	for _, t := range s.things {
		for _, s := range t.springs {

			d := distanceBetweenLines(start, end, s.m1.p, s.m2.p)
			if d <= closestDistance {
				closestDistance = d
				closestSpring = s
				closestThing = t
			}

		}
	}

	return closestThing, closestSpring
}

func (s *state) closestMassToRay(start *vec3, end *vec3, exclude *mass) *mass {

	closestDistance := float64(1000)
	var closestMass *mass = nil

	for _, m := range s.masses {
		if m != exclude {
			d := m.p.distanceFromLine(start, end)
			if d < m.r && d <= closestDistance { //<= selected the LAST mass at the point (mirrored masses, on the mirror)
				closestDistance = d
				closestMass = m
			}
		}
	}

	return closestMass
}

func (state *state) step(subSteps int) { //this is called every 33ms from stepWorlds()  (on a timer)

	if state.running {
		state.moveAll(subSteps) //<- this is a physics step - move, count coins and deaths, falls etc

		//ugly doing this twice TODO
		moved := int(0)
		for _, m := range state.masses {
			if !m.p.equals(m.op) {
				moved++
			}
		}

		if moved > 0 {
			buff := new(bytes.Buffer)
			binary.Write(buff, le, byte(msgMovement))
			binary.Write(buff, le, uint16(moved))
			for i, m := range state.masses {
				if !m.p.equals(m.op) {
					idx := uint16(i)
					binary.Write(buff, le, &idx)
					m.p.toByteBuffer(buff)
				}
			}
			state.sendBinary(buff.Bytes())
		}

		//updateLabels(state)
	}

	state.fire.burn(state.fire.root)

	for _, p := range state.players {
		if p.socket != nil {

			//sendInstancePositions(p, 201, flamePositions[:wp])

			if state.running && p.follow {
				p.camera.follow(p.vehicle)
			}
			p.sendCamera()
			p.sendLabels()

			if p.lastLandPos != nil {

				dist := p.camera.position.distanceFrom(p.lastLandPos)
				dir := p.camera.direction.dot(p.lastCamDir)
				if dist > 100 || dir < .95 {
					go p.getFlames()
					go p.makeLand(p.camera.position.clone(), p.camera.position.add(p.camera.direction), 12, 500, 10000.0, true) //makes and sends new land

				}
			}

			//remake land when we have moved
			if p.vehicle != nil {
				//o := p.vehicle.springs[0].m2.p
				// if p.landTri != nil {
				// 	penetrations := make([]*vec3, 200)
				// 	hits := 0
				// 	p.landTri.probe(p.camera.position, p.camera.position.add(p.camera.direction.multiply(100000)), penetrations, &hits) //find the land under the camera

				// 	if hits > 0 {
				// 		sd := 10000.0
				// 		focus := newVec3(0, 0, 0)
				// 		for i := 0; i < hits; i++ {
				// 			d := penetrations[i].distanceFrom(p.camera.position)
				// 			if d < sd {
				// 				focus = penetrations[i]
				// 			}
				// 		}

				// 		if focus.distanceFrom(p.lastLandPos) > 100 {
				// 			//go p.makeLand(y0pos, 10, 2000, 10000) //makes and sends new land
				// 			//lm := p.landMesh
				// 			//p.makeLand(p.camera.position, p.camera.position.add(p.camera.direction), lm.splits, lm.height, lm.size)
				// 		}
				// 	}
				// }
			}

		}
	}

	// //count money and send to player
	// for _, p := range state.players {

	// 	if p.stepCoinsValue > 0 { //did we win any coins this step
	// 		p.coins += p.stepCoinsValue * p.stepCoinCount
	// 		if p.stepCoinCount > 1 {
	// 			p.send(&reply{Cmd: "banner", Payload: "X" + strconv.Itoa(p.stepCoinCount)})
	// 		}
	// 		p.send(&reply{Cmd: "coins", Payload: p.coins})
	// 	}
	// 	p.stepCoinsValue = 0 //reset for next step
	// 	p.stepCoinCount = 0
	// }

}

// func loadGame(filename string) *state {

// 	file, err := os.Open(filename)
// 	if err != nil {
// 		logit(err.Error() + " load failed")
// 		return &state{}
// 	}
// 	defer file.Close()

// 	reader := bufio.NewReader(file)

// 	state := NewState()
// 	state.readBinaryMasses(reader)
// 	state.readBinaryThings(reader)
// 	state.readBinaryPlayers(reader)

// 	logit("Loaded state from " + filename)

// 	return state

// }

func (s *state) save(filename string, selectedMasses map[*mass]bool) {

	file, err := os.Create(filename + ".bin")
	if err != nil {
		logit(err.Error() + " save failed")
		return
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	writer.Write(massesToBytes(s.masses, true, selectedMasses)) //write all masses, with detail
	writer.Write(thingsToBytes(s.things))                       //write all things (springs, meshnames, offsets, scales, rotations)
	writer.Write(playersToBytes(s.players))                     //write all players (dozer index, name, coins, damage, temperature, cameras, selections)
	writer.Flush()

	//write the bytes slice to a file

}

func (s *state) massesFromByteBuffer(buff *bytes.Buffer) {

	le := binary.LittleEndian

	msgType := byte(0)
	binary.Read(buff, le, &msgType)
	if msgType != byte(msgMasses) {
		panic("Masses are not next")
	}

	nm := uint32(0)
	binary.Read(buff, binary.LittleEndian, &nm)
	s.masses = make([]*mass, nm)
	withDetail := byte(0)
	binary.Read(buff, le, &withDetail)

	for i := 0; i < int(nm); i++ {

		m := newMass(newVec3(0, 0, 0), 0, false, false, false, nil, nil)

		m.fromByteBuffer(buff, withDetail, s)
		if m.index != int32(i) {
			panic("mass index mismatch")
		}
		s.masses[m.index] = m
	}
}

func load(filename string) *state {

	file, err := os.Open(filename + ".bin")
	if err != nil {
		logit(err.Error() + " load failed")
		return &state{}
	}
	defer file.Close()

	reader := io.Reader(file)
	allBytes, err := io.ReadAll(reader)
	if err != nil {
		logit(err.Error())
	}

	//buff := bytes.NewBuffer(mb)
	buff := bytes.NewBuffer(allBytes)

	state := NewState(filename)
	state.massesFromByteBuffer(buff)
	state.referenceMasses() //uses mass.axi and mass.wri to restore the m.wingroot and m.axle mass references
	state.thingsFromByteBuffer(buff)
	state.playersFromByteBuffer(buff)

	//fix up wing areas on loading
	for _, m := range state.masses {
		if m.axle != nil && m.wingRoot != nil && m.wingArea == 0 {
			//panic("no wing area")
			m.wingArea = m.p.sub(m.axle.p).length() * m.axle.p.sub(m.wingRoot.p).length()
		}
	}

	logit("Loaded state from " + filename)

	return state

}

func (s *state) playersFromByteBuffer(buff *bytes.Buffer) {

	le := binary.LittleEndian

	msgType := byte(0)
	binary.Read(buff, le, &msgType)
	if msgType != byte(msgPlayers) {
		panic("Players are not next")
	}

	numPlayers := uint32(0)
	binary.Read(buff, binary.LittleEndian, &numPlayers)
	s.players = make(map[uint32]*player)

	for i := 0; i < int(numPlayers); i++ {
		p := playerFromByteBuffer(buff, le, s)
		s.players[p.id] = p
	}
}

func (s *state) thingsFromByteBuffer(buff *bytes.Buffer) {

	le := binary.LittleEndian

	msgType := byte(0)
	binary.Read(buff, le, &msgType)
	if msgType != byte(msgThings) {
		panic("Things are not next")
	}

	nt := uint32(0)
	binary.Read(buff, le, &nt)
	s.things = make([]*thing, 0)

	for i := 0; i < int(nt); i++ {
		idx := int32(-1)
		binary.Read(buff, le, &idx)
		t := s.addThing(newThing(""))
		if idx != t.index {
			panic("thing index mismatch")
		}
		t.fromByteBuffer(buff, le)
	}
}

func (s *state) addThing(t *thing) *thing {

	s.things = append(s.things, t)
	t.index = int32(len(s.things) - 1)
	t.state = s

	return t //len(s.Things) - 1 // return the index of the new thing
}

func (t *thing) AddSpring(m1 *mass, m2 *mass, collideable byte, actuatorTag ActuatorEnum) *spring {
	s := NewSpring(m1, m2, collideable, actuatorTag)
	t.springs = append(t.springs, s)

	s.index = int32(len(t.springs) - 1)

	t.uniqueMasses[m1] = true
	t.uniqueMasses[m2] = true

	return s
}

func (t *thing) addFace(m ...*mass) {
	f := newFace(m)
	t.faces = append(t.faces, f)
}

func (state *state) AddPlayer(playerId uint32, name string, position *vec3, ws *websocket.Conn) *player {

	p := NewPlayer(playerId, name, state, ws)

	state.players[playerId] = p //players don't have an index

	return p

}

func (state *state) moveCameras() {
	for _, p := range state.players {
		//p.mtx.Lock()
		//if !p.state.running {
		p.moveCamera()
		//}

		//p.mtx.Unlock()

	}

}

func (p *player) makeDozer(y0pos *vec3) {

	state := p.state

	pos := newVec3(0, 0, 0)
	if p.landTri != nil {
		pos, _ = p.landTri.probeLand(y0pos)
	}

	p.grid.origin = pos.clone()
	p.grid.send(p)

	pos.y += 5 //lift it 5metres

	dozer := state.addThing(newThing("plane")) // position,0,radius,"",0,false,"dozers",1)
	p.currentThing = dozer

	//note masses do not belong to things .. this allows two things to be joined by a spring

	massRadius := float64(0.15)

	rl := state.addMass(newMass(pos, massRadius, false, false, true, dozer, nil))
	w := float64(3)
	h := float64(5)
	rr := state.addMass(newMass(pos.add(newVec3(w, 0, 0)), massRadius, false, false, true, dozer, nil))
	fl := state.addMass(newMass(pos.add(newVec3(0, 0, -h)), massRadius, false, false, true, dozer, nil))
	fr := state.addMass(newMass(pos.add(newVec3(w, 0, -h)), massRadius, false, false, true, dozer, nil))

	rl.axle = rr
	rr.axle = rl
	fl.axle = fr
	fr.axle = fl

	collideable := byte(1)
	dozer.AddSpring(rr, rl, collideable, fcNONE) // bottom
	dozer.AddSpring(rl, fl, collideable, fcNONE) // left
	dozer.AddSpring(fl, fr, collideable, fcNONE) // top
	dozer.AddSpring(fr, rr, collideable, fcNONE) // right

	dozer.AddSpring(rr, fl, 0, fcNONE) // cross members (not collideable)
	dozer.AddSpring(rl, fr, 0, fcNONE)

	bh := newVec3(0, 1, 0)                                                                 //blade height
	bl := state.addMass(newMass(fl.p.add(bh), massRadius, false, false, true, dozer, nil)) //blade left top
	br := state.addMass(newMass(fr.p.add(bh), massRadius, false, false, true, dozer, nil)) //blade left top

	dozer.AddSpring(bl, rr, collideable, fcNONE) //blade diagonal suppport
	dozer.AddSpring(br, rl, collideable, fcNONE) //blade diagonal suppport
	dozer.AddSpring(fl, bl, collideable, fcNONE) //blade left side
	dozer.AddSpring(fr, br, collideable, fcNONE) //blade right side
	dozer.AddSpring(rr, br, collideable, fcNONE) //blade right side support
	dozer.AddSpring(rl, bl, collideable, fcNONE) //blade left side support

	dozer.AddSpring(bl, br, collideable, fcNONE) //blade top edge

	dozer.addFace(fl, fr, br, bl)

	//set the orientation masses
	dozer.om = rl
	dozer.rm = rr
	dozer.fm = fl

	p.vehicle = dozer

}

func (state *state) closestSpring(wp *vec3) (*spring, *thing) {

	var closestSpring *spring
	var closestThing *thing

	closestDistance := float64(1000)

	for _, thing := range state.things {
		for _, tspring := range thing.springs {

			if wp.liesBetween(tspring.m1.p, tspring.m2.p) {
				d := wp.distanceFromLine(tspring.m1.p, tspring.m2.p)
				if d < closestDistance {
					closestDistance = d
					closestSpring = tspring
					closestThing = thing
				}
			}
		}
	}
	return closestSpring, closestThing

}

func (s *state) massAt(p *vec3, tol float64) *mass {

	for _, m := range s.masses {
		if m.p.distanceFrom(p) <= tol {
			return m
		}
	}
	return nil
}

func (state *state) closestMass(wp *vec3) *mass {

	//let closestDistance=within
	for _, m := range state.masses {
		d := wp.distanceFrom(m.p)
		if d < m.r {
			return m
		}
	}
	return nil
}

func (state *state) checkHoles() {

	// //check for escapes
	// for _, m := range state.Masses {
	// 	if !m.IsCoin { //coins can never escape holes
	// 		if m.fallingInto > -1 {
	// 			if !state.Things[m.fallingInto].contains(&m.P, state.Masses) {
	// 				m.fallingInto = -1 //phew, escaped
	// 			}
	// 		}
	// 	}
	// }

	// for ti, t := range state.Things {

	// 	if t.IsHole {
	// 		for _, m := range state.Masses {
	// 			if m.ThingNum != ti { //masses cannot fall into things they belong to
	// 				if !m.Fixed && m.fallingInto == -1 { //you can only be falling into one hole at once - and fixed masses can't fall into anything
	// 					if t.contains(m.P, state.Masses) { //todo - optimise - non moving masses cant fall in holes
	// 						m.fallingInto = ti
	// 						if m.IsCoin {

	// 							state.qSound("coin-flip", m.P, 0.2, "", false)
	// 						} else {
	// 							state.qSound("clank", m.P, 0.2, "", false)
	// 						}
	// 					}
	// 				}
	// 			}
	// 		}
	// 	}
	// }
}

func (state *state) resolvePenetrations() bool {

	penetrated := false
	for _, m := range state.masses {
		if m.collideable && m.enabled {
			for _, t := range state.things {
				if t != m.thing { //don't collide masses against the things they belong to
					if state.pushApart(m, t) {
						penetrated = true
					}
				}
			}

			//check for penetration of the land

			//surface := state.landTri.probeLand(newVec3(m.P.X, m.P.Y-m.R, m.P.Z), m.P.add(up))
			np := state.nearestPlayer(m.p)
			if np != nil {
				if np.landTri != nil {
					impact, surface := np.landTri.probeLand(newVec3(m.p.x, 0, m.p.z))
					//am i beneath the land
					if impact != nil {

						pen := impact.y - (m.p.y - m.r)

						if pen > 0 { //m.p.y < surface.y+m.r {
							v := m.p.sub(m.op)
							vr := v.reflect(surface.normal)

							if pen > 0.1 {
								logit("deep penetration", v.length()*30, "m/s")
							}

							m.p.y = impact.y + m.r

							vr = vr.sub(surface.normal.multiply(vr.dot(surface.normal) * .8)) //kill 80% of the vertical velocity (20% bounce)

							if m.axle != nil {
								axle := m.axle.p.sub(m.p).normalise()
								vr = vr.sub(axle.multiply(vr.dot(axle) * .85)) //.95)) //kill (95% of the) sideways velocity of the wheel
								vr = vr.multiply(0.95)                         //some wheel friciton

								m.fixed = false
								if m.brake > .01 {
									maxBrakeForce := 0.03                 //metres per cycle
									brakeForce := m.brake * maxBrakeForce //brake force in metres per cycle
									vrl := vr.length()
									if vrl > brakeForce {
										vr.subIn(vr.normalise().multiply(brakeForce)) //some wheel friciton
									} else {
										//vr = newVec3(0, 0, 0) //vr.multiply(-0.001) //dead stop
										//m.fixed = true
										//we have enough brake force - the brakes are holding
										m.p.x = m.op.x
										m.p.z = m.op.z

										continue
									}
								}

							} else { //not a wheel
								//vr = vr.multiply(.8) //kill 80% of the velocity
							}

							m.op = m.p.sub(vr)

						}
					}
				}
			}

		}
	}
	return penetrated
}

func (state *state) nearestPlayer(p *vec3) *player {

	var nearestPlayer *player
	var nearestDistance float64 = 100000

	for _, player := range state.players {
		d := p.distanceFrom(player.vehicle.springs[0].m1.p)
		if d < nearestDistance {
			nearestDistance = d
			nearestPlayer = player
		}
	}
	return nearestPlayer
}

func (state *state) pushApart(m *mass, thing *thing) bool {

	penetrated := false

	for _, face := range thing.faces {

		pen := face.penetration(m)
		if pen > 0 && pen < 1 { //we're on the wrong side

			m.lastThingTouched = thing
			m.p.subIn(face.normal().multiply(pen)) //TODO  CONSIDER EDGES PROPERLY
			penetrated = true

		}

	}

	return penetrated

}

// Used client side as the objects are dehyrdrated
func (state *state) centreOf(thingNum int) *vec3 {

	thing := state.things[thingNum]
	r := newVec3(0, 0, 0) //Vector{0, 0}
	for _, spring := range thing.springs {
		r.addIn(spring.m1.p)
	}

	f := 1 / float64(len(thing.springs))
	return r.multiply(f)
}

func (state *state) countCoin(lastThingCoinTouched *thing, value int) {
	//for each player

	for _, p := range state.players {
		if p.vehicle == lastThingCoinTouched {
			p.stepCoinsValue += value //value of coins won this step
			p.stepCoinCount += 1      //number of coins won this step
		}
	}
}

func (state *state) tumbleCoins() {
	// for _, m := range state.Masses {
	// 	if m.fallingInto > -1 && m.enabled {
	// 		hole := state.centreOf(m.fallingInto) //state.things[m.fallingInto].centre(state.masses)
	// 		m.moveTowards(&hole, 1)               //also dragging dozer tracks into holes 2 was too strong

	// 		if m.IsCoin {
	// 			m.Z -= 0.1 //fall down

	// 			if m.Z < -5 {
	// 				state.qSound("coin-drop", m.P, 0.2, "", false) //hit the bottom
	// 				state.countCoin(m.lastThingTouched, int(m.R))  //this is a server side function
	// 				m.enabled = false
	// 			}
	// 		}

	// 	}
	// }
}

// use the fuel burn (and KW) to accelerate the prop disc/engineRPM (frm whence thrust is derived)
func (state *state) runEngines() {
	for _, p := range state.players {
		if p.vehicle != nil {
			for engineIndex, e := range p.vehicle.engines {
				if e.rpm > 0 { //is the engine started/running
					av := e.rpm / 60 * 2 * math.Pi //radians per second
					torque := e.kw * 1000 / av     //watts to torque (Nm)
					av += torque / (e.moi * 100)   //divide by the time slice (100 cycles per second)

					//generate thrust
					v := av * e.propRadius * 0.6 //generate thrust at 60% of the prop radius (0.6 is a guess)
					aoa := e.pitch               //todo - account for forward speed (and the reducing angle of attack)) - although i imagine aircraft systems handle this to keep pitch optimal
					cl := lerp(aoa, state.liftCurves[0])
					lift := v * v * cl * rho * .5 * e.propTotalBladeArea
					e.thrustNewtons = lift

					if e.thrustNewtons > 10000 { //more than 5000kg of thrust
						logit("excess thrust", e.thrustNewtons)

					}

					cd := lerp(aoa, state.dragCurves[0])
					drag := v * v * cd * e.propTotalBladeArea

					dragTorque := drag * e.propRadius * .6 //torque is the drag on the prop (Nm)
					av -= dragTorque / (e.moi * 100)       //divide by the time slice (100 cycles per second)
					e.rpm = av * 60 / (2 * math.Pi)        //radians/second to rpm

					if e.rpm > e.lastRpmSent+10 || e.rpm < e.lastRpmSent-10 {
						e.sendRpm(engineIndex + 1)

					}

					svn := e.spring.m1.p.sub(e.spring.m2.p).normalise()

					//acceleration = force / mass

					//mv1 := svn.multiply(e.thrustNewtons / e.spring.m1.mass * 150)
					mv := svn.multiply(e.thrustNewtons / (500000 * 150))
					e.spring.m1.p.addIn(mv)
					e.spring.m2.p.addIn(mv)
				}
			}
		}
	}
}

// pass vms as 0 to use actual mass velocities
func (state *state) flyMasses() {

	//	gravity := 9.81 * (1 / 30.0 * 1 / 30.0) //DONT half this

	for _, m := range state.masses {
		if m.wingRoot != nil {
			if m.axle == nil {
				logit("no wing axis")
				return
			}

			//wingAxis := (m.p.sub(m.axle.p)).normalise()
			wingAxis := (m.p.sub(m.wingRoot.p)).normalise() //TE
			if m.flip {
				wingAxis = wingAxis.multiply(-1)
			}

			rootChord := (m.axle.p.sub(m.wingRoot.p)).normalise()

			vms := m.p.sub(m.op).length() * 30.0 * 5.0 //cyles per second * steps per cycle

			if vms > 0.1 {
				direction := (m.p.sub(m.op)).normalise()
				//logit(vms, "m/s")
				v2 := vms * vms
				m.vms = vms

				if vms > 100 {
					logit("Overspeed", vms)
				}

				dd := direction.dot(wingAxis)
				if dd == 1.0 || dd == -1.0 {
					continue //parallel to the wing axis - no lift (for example the fin moving up)
				}

				//the lift direction is always orthogonal to the direction of travel (regardless of the AoA)
				liftDir := (direction.cross(wingAxis)).normalise() //.rotateAbout(rootAxis, m.dihedralDegrees)
				wingUp := (rootChord.cross(wingAxis)).normalise()  //orthogonal to the chord of the wing (le-te)

				aoa := -math.Asin(direction.dot(wingUp)) // + math.Pi/2 //+ m.aoaRads
				// if aoa < -math.Pi {
				// 	aoa += math.Pi * 2
				// } else if aoa > math.Pi {
				// 	aoa -= math.Pi * 2
				// }

				m.aoaDegrees = aoa / (math.Pi * 2) * 360

				if m.wingArea > 50 {
					m.aoaDegrees -= 2
				} //reduce incidence of the main wing

				if m.aoaDegrees < -20 || m.aoaDegrees > 20 {
					logit("aoa", m.aoaDegrees)
				}
				cl := lerp(m.aoaDegrees, state.liftCurves[int(m.section)])
				cd := lerp(m.aoaDegrees, state.dragCurves[int(m.section)])
				liftNewtons := v2 * cl * rho * .5 * m.wingArea //cycles per second * steps per cycle
				if liftNewtons > 100000 {
					logit("excess lift", liftNewtons)
				}
				lift := liftDir.multiply(liftNewtons)

				dragNewtons := v2 * cd * m.wingArea
				drag := direction.multiply(-dragNewtons)

				m.lift = lift //.multiply(0.001) //visualise at 1mm per newton
				m.drag = drag

				if state.running {
					// if m.thrust != 0 {
					// 	m.p.addIn(rootAxis.multiply(m.thrust * ntm))
					// }

					f := float64(2 * 3 * 15000) // half (acceleration to distance travelled) 1/3rd of the lift distribution  150 steps per second
					d := lift.divide(m.wingRoot.mass() * f)

					m.wingRoot.p.addIn(d) // ntm * .33))
					d = lift.divide(m.axle.mass() * f)
					m.axle.p.addIn(d)
					d = lift.divide(m.mass() * f)
					m.p.addIn(d)

					m.p.addIn(drag.divide(m.mass() * f)) //multiply(ntm * 1))
				}

			}

			//todo - spread across the three masess

		}
	}

}
func (state *state) stretchSprings() {

	for _, t := range state.things {
		for _, s := range t.springs {
			s.stretch()
		}
	}

	// for _, m := range state.masses {
	// 	if m.correction != nil {
	// 		m.p.addIn(m.correction) //.multiply(1 / float64(m.contribs)))
	// 		m.correction.x = 0
	// 		m.correction.y = 0
	// 		m.correction.z = 0
	// 		m.contribs = 0
	// 	}

	// }
}

func (state *state) anyPlayers() bool {
	return len(state.players) > 0
}

func (state *state) deleteMass(m *mass) {
	state.masses = append(state.masses[:m.index], state.masses[m.index+1:]...)
	//todo reindex all springs above

	for i, j := range state.masses {
		if j.index > m.index {
			j.index--
		}
		if j.index != int32(i) {
			panic("mass index mismatch")
		}

	}
}

// if this mass unattached (to a any spring)
func (state *state) massFree(m *mass) bool {
	for _, t := range state.things {
		for _, s := range t.springs {
			if s.m1 == m || s.m2 == m {
				return false
			}
		}
	}
	return true
}

func (t *thing) deleteSpring(s *spring) {

	//TODO sanity check/test

	if t.springs[s.index] != s {
		panic("spring does not belong to thing")
	}
	t.springs = append(t.springs[:s.index], t.springs[s.index+1:]...)
	for _, rs := range t.springs {
		if rs.index > s.index {
			rs.index--
		}
	}

}

// func (state * State) deleteProp(layer string, i int){
// 	layer := state.Layers[layer]
// 	layer.Props = append(layer.Props[:i], layer.Props[i+1:]...)
// }

func (state *state) resetDozer(player *player) {

	var pos *vec3 = state.RandomStartPos(10000)

	w := float64(100)
	h := float64(140)

	m := player.getMasses(state)

	//fl, rl, fr, rr
	m[0].p = pos
	m[1].p = pos.add(newVec3(0, 0, h))
	m[2].p = pos.add(newVec3(w, 0, 0))
	m[3].p = pos.add(newVec3(w, 0, h))

	//kill their velocity
	for _, m := range m {
		m.op.x = m.p.x
		m.op.y = m.p.y
	}

}

func (state *state) makeHoles(numHoles int, w float64, h float64) {
	for i := 0; i < numHoles; i++ {
		x := math.Floor(rand.Float64() * w)
		y := 400 + math.Floor(rand.Float64()*h)
		r := 100 + rand.Float64()*400
		state.MakeHole(x, y, r)
	}
}

func (state *state) MakeHole(x float64, y float64, r float64) {

}

// executes a physics step and returns the index and new position for all the masses that move
func (state *state) moveAll(substeps int) {

	//movedMasses := []int{} //return the index, x and y of all masses that move

	//distance an object falls in 1/30th of a second
	//0.5 * G * T^2
	//0.5 * 9.81 * 1/30^2 = 0.0054

	gravity := 1 * 9.81 * math.Pow(1/(30*float64(substeps)), 2)
	if state.zeroG {
		gravity = 0
	}

	if state.anyPlayers() && len(state.masses) > 3 {

		for substep := 0; substep < substeps; substep++ {

			//move by inertia and friction
			for _, m := range state.masses {

				v := m.p.sub(m.op)
				m.op = m.p.clone()
				m.p.addIn(v.multiply(.999)) //inertia and friction (and damping)
				m.p.y -= gravity

			}

			//socket will b closed - in gofu.go gameTraffic()

			for _, p := range state.players {
				if p.socket != nil {
					p.sendVectors() //new
					p.sendTelemetry()
				}

				//if p.vehicle != nil { //controller players don't have dozers
				//p.updateActuators() - now done on arrival of controlInputs
				//	p.Move(state) //state.movePlayer(p) //based on a players keyboard/touch inputs move their track masses
				//}
			}

			state.runEngines() //places thrust on some springs
			state.flyMasses()  //player is used for thrust values

			state.stretchSprings()
			state.stretchSprings()
			state.stretchSprings()

			state.resolvePenetrations()

			// if state.stretchDir {
			// 	state.stretchSprings()
			// }
			// state.stretchDir = !state.stretchDir

			//masses are pushed out of things (and things away from masses)
			state.resolveMassOverlaps()

		}
	}

	// for _, p := range state.players {
	// 	if p.vehicle != nil {
	// 		if p.moved(state) {
	// 			state.RecordTrack(p)
	// 		}
	// 	}
	// }

}

func (player *player) getMasses(state *state) []*mass {

	port := player.vehicle.springs[1]
	starboard := player.vehicle.springs[3]

	fl := port.m2
	rl := port.m1
	fr := starboard.m2
	rr := starboard.m2

	return []*mass{fl, rl, fr, rr}
}

func (state *state) RecordTrack(player *player) {

	if state.Tracks[player.name] == nil {
		state.Tracks[player.name] = &track{Pointer: 0, Points: make([]float64, 800)}
	}
	track := state.Tracks[player.name]

	track.record(player.getMasses(state))

}

// encode float 64's into the tracks points
// A track is a stream of float 64's 8 per frame per player, 2 (x/y)  values per vert, 4 verts
// this is to (massively) reduce the JSON overhead
func (track *track) record(masses []*mass) {
	if len(masses) > 4 {
		panic("more than 4 track masses!" + strconv.Itoa(len(masses)))
	}
	if len(track.Points) < 8 {
		panic("track points too small!")
	}
	for _, m := range masses {
		track.Points[track.Pointer] = m.p.x
		track.Points[track.Pointer+1] = m.p.y
		track.Pointer += 2
	}
	if track.Pointer >= len(track.Points) {
		track.Pointer = 0
	}
}

// func (player *player) moved(state *state) bool {

// 	for _, m := range player.getMasses(state) {
// 		if m.v.lengthSq() > 0.01 {
// 			return true
// 		}
// 	}

// 	return false
// }

func (state *state) resolveMassOverlaps() {

	for o, a := range state.masses {

		for i := o + 1; i < len(state.masses); i++ {
			b := state.masses[i]

			if a.fixed || b.fixed || !a.enabled || !b.enabled {
				continue
			} //no need to check fixed masses
			//optimise here - we dont need to do the full distance calculation
			d := a.p.distanceFrom(b.p) //Vector.distanceBetween(a.position,b.position)
			overlap := (a.r + b.r) - d

			if overlap > 0 {
				//let v = ap.subtract(bp).normalise().multiply(0.5)
				delta := b.p.sub(a.p)
				if delta.lengthSq() == 0 {
					logit("zero length delta")
				} else {
					delta = delta.normalise()
					delta = delta.multiply(overlap)

					afix := .5 //b.mass/(a.mass+b.mass)
					if b.fixed {
						afix = 1
					} //if b is fixed then a is pushed out of b
					if !a.fixed {
						a.p.subIn(delta.multiply(afix))
					}
					if !b.fixed {
						b.p.addIn(delta.multiply((1 - afix)))
					}

					//transfer the last touch from the coin moving fastest
					// if a.isCoin && b.isCoin {
					// 	if a.v.lengthSq() > b.v.lengthSq() {
					// 		b.lastThingTouched = a.lastThingTouched
					// 	} else {
					// 		a.lastThingTouched = b.lastThingTouched
					// 	}
					// } else if a.isCoin { //or off the object touched if one of the masses is not a coin
					// 	a.lastThingTouched = b.thing
					// } else if b.isCoin {
					// 	b.lastThingTouched = a.thing

					// }
				}
			}
		}
	}
}

func (state *state) setupTiledLayer(layerName string, pic string, tileSize float64, extension string, w float64, h float64) {

	layer := NewLayer(layerName, []string{pic}, extension)
	state.Layers[layerName] = layer
	x := -tileSize * 2
	y := -tileSize * 2
	down := w / tileSize
	for i := float64(0); i < down+4; i++ {
		for j := float64(0); j < h/tileSize+4; j++ {
			p := Prop{Position: vec2{x, y}, Angle: 0, Radius: tileSize / 2, Pic: pic}
			layer.Props = append(layer.Props, p)
			x += tileSize
		}
		x = -tileSize * 2
		y += tileSize
	}
}

func (s *state) referenceMasses() {
	for _, m := range s.masses {
		if m.axi > -1 {
			if s.masses[m.axi] == nil {
				panic("mass axis not found")
			}
			m.axle = s.masses[m.axi]
		}
		if m.wri > -1 {
			if s.masses[m.wri] == nil {
				panic("mass wri not found")
			}
			m.wingRoot = s.masses[m.wri]
		}

	}
}
func NewState(filename string) *state {
	gameId := filename //uint32(rand.Float32() * 1000000)

	//looseley basedon  https://aerospaceweb.org/question/airfoils/q0150b.shtml (for high alpha values)
	liftCurves := [][]float64{
		{ //cambered lift
			-90, 0,
			-45, -1.5,
			-5, 0,
			10, 1.5,
			15, 1.75,
			20, 1.5,
			50, 1.7,
			90, 0,
		},
		{
			-90, 0,
			-45, -1.5,
			0, 0,
			45, 1.5,
			90, 0,
		},
	}

	dragCurves := [][]float64{
		{ //cambered drag
			-90, 1,
			-45, 0.5,
			-20, 0.1,
			-0, 0.01,
			20, 0.1,
			45, 0.5,
			90, 1,
		},
		{ //symetrical drag
			-90, 1,
			-45, 0.5,
			-5, .1,
			0, .01,
			5, .1,
			45, 0.5,
			90, 1,
		},
	}

	return &state{filename: gameId, players: map[uint32]*player{}, labels: []*label{}, masses: []*mass{}, things: []*thing{}, deathList: []*player{}, Layers: map[string]*Layer{}, Tracks: map[string]*track{}, liftCurves: liftCurves, dragCurves: dragCurves}
}

func lerp(x float64, data []float64) float64 {

	if x <= data[0] {
		return data[1]
	}

	if x >= data[len(data)-2] {
		return data[len(data)-1]
	}

	for i := 0; i < len(data); i += 2 {
		if data[i] > x {
			t := (x - data[i-2]) / (data[i] - data[i-2])
			return data[i-1] + t*(data[i+1]-data[i-1])
		}
	}

	panic("lerp failed")

}

func (state *state) deTune(handle uint16, cents int16) {

	buff := new(bytes.Buffer)
	writeByte(buff, byte(msgDetune))
	binary.Write(buff, le, &handle)
	binary.Write(buff, le, &cents)

	state.sendBinary(buff.Bytes())

}

func (state *state) qSound(sound string, position *vec3, volume float32, loop bool, playAfter uint16) uint16 {

	state.sounds++
	handle := state.sounds

	buff := new(bytes.Buffer)
	writeByte(buff, byte(msgSound))
	writeString(buff, sound)
	writeVec3(buff, position)       //position
	writeFloat32(buff, volume)      //volume
	binary.Write(buff, le, &handle) //handle
	writeBool(buff, loop)
	binary.Write(buff, le, &playAfter) //handle

	state.sendBinary(buff.Bytes())

	return handle

}

// TODO - this should be obsolete - use sendBinary instead
func (state *state) send(player *player, msg *reply) {

	if player == nil { //send to all
		for _, p := range state.players { //for every outbound que (player)
			if p.socket != nil {
				p.send(msg)
			}
		}
	} else {
		if player.socket != nil {
			player.send(msg)
		}
	}

}

func (state *state) sendBinary(msg []byte) {

	for _, p := range state.players { //for every outbound que (player)
		p.sendBytes(msg)
	}

}

// func (state *State) q4one(p *Player, msg *reply) {

// 	if !strings.HasPrefix(p.Name, "bot") {
// 		p.qh.mutex.Lock()
// 		p.qh.q[state.Sqn] = append(p.qh.q[state.Sqn], msg)
// 		p.qh.mutex.Unlock()
// 	}

// }
