package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"maps"
	"math"
	"math/rand"
	"slices"
	"sync"
	//"unsafe"
	"github.com/gorilla/websocket"
)

type highlitType struct {
	mass   *mass
	spring *spring
	thing  *thing
}

type player struct {
	id             uint32 //used in the players map
	name           string
	state          *state //the game he is in
	vehicle        *thing //int    `json:"dozer"` //index of dozer in the things array
	maxDamage      byte
	maxTemperature byte
	//worldCursor    *Vec3 //tor
	damage      byte
	temperature byte

	leftDrive  float64
	rightDrive float64
	thrust     float64
	//touchControlled:boolean = false //set to true as soon as we get a touch event
	oRevs float32

	coins          int
	stepCoinsValue int
	stepCoinCount  int

	highlit highlitType

	//springStart  *mass
	currentThing *thing
	mode         ModeEnum

	killer int
	dying  bool
	dead   bool
	lives  int
	//qh          *qHolder //pointer to the queue of messages for this player
	//waitChannel chan bool
	mtx   *sync.Mutex // a mutex is required to 'lock' access to each users connection (for writing)
	inMtx *sync.Mutex //inbound mutex, ensure only one command is processed at a time
	// many calls (to wsEndpoint) can be running in paralell - and more than one of them may attempt to write to a single users socket at the same time (not allowed!)
	socket           *websocket.Conn // a pointer to the socket - no players shoundnt have a socket, sockets should have a player (more than one socket can feed a player)
	controllerSocket *websocket.Conn //each player can only have one controller - but more than one player can drive/fly the same vehicle(e.g. pilot/co-pilot)

	landTri  *tri
	landMesh *landMesh

	lastLandPos *vec3 //where were we when we last generated land
	lastCamDir  *vec3 //direction of the camera when we last generated land

	grid *grid

	camera *camera
	// camDirection   *Vec3
	// camUp          *Vec3
	gridPos      *vec3
	zOff         float64 //offset from the grid (along the grid normal)
	spacePos     *vec3
	springCursor *mass

	selectedMasses map[*mass]bool
	massStartPos   map[*mass]*vec3 //for dragging - we record the start positions
	moveStart      *vec3           //for dragging
	//higlitMass     *mass

	downCam     *camera
	downGridPos *vec3 //where were we when they pressed the mouse button down
	//downCamPos  *vec3

	cursor   *vec2 //current mouse pos in normalised screen coords (-1/+1)
	grab     *vec2 //where we moused down in normalised screen coords (-1/+1)
	meshGrab *vec3 //a 'source' point on the mesh - we will translate to some target point

	//camFarPos *Vec3 //mouse on the far plane
	// downCamFarPos *Vec3 //recorded on mousedown

	//mouseDown     bool
	buttons     byte
	keys        map[string]bool
	boundValues map[string]*float64 //boundValue //these form a popup dialog box (mass properties)

	controls map[controlInput]float64

	//each mixer (of the player) adds a contribution to to one mass (e.g. an aileron)
	mixers []*mix

	movedSinceMouseDown bool
	follow              bool
	heading             float64 //heading of the vehicle in degrees
	//engineSounds        []uint16 //sound ids for the engine sounds (multi-engined aircraft)
}

func (s *state) tidy() {
	//remove masses not attached to a spring
	for {
		allGood := true
		for i, m := range s.masses {
			if s.massFree(m) {
				s.deleteMass(m)
				logit("tidy - deleted mass", i, "of", len(s.masses))
				allGood = false
				break
			} else {
				logit("tidy - kept mass", i, "of", len(s.masses))
			}
		}
		if allGood {
			break
		}
	}
}

func (p *player) bindMixers(vehicle *thing) {

	for _, mx := range p.mixers {
		mx.spring = vehicle.findSpringActuator(ActuatorEnum(mx.actuator))
		if mx.engineNum > 0 {
			vehicle.engines[mx.engineNum-1].spring = mx.spring
		}
		if mx.spring == nil { //we didnt bind it to a spring - try a mass
			mx.mass = vehicle.findMassActuator(ActuatorEnum(mx.actuator))
		}
	}

}

func (t *thing) findMassActuator(act ActuatorEnum) *mass {
	for m, _ := range t.uniqueMasses {
		if m.actuatorTag > 0 {
			if ActuatorEnum(m.actuatorTag) == act {
				return m
			}
		}
	}

	logit(t.meshName, " has no mass actuator for", springActuators[act])
	return nil
}

func (t *thing) findSpringActuator(act ActuatorEnum) *spring {
	for _, s := range t.springs {
		if s.actuatorTag > 0 {
			if ActuatorEnum(s.actuatorTag) == act {
				return s
			}
		}
	}

	logit(t.meshName, " has no spring actuator for", springActuators[act])
	return nil
}

