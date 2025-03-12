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
	players   map[uint32]*player
	masses    []*mass
	things    []*thing
	deathList []*player
	Tracks    map[string]*track
	Layers    map[string]*Layer
	liftCurve []float64 //alternating x,y values
	dragCurve []float64 //alternating x,y values
	running   bool
}

func (s *state) addMass(m *mass) *mass {
	s.masses = append(s.masses, m)
	m.index = int32(len(s.masses) - 1)
	return m
}

// func (p Vec3) distanceFrom(t *Tri, v []vert) float64 {
// 	return t.distanceFrom(v, p)
// }

// flow water between v1 and v2 acording to the absolute wayter level and y-coord of the land
func flow(a *vert, b *vert) {

	diff := a.wl - b.wl //uses the absolute water level

	rate := float64(1) //free flow - water is above ground at both ends

	if a.wl < a.p.y && b.wl < b.p.y {
		rate = 0
	} //ground percolation

	// if b.wl < b.p.Y {
	// 	rate *= .1
	// } //ground percolation

	//use an accumulator per vertex for the in/out flow

	a.acc -= diff / 10 * rate //todo - rate (velocity), depending on the difference in water level ground percolation
	b.acc += diff / 10 * rate

}

func (player *player) sendLand(name string, init bool) {

	//va's are vert indices into the mesh - we need to send the actual verts (positions, normals, UVs)
	//calc normals and uvs on new face verts

	vc := uint32(len(player.landMesh.verts))
	fis := make([]uint16, vc*6) //the will actually many less faces than verts - but we need 3 uints per face

	p := uint32(0)
	player.landTri.getFacesInto(fis, &p)
	fis = fis[:p]
	fc := uint32(len(fis) / 3)

	if init {
		player.sendMakeMesh(name, uint32(vc), uint32(fc))
	}

	player.sendData(name, 5, 0, vc, player.landMesh.getPositions(vc)) //need to send normals and UVs too
	player.sendData(name, 6, 0, vc, player.landMesh.getNormals(vc))
	player.sendData(name, 7, 0, vc, player.landMesh.getUVs(vc))

	player.sendData(name, 8, 0, fc, fis)

}

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
			if d < closestDistance {
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

func (state *state) step() {

	state.moveAll(5) //<- this is a physics step - move, count coins and deaths, falls etc

	mm := make([]int, 4*len(state.masses))

	moved := int(0)
	for i, m := range state.masses {
		if !m.p.equals(m.op) {
			mm[moved*4] = i
			mm[moved*4+1] = int(m.p.x * 100)
			mm[moved*4+2] = int(m.p.y * 100)
			mm[moved*4+3] = int(m.p.z * 100)
			moved++
		}
	}

	mm = mm[:moved*4] //truncate

	if moved > 0 {
		state.Sqn++
		state.send(nil, &reply{Cmd: "mps", Payload: mm}) //send all moved masses to everyone
	}

	// if player.waterMade {
	// 	state.landMesh.rain(0.3)
	// 	state.landMesh.flowWater()
	// 	state.landMesh.sendWater(state)
	// }

	for _, p := range state.players {
		p.camera.follow(p.vehicle)
		p.sendCamera()
	}

	//count money and send to player
	for _, p := range state.players {

		if p.stepCoinsValue > 0 { //did we win any coins this step
			p.coins += p.stepCoinsValue * p.stepCoinCount
			if p.stepCoinCount > 1 {
				p.send(&reply{Cmd: "banner", Payload: "X" + strconv.Itoa(p.stepCoinCount)})
			}
			p.send(&reply{Cmd: "coins", Payload: p.coins})
		}
		p.stepCoinsValue = 0 //reset for next step
		p.stepCoinCount = 0
	}

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

		m.fromByteBuffer(buff, le, withDetail, s)
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
		t := s.addThing("")
		if idx != t.index {
			panic("thing index mismatch")
		}
		t.fromByteBuffer(buff, le)
	}
}

func (s *state) addThing(meshName string) *thing {
	t := NewThing(meshName)
	s.things = append(s.things, t)
	t.index = int32(len(s.things) - 1)
	t.state = s

	return t //len(s.Things) - 1 // return the index of the new thing
}

