package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"maps"
	"math"
	"slices"
	"sync"

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
	mtx *sync.Mutex // a mutex is required to 'lock' access to each users connection (for writing)
	// many calls (to wsEndpoint) can be running in paralell - and more than one of them may attempt to write to a single users socket at the same time (not allowed!)
	socket *websocket.Conn // a pointer to the socket

	landTri   *Tri
	landMesh  *mesh
	waterMade bool

	lastLandPos *vec3 //where were we when we last generated land

	aileron  float64
	elevator float64
	throttle float64
	rudder   float64

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

	downGridPos      *vec3 //where were we when they pressed the mouse button down
	downCamPos       *vec3
	downCamDirection *vec3
	downCamUp        *vec3

	cursor *Vector //current mouse pos in normalised screen coords (-1/+1)
	grab   *Vector //where we moused down in normalised screen coords (-1/+1)

	//camFarPos *Vec3 //mouse on the far plane
	// downCamFarPos *Vec3 //recorded on mousedown

	//mouseDown     bool
	buttons byte
	keys    map[string]bool

	boundValues map[string]boundValue //these form a popup dialog box (mass properties)

	movedSinceMouseDown bool
}

func (p *player) processMouseMove() {

	if p.state.running == false {

		//a point on the far plane (where the mouse cursor is pointing)

		if p.buttons == 2 { //panning camera

			delta := (p.cursor.subtract(p.grab)).multiply(2)

			if delta.lengthSq() != 0 {

				logit("delta", delta.X, delta.Y)

				camRight := p.downCamDirection.cross(p.downCamUp).normalise()

				p.camera.up = p.downCamUp.rotateAbout(camRight, delta.Y).normalise()
				pitched := p.downCamDirection.rotateAbout(camRight, delta.Y)
				yawed := pitched.rotateAbout(p.camera.up, -delta.X)
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
			delta := p.gridPos.sub(p.downGridPos).multiply(.9)
			p.camera.position = p.downCamPos.sub(delta)
			p.sendCamera()
		}

		p.sendCursor()
	}
}

func (p *player) bindValue(key string, mass *mass, value *float64, min float32, max float32, step float32, conversion float32) {
	p.boundValues[key] = boundValue{id: key, mass: mass, valuePointer: value, min: min, max: max, step: step, conversion: conversion}
}

func (p *player) sendGameId() { //gameId uint32, e binary.ByteOrder) {
	buff := new(bytes.Buffer)
	writeByte(buff, byte(msgGameId))
	writeString(buff, p.state.filename)
	p.sendBytes(buff.Bytes())

}

func (p *player) moveHighlit() {
	moveDelta := p.gridPos.sub(p.moveStart)

	//add any movement normal to the grid to the delta
	gridNormal := p.grid.Xaxis.cross(p.grid.Yaxis).normalise()
	camDGN := gridNormal.multiply(p.camera.position.sub(p.downCamPos).dot(gridNormal))
	moveDelta.addIn(camDGN)

	m := p.highlit.mass

	sp, ok := p.massStartPos[m]
	if ok {
		m.p = sp.add(moveDelta)
		p.sendMasses([]*mass{m}, false) //just send the new positions
	} else {
		logit("no startpos present for mass ", m.index)
	}
}

// consolidates masses at the same point and rewires springs (masses on mirrors for example)
func (p *player) snapMasses() {
	for _, m := range p.state.masses {
		if m.transformOf != nil {
			if m.transformOf.p.distanceFrom(m.p) < 0.01 {
				//rewire the springs
				for t := range p.state.things {
					for _, s := range p.state.things[t].springs {
						if s.m1 == m {
							s.m1 = m.transformOf
						}
						if s.m2 == m {
							s.m2 = m.transformOf

						}
					}
				}
			}
		}
	}

}

// sends position updates for masses we re moving (and when the camera moves up - where selected masses move with the camera)
func (p *player) moveSelected() {
	moveDelta := p.spacePos.sub(p.moveStart)

	//add any movement normal to the grid to the delta
	gridNormal := p.grid.Xaxis.cross(p.grid.Yaxis).normalise()
	camDGN := gridNormal.multiply(p.camera.position.sub(p.downCamPos).dot(gridNormal))
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

func (p *player) moveSpringCursor() {
	moveDelta := p.gridPos.sub(p.moveStart)

	//add any movement normal to the grid to the delta
	gridNormal := p.grid.Xaxis.cross(p.grid.Yaxis).normalise()
	camDGN := gridNormal.multiply(p.camera.position.sub(p.downCamPos).dot(gridNormal))
	moveDelta.addIn(camDGN)

	m := p.springCursor
	m.p = p.massStartPos[m].add(moveDelta)

	p.sendMasses([]*mass{m}, false) //just send the new positions

}

func (p *player) makeNextSpring() {
	if p.highlit.mass == nil {
		p.highlit.mass = p.state.addMass(newMass(p.spacePos, .05, false, false, true, p.currentThing, nil))
	}

	m1 := p.highlit.mass
	m2 := p.state.addMass(newMass(m1.p.clone().add(newVec3(0, .001, 0)), 0.05, false, false, true, p.currentThing, nil))

	p.springCursor = m2

	p.highlit.spring = p.currentThing.AddSpring(m1, m2, 1)
	logit("made spring", m1.index, m2.index)

	//p.recordMassPositions() //we need to (re) do this as we have added masses

	// p.selectedMasses = make(map[*mass]bool)
	// p.selectedMasses[m2] = true
	p.sendMasses([]*mass{m1, m2}, false)
	p.sendThings([]*thing{p.currentThing})
	p.sendHighlit()

}

func (p *player) sendBoundValues() {

	e := binary.LittleEndian
	buff := new(bytes.Buffer)
	binary.Write(buff, e, byte(msgBoundValues)) //masses
	binary.Write(buff, e, byte(len(p.boundValues)))
	for _, bv := range p.boundValues {
		bv.toByteBuffer(buff, e)
	}

	p.sendBytes(buff.Bytes())
}

func (p *player) sendCamera() {

	// up := newVec3(0, 1, 0)
	// p.camera.up = p.camera.direction.cross(up).cross(p.camera.direction).normalise()
	p.sendBytes(p.camera.toBytes())

}

func (player *player) sendMasses(masses []*mass, withDetail bool) {
	player.sendBytes(massesToBytes(masses, withDetail, player.selectedMasses))
	player.sendVectors()

}

func (player *player) sendMakeMesh(name string, numVerts uint32, numFaces uint32) {
	buff := new(bytes.Buffer)
	e := binary.LittleEndian
	binary.Write(buff, e, byte(msgMesh))
	binary.Write(buff, e, byte(len(name)))
	binary.Write(buff, e, []byte(name))
	binary.Write(buff, e, make([]byte, len(name)%4+2)) //padding
	binary.Write(buff, e, numVerts)
	binary.Write(buff, e, numFaces)

	player.sendBytes(buff.Bytes())
}

func (player *player) sendVectors() {

	//send the mass index, vector and color - show lift at the wingtips (althoug it is actually shared between the three verts)

	buff := new(bytes.Buffer)
	e := binary.LittleEndian
	binary.Write(buff, e, byte(msgVectors))

	white := uint32(0xffffff)
	yellow := uint32(0xffff00)

	for _, m := range player.state.masses {
		if m.axle != nil {
			m.p.toByteBuffer(buff)
			m.axle.p.toByteBuffer(buff)
			binary.Write(buff, e, white) //16 (or whatever) standard colours
		}
		if m.wingRoot != nil {

			m.p.toByteBuffer(buff)
			m.wingRoot.p.toByteBuffer(buff)
			binary.Write(buff, e, white)

			if m.axle != nil {
				centreOfLift := m.p.add(m.wingRoot.p).add(m.axle.p).multiply(1.0 / 3.0)
				centreOfLift.toByteBuffer(buff) //end1

				upVector := m.axle.p.sub(m.wingRoot.p).cross(m.axle.p.sub(m.p)).normalise()
				wingAxis := m.p.sub(m.axle.p).normalise()
				rootAxis := m.axle.p.sub(m.wingRoot.p).normalise()
				upVector = upVector.rotateAbout(wingAxis, m.aoaRads)         //additional angle of attack (in radians)
				upVector = upVector.rotateAbout(rootAxis, m.dihedralDegrees) //dihedral (in radians)
				centreOfLift.add(upVector).toByteBuffer(buff)                //end2

				binary.Write(buff, e, white)

				if player.state.running == true {
					if m.lift != nil {
						centreOfLift.toByteBuffer(buff)
						centreOfLift.add(m.lift).toByteBuffer(buff)
						binary.Write(buff, e, yellow)
					}
				}

			}

			//todo - acutal lift and drag vectors

		}
	}

	player.sendBytes(buff.Bytes())
}

// used for sending section of the interleaved (often) floating point data that makes up vertex, normal, position and index buffers
// in a format very close to that need by the GPU (or three.js buffers)
func (player *player) sendData(name string, opCode msgEnum, elementOffset uint32, elementCount uint32, data interface{}) {
	buff := new(bytes.Buffer)
	e := binary.LittleEndian
	binary.Write(buff, e, opCode)
	binary.Write(buff, e, byte(len(name)))
	binary.Write(buff, e, []byte(name))
	binary.Write(buff, e, make([]byte, len(name)%4+2)) //padding

	binary.Write(buff, e, elementOffset)
	binary.Write(buff, e, elementCount)
	binary.Write(buff, e, data)    //x,y,z float32 triples (or uint16 face indices)
	player.sendBytes(buff.Bytes()) //&reply{Cmd: "mesh", Payload: meshPayload})

}

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
	e := binary.LittleEndian
	binary.Write(buff, e, byte(msgHighlit)) //masses

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
	binary.Write(buff, e, hm)
	binary.Write(buff, e, ht)
	binary.Write(buff, e, hs) //spring index within the thing
	p.sendBytes(buff.Bytes())

}

func (p *player) sendMessage(msg string, sev string) {

	logit(msg)
	buff := new(bytes.Buffer)
	e := binary.LittleEndian
	binary.Write(buff, e, byte(msgMessage))
	binary.Write(buff, e, byte(len(msg)))
	binary.Write(buff, e, []byte(msg))
	binary.Write(buff, e, byte(len(sev)))
	binary.Write(buff, e, []byte(sev))

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
		boundValues:    make(map[string]boundValue),
	}

	direction := newVec3(0, -1, .1).normalise()
	p.camera = &camera{position: newVec3(0, 50, -25), direction: direction, up: newVec3(0, 1, 0).cross(direction).normalise(), farPos: newVec3(0, 0, 0)}

	//p.qh = &qHolder{mutex: &sync.Mutex{}, q: make(map[int][]*reply, 0)} //initialise their outbound queue
	//p.waitChannel = make(chan bool)
	p.mtx = &sync.Mutex{}
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
	binary.Write(buff, le, byte(len(string(m))))
	binary.Write(buff, le, []byte(string(m)))
	p.sendBytes(buff.Bytes())
}
func (p *player) sendBytes(msg []byte) {
	p.mtx.Lock()         //<<---MUTEX
	defer p.mtx.Unlock() //deferred unlock
	messageType := websocket.BinaryMessage

	//logit("sent", len(msg), " binary bytes")
	p.socket.WriteMessage(messageType, msg) //write the message (and return any error)
}