func (p *player) updateActuators() {

	for _, mix := range p.mixers {
		if mix.spring != nil {
			mix.spring.expansion = 0
			//mix.spring.thrust = 0
		} else {
			//logit("unbound control surface", id)
		}
	}

	for _, mix := range p.mixers {

		if mix.engineNum > 0 { //don't think this is needed the mixers could target a float64 by pointer

			e := p.vehicle.engines[mix.engineNum-1]
			e.kw = e.kwMax * mix.output(p) //this is KW

			//thrust is genrated and applied to the engines spring in runEngines()

			//svn := mix.spring.m1.p.sub(mix.spring.m2.p).normalise()
			//mix.spring.m1.p.addIn(svn.multiply(e.thrustNewtons * ntm))
			//mix.spring.m2.p.addIn(svn.multiply(e.thrustNewtons * ntm))
			//mix.spring.thrust = e.thrustNewtons * ntm //newtons

		} else {
			//o := mix.output(p)
			//logit(actLabels[int(mix.actuator)], o)
			if mix.mass != nil { //it's a mass actuator (a brake)
				mix.mass.brake = mix.output(p)
			}

			if mix.spring != nil {
				mix.spring.expansion += mix.output(p)
			}
		}

	}
}

func (p *player) processMouseMove() {

	if p.state.running == false {

		//a point on the far plane (where the mouse cursor is pointing)

		if p.buttons == 2 { //panning camera

			delta := (p.cursor.subtract(p.grab)).multiply(2)

			if delta.lengthSq() != 0 {

				logit("delta", delta.x, delta.y)

				camRight := p.downCam.direction.cross(p.downCam.up).normalise()

				p.camera.up = p.downCam.up.rotateAbout(camRight, delta.y).normalise()
				pitched := p.downCam.direction.rotateAbout(camRight, delta.y)
				yawed := pitched.rotateAbout(p.camera.up, -delta.x)
				//player.camUp = player.camUp.rotateAbout(player.downCamUp, delta.X).normalise()
				p.camera.direction = yawed
				p.camera.up = newVec3(0, 1, 0) //auto level the camera

				// worldUp := newVec3(0, 1, 0)
				// camDir := (player.camLookAt.sub(player.camPosition)).normalise()
				// player.camUp = camDir.cross(worldUp).normalise().cross(camDir).normalise()

				p.sendCamera()
			}
			return
		}

		if p.buttons == 0 {

			cm := p.state.closestMassToRay(p.camera.position, p.camera.farPos, p.springCursor)

			if cm != p.highlit.mass {
				p.highlit.mass = cm
				p.sendHighlit() //might be nil
			}
		}

		p.grid.updateGridPosAndSpacePos(p)

		if p.mode == editing {

			p.highlit.thing, p.highlit.spring = p.state.closestSpringToRay(p.camera.position, p.camera.farPos)
			p.sendHighlit()

		} else if p.mode == moving {
			p.moveSelected()
		} else if p.mode == stretching {
			p.springCursor.p = p.spacePos.clone() //moveSpringCursor()
			p.sendMasses([]*mass{p.springCursor}, false)
		}

		if p.buttons == 1 && p.mode == editing {
			//dragging/panning the camera
			if p.downGridPos != nil {
				delta := p.gridPos.sub(p.downGridPos).multiply(.9)
				p.camera.position = p.downCam.position.sub(delta)
				p.sendCamera()
			}
		}

		p.sendCursor()
	}
}

func (p *player) bindValue(key string, valuePointer *float64, min float64, max float64, step float64, labelSet byte) {

	p.boundValues[key] = valuePointer //store the address of the value

	buff := new(bytes.Buffer)
	binary.Write(buff, le, byte(msgBindValue)) //masses
	writeString(buff, key)
	binary.Write(buff, le, *(*float64)(valuePointer)) //cast to *float64 and dereference
	binary.Write(buff, le, min)
	binary.Write(buff, le, max)
	binary.Write(buff, le, step)
	binary.Write(buff, le, labelSet) //0 or an index to a if this is an enumeration, this is the number of values

	p.sendBytes(buff.Bytes())

}

func (p *player) sendLabelSets() { //For options on the sliders

	sendLabelSet(p, 1, massActuators)
	sendLabelSet(p, 2, springActuators)
	sendLabelSet(p, 3, sections)

}

func sendLabelSet[E ActuatorEnum | sectionEnum](p *player, idx byte, valueLabelPairs map[E]string) {

	buff := new(bytes.Buffer)

	writeByte(buff, byte(msgLabelSet))
	writeByte(buff, idx)                        //index
	writeByte(buff, byte(len(valueLabelPairs))) //count (of labels)
	for v, l := range valueLabelPairs {
		binary.Write(buff, le, &v)
		writeString(buff, l)
	}
	p.sendBytes(buff.Bytes())

}

