package player

import (
	"bytes"
	"encoding/binary"
	"maps"
	"math"
	"math/rand"
	"slices"
	"strings"
	"sync"
	"unsafe"

	//"unsafe"
	"github.com/gorilla/websocket"
	"github.com/nickax/gofu/cam"

	"github.com/nickax/gofu/fiz/mass"
	"github.com/nickax/gofu/fiz/mixer"
	"github.com/nickax/gofu/fiz/spring"
	"github.com/nickax/gofu/fiz/thing"
	"github.com/nickax/gofu/game/label"

	"github.com/nickax/gofu/game/actuator"
	"github.com/nickax/gofu/game/aero"
	"github.com/nickax/gofu/game/grid"
	"github.com/nickax/gofu/game/input"

	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/jsonmsg"
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/mesh"
	"github.com/nickax/gofu/ray"

	"github.com/nickax/gofu/game/sound"
	"github.com/nickax/gofu/terrain"
	"github.com/nickax/gofu/vec"
)

type ModeEnum string

const (
	editing = "Editing"
	//playing      = "Running"
	adding         = "Adding"
	stretching     = "Stretching"
	startMove      = "Select the start point of the move"
	moving         = "Moving - Select the destination"
	grabbingMesh   = "Grabbing Mesh - select a source point"
	offsettingMesh = "Offsetting Mesh - select a target point"
	props          = "Settings Properties"
)

type highlitType struct {
	mass   *mass.Mass
	spring *spring.Spring
	thing  *thing.Thing
}

type Player struct {
	GameId uint32
	Id     uint32 //used in the players map
	Name   string
	//state          *game.State //the game he is in
	vehicle *thing.Thing //int    `json:"dozer"` //index of dozer in the things array

	highlit highlitType

	currentThing *thing.Thing
	mode         ModeEnum

	mtx   *sync.Mutex // a mutex is required to 'lock' access to each users connection (for writing)
	InMtx *sync.Mutex //inbound mutex, ensure only one command is processed at a time
	// many calls (to wsEndpoint) can be running in paralell - and more than one of them may attempt to write to a single users Socket at the same time (not allowed!)
	Socket           *websocket.Conn // a pointer to the socket - no players shoundnt have a socket, sockets should have a player (more than one socket can feed a player)
	ControllerSocket *websocket.Conn //each player can only have one controller - but more than one player can drive/fly the same vehicle(e.g. pilot/co-pilot)

	landTri *terrain.Tri //terrain is generated JIT for each player
	Camera  *cam.Camera
	lastCam *cam.Camera //where were we positioned/looking when we last generated land

	Grid *grid.Grid

	gridPos      *vec.V3
	zOff         float64 //offset from the grid (along the grid normal)
	spacePos     *vec.V3
	springCursor *mass.Mass

	selectedMasses map[*mass.Mass]bool
	massStartPos   map[*mass.Mass]*vec.V3 //for dragging - we record the start positions
	moveStart      *vec.V3                //for dragging
	//higlitMass     *mass.Mass

	downCam     *cam.Camera
	downGridPos *vec.V3 //where were we when they pressed the mouse button down
	//downCamPos  *vec3

	cursor   *vec.V2 //current mouse pos in normalised screen coords (-1/+1)
	grab     *vec.V2 //where we moused down in normalised screen coords (-1/+1)
	meshGrab *vec.V3 //a 'source' point on the mesh - we will translate to some target point

	//mouseDown     bool
	buttons     byte
	keys        map[string]bool
	boundValues map[string]*float64 //boundValue //these form a popup dialog box (mass properties)

	controls map[input.ControlInput]float64

	//each mixer (of the player) adds a contribution to to one mass (e.g. an aileron)
	mixers []*mixer.Mixer

	movedSinceMouseDown bool
	follow              bool    //whether the camera follows the players vehicle
	heading             float64 //heading of the vehicle in degrees

	labels []*label.Label
	//engineSounds        []uint16 //sound ids for the engine sounds (multi-engined aircraft)
}

func (p *Player) SetVehicle(t *thing.Thing) {
	p.vehicle = t
}

// collects and sends the flames visible to this player
func (p *Player) GetFlames(fire *terrain.TriMesh) {

	tcs := mesh.NewTcs(0, 1, 1, 0)                    //texture atlas coordinates
	flameMesh := mesh.New(201, "flame", 10000, 30000) //10k faces, 30k verts
	fire.Root.GetFlames(p.landTri, fire, flameMesh, p.Camera, tcs)

	p.Send(flameMesh.ToMsg(1))

}

func (player *Player) updateCamera() {
}

func (player *Player) clearContextMenu() {
	//clear the context menu (on the client)
	m := msg.NewMsg(msg.ClearContextMenu)
	player.Send(m)

}

func (player *Player) updateActuators() {

	for _, mix := range player.mixers {
		mix.ZeroOutputs()
	}

	for _, mixer := range player.mixers {
		mixer.Mix(player.controls, player.vehicle.Engines)
	}
}