func (t *thing) AddSpring(m1 *mass, m2 *mass, collideable byte) *spring {
	s := NewSpring(m1, m2, collideable)
	t.springs = append(t.springs, s)
	t.masses[m1] = true
	t.masses[m2] = true

	s.index = int32(len(t.springs) - 1)
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
		p.moveCamera()
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

	dozer := state.addThing("plane") // position,0,radius,"",0,false,"dozers",1)
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
	dozer.AddSpring(rr, rl, collideable) // bottom
	dozer.AddSpring(rl, fl, collideable) // left
	dozer.AddSpring(fl, fr, collideable) // top
	dozer.AddSpring(fr, rr, collideable) // right

	dozer.AddSpring(rr, fl, 0) // cross members (not collideable)
	dozer.AddSpring(rl, fr, 0)

	bh := newVec3(0, 1, 0)                                                                 //blade height
	bl := state.addMass(newMass(fl.p.add(bh), massRadius, false, false, true, dozer, nil)) //blade left top
	br := state.addMass(newMass(fr.p.add(bh), massRadius, false, false, true, dozer, nil)) //blade left top

	dozer.AddSpring(bl, rr, collideable) //blade diagonal suppport
	dozer.AddSpring(br, rl, collideable) //blade diagonal suppport
	dozer.AddSpring(fl, bl, collideable) //blade left side
	dozer.AddSpring(fr, br, collideable) //blade right side
	dozer.AddSpring(rr, br, collideable) //blade right side support
	dozer.AddSpring(rl, bl, collideable) //blade left side support

	dozer.AddSpring(bl, br, collideable) //blade top edge

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
					surface, normal := np.landTri.probeLand(newVec3(m.p.x, 0, m.p.z))
					//am i beneath the land
					if surface != nil {

						pen := surface.y - (m.p.y - m.r)

						if pen > 0 { //m.p.y < surface.y+m.r {
							v := m.p.sub(m.op)
							vr := v.reflect(normal)

							if pen > 0.1 {
								logit("deep penetration", v.length()*30, "m/s")
							}

							m.p.y = surface.y + m.r

							vr = vr.sub(normal.multiply(vr.dot(normal) * .8)) //kill 80% of the vertical velocity (20% bounce)

							if m.axle != nil {
								axle := m.axle.p.sub(m.p).normalise()
								vr = vr.sub(axle.multiply(vr.dot(axle))) //kill (only the) sideways velocity of the wheel
							} else { //not a wheel
								vr = vr.multiply(.8) //kill 80% of the velocity
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

func (state *state) flyMasses(p *player) {

	//	gravity := 9.81 * (1 / 30.0 * 1 / 30.0) //DONT half this

	ntm := 0.00000005 //newtons to metres of movement per substep

	for _, m := range state.masses {
		if m.wingRoot != nil {
			if m.axle == nil {
				panic("no wing axis")
			}
			if m.wingArea == 0 {
				panic("no wing area")
			}
			wingAxis := m.p.sub(m.axle.p).normalise()
			rootAxis := m.axle.p.sub(m.wingRoot.p).normalise()
			v := m.p.sub(m.op)
			vms := v.length() * 30.0 * 5.0

			if vms > 0.1 {
				logit(vms, "m/s")
				v2 := v.lengthSq()
				direction := v.normalise()

				//the lift direction is always orthogonal to the direction of travel (regardless of the Aoa)
				liftDir := wingAxis.cross(direction).normalise().rotateAbout(rootAxis, m.dihedralDegrees)
				wingUp := rootAxis.cross(wingAxis).normalise()
				aoaDegrees := (math.Asin(direction.dot(wingUp)) + m.aoaRads) / math.Pi * 180
				cl := lerp(aoaDegrees, state.liftCurve)
				cd := lerp(aoaDegrees, state.dragCurve)
				liftNewtons := v2 * cl * m.wingArea
				lift := liftDir.multiply(liftNewtons)

				dragNewtons := v2 * cd * m.wingArea
				drag := direction.multiply(-dragNewtons)

				m.lift = lift.multiply(ntm * 100.0) //visualise at 10x movement
				//todo - spread across the three masess
				if p.thrust > 0 {
					logit("thrust", p.thrust)
				}
				m.p.addIn(rootAxis.multiply(p.thrust * ntm))
				m.p.addIn(lift.multiply(ntm))
				m.p.addIn(drag.multiply(ntm))
			}

		}
	}

}
func (state *state) stretchSprings() {
	for _, t := range state.things {
		for _, s := range t.springs {
			s.stretch(state.masses)
		}
	}
}

func (state *state) anyPlayers() bool {
	return len(state.players) > 0
}

func (s *state) scatterCoins(w float64, h float64) {

	countValues := []int{100, 1, 20, 2, 10, 5, 5, 10}
	for i := 0; i < len(countValues); i += 2 {
		v := countValues[i+1]
		for j := 0; j < countValues[i]; j++ {
			p := vec3{x: rand.Float64() * w, y: -400, z: rand.Float64() * h}
			s.addMass(newMass(&p, float64(v), false, true, true, nil, nil)) //coins don't have a thingNum
		}
	}
}

func (state *state) checkDeaths() {

	for _, p := range state.players {

		if !p.dead {
			p.lives--

			if p.lives > 0 {
				//dozer.Scale.Y = 140 / float64(100)
				//dozer.Scale.X = 1
				//dozer.Rotation = 0
				p.dying = false
				state.resetDozer(p)
				state.send(nil, &reply{Cmd: "banner", Payload: p.name + " has " + strconv.Itoa(p.lives) + " lives left"})
				//	p.dozer.send(nil) //send the dozer to all
			} else {
				p.dead = true
				p.dying = false
				state.send(nil, &reply{Cmd: "banner", Payload: p.name + " is dead"})
				p.send(&reply{Cmd: "dead", Payload: ""})
				state.deathList = append(state.deathList, p) //TODO - respawn/ spectate etc
			}

			//r := reply{Cmd: "skin", Payload: skinPayload{Ti: p.dozer.index, Scale: dozer.Scale, Rotation: dozer.Rotation}}
			//state.send(nil, &r)

		}
	}
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
	//gravity := 0.25 * 9.81 * (1 / 30.0) //* (1 / 30.0))

	//gravity := (9.81 * (1 / 30.0 * 1 / 30.0)) / float64(substeps) //DONT half this
	gravity := 9.81 * math.Pow(1/(30*float64(substeps)), 2)

	if state.anyPlayers() && len(state.masses) > 3 {

		for substep := 0; substep < substeps; substep++ {

			for _, m := range state.masses {
				m.op = m.p.clone()
			}

			//move by inertia and friction
			for _, m := range state.masses {
				m.v.addIn(newVec3(0, -gravity, 0))

				m.p.addIn(m.v.multiply(.999)) //inertia and friction

			}

			state.flyMasses(state.players[0])

			for _, p := range state.players {
				if p.vehicle != nil { //controller players don't have dozers
					p.Move(state) //state.movePlayer(p) //based on a players keyboard/touch inputs move their track masses
				}
			}

			state.stretchSprings()
			state.stretchSprings()
			state.stretchSprings()

			state.resolvePenetrations()

			// do{
			// }while (this.resolvePenetrations()) //loop unit all mass-thing pepetrations are resolved

			//state.checkHoles()
			//state.checkDeaths()

			//masses are pushed out of things (and things away from masses)
			state.resolveMassOverlaps()

			//state.tumbleCoins() //may change angle

			// //calculate velocity based on moevent
			for _, m := range state.masses {
				m.v = m.p.sub(m.op)

				// if m.v.lengthSq() < 0.01 {
				// 	m.v = newVec3(0, 0, 0)
				// }
			}

		}
	}

	for _, p := range state.players {
		if p.vehicle != nil {
			if p.moved(state) {
				state.RecordTrack(p)
			}
		}
	}

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

func (player *player) moved(state *state) bool {

	for _, m := range player.getMasses(state) {
		if m.v.lengthSq() > 0.01 {
			return true
		}
	}

	return false
}

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
				if a.isCoin && b.isCoin {
					if a.v.lengthSq() > b.v.lengthSq() {
						b.lastThingTouched = a.lastThingTouched
					} else {
						a.lastThingTouched = b.lastThingTouched
					}
				} else if a.isCoin { //or off the object touched if one of the masses is not a coin
					a.lastThingTouched = b.thing
				} else if b.isCoin {
					b.lastThingTouched = a.thing

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
			p := Prop{Position: Vector{x, y}, Angle: 0, Radius: tileSize / 2, Pic: pic}
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
	liftCurve := []float64{
		-90, 0,
		-45, -1.5,
		-5, 0,
		10, 1.5,
		15, 1.75,
		20, 1.5,
		50, 1.7,
		90, 0,
	}

	dragCurve := []float64{
		-90, 1,
		-45, 0.5,
		-20, 0.1,
		-0, 0.01,
		20, 0.1,
		45, 0.5,
		90, 1,
	}

	return &state{filename: gameId, players: map[uint32]*player{}, masses: []*mass{}, things: []*thing{}, deathList: []*player{}, Layers: map[string]*Layer{}, Tracks: map[string]*track{}, liftCurve: liftCurve, dragCurve: dragCurve}
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

func (state *state) qSound(sound string, position *vec3, volume float32, label string, loop bool) {

	payload := soundPayload{Sound: sound, Position: *position, Volume: volume, Label: label, Loop: loop}
	state.send(nil, &reply{Cmd: "sound", Payload: payload})

}

func (state *state) send(player *player, msg *reply) {

	if player == nil { //send to all
		for _, p := range state.players { //for every outbound que (player)
			p.send(msg)
		}
	} else {
		player.send(msg)
	}

}

func (state *state) sendBinary(player *player, msg []byte) {

	if player == nil { //send to all
		for _, p := range state.players { //for every outbound que (player)
			p.sendBytes(msg)
		}
	} else {
		player.sendBytes(msg)
	}

}

// func (state *State) q4one(p *Player, msg *reply) {

// 	if !strings.HasPrefix(p.Name, "bot") {
// 		p.qh.mutex.Lock()
// 		p.qh.q[state.Sqn] = append(p.qh.q[state.Sqn], msg)
// 		p.qh.mutex.Unlock()
// 	}

// }