func (p *player) sendCentreOfMass(t *thing) {
	buff := new(bytes.Buffer)
	writeByte(buff, byte(msgCentreOfMass))
	binary.Write(buff, le, int32(t.index))
	cg, m := t.centreOfMass()
	cg.toByteBuffer(buff)
	binary.Write(buff, le, float32(m))
	p.sendBytes(buff.Bytes())
}

func (p *player) sendControlPin() {
	token := randomPin()
	controlTokens[token] = p
	buff := new(bytes.Buffer)
	writeByte(buff, byte(msgControlToken))
	binary.Write(buff, le, token)
	p.sendBytes(buff.Bytes())

}

func randomPin() uint32 { //TODO - Check for existing token
	return uint32(math.Round(1000 + rand.Float64()*8999))
}

func (p *player) sendGameId() { //gameId uint32, e binary.ByteOrder) {
	buff := new(bytes.Buffer)
	writeByte(buff, byte(msgGameId))
	writeString(buff, p.state.filename)

	p.sendBytes(buff.Bytes())

	p.sendLabelSets()

}

// func (p *player) moveHighlit() {
// 	moveDelta := p.gridPos.sub(p.moveStart)

// 	//add any movement normal to the grid to the delta
// 	gridNormal := p.grid.Xaxis.cross(p.grid.Yaxis).normalise()
// 	camDGN := gridNormal.multiply(p.camera.position.sub(p.downCam.position).dot(gridNormal))
// 	moveDelta.addIn(camDGN)

// 	m := p.highlit.mass

// 	sp, ok := p.massStartPos[m]
// 	if ok {
// 		m.p = sp.add(moveDelta)
// 		p.sendMasses([]*mass{m}, false) //just send the new positions
// 	} else {
// 		logit("no startpos present for mass ", m.index)
// 	}
// }

// consolidates masses at the same point and rewires springs (masses on mirrors for example)
func (p *player) snapMasses() {
	for _, tm := range p.state.masses {
		if tm.transformOf != nil {
			if tm.transformOf.p.distanceFrom(tm.p) < 0.01 { // am i on the mirror plane
				//rewire the springs
				for t := range p.state.things {
					for _, s := range p.state.things[t].springs {
						if s.m1 == tm {
							s.m1 = tm.transformOf
						}
						if s.m2 == tm {
							s.m2 = tm.transformOf

						}
					}
				}

				//substitute the original for any reference to this (reflected) mass
				//only if they are at the same point
				for _, im := range p.state.masses {
					if im.wingRoot == tm {
						im.wingRoot = tm.transformOf
					}
					if im.axle == tm {
						im.axle = tm.transformOf
					}

				}

			}
		}
	}

	for t := range p.state.things {
		for _, s := range p.state.things[t].springs {
			s.restLength = s.m1.p.distanceFrom(s.m2.p)
		}
	}

	for _, m := range p.state.masses {
		for _, j := range p.state.masses {
			if m != j && m.p.distanceFrom(j.p) < 0.01 {
				logit("coincident masses", m.index, j.index)
			}
		}

	}

}

// sends position updates for masses we re moving (and when the camera moves up - where selected masses move with the camera)
func (p *player) moveSelected() {
	moveDelta := p.spacePos.sub(p.moveStart)

	//add any movement normal to the grid to the delta
	gridNormal := p.grid.Xaxis.cross(p.grid.Yaxis).normalise()
	camDGN := gridNormal.multiply(p.camera.position.sub(p.downCam.position).dot(gridNormal))
	moveDelta.addIn(camDGN)

	for m := range p.selectedMasses {
		p, present := p.massStartPos[m]
		if present {
			m.p = p.add(moveDelta)
		} else {
			logit("no startpos present for mass ", m.index)
		}

	}

	p.regenTransformed()

	s := slices.Collect(maps.Keys(p.selectedMasses))
	if len(s) > 0 {
		p.sendMasses(s, false) //just send the new positions
	}

}

func (p *player) regenTransformed() {
	//reflected := make(map[*mass]*mass)
	for _, m := range p.state.masses {
		if m.transformOf != nil {
			m.p = m.transformOf.p.reflectInPlane(p.grid.origin, p.grid.normal())
		}
	}

	p.sendMasses(p.state.masses, true)

}

// func (p *player) moveSpringCursor() {
// 	moveDelta := p.gridPos.sub(p.moveStart)

// 	//add any movement normal to the grid to the delta
// 	gridNormal := p.grid.Xaxis.cross(p.grid.Yaxis).normalise()
// 	camDGN := gridNormal.multiply(p.camera.position.sub(p.downCam.position).dot(gridNormal))
// 	moveDelta.addIn(camDGN)