func (player *Player) processMouseMove(isRunning bool, masses []*mass.Mass, things []*thing.Thing) {

	if isRunning == false {

		//a point on the far plane (where the mouse cursor is pointing)

		if player.buttons == 2 { //panning camera

			delta := (player.cursor.Sub(player.grab)).Mul(2)

			if delta.LengthSq() != 0 {

				//logit("delta", delta.x, delta.y)

				camRight := player.downCam.Direction.Cross(player.downCam.Up).Normalise()

				player.Camera.Up = player.downCam.Up.RotateAbout(camRight, delta.Y).Normalise()
				pitched := player.downCam.Direction.RotateAbout(camRight, delta.Y)
				yawed := pitched.RotateAbout(player.Camera.Up, -delta.X)
				//player.camUp = player.camUp.rotateAbout(player.downCamUp, delta.X).normalise()
				player.Camera.Direction = yawed
				player.Camera.Up = vec.NewVec3(0, 1, 0) //auto level the camera

				// worldUp := NewVec3(0, 1, 0)
				// camDir := (player.camLookAt.sub(player.camPosition)).normalise()
				// player.camUp = camDir.cross(worldUp).normalise().cross(camDir).normalise()

				player.SendCamera()
			}
			return
		}

		if player.mode == editing {

			if player.buttons == 0 {
				pickRay := ray.New(player.Camera.Position, player.Camera.FarPos)
				cm, _ := pickRay.ClosestMass(masses, player.springCursor) //state.ClosestMassToRay(player.Camera.Position, player.Camera.FarPos, player.springCursor)

				if cm != player.highlit.mass {
					player.highlit.mass = cm
					player.sendHighlit() //might be nil
				}
			}

			//mutates the players gridPos and spacePos (by reference)
			player.Grid.UpdateGridPosAndSpacePos(player.Camera, player.zOff, player.gridPos, player.spacePos)

			pickRay := ray.New(player.Camera.Position, player.Camera.FarPos)

			closest := math.MaxFloat64
			for _, thing := range things {
				spring, d := pickRay.ClosestSpring(thing.Springs)
				if d < closest {
					player.highlit.thing = thing
					player.highlit.spring = spring
					closest = d
				}
			}

			player.sendHighlit()

		} else if player.mode == moving {
			player.moveSelected(masses)
		} else if player.mode == stretching {
			player.springCursor.P = player.spacePos.Clone() //moveSpringCursor()
			player.sendMasses([]*mass.Mass{player.springCursor}, false)
		}

		if player.buttons == 1 && player.mode == editing {
			//dragging/panning the camera
			if player.downGridPos != nil {
				delta := player.gridPos.Sub(player.downGridPos).Multiply(.9)
				player.Camera.Position = player.downCam.Position.Sub(delta)
				player.SendCamera()
			}
		}

		player.SendCursor()
	}
}

func (player *Player) bindValue(key string, valuePointer *float64, min float64, max float64, step float64, labelSet byte) {

	player.boundValues[key] = valuePointer //store the address of the value

	m := msg.NewMsg(msg.BindValue)
	m.Write(key, *valuePointer, min, max, step, labelSet)

	// msg.GenericWrite(bf,key)
	// m.WriteFloat64(*valuePointer)
	// m.WriteFloat64(min)
	// m.WriteFloat64(max)
	// m.WriteFloat64(step)
	// m.WriteByte(labelSet) //0 or an index to a if this is an enumeration, this is the number of values
	player.Send(m)

}

func (player *Player) SendLabelSets() { //For options on the sliders

	sendLabelSet(player, 1, actuator.MassActuators)
	sendLabelSet(player, 2, actuator.SpringActuators)
	sendLabelSet(player, 3, aero.SectionNames)
}

func sendLabelSet[E actuator.ActuatorEnum | aero.Section](p *Player, idx byte, valueLabelPairs map[E]string) {

	m := msg.NewMsg(msg.LabelSet)
	m.Write(idx, byte(len(valueLabelPairs))) //index of the label set/ count of labels

	for value, label := range valueLabelPairs {
		m.Write(int32(value), label)
	}
	p.Send(m)

}

func (player *Player) SetControlInputsFromBlob(blobId byte, x float64, y float64) {

	switch blobId {

	case 1:
		player.controls[input.StickX] = float64(x)/128 - 1 //normalise to +/- 1
		player.controls[input.StickY] = float64(y)/128 - 1
		log.Logit("right stick", player.controls[input.StickX], player.controls[input.StickY])

	case 2:
		player.controls[input.Rudder] = float64(x)/128 - 1
		player.controls[input.Throttle] = float64(y)/128 - 1
		log.Logit("left stick", player.controls[input.Rudder], player.controls[input.Throttle])

	case 3:
		player.controls[input.WheelBrakeLeft] = float64(y)/128 - 1

	case 4:
		player.controls[input.WheelBrakeRight] = float64(y)/128 - 1

	default:
		log.Logit("warning - unhandled blob id ", blobId)
	}

}

func (player *Player) sendCentreOfMass(t *thing.Thing) {

	cg, weight := t.CentreOfMass()
	player.Send(msg.NewMsg(msg.CentreOfMass, t.Index, cg, float32(weight)))

}

func (player *Player) Send(msg ...*msg.Msg) {
	for _, msg := range msg {
		player.sendBytes(msg.AllBytes())
	}

}

func (player *Player) ViewChangedSignificantly() bool {
	if player.lastCam == nil {
		return true
	}

	dist := player.Camera.Position.DistanceFrom(player.lastCam.Position)
	dir := player.Camera.Direction.Dot(player.lastCam.Direction)
	if dist > 100 || dir < .95 {
		player.lastCam = player.Camera.Clone() //store this as the new old position
		return true
	}
	return false
}

func (player *Player) sendControlPin() uint32 {

	m := msg.NewMsg(msg.ControlToken)
	token := randomPin()

	m.Write(token)
	player.Send(m)

	return token

}

func randomPin() uint32 { //TODO - Check for existing token
	return uint32(math.Round(1000 + rand.Float64()*8999))
}

// SendGameId - causes the client to start the game
func (player *Player) sendGameId(gameId uint32) {
	m := msg.NewMsg(msg.GameId)
	m.Write(gameId)
	player.Send(m)
}

// func (p *Player) moveHighlit() {
// 	moveDelta := p.gridPos.sub(p.moveStart)

// 	//add any movement normal to the grid to the delta
// 	gridNormal := p.grid.Xaxis.cross(p.grid.Yaxis).normalise()
// 	camDGN := gridNormal.multiply(p.camera.Position.sub(p.downCam.position).dot(gridNormal))
// 	moveDelta.addIn(camDGN)

// 	m := p.highlit.mass

// 	sp, ok := p.massStartPos[m]
// 	if ok {
// 		m.p = sp.add(moveDelta)
// 		p.sendMasses([]*mass.Mass{m}, false) //just send the new positions
// 	} else {
// 		logit("no startpos present for mass ", m.index)
// 	}
// }

