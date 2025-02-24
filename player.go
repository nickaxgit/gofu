package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
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

	springStart  *mass
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

	lastLandPos *Vec3 //where were we when we last generated land

	aileron  float64
	elevator float64
	throttle float64
	rudder   float64

	gridOrigin *Vec3 //deifnes the plane of this players construction grid
	gridXaxis  *Vec3 //deifnes the plane of this players construction grid
	gridYaxis  *Vec3 //deifnes the plane of this players construction grid

	camera *camera
	// camDirection   *Vec3
	// camUp          *Vec3
	gridPos        *Vec3
	selectedMasses map[*mass]bool
	massStartPos   map[*mass]*Vec3 //for dragging - we record the start positions
	moveStart      *Vec3           //for dragging
	//higlitMass     *mass

	downGridPos      *Vec3 //where were we when they pressed the mouse button down
	downCamPos       *Vec3
	downCamDirection *Vec3
	downCamUp        *Vec3

	cursor *Vector //current mouse pos in normalised screen coords (-1/+1)
	grab   *Vector //where we moused down in normalised screen coords (-1/+1)

	//camFarPos *Vec3 //mouse on the far plane
	// downCamFarPos *Vec3 //recorded on mousedown

	//mouseDown     bool
	buttons byte
	keys    map[string]bool

	movedSinceMouseDown bool
}

func (p *player) recordSelectedMassPositions() {
	p.massStartPos = make(map[*mass]*Vec3) //reset each time
	for m := range p.selectedMasses {
		p.massStartPos[m] = m.p
	}
}

func (p *player) moveCamera() {

	dir := newVec3(0, 0, 0)

	if p.keys["w"] {
		dir.z = .1
	}
	if p.keys["s"] {
		dir.z = -.1
	}
	if p.keys["a"] {
		dir.x = -.1
	}
	if p.keys["d"] {
		dir.x = .1
	}
	if p.keys["upArrow"] {
		dir.y = .1
	}
	if p.keys["downArrow"] {
		dir.y = .1
	}

	camDir := p.camera.direction

	right := camDir.cross(p.camera.up).normalise()
	up := right.cross(camDir).normalise()
	delta := right.multiply(dir.x).add(up.multiply(dir.y)).add(camDir.multiply(dir.z))

	if delta.lengthSq() > 0 {
		p.camera.position.addIn(delta)
		p.sendCamera(binary.LittleEndian)
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
		springStart:    nil,
		currentThing:   nil,
		mode:           editing,
		killer:         -1,
		dying:          false,
		dead:           false,
		lives:          3,
		socket:         socket,
		gridOrigin:     gridOrigin,
		gridXaxis:      gridX,
		gridYaxis:      gridY,
		cursor:         newVector(0, 0),
		grab:           nil,
		keys:           make(map[string]bool), //which keys are pressed
		selectedMasses: make(map[*mass]bool),  //which masses are selected, values are the order in which they were selected
	}

	direction := newVec3(0, -1, .1).normalise()
	p.camera = &camera{position: newVec3(0, 50, -25), direction: direction, up: newVec3(0, 1, 0).cross(direction).normalise(), farPos: newVec3(0, 0, 0)}

	//p.qh = &qHolder{mutex: &sync.Mutex{}, q: make(map[int][]*reply, 0)} //initialise their outbound queue
	//p.waitChannel = make(chan bool)
	p.mtx = &sync.Mutex{}
	state.Tracks[p.name] = &track{Pointer: 0, Points: make([]float64, 800)}

	return &p

}

func (p *player) setMode(m ModeEnum) {
	p.mode = m
	logit("player", p.name, "mode set to", m)
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
	e := binary.LittleEndian

	cmd := [1]byte{}
	binary.Read(buff, e, &cmd)

	gameId := uint32(0)
	playerId := uint32(0)
	sl := byte(0)
	if msgEnum(cmd[0]) == msgCreateGame {
		binary.Read(buff, e, &gameId)
		binary.Read(buff, e, &playerId)
		binary.Read(buff, e, &sl)
		playerName := make([]byte, sl)
		binary.Read(buff, e, &playerName)

	} else {

		panic("other Inbound binary messages not implemented" + string(cmd[0]))
	}

}