// 	m := p.springCursor
// 	m.p = p.massStartPos[m].add(moveDelta)

// 	p.sendMasses([]*mass{m}, false) //just send the new positions

// }

func (p *player) makeNextSpring() {
	if p.highlit.mass == nil {
		p.highlit.mass = p.state.addMass(newMass(p.spacePos, .05, false, false, true, p.currentThing, nil))
	}

	m1 := p.highlit.mass
	m2 := p.state.addMass(newMass(m1.p.clone().add(newVec3(0, .001, 0)), 0.05, false, false, true, p.currentThing, nil))

	p.springCursor = m2

	p.highlit.spring = p.currentThing.AddSpring(m1, m2, 1, fcNONE)
	logit("made spring", m1.index, m2.index)

	//p.recordMassPositions() //we need to (re) do this as we have added masses

	// p.selectedMasses = make(map[*mass]bool)
	// p.selectedMasses[m2] = true
	p.sendMasses([]*mass{m1, m2}, false)
	p.sendThings([]*thing{p.currentThing})
	p.sendHighlit()

}

// func (p *player) sendBoundValues() {

// 	e := binary.LittleEndian
// 	buff := new(bytes.Buffer)
// 	binary.Write(buff, e, byte(msgBoundValues)) //masses
// 	binary.Write(buff, e, byte(len(p.boundValues)))
// 	for _, bv := range p.boundValues {
// 		bv.toByteBuffer(buff, e)
// 	}

// 	p.sendBytes(buff.Bytes())
// }

func (p *player) sendCamera() {

	//TODO should only send if position or direction has changed

	// up := newVec3(0, 1, 0)
	// p.camera.up = p.camera.direction.cross(up).cross(p.camera.direction).normalise()
	p.sendBytes(p.camera.toBytes())

}

// optimise - to send only changed labels - maybe send two mass indices (instead of a point)
func (p *player) sendLabels() {

	if len(p.state.labels) == 0 {
		return
	}

	buff := new(bytes.Buffer)

	binary.Write(buff, le, msgLabels)
	binary.Write(buff, le, uint16(len(p.state.labels)))
	for _, l := range p.state.labels {
		l.toByteBuffer(buff)
	}
	p.sendBytes(buff.Bytes())
}

func (player *player) sendMasses(masses []*mass, withDetail bool) {
	player.sendBytes(massesToBytes(masses, withDetail, player.selectedMasses))
	player.sendVectors()

}

func (player *player) sendClear() {
	player.sendBytes([]byte{byte(msgClear)})
}

func (p *player) velocity() float32 {
	tv := 0.0
	for um := range p.vehicle.uniqueMasses {
		tv += um.vms
	}
	return float32(tv / float64(len(p.vehicle.uniqueMasses)))
}

func (p *player) climbRate() float32 {

	dv := 0.0
	for um := range p.vehicle.uniqueMasses {
		dv += um.p.y - um.op.y
	}
	dv = dv / float64(len(p.vehicle.uniqueMasses))

	return float32(dv * 150) //m/s

}

// turnRate is the rate of change of heading in degrees per minute
func (p *player) turnRate() float32 {

	v := p.vehicle
	direction := v.fm.p.sub(v.om.p)
	heading := math.Atan2(direction.x, direction.z) / (math.Pi * 2) * 360 //angle in degrees

	defer func() { p.heading = heading }()
	return float32((p.heading - heading) * 150 * 60.0) //degrees per minute

}

func (p *player) sendTelemetry() {
	buff := new(bytes.Buffer)
	binary.Write(buff, le, byte(msgTelemetry))

	channels := byte(4)
	binary.Write(buff, le, channels)
	writeString(buff, "vel")
	binary.Write(buff, le, p.velocity())
	writeString(buff, "climb")
	binary.Write(buff, le, p.climbRate())
	writeString(buff, "turn")
	binary.Write(buff, le, p.turnRate())
	writeString(buff, "head")
	binary.Write(buff, le, p.heading)

	p.sendBytes(buff.Bytes())
}