// consolidates masses at the same point and rewires springs (masses on mirrors for example)

// sends position updates for masses we re moving (and when the camera moves up - where selected masses move with the camera)
func (player *Player) moveSelected(masses []*mass.Mass) {
	moveDelta := player.spacePos.Sub(player.moveStart)

	//add any movement normal to the grid to the delta
	gridNormal := player.Grid.Xaxis.Cross(player.Grid.Yaxis).Normalise()
	camDGN := gridNormal.Multiply(player.Camera.Position.Sub(player.downCam.Position).Dot(gridNormal))
	moveDelta.AddIn(camDGN)

	for m := range player.selectedMasses {
		p, present := player.massStartPos[m]
		if present {
			m.P = p.Add(moveDelta)
		} else {
			log.Logit("no startpos present for mass ", m.Index)
		}

	}

	player.regenTransformed(masses)

	s := slices.Collect(maps.Keys(player.selectedMasses))
	if len(s) > 0 {
		player.sendMasses(s, false) //just send the new positions
	}

}

func (player *Player) Start(masses []*mass.Mass, things []*thing.Thing) {

	player.SendCamera()
	player.Send(player.Grid.AsMsg())

	player.currentThing = things[0]

	player.sendMasses(masses, true)
	player.sendThings(things)        //[]*thing.Thing{p.vehicle}) //sends mesh name and springs
	player.sendGameId(player.gameId) //game id starts it running
	player.SendLabelSets()
	player.vehicle = things[0]
	player.vehicle.SetVelocity(aero.TestFlight)
	player.sendVectors(masses)
	player.sendCentreOfMass(player.currentThing)

	player.Notify("Loaded", "info")
}

func (player *Player) SetBoundValue(key string, value float64, masses []*mass.Mass, things []*thing.Thing) {
	pointer := player.boundValues[key]
	*(*float64)(pointer) = value //cast to *float64 and then dereference/set value

	//TODO - optimise/reduce chatter
	player.sendVectors(masses) //send the new vectors

	player.sendMasses(masses, true) //send the potentially) modified mass
	if player.currentThing == nil {
		player.currentThing = things[0]
	}
	player.sendCentreOfMass(player.currentThing)

}

// regenTransformed updates any masses that are transforms of other masses (e.g. on the other side of a mirror)
func (player *Player) regenTransformed(masses []*mass.Mass) {
	//reflected := make(map[*mass.Mass]*mass.Mass)
	for _, m := range masses {

		//this could be another transform such as a rotation
		reflect := func(p *vec.V3) *vec.V3 {
			return p.ReflectInPlane(player.Grid.Origin, player.Grid.Normal())
		}
		m.RegenFromMaster(reflect)

	}

	player.sendMasses(masses, true)

}

// func (p *Player) moveSpringCursor() {
// 	moveDelta := p.gridPos.sub(p.moveStart)

// 	//add any movement normal to the grid to the delta
// 	gridNormal := p.grid.Xaxis.cross(p.grid.Yaxis).normalise()
// 	camDGN := gridNormal.multiply(p.camera.Position.sub(p.downCam.position).dot(gridNormal))
// 	moveDelta.addIn(camDGN)

// 	m := p.springCursor
// 	m.p = p.massStartPos[m].add(moveDelta)

// 	p.sendMasses([]*mass.Mass{m}, false) //just send the new positions

// }

// func (p *Player) sendBoundValues() {

// 	e := binary.LittleEndian
// 	buff := new(bytes.Buffer)
// 	binary.Write(buff, e, byte(msgBoundValues)) //masses
// 	binary.Write(buff, e, byte(len(p.boundValues)))
// 	for _, bv := range p.boundValues {
// 		bv.toByteBuffer(buff, e)
// 	}

// 	p.sendBytes(buff.Bytes())
// }

func (player *Player) SendCamera() {

	msg := msg.NewMsg(msg.Camera)
	player.Camera.WriteTo(msg)

	player.Send(msg)

}

// optimise - to send only changed labels
func (player *Player) SendLabels() {

	msg := msg.NewMsg(msg.Labels)
	msg.Write(uint16(len(player.labels)))
	for _, l := range player.labels {
		l.WriteTo(msg)

		// msg.WriteUint16(l.index)
		// msg.WriteByte(l.backgroundColor)
		// msg.WriteUint16(l.m1.Index)
		// msg.WriteUint16(l.m2.Index)
		// msg.WriteByte(l.voff)
		// msg.WriteString(l.text)
		//m.WriteFloat32(*l.boundTo)

	}
	player.Send(msg)
}

func (player *Player) sendInstancePositions(meshId uint16, positions []float32) {

	msg := msg.NewMsg(msg.PositionInstances)

	instanceCount := uint16((len(positions) - 1) / 3)
	msg.Write(meshId, uint16(0), instanceCount, byte(0), positions)

	player.Send(msg)

}

func (player *Player) sendMasses(masses []*mass.Mass, withDetail bool) {

	msg := mass.MassesAsMsg(masses, withDetail, player.selectedMasses)

	player.Send(msg)

}

func (player *Player) sendClear() {
	msg := msg.NewMsg(msg.Clear)
	player.Send(msg)
}

func (player *Player) velocity() float32 {

	return float32(player.vehicle.Springs[0].M1.GetVelocity().Length() * 150.0) //m/s
}

func (player *Player) climbRate() float32 {

	return float32(player.vehicle.Springs[0].M1.GetVelocity().Y * 150) //m/s

}

// turnRate is the rate of change of heading in degrees per minute
func (player *Player) turnRate() float32 {

	v := player.vehicle
	direction := v.Forward().Sub(v.Focus())
	newHeading := math.Atan2(direction.X, direction.Z) / (math.Pi * 2) * 360 //angle in degrees

	rate := float32((newHeading - player.heading) * 150 * 60.0) //degrees per minute
	player.heading = newHeading
	return rate

}