func (p *player) send(msg *reply) {
	p.mtx.Lock()         //<<---MUTEX
	defer p.mtx.Unlock() //deferred unlock
	messageType := websocket.TextMessage

	bytes, err := json.Marshal(msg)
	if err != nil {
		logit(err.Error())
		return
	}

	p.socket.WriteMessage(messageType, bytes) //write the message (and return any error)
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
		idl := uint16(0)
		binary.Read(buff, le, &idl)
		idb := make([]byte, idl)
		binary.Read(buff, le, &idb)
		id := string(idb)
		value := float32(0)
		binary.Read(buff, le, &value)
		bv := p.boundValues[id]
		*bv.valuePointer = float64(value) / float64(bv.conversion) //set the value at the pointer (converting (for example from degress to radians) in necessary)

		p.sendVectors()                                     //send the new vectors
		p.sendMasses([]*mass{p.boundValues[id].mass}, true) //send the potentially) modified mass
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

			p.sendMessage("Loaded", "info")
			logit("Game loaded", p.state.filename)

		} else {
			p.state.save(filename, p.selectedMasses)
			p.sendMessage("Saved OK", "info")
		}

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

func (player *player) makeWater() {
	//pour an amount on every vertex proportional to altitude

	lnd := player.landMesh

	for _, v := range lnd.verts {
		v.wl = v.p.y
	}

	player.waterMade = true

}