func (p *player) sendVectors() {

	//send the mass index, vector and color - show lift at the wingtips (althoug it is actually shared between the three verts)

	buff := new(bytes.Buffer)

	binary.Write(buff, le, byte(msgVectors))

	// white := uint32(0xffffff00)
	// yellow := uint32(0xffff0000) //lift vector
	// orange := uint32(0xff800000) //axle/leading edge
	// blue := uint32(0x8080ff00)   //wing root/trailing eddge

	//black := uint8(0)
	orange := uint8(6)
	blue := uint8(1)
	//yellow := uint8(14)
	red := uint8(4)

	// new THREE.Color("black"), //0
	// new THREE.Color("blue"), //1
	// new THREE.Color("green"), //2
	// new THREE.Color("cyan"), //3
	// new THREE.Color("red"),  //4
	magenta := uint8(5)
	// new THREE.Color("orange"),  //6
	// new THREE.Color("gray"), //7
	// new THREE.Color("darkgray"), // 8
	// new THREE.Color("lightblue"), // 9
	// new THREE.Color("lightgreen"), // 10
	// new THREE.Color("lightcyan"), // 11
	// new THREE.Color("lightcoral"), // 12
	// new THREE.Color("lightpink"), // 13
	// new THREE.Color("yellow"), // 14
	// new THREE.Color("white") // 15

	//NEED PAUSED

	numVecs := 0
	for _, m := range p.state.masses {
		numVecs++
		if m.axle != nil {
			numVecs++
		}
		if m.lift != nil {
			numVecs++
			if m.axle != nil {
				numVecs++
			}
		}
	}

	binary.Write(buff, le, uint16(numVecs)) //number of vectors

	for _, m := range p.state.masses {

		//velocity vector
		m.p.toByteBuffer(buff)
		was := m.p.sub((m.p.sub(m.op)).multiply(10))
		was.toByteBuffer(buff)
		binary.Write(buff, le, magenta)

		if m.axle != nil {
			m.p.toByteBuffer(buff)
			quarter := m.p.add(m.axle.p.sub(m.p).multiply(0.25))
			quarter.toByteBuffer(buff)
			binary.Write(buff, le, orange) //Axle/leading edge
		}
		if m.lift != nil {

			m.p.toByteBuffer(buff)
			m.wingRoot.p.toByteBuffer(buff)
			binary.Write(buff, le, blue) //trailing edge

			if m.axle != nil {
				centreOfLift := m.p.add(m.wingRoot.p).add(m.axle.p).multiply(1.0 / 3.0)

				if m.axle.p == m.wingRoot.p {
					logit("axle and wingroot are the same")
				}

				centreOfLift.toByteBuffer(buff)                              //end1
				centreOfLift.add(m.lift.multiply(0.0001)).toByteBuffer(buff) //end 2
				binary.Write(buff, le, red)

			}

			//todo - acutal lift and drag vectors

		}
	}

	p.sendBytes(buff.Bytes())
	//logit(len(buff.Bytes()), "bytes sent for vectors")
	//logit("sent vectors", len(buff.Bytes()), "bytes")
}

// used for sending section of the interleaved (often) floating point data that makes up vertex, normal, position and index buffers
// in a format very close to that need by the GPU (or three.js buffers)

func (p *player) sendCursor() {
	buff := new(bytes.Buffer)
	e := binary.LittleEndian
	binary.Write(buff, e, byte(msgCursor)) //masses
	p.gridPos.toByteBuffer(buff)
	if p.spacePos == nil {
		p.spacePos = newVec3(0, 0, 0)
	}
	p.spacePos.toByteBuffer(buff)

	if p.highlit.mass != nil {
		binary.Write(buff, e, float32(0)) //cursor sphere radius
	} else {
		binary.Write(buff, e, float32(.05)) //cursor sphere radius
	}

	p.sendBytes(buff.Bytes())
}

func (p *player) sendHighlit() {
	buff := new(bytes.Buffer)

	binary.Write(buff, le, byte(msgHighlit)) //masses

	hm, ht, hs := int32(-1), int32(-1), int32(-1)
	if p.highlit.mass != nil {
		logit("hm", p.highlit.mass.index)
		hm = p.highlit.mass.index
	}
	if p.highlit.thing != nil {
		ht = p.highlit.thing.index
		logit("ht", p.highlit.thing.index)
	}
	if p.highlit.spring != nil {
		hs = p.highlit.spring.index
		logit("hs", p.highlit.spring.index)
	}
	binary.Write(buff, le, hm)
	binary.Write(buff, le, ht)
	binary.Write(buff, le, hs) //spring index within the thing
	p.sendBytes(buff.Bytes())

}

func (p *player) sendMessage(msg string, sev string) {

	logit(msg)
	buff := new(bytes.Buffer)
	e := binary.LittleEndian
	binary.Write(buff, e, byte(msgMessage))
	writeString(buff, msg)
	//binary.Write(buff, e, byte(len(sev)))
	//binary.Write(buff, e, []byte(sev))

	p.sendBytes(buff.Bytes())

}

func playersToBytes(players map[uint32]*player) []byte {
	buff := new(bytes.Buffer)

	binary.Write(buff, le, byte(msgPlayers))
	binary.Write(buff, le, uint32(len(players))) //number of players
	for _, p := range players {
		p.toByteBuffer(buff, le)
	}

	return buff.Bytes()
}