func (player *Player) sendTelemetry() {

	msg := msg.NewMsg(msg.Telemetry,
		uint16(4), //4 name value pairs
		"vel", player.velocity(),
		"climb", player.climbRate(),
		"turn", player.turnRate(),
		"head", float32(player.heading),
	)

	player.Send(msg)

}

func (player *Player) sendVectors(masses []*mass.Mass) {

	//send the mass index, vector and color - show lift at the wingtips (althoug it is actually shared between the three verts)

	msg := msg.NewMsg(msg.Vectors)

	orange := uint8(6)
	blue := uint8(1)
	red := uint8(4)

	magenta := uint8(5)

	//NEED PAUSED

	numVecs := 0
	for _, m := range masses {
		numVecs++
		if m.Axle != nil {
			numVecs++
		}
		if m.Lift != nil {
			numVecs++
			if m.Axle != nil {
				numVecs++
			}
		}
	}

	msg.Write(uint16(numVecs)) //number of vectors

	for _, m := range masses {
		m.WriteVectorsTo(msg, red, blue, orange, magenta)
	}

	player.Send(msg)

}

// used for sending section of the interleaved (often) floating point data that makes up vertex, normal, position and index buffers
// in a format very close to that need by the GPU (or three.js buffers)

func (player *Player) SendCursor() {

	msg := msg.NewMsg(msg.Cursor, player.cursor, player.gridPos, player.spacePos)

	if player.highlit.mass != nil {
		msg.Write(0) //cursor sphere radius
	} else {
		msg.Write(float32(0.05)) //cursor sphere radius
	}
	player.Send(msg)

}

func (player *Player) sendHighlit() {

	hm, ht, hs := int32(-1), int32(-1), int32(-1)
	if player.highlit.mass != nil {
		hm = player.highlit.mass.Index
	}
	if player.highlit.thing != nil {
		ht = int32(player.highlit.thing.Index)

	}
	if player.highlit.spring != nil {
		hs = player.highlit.spring.Index
	}

	msg := msg.NewMsg(msg.Highlit, hm, ht, hs)
	player.Send(msg)
}

func (player *Player) Notify(text string, severity string) {

	msg := msg.NewMsg(msg.Message, text, severity)
	player.Send(msg)
	log.Logit(msg, severity)

}

func playersToMsg(players map[uint32]*Player) msg.Msg {

	msg := msg.NewMsg(msg.Players, uint32(len(players)))

	for _, p := range players {
		p.WriteTo(msg)
	}

	return msg()
}

func (player *Player) recordMassPositions(masses []*mass.Mass) {
	player.massStartPos = make(map[*mass.Mass]*vec.V3) //reset each time
	for _, m := range masses {                         //selectedMasses {
		player.massStartPos[m] = m.P.Clone()
	}
}

func (p *Player) RunEngines() []*msg.Msg {
	msgs := []*msg.Msg{}
	if p.vehicle != nil {
		for _, engine := range p.vehicle.Engines {
			msgs = append(msgs, engine.Run()...) //Moves the masses (of the engines spring)
		}
	}
	return msgs
}

// MoveCamera moves the camera (and any selected masses) based on key presses
func (player *Player) MoveCamera(masses []*mass.Mass) {

	dir := vec.NewVec3(0, 0, 0)

	speed := .5
	if player.keys["Alt"] {
		speed = 10
	}

	if player.keys["w"] {
		dir.SetZ(speed)
	}
	if player.keys["s"] {
		dir.SetZ(-speed)
	}
	if player.keys["a"] {
		dir.SetX(-speed)
	}
	if player.keys["d"] && player.keys["Control"] == false {
		dir.SetX(speed)
	}
	if player.keys["ArrowUp"] {
		dir.SetY(speed)
	}
	if player.keys["ArrowDown"] {
		dir.SetY(-speed)
	}

	camDir := player.Camera.Direction

	right := camDir.Cross(player.Camera.Up).Normalise()
	up := right.Cross(camDir).Normalise()
	delta := right.Multiply(dir.X).Add(up.Multiply(dir.Y)).Add(camDir.Multiply(dir.Z))

	if delta.LengthSq() > 0 {
		player.Camera.Position.AddIn(delta)
		player.SendCamera()
		if player.mode == moving {
			player.moveSelected(masses)
		}
	}

}

func New(globalPlayers map[string]*Player, id uint32, name string, gameId string, socket *websocket.Conn, gridOrigin *vec.V3) *Player {

	gridX := vec.NewVec3(1, 0, 0)
	gridY := vec.NewVec3(0, 0, 1) //this is a bit confusing but the 2d grid is initialised on the word xz plane

	p := Player{

		Id:     id,
		Name:   name,
		GameId: gameId,

		gridPos: vec.NewVec3(0, 0, 0),

		highlit: highlitType{nil, nil, nil},
		//springStart:    nil,
		currentThing: nil,
		mode:         editing,

		Socket:         socket,
		Grid:           grid.New(gridOrigin, gridX, gridY),
		cursor:         vec.NewVec2(0, 0),
		grab:           nil,
		keys:           make(map[string]bool),     //which keys are pressed
		selectedMasses: make(map[*mass.Mass]bool), //which masses are selected, values are the order in which they were selected
		boundValues:    make(map[string]*float64, 0),
		//boundValues:    make(map[string]unsafe.Pointer, 0),
		mixers:   mixer.StandardMixers,
		controls: make(map[input.ControlInput]float64),
	}

	globalPlayers[name] = &p

	//create and set control inputs for all 'channels'
	for i, _ := range input.InLabels {
		p.controls[input.ControlInput(i)] = 0
	}

	direction := vec.NewVec3(0, -1, .1).Normalise()
	p.Camera = cam.New(vec.NewVec3(0, 50, -25), direction, vec.NewVec3(0, 1, 0))

	//p.qh = &qHolder{mutex: &sync.Mutex{}, q: make(map[int][]*reply, 0)} //initialise their outbound queue
	//p.waitChannel = make(chan bool)
	p.mtx = &sync.Mutex{}
	p.InMtx = &sync.Mutex{}

	return &p

}