func (p *player) Move(state *state) {

	gravity := 9.81 * (1 / 30.0 * 1 / 30.0) //DONT half this

	ntm := 0.000000005 //newtons to metres of movement per substep

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

		//add thrust to all masses in the dozer so we don't create a pitching moment
		fwd := lt.add(rt).multiply(.5)

		ds := p.vehicle.springs
		top := ds[8].m2.p
		bottom := ds[8].m1.p
		up := top.sub(bottom).normalise()

		v := fl.p.sub(fl.op)

		vms := v.length() * 30.0 * 5.0
		logit(vms, "m/s")

		if v.length() > 0 {

			direction := v.normalise()

			wingAxis := fr.p.sub(fl.p).normalise()
			liftDir := wingAxis.cross(direction).normalise() //perpendicular to the direction of travel (which is - the direction of the airflow (in still air at least))

			v2 := vms * vms   //v.lengthSq()
			wingArea := 100.0 //m^2

			aoa := math.Asin(direction.dot(up))/math.Pi*180 + 5 //main wing incidence
			cl := lerp(aoa, state.liftCurve)
			cd := lerp(aoa, state.dragCurve)

			liftNewtons := v2 * cl * wingArea
			lift := liftDir.multiply(liftNewtons)
			//parasitic drag + induced drag
			drag := liftNewtons / 10 * cd //(v2 * cd * wingArea)
			logit("drag", drag)

			if lift.y > gravity && fl.p.y < 0.2 {
				logit("takeoff")
			}

			//stabilise with a tailplane
			taoa := math.Asin(direction.dot(up))/math.Pi*180 + p.elevator //tailplane incidence
			tcl := lerp(taoa, state.liftCurve)
			//logit("aoa", 90-math.Acos(aoa)*180/math.Pi)

			tailArea := 20.0 //m^2
			tailLiftNewtons := v2 * tcl * tailArea
			tailLift := up.multiply(-tailLiftNewtons * ntm)

			rl.p.addIn(tailLift)
			fl.p.subIn(tailLift)

			rr.p.addIn(tailLift)
			fr.p.subIn(tailLift)

			//dihedral/roll stability

			if v2 > 0.1 && direction.x != 0 || direction.z != 0 { //we cannot roll stabilise something falling straight down
				worldUp := newVec3(0, 1, 0)
				//up - already lies on the direction plane
				worldUpInDirectionPlane := worldUp.projectOntoPlane(newVec3(0, 0, 0), direction).normalise()
				roll := worldUpInDirectionPlane.dot(up)
				sign := worldUpInDirectionPlane.cross(up).dot(direction)

				rollAngle := math.Acos(roll) * 180 / math.Pi
				if sign < 0 {
					rollAngle = -rollAngle
					roll = -roll
				}

				// rr.addIn(up.multiply(velocity2 * .01 * roll))
				// fr.addIn(up.multiply(velocity2 * .01 * roll))
				// rl.addIn(up.multiply(velocity2 * .01 * -roll))
				// fl.addIn(up.multiply(velocity2 * .01 * -roll))

				//logit("roll", rollAngle)
			}

			//logit(fwd, drag, lift)
			//if p.thrust > 0 {
			for _, m := range []*mass{fl, fr} {
				m.p.addIn(fwd.multiply(p.thrust * ntm * 3))
				m.p.addIn(lift.multiply(.6 * ntm))
				m.p.addIn(direction.multiply(-drag * .7 * ntm))
			}

			for _, m := range []*mass{rl, rr} {

				m.p.addIn(lift.multiply(.4 * ntm))
				m.p.addIn(direction.multiply(-drag * .3 * ntm))

			}
		}
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