func (p *player) recordMassPositions() {
	p.massStartPos = make(map[*mass]*vec3) //reset each time
	for _, m := range p.state.masses {     //selectedMasses {
		p.massStartPos[m] = m.p
	}
}

func (p *player) moveCamera() {

	dir := newVec3(0, 0, 0)

	speed := .5
	if p.keys["Alt"] {
		speed = 10
	}

	if p.keys["w"] {
		dir.z = speed
	}
	if p.keys["s"] {
		dir.z = -speed
	}
	if p.keys["a"] {
		dir.x = -speed
	}
	if p.keys["d"] && p.keys["Control"] == false {
		dir.x = speed
	}
	if p.keys["ArrowUp"] {
		dir.y = speed
	}
	if p.keys["ArrowDown"] {
		dir.y = -speed
	}

	camDir := p.camera.direction

	right := camDir.cross(p.camera.up).normalise()
	up := right.cross(camDir).normalise()
	delta := right.multiply(dir.x).add(up.multiply(dir.y)).add(camDir.multiply(dir.z))

	if delta.lengthSq() > 0 {
		p.camera.position.addIn(delta)
		p.sendCamera()
		if p.mode == moving {
			p.moveSelected()
		}
	}

}

func NewPlayer(id uint32, name string, state *state, socket *websocket.Conn) *player {

	gridOrigin := newVec3(0, 0, 0)
	gridX := newVec3(1, 0, 0)
	gridY := newVec3(0, 0, 1) //this is a bit confusing but the 2d grid is initialised on the word xz plane

	p := player{state: state,
		id:             id,
		name:           name,
		maxDamage:      100,
		maxTemperature: 100,
		gridPos:        newVec3(0, 0, 0),
		damage:         0,
		temperature:    0,
		leftDrive:      0,
		rightDrive:     0,
		oRevs:          0,
		coins:          0,
		stepCoinsValue: 0,
		stepCoinCount:  0,
		highlit:        highlitType{nil, nil, nil},
		//springStart:    nil,
		currentThing:   nil,
		mode:           editing,
		killer:         -1,
		dying:          false,
		dead:           false,
		lives:          3,
		socket:         socket,
		grid:           &grid{origin: gridOrigin, Xaxis: gridX, Yaxis: gridY},
		cursor:         newVector(0, 0),
		grab:           nil,
		keys:           make(map[string]bool), //which keys are pressed
		selectedMasses: make(map[*mass]bool),  //which masses are selected, values are the order in which they were selected
		boundValues:    make(map[string]*float64, 0),
		//boundValues:    make(map[string]unsafe.Pointer, 0),
		mixers:   standardMixers,
		controls: make(map[controlInput]float64),
	}

	for i, _ := range inLabels {
		p.controls[controlInput(i)] = 0
	}

	direction := newVec3(0, -1, .1).normalise()
	p.camera = &camera{position: newVec3(0, 50, -25), direction: direction, up: newVec3(0, 1, 0).cross(direction).normalise(), farPos: newVec3(0, 0, 0)}

	//p.qh = &qHolder{mutex: &sync.Mutex{}, q: make(map[int][]*reply, 0)} //initialise their outbound queue
	//p.waitChannel = make(chan bool)
	p.mtx = &sync.Mutex{}
	p.inMtx = &sync.Mutex{}

	state.Tracks[p.name] = &track{Pointer: 0, Points: make([]float64, 800)}

	return &p

}

func playerFromByteBuffer(buff *bytes.Buffer, e binary.ByteOrder, s *state) *player {
	id := uint32(0)
	binary.Read(buff, e, &id)
	p := NewPlayer(id, "", s, nil)
	lengthOfPlayerName := byte(0)
	binary.Read(buff, e, &lengthOfPlayerName)
	name := make([]byte, lengthOfPlayerName)
	binary.Read(buff, e, &name)
	p.name = string(name)
	vidx := int32(0)
	binary.Read(buff, e, &vidx)
	if vidx > -1 {
		p.vehicle = s.things[vidx]
	}
	p.camera.fromByteBuffer(buff, e)
	p.grid.fromByteBuffer(buff, e)

	return p
}

func (p *player) toByteBuffer(buff *bytes.Buffer, e binary.ByteOrder) {
	binary.Write(buff, e, p.id) //unique player ID (Uint32)
	binary.Write(buff, e, byte(len(p.name)))
	binary.Write(buff, e, []byte(p.name))  //curent player name (may change)
	binary.Write(buff, e, p.vehicle.index) //thing index of their current vehicle
	binary.Write(buff, e, p.camera.toBytes())
	binary.Write(buff, e, p.grid.toBytes())

}