func NewFromMsg(players map[string]*Player, m *msg.Msg, gameId string, things []*thing.Thing) *Player {

	le := binary.LittleEndian
	playerId := uint32(0)
	playerName := ""
	m.Read(&playerId, &playerName, &gameId)

	p := New(playerId, playerName, gameId, nil, vec.NewVec3(0, 0, 0))

	vidx := int32(0)
	binary.Read(buff, le, &vidx)
	if vidx > -1 {
		p.vehicle = things[vidx]
	}
	p.Camera.fromByteBuffer(buff, le)
	p.Grid.fromByteBuffer(buff, le)

	return p
}

func (player *Player) WriteTo(msg msg.Msg) {
	msg.Write(player.Id, //unique player ID (Uint32)
		player.Name,          //curent player name (may change)
		player.vehicle.index) //thing index of their current vehicle
	player.Camera.WriteTo(msg)
	binary.Grid.WriteTo(msg)

}

func (player *Player) setMode(mode ModeEnum) {
	player.mode = mode
	log.Logit("player", player.Name, "mode set to", mode)

	msg := msg.NewMsg(msg.Mode)
	msg.Write(mode)
	player.Send(msg)

}

func (player *Player) checkHighlitMass() bool {
	if player.highlit.mass == nil {
		player.Notify("Highlight a mass and press the key", "error")
		return false
	}
	return true

}

func PlayersFromBuff(buff *bytes.Buffer, gameId string, things []*thing.Thing) map[uint32]*Player {

	players := make(map[uint32]*Player)

	msg.NewFromBuff(buff, msg.Players)

	numPlayers := uint32(0)
	msg.Read(buff, &numPlayers)

	for i := 0; i < int(numPlayers); i++ {
		p := newFromBuff(buff, gameId, things)
		players[p.Id] = p
	}

	return players
}

func (player *Player) sendBytes(msg []byte) {

	if player.Socket == nil {
		log.Logit(player.Name + "player socket is disconnected")
		return
	}

	player.mtx.Lock()         //<<---MUTEX
	defer player.mtx.Unlock() //deferred unlock
	messageType := websocket.BinaryMessage

	//logit("sent", len(msg), " binary bytes")
	player.Socket.WriteMessage(messageType, msg) //write the message (and return any error)
}

// func (player *Player) send(msg *reply) { //this is fo JSON message s- Dperecated

// 	if player.Socket != nil {
// 		player.mtx.Lock()         //<<---MUTEX
// 		defer player.mtx.Unlock() //deferred unlock
// 		messageType := websocket.TextMessage

// 		bytes, err := json.Marshal(msg)
// 		if err != nil {
// 			logit(err.Error())
// 			return
// 		}

// 		player.Socket.WriteMessage(messageType, bytes) //write the message (and return any error)
// 	} else {
// 		logit("player socket is disco'd")
// 	}

// }

// Tidy Remove masses not attached to a spring
func (player *Player) Tidy(masses []*mass.Mass, things []*thing.Thing) {

	for {
		allGood := true
		for i, m := range masses {
			if !m.ReferencedByAnyOf(things) {
				m.Delete(masses)
				log.Logit("tidy - deleted mass", i, "of", len(masses))
				allGood = false
				break
			} else {
				log.Logit("tidy - kept mass", i, "of", len(masses))
			}
		}
		if allGood {
			break
		}
	}
}

func (player *Player) ProcessStructuredMsg(isRunning *bool, masses []*mass.Mass, things []*thing.Thing, msg *jsonmsg.Msg, ws *websocket.Conn, sounds []*sound.Sound, players []*Player) {

	//var fpn string //firstPlayer *Player

	dx, dy := float64(0), float64(0) //used for sliding skins
	prop := "offset"
	step := float64(3)

	if player == nil && msg.Cmd != "createGame" && msg.Cmd != "joinGame" {
		log.Logit("No player for message", msg)
	}

	//logit(msg.Cmd)

	switch msg.Cmd {
	case "keyUp":
		//a key was released
		player.keys[msg.Key] = false

		switch msg.Key {
		case "ArrowLeft", "ArrowRight":
			dx = 0
		case "ArrowUp", "ArrowDown":
			dy = 0
		}

	case "step":
		//<-player.waitChannel //see stepworlds fo rthe sending end which releases this

	case "drive":

		// if strings.HasPrefix(player.name, "control-") {
		// 	player = state.players[strings.TrimPrefix(player.name, "control-")]
		// }
		// player.leftDrive = msg.Payload[0]  //no need to echo them back - local versions are used for knobs only
		// player.rightDrive = msg.Payload[1] //no need to echo them back - local versions are used for knobs only
		// player.thrust = msg.Payload[2]

	case "mw": //mousewheel

		//player.Camera.Position.y += msg.Payload[0] * -0.01 //up and down

		player.Camera.Position.AddIn(player.Grid.Normal().Multiply(msg.Payload[0] * -0.005))
		player.zOff += msg.Payload[0] * -0.005

		player.processMouseMove(*isRunning, masses, things)
		player.SendCamera()

	case "mm": //mouse move

		player.movedSinceMouseDown = true

		player.buttons = byte(msg.Payload[0])
		player.Camera.FarPos = vec.NewVec3(msg.Payload[1], msg.Payload[2], msg.Payload[3])
		player.cursor.X = msg.Payload[4]
		player.cursor.Y = msg.Payload[5]

		player.processMouseMove(*isRunning, masses, things)

	case "mu":

		if player.buttons == 2 && player.movedSinceMouseDown == false {

			player.sendVectors() //send the new vectors

			if player.highlit.mass != nil {

				//degreesToRadians := float32(180.0) / float32(math.Pi)
				player.boundValues = make(map[string]*float64, 0)

				//p.bindValue("radius", &p.highlit.mass.r, 0.01, 1.00, .01, 0)
				player.bindValue("radius", &player.highlit.mass.R, 0.01, 1.00, .01, 0)
				player.bindValue("Section", &player.highlit.mass.Section, 0, 1, 1, 3)
				//p.bindValue("aoa", p.highlit.mass, &p.highlit.mass.aoaRads, -20, +20, 1, 0)
				player.bindValue("wingArea", &player.highlit.mass.WingArea, 0.1, 500.00, 1, 0)
				//p.bindValue("dihedral", p.highlit.mass, &p.highlit.mass.dihedralDegrees, -10, 10, 1, 0)
				//p.bindValue("controlSurface", p.highlit.mass, &p.highlit.mass.flightOutput, 0, 10, 1, 1) //use labelt set 1 (outoput flight controls)

				player.bindValue("massActuator", (*float64)(unsafe.Pointer(&player.highlit.mass.ActuatorTag)), 0, float64(len(actuator.MassActuators)), 1, 1) //use label set 1 (mass actuator labels)
				//p.sendBoundValues() //will pop up a context menu clientside
				player.setMode(props)
			} else if player.highlit.spring != nil {
				//p.boundValues = make(map[string]boundValue, 0)
				player.bindValue("springActuator", (*float64)(unsafe.Pointer(&player.highlit.spring.ActuatorTag)), 0, float64(len(actuator.SpringActuators)), 1, 2) //use label set 2 (spring actuator labels)
				//p.bindValue("springActuator", &p.highlit.spring.actuatorTag, 0, float64(len(springActuators)), 1, 1) //use label set 1 (actuator labels)
				//p.sendBoundValues()                                                                      //will pop up a context menu clientside
				player.setMode(props)
			}
		}

		player.buttons = byte(msg.Payload[0])

	case "md":

		player.buttons = byte(msg.Payload[0])

		player.movedSinceMouseDown = false

		player.grab = player.cursor.Clone()

		player.downGridPos = player.gridPos.Clone()

		player.downCam = player.Camera.Clone()

		if player.buttons == 1 {

			if player.highlit.mass != nil {
				player.moveStart = player.highlit.mass.P.Clone()
			} else {
				player.moveStart = player.spacePos.Clone() //may be snapped
			}

			player.recordMassPositions(masses)

			if player.mode == editing {
				//toggle selection of highlit mass
				phm := player.highlit.mass
				if phm != nil {
					psm := player.selectedMasses

					there := psm[phm]
					if there {
						delete(psm, phm)
					} else {
						psm[phm] = true
					}
					player.sendMasses([]*mass.Mass{phm}, true)
				}
			}

			if player.highlit.mass != nil {
				m := player.highlit.mass
				gridPlane := player.Grid.Plane()
				player.zOff = gridPlane.DistanceFrom(m.P)

				player.SendCursor()
			}

			if player.mode == startMove {

				player.setMode(moving)

			} else if player.mode == moving {
				player.setMode(editing)
			} else if player.mode == grabbingMesh {
				player.meshGrab = player.spacePos.Clone()
				player.setMode(offsettingMesh)
			} else if player.mode == offsettingMesh {
				delta := player.spacePos.Sub(player.meshGrab)
				//delta.x *= -1 //UGLY - but the scenes x axis is inverted
				player.currentThing.MeshOffset.AddIn(delta)
				player.sendThings([]*thing.Thing{player.currentThing})
				player.setMode(editing)

			} else if player.mode == adding {
				//we will make the spring between the highlit mass and a new mass
				//if there is no highlit mass, then we add one
				//on mouseup - we will collapse the new mass into any we are on top op
				//first (possibly highlit) mass
				player.makeNextSpring(masses)

				player.setMode(stretching)

			} else if player.mode == stretching {

				if player.highlit.mass != nil {
					log.Logit("substituting mass")
					player.highlit.spring.M2 = player.highlit.mass

					//remove it clientside
					player.springCursor.R = 0
					player.sendMasses([]*mass.Mass{player.springCursor}, true)

					masses = masses[:len(masses)] //delete the last mass

					player.sendThings([]*thing.Thing{player.currentThing}) //sends the new spring (once on mousedown)
				} else {
					player.highlit.mass = player.springCursor
				}

				player.makeNextSpring(masses)

			}

		}

		//player.send(&reply{Cmd: "gridPos", Payload: player.gridPos.payload()})

	case "keyDown":

		k := msg.Key
		kl := strings.ToLower(k)

		player.keys[k] = true

		log.Logit("key down", k)
		shift := msg.Payload[0]
		ctrl := msg.Payload[1]
		//alt := pl["alt"].(bool)

		if shift > 0 {
			prop = "scale"
			step = .1
		}
		if ctrl > 0 {
			prop = "rotation"
			step = .1
		}

		if k == "ArrowLeft" {
			dx = -step
			if *isRunning {
				player.controls[input.StickX] -= 0.05
			}

		} else if k == "ArrowRight" {
			dx = +step
			if *isRunning {
				player.controls[input.StickX] += 0.05
			}

		} else if k == "ArrowUp" {
			if *isRunning {
				player.controls[input.StickY] += 0.05
			}

			dy = +step
		} else if k == "ArrowDown" {
			if *isRunning {
				player.controls[input.StickY] -= 0.05
			}

			dy = -step //see the end of the if block for where the transform is send if dx or dy are set
		} else if kl == "t" {
			if player.currentThing == nil {
				player.currentThing = things[0]
			}
			player.setMode(adding)
		} else if kl == "y" {
			player.SnapMasses(things)
			player.Tidy(masses, things)
			player.sendClear()
			log.Logit("tidy")
			player.sendMasses(masses, true)
			player.sendThings(things)
		} else if kl == "-" {
			player.controls[input.Throttle] -= 0.05
		} else if kl == "+" {
			player.controls[input.Throttle] += 0.05
		} else if kl == "b" {

			// pens := make([]*vec3, 100)

			// count := int(0)
			// p.landTri.probe(p.camera.Position, p.camera.Farpos, pens, &count)

			// nearestPen := NewVec3(0, 0, 0)
			// sd := float64(1000000.0)
			// for _, pen := range pens {
			// 	if pen != nil {
			// 		if p.camera.Position.distanceFrom(pen) < sd {
			// 			nearestPen = pen
			// 		}
			// 	}
			// }

			// //grow a tree, offset it, and send it to player

		} else if kl == "e" {
			player.setMode(editing)
		} else if player.keys["Control"] && kl == "d" { //deselect all
			//deselect all masses
			player.selectedMasses = make(map[*mass.Mass]bool)
			player.sendMasses(masses, true)
		} else if player.keys["Control"] && kl == "a" { //select all
			//deselect all masses
			for _, m := range masses {
				player.selectedMasses[m] = true
			}
			player.sendMasses(masses, true)
		} else if k == "0" {
			player.zOff = 0
			player.processMouseMove()
			player.SendCamera()
			player.controls[input.Throttle] = 0
		} else if k == "1" {
			//start port engine			cg, _ := player.vehicle.CentreOfMass()

			SendToMany(players, player.vehicle.Engines[0].Start(sounds)...)

		} else if k == "2" {
			//start starboard engine
			SendToMany(players, player.vehicle.Engines[1].Start(sounds)...)

		} else if kl == "o" {

			if player.checkHighlitMass() {
				player.currentThing.Om = player.highlit.mass
				player.sendThings([]*thing.Thing{player.currentThing})
			}

		} else if kl == "f" { //set the forward direction mass
			if player.checkHighlitMass() {
				player.currentThing.Fm = player.highlit.mass
				player.sendThings([]*thing.Thing{player.currentThing})
			} else {
				player.follow = !player.follow
			}
		} else if kl == "r" {
			if player.keys["Control"] {
				//rotate thing 90 degrees more
				player.currentThing.MeshRotation.AddIn(player.currentThing.MeshRotation.Normalise().Multiply(math.Pi / 2))
			} else {
				//right mass  (x axis mass) of thing mesh
				if player.checkHighlitMass() {
					player.currentThing.Rm = player.highlit.mass
				}
			}
			player.sendThings([]*thing.Thing{player.currentThing})

		} else if kl == "g" { //align the grid
			if player.keys["Control"] {
				player.state.zeroG = !player.state.zeroG
			} else {

			}
		} else if kl == "m" && player.keys["Shift"] {
			if player.currentThing == nil {
				player.currentThing = things[0]
			}
			player.currentThing.Visibility = 1 - player.currentThing.Visibility
			player.sendThings([]*thing.Thing{player.currentThing})
		} else if kl == "m" && player.keys["Alt"] {
			player.setMode(grabbingMesh)

		} else if kl == "m" && player.keys["Control"] {
			//mirror the selected masses (in the grid)
			//more generally - we will add the selected masses to the current transformation
			//note - some masses will map the the same position (we will want to discard/reinstate them when hooking up springs)

			sm := maps.Keys(player.selectedMasses)
			transformed := make(map[*mass.Mass]*mass.Mass)

			for m := range sm {
				tp := m.P.ReflectInPlane(player.Grid.Origin, player.Grid.Normal())
				//is there already one at the transformed point?
				transformed[m] = mass.FindAt(masses, tp, 0.01) //some masses (those on the plane) will map to themselves
				if transformed[m] == nil {
					//nop, make a new mass
					transformed[m] = mass.New(masses, int32(len(masses)), tp, m.R, m.Fixed, m.IsCoin, m.Collideable, m) //add 'shadow' mass
				}
			}
			player.regenTransformed() //(re)mirror all transformed masses (in the grid plane)

			for k, m := range transformed {
				m.WingRoot = transformed[k.WingRoot]
				m.Axle = transformed[k.Axle]
				//v.aoaRads = k.aoaRads + math.Pi
				//v.dihedralDegrees = -k.dihedralDegrees
				m.WingArea = k.WingArea
				m.Flip = !k.Flip
			}

			//wire up the springs - once. We will need to remove transformed masses which map onto their own point of origin
			//note we are adding to the collection we are iterating over - but that it ok (in Go)
			//mirror the springs
			for _, s := range player.currentThing.Springs {
				//todo - if only one end is selected (and transformed) we should still create a spring
				tm1 := transformed[s.M1]
				tm2 := transformed[s.M2]
				if tm1 != nil && tm2 != nil {

					if tm1 == tm2 {
						log.Logit("spring to self")
					}
					player.currentThing.AddSpring(tm1, tm2, s.Collideable, actuator.NONE)
				}
			}
			player.sendThings([]*thing.Thing{player.currentThing})

		} else if k == "Escape" {
			if player.mode == stretching {
				player.springCursor.R = 0
				player.sendMasses([]*mass.Mass{player.springCursor}, true)
				masses = masses[:len(masses)-1] //delete the last mass
				player.currentThing.DeleteLastSpring()
				player.sendThings([]*thing.Thing{player.currentThing}) //one less spring
				player.setMode(adding)
			} else if player.mode == moving { // cancel a move
				for m := range player.selectedMasses {
					m.P = player.moveStart
				}

				s := slices.Collect(maps.Keys(player.selectedMasses))
				player.sendMasses(s, false)
				player.setMode(editing)
			} else if player.mode == props {
				player.boundValues = make(map[string]*float64, 0)
				player.clearContextMenu()
				player.setMode(editing)

			} else {

				//pressing escape to run
				player.SnapMasses(things)
				player.sendThings(things)
				//p.vehicle = p.currentThing

				player.sendVectors(masses)
				player.vehicle.BindMixers(player.mixers)
				//p.vehicle.setVelocity(testFlight)

				//p.state.setVelocity(NewVec3(0, 0, 1/float64(30*5)*50)) //100mph
				//p.labelLiftingMasses()

				*isRunning = !*isRunning
				log.Logit("running", *isRunning)
			}

		} else if kl == "m" {
			player.setMode(startMove)
			// } else if k == "x" { //define the axle/wing axis
			// 	selected:=slices.Collect(maps.Keys(player.selectedMasses))
			// 	if len(player.selectedMasses) == 1 && player.highlit.mass != nil && player.highlit.mass != selected[0] {
			// 		for m := range player.selectedMasses {
			// 			m.axle = player.highlit.mass
			// 		}
			// 		player.sendMessage("Wing axis/wheel axle defined", "info")
			// 	} else {
			// 		player.sendMessage("Select the wingtip/wheel hub mass, and higlight the axle mass when defining it", "error")
			// 	}
		} else if kl == "z" || kl == "x" { //define wing root/plane
			selectedMass := slices.Collect(maps.Keys(player.selectedMasses))[0]

			if player.highlit.mass != nil && len(player.selectedMasses) == 1 && player.highlit.mass != selectedMass {
				if k == "z" {
					m := selectedMass
					wr := player.highlit.mass
					m.WingRoot = wr
					span := m.P.Sub(m.Axle.P).Length()
					chord := m.Axle.P.Sub(m.WingRoot.P).Length()
					m.WingArea = span * chord
					player.vehicle.SetVelocity(aero.TestFlight)
					player.state.FlyMasses() //*pretend* we are flying at 20ms
					player.sendVectors()
					player.Notify("Wing root defined", "info")
				} else if k == "x" {
					if selectedMass.Axle == player.highlit.mass {
						selectedMass.Axle = nil //remove the axle
						player.Notify("Axle removed", "info")
					} else {
						selectedMass.Axle = player.highlit.mass
						player.Notify("Axis/Axle defined", "info")
					}
				}
				player.sendMasses([]*mass.Mass{selectedMass}, true)

			} else {
				player.sendMessage("Select one mass, and higlight another when setting axes", "error")
			}
		} else if kl == "l" {

			player.SnapMasses(things)
			if player.highlit.mass != nil {
				player.highlit.mass.Flip = !player.highlit.mass.Flip
				player.vehicle.SetVelocity(aero.TestFlight)
				player.state.FlyMasses()   //*pretend* we are flying at 20ms
				player.sendVectors(masses) //does a fake flyMasses()
				player.SendLabels()
			}
		} else if kl == "i" { //turin on AoA labels on wings, and brake force on brake masses, extension on spring actuators
			player.labels = make([]*label.Label, 0)

			for _, m := range masses {
				if m.WingRoot != nil {
					label.New(player.labels, "AOA", m, m, 1, 30, &m.AoaDegrees)
				}
				if m.ActuatorTag == actuator.LeftWheelBrake || m.ActuatorTag == actuator.RightWheelBrake {
					label.New(player.labels, "BRK", m, m, 5, 20, &m.Brake)
				}
			}
			if player.currentThing == nil {
				player.currentThing = things[0]
			}
			for _, s := range player.currentThing.Springs {
				if s.ActuatorTag > 0 {
					label.New(player.labels, actuator.SpringActuators[s.ActuatorTag], s.M1, s.M2, 5, 20, &s.Expansion)
				}
			}

			player.SendLabels()

		} else if kl == "p" {
			m := player.highlit.mass
			if m != nil {
				m.Fixed = !m.Fixed
				player.sendMasses([]*mass.Mass{m}, true)
			}

		} else if k == "Delete" {
			if player.highlit.mass == nil && player.highlit.spring != nil {
				player.highlit.thing.DeleteSpring(player.highlit.spring)
				player.sendThings([]*thing.Thing{player.highlit.thing})
			} else if player.highlit.mass != nil {
				if player.highlit.mass.NotAttached() {
					player.highlit.mass.R = 0
					player.sendMasses([]*mass.Mass{player.highlit.mass}, true)

					player.highlit.mass.Delete(masses) //less than straightforward
					player.sendMasses(masses, true)
					player.sendThings(things)
				}
			}
		}
	}

	//sends any transform of the skin (done with cursor keys and/or shift)
	if dx != 0 || dy != 0 {
		if player.currentThing != nil {
			ct := player.currentThing
			if prop == "rotation" {
				//ct.Rotation += dx
			} else { //if prop=="scale" {
				ct.MeshScale.X += dx
				ct.MeshScale.Y += dy
			}
			// - may be required in future (aliging tex/mesh/masses) p.sendThings([]*thing.Thing{ct}) //will need to send the offset, rotation and scale of the mesh/skin
		}
	}
}