func (p *player) setMode(m ModeEnum) {
	p.mode = m
	logit("player", p.name, "mode set to", m)

	buff := new(bytes.Buffer)
	binary.Write(buff, le, byte(msgMode)) //masses
	writeString(buff, string(m))

	p.sendBytes(buff.Bytes())
}

func (p *player) checkHighlitMass() bool {
	if p.highlit.mass == nil {
		p.sendMessage("Highlight a mass and press the key", "error")
		return false
	}
	return true

}

func (p *player) sendBytes(msg []byte) {

	if p.socket == nil {
		logit(p.name + "player socket is disconnected")
		return
	}

	p.mtx.Lock()         //<<---MUTEX
	defer p.mtx.Unlock() //deferred unlock
	messageType := websocket.BinaryMessage

	//logit("sent", len(msg), " binary bytes")
	p.socket.WriteMessage(messageType, msg) //write the message (and return any error)
}

func (p *player) send(msg *reply) { //this is fo JSON message s- Dperecated

	if p.socket != nil {
		p.mtx.Lock()         //<<---MUTEX
		defer p.mtx.Unlock() //deferred unlock
		messageType := websocket.TextMessage

		bytes, err := json.Marshal(msg)
		if err != nil {
			logit(err.Error())
			return
		}

		p.socket.WriteMessage(messageType, bytes) //write the message (and return any error)
	} else {
		logit("player socket is disco'd")
	}

}

func (p *player) processBinaryMsg(mb []byte) {

	buff := bytes.NewBuffer(mb)

	cmd := [1]byte{}
	binary.Read(buff, le, &cmd)

	gameId := uint32(0)
	playerId := uint32(0)
	sl := byte(0)

	cm := msgEnum(cmd[0])
	if cm == msgCreateGame {
		binary.Read(buff, le, &gameId)
		binary.Read(buff, le, &playerId)
		binary.Read(buff, le, &sl)
		playerName := make([]byte, sl)
		binary.Read(buff, le, &playerName)

	} else if cm == msgValueChange {
		key := readString(buff)
		value := float64(0)
		binary.Read(buff, le, &value)
		pointer := p.boundValues[key]
		*(*float64)(pointer) = value //cast to *float64 and then dereference

		p.sendVectors() //send the new vectors
		//if p.highlit.mass != nil {
		p.sendMasses(p.state.masses, true) //send the potentially) modified mass
		if p.currentThing == nil {
			p.currentThing = p.state.things[0]
		}
		p.sendCentreOfMass(p.currentThing)
		//}

	} else if cm == msgLoad || cm == msgSave {

		idl := uint16(0)
		binary.Read(buff, le, &idl)
		idb := make([]byte, idl)
		binary.Read(buff, le, &idb)
		filename := string(idb)

		if cm == msgLoad {

			sock := p.socket //we'll push this onto me when i'm loaded

			p.state = load((filename))

			games[filename] = p.state

			p.camera = p.state.players[p.id].camera
			p.state.players[p.id] = p //put me (and my connected socket) back in the game i just loaded

			p = p.state.players[p.id] //this is imporant - pass the socket on
			p.socket = sock

			p.sendCamera()
			p.grid.send(p)

			p.currentThing = p.state.things[0]

			p.sendMasses(p.state.masses, true)
			p.sendThings(p.state.things) //[]*thing{p.vehicle}) //sends mesh name and springs
			p.sendGameId()               //game id starts it running
			p.vehicle = p.state.things[0]
			p.vehicle.setVelocity(testFlight)
			p.state.flyMasses()
			p.sendVectors()
			p.sendCentreOfMass(p.currentThing)

			p.sendMessage("Loaded", "info")
			logit("Game loaded", p.state.filename)

		} else {
			p.state.save(filename, p.selectedMasses)
			p.sendMessage("Saved OK", "info")
		}
	} else if cm == msgControlPositions {

		blobs := byte(0)
		binary.Read(buff, le, &blobs)
		var id, x, y = byte(0), byte(0), byte(0)
		for i := 0; i < int(blobs); i++ {
			binary.Read(buff, le, &id)
			binary.Read(buff, le, &x)
			binary.Read(buff, le, &y)
			if id == 1 {
				p.controls[ciStickX] = float64(x)/128 - 1 //normalise to +/- 1
				p.controls[ciStickY] = float64(y)/128 - 1
				logit("right stick", p.controls[ciStickX], p.controls[ciStickY])
			}

			if id == 2 {
				p.controls[ciRudder] = float64(x)/128 - 1
				p.controls[ciThrottle] = float64(y)/128 - 1
				logit("left stick", p.controls[ciRudder], p.controls[ciThrottle])
			}

			if id == 3 {
				p.controls[ciWheelBrakeLeft] = float64(y)/128 - 1
			}

			if id == 4 {
				p.controls[ciWheelBrakeRight] = float64(y)/128 - 1
			}

		}
		p.updateActuators()

	} else {
		panic("other Inbound binary messages not implemented" + string(cmd[0]))
	}

}
func (p *player) Move(state *state) {

	if !p.dying && !p.dead { //you loose all traction when dying

		rr := p.vehicle.springs[0].m1
		rl := p.vehicle.springs[0].m2
		fl := p.vehicle.springs[1].m2
		fr := p.vehicle.springs[2].m2

		leftside := fl.p.sub(rl.p)
		lt := leftside.normalise() //left track

		rightside := fr.p.sub(rr.p)
		rt := rightside.normalise() //right track

		const speed = .02 //this is actually an acceleration ..(of 2cm/1/30th of a second)
		fl.p.addIn(lt.multiply(p.leftDrive * speed))
		rl.p.addIn(lt.multiply(p.leftDrive * speed))

		fr.p.addIn(rt.multiply(p.rightDrive * speed))
		rr.p.addIn(rt.multiply(p.rightDrive * speed))

		//OLD
		// direction := v.normalise()

		// wingAxis := fr.p.sub(fl.p).normalise()
		// liftDir := wingAxis.cross(direction).normalise() //perpendicular to the direction of travel (which is - the direction of the airflow (in still air at least))

		// v2 := vms * vms   //v.lengthSq()
		// wingArea := 100.0 //m^2

		// aoa := math.Asin(direction.dot(up))/math.Pi*180 + 5 //main wing incidence
		// cl := lerp(aoa, state.liftCurve)
		// cd := lerp(aoa, state.dragCurve)

		// liftNewtons := v2 * cl * wingArea
		// lift := liftDir.multiply(liftNewtons)
		// //parasitic drag + induced drag
		// drag := liftNewtons / 10 * cd //(v2 * cd * wingArea)
		// logit("drag", drag)

		// if lift.y > gravity && fl.p.y < 0.2 {
		// 	logit("takeoff")
		// }

		// //stabilise with a tailplane
		// taoa := math.Asin(direction.dot(up))/math.Pi*180 + p.elevator //tailplane incidence
		// tcl := lerp(taoa, state.liftCurve)
		// //logit("aoa", 90-math.Acos(aoa)*180/math.Pi)

		// tailArea := 20.0 //m^2
		// tailLiftNewtons := v2 * tcl * tailArea
		// tailLift := up.multiply(-tailLiftNewtons * ntm)

		// rl.p.addIn(tailLift)
		// fl.p.subIn(tailLift)

		// rr.p.addIn(tailLift)
		// fr.p.subIn(tailLift)

		// //dihedral/roll stability

		// if v2 > 0.1 && direction.x != 0 || direction.z != 0 { //we cannot roll stabilise something falling straight down
		// 	worldUp := newVec3(0, 1, 0)
		// 	//up - already lies on the direction plane
		// 	worldUpInDirectionPlane := worldUp.projectOntoPlane(newVec3(0, 0, 0), direction).normalise()
		// 	roll := worldUpInDirectionPlane.dot(up)
		// 	sign := worldUpInDirectionPlane.cross(up).dot(direction)

		// 	rollAngle := math.Acos(roll) * 180 / math.Pi
		// 	if sign < 0 {
		// 		rollAngle = -rollAngle
		// 		roll = -roll
		// 	}

		// 	// rr.addIn(up.multiply(velocity2 * .01 * roll))
		// 	// fr.addIn(up.multiply(velocity2 * .01 * roll))
		// 	// rl.addIn(up.multiply(velocity2 * .01 * -roll))
		// 	// fl.addIn(up.multiply(velocity2 * .01 * -roll))

		// 	//logit("roll", rollAngle)
		// }

		// //logit(fwd, drag, lift)
		// //if p.thrust > 0 {
		// for _, m := range []*mass{fl, fr} {
		// 	m.p.addIn(fwd.multiply(p.thrust * ntm * 3))
		// 	m.p.addIn(lift.multiply(.6 * ntm))
		// 	m.p.addIn(direction.multiply(-drag * .7 * ntm))
		// }

		// for _, m := range []*mass{rl, rr} {

		// 	m.p.addIn(lift.multiply(.4 * ntm))
		// 	m.p.addIn(direction.multiply(-drag * .3 * ntm))

		// }
		//}
		//}

		revs := float32(math.Abs(float64(p.leftDrive)) + math.Abs(float64(p.rightDrive)))
		if revs != p.oRevs {
			p.oRevs = revs
			state.send(nil, &reply{Cmd: "revs", Payload: revsPayload{Player: p.name, Revs: revs}})
		}

	}
}