func (player *Player) FollowVehicleWithCamera() {
	if player.follow && player.vehicle != nil {
		player.vehicle.FollowWith(player.Camera)
	}

}

func (player *Player) GetCamera() *cam.Camera {
	return player.Camera
}

func (player *Player) sendThings(things []*thing.Thing) {

	msg := msg.NewMsg(msg.Things, uint32(len(things))) //placeholder for number of things

	for _, thing := range things {
		thing.WriteTo(msg)
	}
	player.Send(msg)

}

func (player *Player) SnapMasses(things []*thing.Thing) {
	// where we have mirrored, or rotationally copied springs - collapse the coincident masses and rewire the springs

	for _, t := range things {
		t.Rewire()
	}

}

func (player *Player) makeNextSpring(masses []*mass.Mass) {
	if player.highlit.mass == nil {
		player.highlit.mass = mass.New(masses, int32(len(masses)), player.spacePos, .05, false, false, true, nil)
	}

	m1 := player.highlit.mass
	m2 := mass.New(masses, int32(len(masses)), m1.P.Clone().Add(vec.NewVec3(0, .001, 0)), 0.05, false, false, true, nil)

	player.springCursor = m2

	player.highlit.spring = player.currentThing.AddSpring(m1, m2, 1, actuator.NONE)
	log.Logit("made spring", m1.index, m2.index)

	//p.recordMassPositions() //we need to (re) do this as we have added masses

	// p.selectedMasses = make(map[*mass.Mass]bool)
	// p.selectedMasses[m2] = true
	player.SendMasses([]*mass.Mass{m1, m2}, false)
	player.SendThings([]*thing.Thing{player.currentThing})
	player.sendHighlit()

}

func SendToMany(players []*Player, msg ...*msg.Msg) {
	if len(msg) == 0 {
		return
	}
	for _, p := range players {
		p.Send(msg...)
	}
}
