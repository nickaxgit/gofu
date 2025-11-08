package device

import (
	"fmt"
	"maps"
	"math"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/gorilla/websocket"
	"github.com/nickax/gofu/cam"
	"github.com/nickax/gofu/errorplus"
	"github.com/nickax/gofu/fiz/mass"
	"github.com/nickax/gofu/fiz/spring"
	"github.com/nickax/gofu/fiz/thing"
	"github.com/nickax/gofu/game"
	"github.com/nickax/gofu/game/actuator"
	"github.com/nickax/gofu/game/aero"
	"github.com/nickax/gofu/game/grid"
	"github.com/nickax/gofu/game/input"
	"github.com/nickax/gofu/game/label"
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/game/player"

	"github.com/nickax/gofu/jsonmsg"
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/mesh"
	"github.com/nickax/gofu/next"
	"github.com/nickax/gofu/ray"

	"github.com/nickax/gofu/fiz/mixer"
	"github.com/nickax/gofu/mutex"
	"github.com/nickax/gofu/persist"
	"github.com/nickax/gofu/terrain"
	"github.com/nickax/gofu/vec"
)

type Device struct {
	Id        uint32
	Name      string
	token     string          //used to authenticate a device
	WebSocket *websocket.Conn //if this is nil, they are disconnected

	//viewingPlayerId int32
	ViewingPlayer     *player.Player
	isIndependent     bool           //this device has its own camera (otherwise it sees exaclty what the player sees)
	primaryControls   *player.Player //this is set to a target player when permission is granted, and set back to the owner if it is revoked, expires, or the controller leaves
	secondaryControls *player.Player //this is set to a target player when permission is granted, and set back to the owner if it is revoked, expires, or the controller leaves
	owner             *player.Player
	pov               string      //point of pilot, copilot, instruments, overhead panel, satellite etc
	Camera            *cam.Camera //initially a clone of viewPoint(within the vehicle) - Ongoing, additional position direction and up in vehicle space (our head swivel/slew)
	lastCam           *cam.Camera //where were we positioned/looking when we last sent an update

	mtx          *sync.Mutex // a mutex is required to 'lock' access to each users connection (for writing)
	InMtx        *sync.Mutex //inbound mutex, ensure only one command is processed at a time
	highlit      highlitType
	currentThing *thing.Thing
	mode         ModeEnum
	landTri      *terrain.Tri //terrain is generated JIT for each viewer
	Grid         *grid.Grid

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
	buttons             byte
	keys                map[string]bool
	boundValues         map[string]*float64 //boundValue //these form a popup dialog box (mass properties)
	movedSinceMouseDown bool
	labels              []*label.Label
	follow              bool                           //whether the camera follows the players vehicle
	controls            map[input.ControlInput]float64 //an array fon control channel inputs
	//each mixer (of the player) adds a contribution to to one mass (e.g. an aileron)
	mixers []*mixer.Mixer //scales the output of a control (channles 1-7) from (-1 to +1) to a mass/spring - see updateActuators()

}

// New id is either a known device id and correct token, OR -1
// which will create a device add it to the map - send ID/token back
func New(devices map[uint32]*Device, id uint32, name string,
	owner *player.Player, viewing *player.Player,
	primaryControls *player.Player, secondaryControls *player.Player,
	ws *websocket.Conn) *Device {
	//pov string, ws *websocket.Conn, gridOrigin *vec.V3, controls map[input.ControlInput]float64) *Viewer {

	gridOrigin := vec.NewVec3(0, 0, 0)
	gridX := vec.NewVec3(1, 0, 0)
	gridY := vec.NewVec3(0, 0, 1) //this is a bit confusing but the 2d grid is initialised on the word xz plane

	device := &Device{
		//viewingPlayerId: -1,
		Id:                id,
		Name:              name,
		owner:             owner,
		ViewingPlayer:     viewing, //can be nil at the very begining -
		primaryControls:   primaryControls,
		secondaryControls: secondaryControls,
		token:             fmt.Sprintf("%06d", rand.Int31n(999999)),
		WebSocket:         ws,
		pov:               "none",
		Camera:            cam.New(vec.NewVec3(0, 0, 0), vec.NewVec3(1, 0, 0), vec.NewVec3(0, 1, 0)), //default camera if none found
		gridPos:           vec.NewVec3(0, 0, 0),

		highlit: highlitType{nil, nil, nil},
		//springStart:    nil,
		currentThing: nil,
		mode:         editing,

		//Socket:         socket,
		Grid:           grid.New(gridOrigin, gridX, gridY),
		cursor:         vec.NewVec2(0, 0),
		grab:           nil,
		keys:           make(map[string]bool),     //which keys are pressed
		selectedMasses: make(map[*mass.Mass]bool), //which masses are selected, values are the order in which they were selected
		boundValues:    make(map[string]*float64, 0),
		controls:       make(map[input.ControlInput]float64),
		mtx:            &sync.Mutex{},
		InMtx:          &sync.Mutex{},
		mixers:         mixer.StandardMixers,
	}

	//create and set control inputs for all 'channels'
	for i := range input.InLabels {
		device.controls[input.ControlInput(i)] = 0
	}

	mutex.Devices.Lock()
	devices[uint32(device.Id)] = device
	mutex.Devices.Unlock()

	return device
}

func (device *Device) Persist() *errorplus.Event {

	deviceMsg := msg.NewMsg(msg.P_Device)
	device.WriteTo(deviceMsg)
	return persist.Append("repo.bin", deviceMsg)
}

func NewFromMsg(m *msg.Msg, devices map[uint32]*Device, players map[uint32]*player.Player) *Device {

	did, name, token, oid, vpid, pcid, scid, ii, pov := uint32(0), "", "", uint32(0), uint32(0), uint32(0), uint32(0), true, ""
	m.Read(&did, &name, &token, &oid, &vpid, &pcid, &scid, &ii, &pov)
	return New(devices, did, name, players[oid], players[vpid], players[pcid], players[scid], nil)

}

func (d *Device) WriteTo(m *msg.Msg) {
	m.Write(d.Id, d.Name, d.token, d.owner.Id, d.ViewingPlayer.Id,
		d.primaryControls.Id, d.secondaryControls.Id, d.isIndependent,
		d.pov)
}

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

func (device *Device) GetPlayer() *player.Player {
	return device.ViewingPlayer
}

func (device *Device) Status() string {
	if device.WebSocket != nil {
		return "Connected"
	}
	return "Disconnected"
}

// Watch a players vehicle from a given pov (defined within that vehicle)
func (device *Device) Watch(player *player.Player, pov string) {

	v := player.GetVehicle()
	if v != nil {
		cam := v.FindCam(pov)
		if cam != nil {
			device.Camera = cam
			device.pov = pov //the vehicle.cameras[pov] can be used to find the 'home' position
		}
	}

}

func (device *Device) clearContextMenu() {
	//clear the context menu (on the client)
	m := msg.NewMsg(msg.ClearContextMenu)
	device.Send(m)

}

// collects and sends the flames visible to this viewer
func (device *Device) GetFlames(fire *terrain.TriMesh, message *msg.Msg) {

	tcs := mesh.NewTcs(0, 1, 1, 0)                    //texture atlas coordinates
	flameMesh := mesh.New(201, "flame", 10000, 30000) //10k faces, 30k verts

	fire.Root.GetFlames(device.landTri, fire, flameMesh, device.Camera, tcs)

	flameMesh.WriteTo(message, 1)

}

func (device *Device) SendCamera() {

	msg := msg.NewMsg(msg.Camera)
	device.Camera.WriteTo(msg)
	device.Send(msg)

}

func (device *Device) GetLandRoot() *terrain.Tri {
	return device.landTri
}

func (device *Device) processMouseMove(game *game.Game) { //isRunning bool, masses []*mass.Mass, things []*thing.Thing) {

	if !game.Running {

		//a point on the far plane (where the mouse cursor is pointing)

		if device.buttons == 2 { //panning camera

			delta := (device.cursor.Sub(device.grab)).Mul(2)

			if delta.LengthSq() != 0 {

				//logit("delta", delta.x, delta.y)

				camRight := device.downCam.Direction.Cross(device.downCam.Up).Normalise()

				device.Camera.Up = device.downCam.Up.RotateAbout(camRight, delta.Y).Normalise()
				pitched := device.downCam.Direction.RotateAbout(camRight, delta.Y)
				yawed := pitched.RotateAbout(device.Camera.Up, -delta.X)

				device.Camera.Direction = yawed
				device.Camera.Up = vec.NewVec3(0, 1, 0) //auto level the camera

				device.SendCamera()
			}
			return
		}

		switch device.mode {

		case editing:

			if device.buttons == 0 {
				pickRay := ray.New(device.Camera.Position, device.Camera.FarPos)

				cm, _ := mass.ClosestMassToRay(game.Masses, device.springCursor, pickRay)

				if cm != device.highlit.mass {
					device.highlit.mass = cm
					device.sendHighlit() //might be nil
				}
			}

			//mutates the viewers gridPos and spacePos (by reference)
			device.Grid.UpdateGridPosAndSpacePos(device.Camera, device.zOff, device.gridPos, device.spacePos)

			pickRay := ray.New(device.Camera.Position, device.Camera.FarPos)

			closest := math.MaxFloat64
			for _, thing := range game.Things {
				//spring, d := pickRay.ClosestSpring(thing.Springs)
				spring, d := spring.ClosestSpringToRay(thing.Springs, pickRay)

				if d < closest {
					device.highlit.thing = thing
					device.highlit.spring = spring
					closest = d
				}
			}

			device.sendHighlit()

		case moving:
			device.moveSelected(game)
		case stretching:
			device.springCursor.P = device.spacePos.Clone() //moveSpringCursor()
			device.sendMasses([]*mass.Mass{device.springCursor}, false)
		}

		if device.buttons == 1 && device.mode == editing {
			//dragging/panning the camera
			if device.downGridPos != nil {
				delta := device.gridPos.Sub(device.downGridPos).Multiply(.9)
				device.Camera.Position = device.downCam.Position.Sub(delta)
				device.SendCamera()
			}
		}

		device.SendCursor()
	}
}

func (device *Device) sendBytes(msg []byte) {

	if device.WebSocket == nil {
		log.Logit(device.pov + " viewer socket is disconnected")
		return
	}

	device.mtx.Lock()         //<<---MUTEX
	defer device.mtx.Unlock() //deferred unlock
	messageType := websocket.BinaryMessage

	//logit("sent", len(msg), " binary bytes")
	device.WebSocket.WriteMessage(messageType, msg) //write the message (and return any error)
}

func (device *Device) ViewChangedSignificantly() bool {
	if device.lastCam == nil {
		return true
	}

	dist := device.Camera.Position.DistanceFrom(device.lastCam.Position)
	dir := device.Camera.Direction.Dot(device.lastCam.Direction)
	if dist > 100 || dir < .95 {
		device.lastCam = device.Camera.Clone() //store this as the new old position
		return true
	}
	return false
}

func (device *Device) Send(msg ...*msg.Msg) {
	for _, msg := range msg {
		device.sendBytes(msg.AllBytes())
	}

}

// []**gameId uint32, masses []*mass.Mass, things []*thing.Thing, selectedMasses map[*mass.Mass]bool, grid *grid.Grid) {

func (device *Device) SendLabels() {

	msg := msg.NewMsg(msg.Labels)
	msg.Write(uint16(len(device.labels)))
	for _, l := range device.labels {
		l.WriteTo(msg)
	}
	device.Send(msg)
}

func (device *Device) sendThings(things []*thing.Thing) {
	msg := thing.ThingsAsMsg(things)
	device.Send(msg)
}

func (device *Device) sendMasses(masses []*mass.Mass, withDetail bool) {

	msg := mass.MassesAsMsg(masses, withDetail, device.selectedMasses)

	device.Send(msg)

}

func (device *Device) sendClear() {
	msg := msg.NewMsg(msg.Clear)
	device.Send(msg)
}

func (device *Device) SendLabelSets() { //For options on the sliders

	sendLabelSet(device, 1, actuator.MassActuators)
	sendLabelSet(device, 2, actuator.SpringActuators)
	sendLabelSet(device, 3, aero.SectionNames)
}

func sendLabelSet[E actuator.ActuatorEnum | aero.Section](p *Device, idx byte, valueLabelPairs map[E]string) {

	m := msg.NewMsg(msg.LabelSet)
	m.Write(idx, byte(len(valueLabelPairs))) //index of the label set/ count of labels

	for value, label := range valueLabelPairs {
		m.Write(int32(value), label)
	}
	p.Send(m)

}

func (device *Device) sendControlPin() uint32 {

	m := msg.NewMsg(msg.ControlToken)
	token := randomPin()

	m.Write(token)
	device.Send(m)

	return token

}

func randomPin() uint32 { //TODO - Check for existing token
	return uint32(math.Round(1000 + rand.Float64()*8999))
}

// SendGameId - causes the client to start the game
func (device *Device) sendGameId(gameId uint32) {
	m := msg.NewMsg(msg.GameId)
	m.Write(gameId)
	device.Send(m)
}

func (device *Device) sendCentreOfMass(t *thing.Thing) {

	cg, weight := t.CentreOfMass()
	device.Send(msg.NewMsg(msg.CentreOfMass, t.Index, cg, float32(weight)))

}

func (device *Device) moveSelected(game *game.Game) {
	moveDelta := device.spacePos.Sub(device.moveStart)

	//add any movement normal to the grid to the delta
	gridNormal := device.Grid.Xaxis.Cross(device.Grid.Yaxis).Normalise()
	camDGN := gridNormal.Multiply(device.Camera.Position.Sub(device.downCam.Position).Dot(gridNormal))
	moveDelta.AddIn(camDGN)

	for m := range device.selectedMasses {
		p, present := device.massStartPos[m]
		if present {
			m.P = p.Add(moveDelta)
		} else {
			log.Logit("no startpos present for mass ", m.Index)
		}

	}

	device.regenTransformed(game.Masses)

	s := slices.Collect(maps.Keys(device.selectedMasses))
	if len(s) > 0 {
		device.sendMasses(s, false) //just send the new positions (not details)
	}

}

func (device *Device) SetBoundValue(key string, value float64, masses []*mass.Mass, things []*thing.Thing) {
	pointer := device.boundValues[key]
	*(*float64)(pointer) = value //cast to *float64 and then dereference/set value

	//TODO - optimise/reduce chatter
	device.Send(mass.VectorsAsMsg(masses)) //send the new vectors

	device.sendMasses(masses, true) //send the potentially) modified mass
	if device.currentThing == nil {
		device.currentThing = things[0]
	}
	device.sendCentreOfMass(device.currentThing)

}

// regenTransformed updates any masses that are transforms of other masses (e.g. on the other side of a mirror)
func (device *Device) regenTransformed(masses []*mass.Mass) {
	//reflected := make(map[*mass.Mass]*mass.Mass)
	for _, m := range masses {

		//this could be another transform such as a rotation
		reflect := func(p *vec.V3) *vec.V3 {
			return p.ReflectInPlane(device.Grid.Origin, device.Grid.Normal())
		}
		m.RegenFromMaster(reflect)

	}

	device.sendMasses(masses, true)

}

// used for sending section of the interleaved (often) floating point data that makes up vertex, normal, position and index buffers
// in a format very close to that need by the GPU (or three.js buffers)

func (device *Device) SendCursor() {

	msg := msg.NewMsg(msg.Cursor, device.cursor, device.gridPos, device.spacePos)

	if device.highlit.mass != nil {
		msg.Write(0) //cursor sphere radius
	} else {
		msg.Write(float32(0.05)) //cursor sphere radius
	}
	device.Send(msg)

}

func (device *Device) sendHighlit() {

	hm, ht, hs := int32(-1), int32(-1), int32(-1)
	if device.highlit.mass != nil {
		hm = device.highlit.mass.Index
	}
	if device.highlit.thing != nil {
		ht = int32(device.highlit.thing.Index)

	}
	if device.highlit.spring != nil {
		hs = device.highlit.spring.Index
	}

	msg := msg.NewMsg(msg.Highlit, hm, ht, hs)
	device.Send(msg)
}

func (device *Device) Notify(text string, severity string) {

	msg := msg.NewMsg(msg.Notify, text, severity)
	device.Send(msg)
	log.Logit(text, severity)

}

func (device *Device) recordMassPositions(masses []*mass.Mass) {
	device.massStartPos = make(map[*mass.Mass]*vec.V3) //reset each time
	for _, m := range masses {                         //selectedMasses {
		device.massStartPos[m] = m.P.Clone()
	}
}

// MoveCamera moves the camera (and any selected masses) based on key presses
func (device *Device) MoveCamera(game *game.Game) {

	dir := vec.NewVec3(0, 0, 0)

	speed := .5
	if device.keys["Alt"] {
		speed = 10
	}

	if device.keys["w"] {
		dir.SetZ(speed)
	}
	if device.keys["s"] {
		dir.SetZ(-speed)
	}
	if device.keys["a"] {
		dir.SetX(-speed)
	}
	if device.keys["d"] && !device.keys["Control"] {
		dir.SetX(speed)
	}
	if device.keys["ArrowUp"] {
		dir.SetY(speed)
	}
	if device.keys["ArrowDown"] {
		dir.SetY(-speed)
	}

	camDir := device.Camera.Direction

	right := camDir.Cross(device.Camera.Up).Normalise()
	up := right.Cross(camDir).Normalise()
	delta := right.Multiply(dir.X).Add(up.Multiply(dir.Y)).Add(camDir.Multiply(dir.Z))

	if delta.LengthSq() > 0 {
		device.Camera.Position.AddIn(delta)
		device.SendCamera()
		if device.mode == moving {
			device.moveSelected(game)
		}
	}

}

func (device *Device) setMode(mode ModeEnum) {
	device.mode = mode
	log.Logit("viewers mode set to", mode)

	msg := msg.NewMsg(msg.Mode)
	msg.Write(mode)
	device.Send(msg)

}

func (device *Device) checkHighlitMass() bool {
	if device.highlit.mass == nil {
		device.Notify("Highlight a mass and press the key", "red")
		return false
	}
	return true

}

// Tidy Remove masses not attached to a spring
func (device *Device) Tidy(game *game.Game) {

	thingList := thing.ThingList(game.Things) //allows us to define methods on a slice of things
	for {
		allGood := true
		for i, m := range game.Masses {
			if !thingList.References(m) {
				m.Delete(game.Masses)
				log.Logit("tidy - deleted mass", i, "of", len(game.Masses))
				allGood = false
				break
			} else {
				log.Logit("tidy - kept mass", i, "of", len(game.Masses))
			}
		}
		if allGood {
			break
		}
	}
}

// Processes keystrokes etc.
// Returns a compound message to be broadcast to all devices viewing the game (engine startup sounds)
func (device *Device) ProcessStructuredMsg(ibm *jsonmsg.Msg, response *msg.Msg) *errorplus.Event {

	if device == nil {
		return nil
	}
	if device.ViewingPlayer == nil {
		return nil
	}
	if device.ViewingPlayer.Game == nil {
		return nil
	}

	if device == nil {
		return errorplus.New(nil, errorplus.Warn, "structuredmessage (probably a keystroke) from disconnected device "+ibm.Cmd)
	}
	if device.ViewingPlayer == nil {
		return errorplus.New(nil,
			errorplus.Warn, "device has no viewing player: "+ibm.Cmd)
	}
	game := device.ViewingPlayer.Game

	dx, dy := float64(0), float64(0) //used for sliding skins
	prop := "offset"
	step := float64(3)

	//logit(msg.Cmd)

	switch ibm.Cmd {
	case "keyUp":
		//a key was released
		device.keys[ibm.Key] = false

		switch ibm.Key {
		case "ArrowLeft", "ArrowRight":
			dx = 0
		case "ArrowUp", "ArrowDown":
			dy = 0
		}

	case "mw": //mousewheel

		device.Camera.Position.AddIn(device.Grid.Normal().Multiply(ibm.Payload[0] * -0.005))
		device.zOff += ibm.Payload[0] * -0.005

		device.processMouseMove(game) //*isRunning, masses, things)
		device.SendCamera()

	case "mm": //mouse move

		device.movedSinceMouseDown = true

		device.buttons = byte(ibm.Payload[0])
		device.Camera.FarPos = vec.NewVec3(ibm.Payload[1], ibm.Payload[2], ibm.Payload[3])
		device.cursor.X = ibm.Payload[4]
		device.cursor.Y = ibm.Payload[5]

		device.processMouseMove(game) //*isRunning, masses, things)

	case "mu":

		if device.buttons == 2 && !device.movedSinceMouseDown {

			device.Send(mass.VectorsAsMsg(game.Masses)) //send the new vectors

			if device.highlit.mass != nil {

				//degreesToRadians := float32(180.0) / float32(math.Pi)
				device.boundValues = make(map[string]*float64, 0)

				//p.bindValue("radius", &p.highlit.mass.r, 0.01, 1.00, .01, 0)
				device.bindValue("radius", &device.highlit.mass.R, 0.01, 1.00, .01, 0)
				device.bindValue("Section", &device.highlit.mass.Section, 0, 1, 1, 3)
				//p.bindValue("aoa", p.highlit.mass, &p.highlit.mass.aoaRads, -20, +20, 1, 0)
				device.bindValue("wingArea", &device.highlit.mass.WingArea, 0.1, 500.00, 1, 0)
				//p.bindValue("dihedral", p.highlit.mass, &p.highlit.mass.dihedralDegrees, -10, 10, 1, 0)
				//p.bindValue("controlSurface", p.highlit.mass, &p.highlit.mass.flightOutput, 0, 10, 1, 1) //use labelt set 1 (outoput flight controls)

				device.bindValue("massActuator", (*float64)(unsafe.Pointer(&device.highlit.mass.ActuatorTag)), 0, float64(len(actuator.MassActuators)), 1, 1) //use label set 1 (mass actuator labels)
				//p.sendBoundValues() //will pop up a context menu clientside
				device.setMode(props)
			} else if device.highlit.spring != nil {
				//p.boundValues = make(map[string]boundValue, 0)
				device.bindValue("springActuator", (*float64)(unsafe.Pointer(&device.highlit.spring.ActuatorTag)), 0, float64(len(actuator.SpringActuators)), 1, 2) //use label set 2 (spring actuator labels)
				//p.bindValue("springActuator", &p.highlit.spring.actuatorTag, 0, float64(len(springActuators)), 1, 1) //use label set 1 (actuator labels)
				//p.sendBoundValues()                                                                      //will pop up a context menu clientside
				device.setMode(props)
			}
		}

		device.buttons = byte(ibm.Payload[0])

	case "md":

		device.buttons = byte(ibm.Payload[0])

		device.movedSinceMouseDown = false

		device.grab = device.cursor.Clone()

		device.downGridPos = device.gridPos.Clone()

		device.downCam = device.Camera.Clone()

		if device.buttons == 1 {

			if device.highlit.mass != nil {
				device.moveStart = device.highlit.mass.P.Clone()
			} else {
				device.moveStart = device.spacePos.Clone() //may be snapped
			}

			device.recordMassPositions(game.Masses)

			if device.mode == editing {
				//toggle selection of highlit mass
				phm := device.highlit.mass
				if phm != nil {
					psm := device.selectedMasses

					there := psm[phm]
					if there {
						delete(psm, phm)
					} else {
						psm[phm] = true
					}
					device.sendMasses([]*mass.Mass{phm}, true)
				}
			}

			if device.highlit.mass != nil {
				m := device.highlit.mass
				gridPlane := device.Grid.Plane()
				device.zOff = gridPlane.DistanceFrom(m.P)

				device.SendCursor()
			}

			switch device.mode {
			case startMove:
				device.setMode(moving)
			case moving:
				device.setMode(editing)
			case grabbingMesh:

				device.meshGrab = device.spacePos.Clone()
				device.setMode(offsettingMesh)
			case offsettingMesh:
				delta := device.spacePos.Sub(device.meshGrab)
				//delta.x *= -1 //UGLY - but the scenes x axis is inverted
				device.currentThing.MeshOffset.AddIn(delta)
				device.sendThings([]*thing.Thing{device.currentThing})
				device.setMode(editing)

			case adding:
				//we will make the spring between the highlit mass and a new mass
				//if there is no highlit mass, then we add one
				//on mouseup - we will collapse the new mass into any we are on top op
				//first (possibly highlit) mass
				device.makeNextSpring(game.Masses)

				device.setMode(stretching)

			case stretching:

				if device.highlit.mass != nil {
					log.Logit("substituting mass")
					device.highlit.spring.M2 = device.highlit.mass

					//remove it clientside
					device.springCursor.R = 0
					device.sendMasses([]*mass.Mass{device.springCursor}, true)

					game.Masses = game.Masses[:len(game.Masses)] //delete the last mass

					device.sendThings([]*thing.Thing{device.currentThing}) //sends the new spring (once on mousedown)
				} else {
					device.highlit.mass = device.springCursor
				}

				device.makeNextSpring(game.Masses)
			default:
				return errorplus.New(nil, errorplus.Error, "Unknown mode on mouse down: "+string(device.mode))
			}

		}

		//viewer.send(&reply{Cmd: "gridPos", Payload: viewer.gridPos.payload()})

	case "keyDown":

		k := ibm.Key
		kl := strings.ToLower(k)
		device.keys[k] = true

		log.Logit("key down", k)
		shift := ibm.Payload[0]
		ctrl := ibm.Payload[1]

		if shift > 0 {
			prop = "scale"
			step = .1
		}
		if ctrl > 0 {
			prop = "rotation"
			step = .1
		}

		switch k {
		case "ArrowLeft":
			dx = -step
			if game.Running {
				device.controls[input.StickX] -= 0.05
			}
		case "ArrowRight":
			dx = +step
			if game.Running {
				device.controls[input.StickX] += 0.05
			}

		case "ArrowUp":
			if game.Running {
				device.controls[input.StickY] += 0.05
			}

			dy = +step
		case "ArrowDown":
			if game.Running {
				device.controls[input.StickY] -= 0.05
			}

			dy = -step //see the end of the if block for where the transform is send if dx or dy are set

		}

		switch kl {
		case "t":
			if device.currentThing == nil {
				device.currentThing = game.Things[0]
			}
			device.setMode(adding)
		case "y": //Tidy - permanenty snaps reflection halves together and removes unreferenced masses
			device.SnapMasses(game.Things)
			device.Tidy(game)
			device.sendClear()
			log.Logit("tidy")
			device.sendMasses(game.Masses, true)
			device.sendThings(game.Things)
		case "-":
			device.controls[input.Throttle] -= 0.05
		case "+":
			device.controls[input.Throttle] += 0.05
		case "b":
			//grow a bush

		case "e":
			device.setMode(editing)
		}

		if device.keys["Control"] && kl == "d" { //deselect all
			//deselect all masses
			device.selectedMasses = make(map[*mass.Mass]bool)
			device.sendMasses(game.Masses, true)
		} else if device.keys["Control"] && kl == "a" { //select all
			//deselect all masses
			for _, m := range game.Masses {
				device.selectedMasses[m] = true
			}
			device.sendMasses(game.Masses, true)
		} else if k == "0" { //reset z offset (from the grid)
			device.zOff = 0
			device.processMouseMove(game)
			device.SendCamera()
			device.controls[input.Throttle] = 0
		} else if k == "1" {
			//start port engine
			return device.startEngine(0, game, response)
		} else if k == "2" {
			//start starboard engine
			return device.startEngine(1, game, response)

		} else if kl == "o" {

			if device.checkHighlitMass() {
				device.currentThing.Om = device.highlit.mass
				device.sendThings([]*thing.Thing{device.currentThing})
			}

		} else if kl == "f" { //set the forward direction mass
			if device.checkHighlitMass() {
				device.currentThing.Fm = device.highlit.mass
				device.sendThings([]*thing.Thing{device.currentThing})
			} else {
				device.follow = !device.follow
			}
		} else if kl == "r" {
			if device.keys["Control"] {
				//rotate thing 90 degrees more
				device.currentThing.MeshRotation.AddIn(device.currentThing.MeshRotation.Normalise().Multiply(math.Pi / 2))
			} else {
				//right mass  (x axis mass) of thing mesh
				if device.checkHighlitMass() {
					device.currentThing.Rm = device.highlit.mass
				}
			}
			device.sendThings([]*thing.Thing{device.currentThing})

		} else if kl == "g" { //align the grid
			if device.keys["Control"] { //CTRL-G - toggle gravity
				game.ZeroG = !game.ZeroG
			} else {

			}
		} else if kl == "m" && device.keys["Shift"] {
			if device.currentThing == nil {
				device.currentThing = game.Things[0]
			}
			device.currentThing.MeshVisibility = 1 - device.currentThing.MeshVisibility
			device.sendThings([]*thing.Thing{device.currentThing})
		} else if kl == "m" && device.keys["Alt"] {
			device.setMode(grabbingMesh)

		} else if kl == "m" && device.keys["Control"] {
			//mirror the selected masses (in the grid)
			//more generally - we will add the selected masses to the current transformation
			//note - some masses will map the the same position (we will want to discard/reinstate them when hooking up springs)

			sm := maps.Keys(device.selectedMasses)
			transformed := make(map[*mass.Mass]*mass.Mass)

			for m := range sm {
				tp := m.P.ReflectInPlane(device.Grid.Origin, device.Grid.Normal())
				//is there already one at the transformed point?
				transformed[m] = mass.FindAt(game.Masses, tp, 0.01) //some masses (those on the plane) will map to themselves
				if transformed[m] == nil {
					//nop, make a new mass
					transformed[m] = mass.New(game.Masses, int32(len(game.Masses)), tp, m.R, m.Fixed, m.IsCoin, m.Collideable, m) //add 'shadow' mass
				}
			}
			device.regenTransformed(game.Masses) //(re)mirror all transformed masses (in the grid plane)

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
			for _, s := range device.currentThing.Springs {
				//todo - if only one end is selected (and transformed) we should still create a spring
				tm1 := transformed[s.M1]
				tm2 := transformed[s.M2]
				if tm1 != nil && tm2 != nil {

					if tm1 == tm2 {
						log.Logit("spring to self")
					}
					device.currentThing.AddSpring(tm1, tm2, s.RestLength, s.Collideable, actuator.NONE)

				}
			}
			device.sendThings([]*thing.Thing{device.currentThing})

		} else if k == "Escape" {
			if device.mode == stretching {
				device.springCursor.R = 0
				device.sendMasses([]*mass.Mass{device.springCursor}, true)

				game.DeleteLastMass()
				device.currentThing.DeleteLastSpring()
				device.sendThings([]*thing.Thing{device.currentThing}) //one less spring
				device.setMode(adding)
			} else if device.mode == moving { // cancel a move
				for m := range device.selectedMasses {
					m.P = device.moveStart
				}

				s := slices.Collect(maps.Keys(device.selectedMasses))
				device.sendMasses(s, false)
				device.setMode(editing)
			} else if device.mode == props {
				device.boundValues = make(map[string]*float64, 0)
				device.clearContextMenu()
				device.setMode(editing)

			} else {

				//pressing escape to run
				device.SnapMasses(game.Things)
				device.sendThings(game.Things)

				device.BindMixers(device.ViewingPlayer.GetVehicle())

				game.Running = !game.Running
				log.Logit("running", game.Running)
			}

		} else if kl == "m" {
			device.setMode(startMove)
			// } else if k == "x" { //define the axle/wing axis
			// 	selected:=slices.Collect(maps.Keys(viewer.selectedMasses))
			// 	if len(viewer.selectedMasses) == 1 && viewer.highlit.mass != nil && viewer.highlit.mass != selected[0] {
			// 		for m := range viewer.selectedMasses {
			// 			m.axle = viewer.highlit.mass
			// 		}
			// 		viewer.sendMessage("Wing axis/wheel axle defined", "info")
			// 	} else {
			// 		viewer.sendMessage("Select the wingtip/wheel hub mass, and higlight the axle mass when defining it", "error")
			// 	}
		} else if kl == "z" || kl == "x" { //define wing root/plane
			selectedMass := slices.Collect(maps.Keys(device.selectedMasses))[0]

			if device.highlit.mass != nil && len(device.selectedMasses) == 1 && device.highlit.mass != selectedMass {
				if k == "z" {
					m := selectedMass
					wr := device.highlit.mass
					m.WingRoot = wr
					span := m.P.Sub(m.Axle.P).Length()
					chord := m.Axle.P.Sub(m.WingRoot.P).Length()
					m.WingArea = span * chord

					if device.currentThing != nil {
						device.currentThing.SetVelocity(aero.TestFlight)
						game.FlyMasses() //*pretend* we are flying at 20ms
						device.SendVectors(game)
					}

					device.Notify("Wing root defined", "green")
				} else if k == "x" {
					if selectedMass.Axle == device.highlit.mass {
						selectedMass.Axle = nil //remove the axle
						device.Notify("Axle removed", "green")
					} else {
						selectedMass.Axle = device.highlit.mass
						device.Notify("Axis/Axle defined", "green")
					}
				}
				device.sendMasses([]*mass.Mass{selectedMass}, true)

			} else {
				device.Notify("Select one mass, and higlight another when setting axes", "red")
			}
		} else if kl == "l" { //flip the lift direction of the highlit mass (wing)

			device.SnapMasses(game.Things)
			if device.highlit.mass != nil {
				device.highlit.mass.Flip = !device.highlit.mass.Flip
				device.currentThing.SetVelocity(aero.TestFlight)
				game.FlyMasses() //*pretend* we are flying at 20ms
				device.SendLabels()
			}
		} else if kl == "i" { //turin on AoA labels on wings, and brake force on brake masses, extension on spring actuators
			device.labels = make([]*label.Label, 0)

			for _, m := range game.Masses {
				if m.WingRoot != nil {
					label.New(device.labels, "AOA", m, m, 1, 30, &m.AoaDegrees)
				}
				if m.ActuatorTag == actuator.LeftWheelBrake || m.ActuatorTag == actuator.RightWheelBrake {
					label.New(device.labels, "BRK", m, m, 5, 20, &m.Brake)
				}
			}
			if device.currentThing == nil {
				device.currentThing = game.Things[0]
			}
			for _, s := range device.currentThing.Springs {
				if s.ActuatorTag > 0 {
					label.New(device.labels, actuator.SpringActuators[s.ActuatorTag], s.M1, s.M2, 5, 20, &s.Expansion)
				}
			}

			device.SendLabels()

		} else if kl == "p" {
			m := device.highlit.mass
			if m != nil {
				m.Fixed = !m.Fixed
				device.sendMasses([]*mass.Mass{m}, true)
			}

		} else if k == "Delete" {
			if device.highlit.mass == nil && device.highlit.spring != nil {
				device.highlit.thing.DeleteSpring(device.highlit.spring)
				device.sendThings([]*thing.Thing{device.highlit.thing})
			} else if device.highlit.mass != nil {
				//if viewer.highlit.mass.NotAttached() {
				device.highlit.mass.R = 0
				device.sendMasses([]*mass.Mass{device.highlit.mass}, true)

				device.highlit.mass.Delete(game.Masses) //less than straightforward
				device.sendMasses(game.Masses, true)
				device.sendThings(game.Things)
				//}
			}
		}
	}

	//sends any transform of the skin (done with cursor keys and/or shift)
	if dx != 0 || dy != 0 {
		if device.currentThing != nil {
			ct := device.currentThing
			if prop == "rotation" {
				//ct.Rotation += dx
			} else { //if prop=="scale" {
				ct.MeshScale.X += dx
				ct.MeshScale.Y += dy
			}
			device.sendThings([]*thing.Thing{ct}) //will need to send the offset, rotation and scale of the mesh/skin
		}
	}

	return nil
}

func (device *Device) SendVectors(game *game.Game) {
	device.Send(mass.VectorsAsMsg(game.Masses)) //send the new vectors
}

// Snapmasses - 	where we have mirrored, or rotationally copied springs - collapse the coincident masses and rewire the springs
func (device *Device) SnapMasses(things []*thing.Thing) {

	for _, t := range things {
		t.Rewire()
	}

}

func (device *Device) makeNextSpring(masses []*mass.Mass) {
	if device.highlit.mass == nil {
		device.highlit.mass = mass.New(masses, int32(len(masses)), device.spacePos, .05, false, false, true, nil)
	}

	m1 := device.highlit.mass
	m2 := mass.New(masses, int32(len(masses)), m1.P.Clone().Add(vec.NewVec3(0, .001, 0)), 0.05, false, false, true, nil)

	device.springCursor = m2

	device.highlit.spring = device.currentThing.AddSpring(m1, m2, 1, 1, actuator.NONE)
	log.Logit("made spring", m1.Index, m2.Index)

	//p.recordMassPositions() //we need to (re) do this as we have added masses

	// p.selectedMasses = make(map[*mass.Mass]bool)
	// p.selectedMasses[m2] = true
	device.sendMasses([]*mass.Mass{m1, m2}, false)
	device.sendThings([]*thing.Thing{device.currentThing})
	device.sendHighlit()

}

// // SendToPeers sends messages (masses, vectors etc) to other viewers in the same game
// func (viewer *Viewer) sendToPeers(globalViewers []*Viewer, msgs ...*msg.Msg) {
// 	for _, peer := range globalViewers {
// 		if peer.Player.Game == viewer.Player.Game && peer != viewer {
// 			peer.Send(msgs...)
// 		}
// 	}
// }

func (device *Device) bindValue(key string, valuePointer *float64, min float64, max float64, step float64, labelSet byte) {

	device.boundValues[key] = valuePointer //store the address of the value to be updated

	m := msg.NewMsg(msg.BindValue, key, *valuePointer, min, max, step, labelSet)
	device.Send(m) //we will receive msg.ValueChange messages back

}

func (device *Device) ReleaseWebSocket() {
	device.WebSocket = nil
}

func redact(s string) string {
	// obscure every other character (replace characters at odd rune indices with '*')
	r := []rune(s)
	for i := 1; i < len(r); i += 2 {
		r[i] = '*'
	}
	return string(r)
}

func NewFromConnectDeviceMsg(ibm *msg.Msg, globalDevices map[uint32]*Device, nobody *player.Player, ws *websocket.Conn) (*Device, *errorplus.Event) {

	if ibm.MsgType != msg.ConnectDevice {
		return nil, errorplus.New(nil, errorplus.Error, fmt.Sprintf("First message must be connectdevice was %T %v", ibm.MsgType, ibm.MsgType)) //connects an existing or new device (viewer/controller)
	}

	deviceId := uint32(0)
	vTok := ""
	ibm.Read(&deviceId, &vTok)

	if deviceId == 0 {
		// a new unknown device - create a new device,
		return makeNewDevice(globalDevices, ws, nobody)

	} else {
		//we're reconnecting an existing device
		mutex.Devices.RLock()
		d, present := globalDevices[uint32(deviceId)]
		mutex.Devices.RUnlock()

		time.Sleep(time.Millisecond * 500) //don't provide a response instanltly - to make brute forcing harder
		if !present {
			//device.WebSocket.Close()
			remade, _ := makeNewDevice(globalDevices, ws, nobody)

			return remade, errorplus.New(nil, errorplus.Warn, fmt.Sprintf("No such device (%v) to reconnect.. remade as %v", deviceId, remade.Id))
		}
		if d.token != vTok {
			d.WebSocket = ws
			d.Notify("Device token mismatch on reconnect", "red")
			d.ReleaseWebSocket()
			return nil, errorplus.New(nil, errorplus.Warn, fmt.Sprintf("Device token does not match got %v, want %v", redact(vTok), redact(d.token)))
		}
		//it's a valid token and known viewer
		d.WebSocket = ws
		d.Notify("Reconnected OK", "green")
		return d, nil
	}
}

func makeNewDevice(globalDevices map[uint32]*Device, ws *websocket.Conn, nobody *player.Player) (*Device, *errorplus.Event) {

	ndid := next.Id("device")
	_, present := globalDevices[ndid]
	if present {
		return nil, errorplus.New(nil, errorplus.Error, fmt.Sprintf("Generated next device ID %v already present", ndid))
	}

	newDevice := New(globalDevices, ndid, "Name me!", nobody, nobody, nobody, nobody, ws)
	err := newDevice.Persist()
	if err != nil {
		return newDevice, err
	}

	idMsg := msg.NewMsg(msg.DeviceId, newDevice.Id, newDevice.token)
	newDevice.Send(idMsg) //send the device id and token to the viewer - they are connected - sign in/up/or watch is next
	newDevice.Notify("Connected as new device", "green")
	return newDevice, nil
}

func (device *Device) ProcessBinaryMsg(ibm *msg.Msg, globalGames map[uint32]*game.Game, globalPlayers map[uint32]*player.Player, globalDevices map[uint32]*Device) (*errorplus.Event, *Device) {

	var myGame *game.Game = nil
	if device.ViewingPlayer != nil {
		myGame = device.ViewingPlayer.Game
	}

	if device.Id == 0 && ibm.MsgType != msg.ConnectDevice {
		return errorplus.New(nil, errorplus.Warn, fmt.Sprintf("Device must connect first, anonymous/nobody device tried to send %T %v", ibm.MsgType, ibm.MsgType)), device
	}

	if device.ViewingPlayer == nil && !ibm.IsOneOf(msg.CreatePlayer, msg.SignIn, msg.ConnectDevice) {
		return errorplus.New(nil, errorplus.Warn, "A viewer must connectcreate a player or sign in first"), device

	}

	switch ibm.MsgType {

	case msg.ConnectDevice:
		return errorplus.New(nil, errorplus.Critical, "connectdevice shoul dbe processed elsewhere"), device

	case msg.CreatePlayer:

		playerName, email, password := "", "", ""
		ibm.Read(&playerName, &email, &password)
		salt := player.Salt()               //generate a new salt
		hash := player.Hash(password, salt) //hash the password with the (additonal) salt
		token := player.Salt()              //generate a new token

		npid := next.Id("player")
		_, present := globalPlayers[npid]
		if present {
			return errorplus.New(nil, errorplus.Error, "Generated player ID already present"), device
		}
		newPlayer := player.New(globalPlayers, npid, playerName, email, hash, salt, token, 0, 0, 0, nil)
		err := newPlayer.Persist()
		if err != nil {
			return err, device
		}

		//adopt the device
		device.owner = newPlayer
		device.ViewingPlayer = newPlayer
		device.primaryControls = newPlayer
		device.secondaryControls = newPlayer
		device.Persist()

		response := msg.NewMsg(msg.PlayerId, newPlayer.Id, newPlayer.Token)

		response.Write(msg.ReplaceDiv, "createAccount")

		welcome(newPlayer, response)
		deviceList(newPlayer, response, globalDevices)
		observers(newPlayer, response, globalDevices)
		gamesInProgress(response, globalPlayers)

		device.Send(response)
		device.Notify("Player "+playerName+" created OK", "green")

	case msg.SignIn: //signs a Player in (so that they can manage devices)
	case msg.CreateGame: //creates a game

		playerId := uint32(0)
		playerName := ""
		gameName := ""
		ibm.Read(&playerId, &playerName, &gameName) //who will 'own' this game

		player, present := globalPlayers[playerId]
		if !present {
			return errorplus.New(nil, errorplus.Warn, "No such player "+fmt.Sprint(playerId)), device
		}
		if player.Name != playerName {
			return errorplus.New(nil, errorplus.Warn, "Player name does not match"), device
		}

		//will make a new game with a new ID and add the player to it
		newGame := game.New(globalGames, next.Id("game"), gameName)

		runwayPos := vec.NewVec3(0, 0, 0)
		runwayVec := vec.NewVec3(1000, 0, 1000)
		newGame.SetRunway(runwayPos, runwayVec, 40)

		newGame.Ignite(vec.NewVec3(10, 0, -1000)) //note the Y position has no effect

		//Plough the runway and set the start and end heights
		ogm := msg.Empty()
		landPos, _ := newGame.MakeLand(runwayPos, runwayVec.Normalise(), ogm)
		device.Send(ogm) //send the land

		log.Logit("Runway land made between", landPos, "and", landPos.Add(runwayVec))

		player.Game = newGame
		device.ViewingPlayer = player

		device.startIn(newGame)

		// aircraftScene := game.Load(nil, "wip26")
		// aircraft := aircraftScene.Things[0] //assume first thing is the aircraft

		// cg, weight := aircraft.CentreOfMass()
		// log.Logit("aircraft weighs", weight)
		// aircraft.Translate(landPos.Sub(cg).Add(vec.NewVec3(0, 10, 0)))
		// ng.MergeThing(aircraft, aircraft.masses)

	case msg.AddPlayerToGame:
		pid := uint32(0)
		gid := uint32(0)
		ibm.Read(&pid, &gid)
		globalPlayers[pid].Game = globalGames[gid]

	case msg.JoinAsController:

	case msg.ValueChange:
		key := ""
		value := float64(0)
		ibm.Read(&key, &value)
		if myGame == nil {
			return errorplus.New(nil, errorplus.Warn, "Received a valuechange outside of a game"), device
		}
		device.SetBoundValue(key, value, myGame.Masses, myGame.Things)

	case msg.Load:

		filename := ""
		ibm.Read(&filename)

		//the old game is not destroyed - a new game is created and I am started in it
		oGid := myGame.Id
		gm := game.Load(globalGames, filename) //replace the game (globals - as the game is a pointer)

		gm.Id = oGid

		gm.FlyMasses() //once to get lift vectors

		//put me (and my connected socket, camera and grid)into the game i just loaded
		//gm.Players = append(gm.Players, player)
		device.Notify("loaded "+filename, "green")
		device.startIn(gm)
		return errorplus.New(nil, errorplus.Info, "Loaded game"), device

	case msg.Save:
		filename := ""
		ibm.Read(&filename)
		device.ViewingPlayer.Game.Save(filename, device.selectedMasses)
		device.Notify("Saved OK", "green")
		return errorplus.New(nil, errorplus.Info, "Saved game"), device

	case msg.ControlPositions:

		blobs := byte(0)
		ibm.Read(&blobs)
		var blobId, x, y = byte(0), byte(0), byte(0)
		for i := 0; i < int(blobs); i++ {
			ibm.Read(&blobId, &x, &y)

			device.SetControlInputsFromBlob(blobId, float64(x), float64(y))

		}
		device.updateActuators()

	default:
		panic("other Inbound binary messages not implemented")
	}

	return nil, device

}

func (device *Device) startIn(game *game.Game) {
	//state.send(nil, &reply{Cmd: "playerJoined", Payload: player})    //tell everyone about the new player
	device.SendCamera()                  //send the camera position
	device.sendMasses(game.Masses, true) //send all the masses
	device.sendThings(game.Things)
	device.SendLabelSets()

	device.sendGameId(game.Id) //game id starts it running

	//	controlTokens[p] = p.sendControlPin() //send a PIN to them so they can take control from another device
}

func (device *Device) updateActuators() {
	//note that the mixer contains the bound actuators (springs/masses/engines)
	//  so need no knowledge of the vehicle
	for _, mix := range device.mixers {
		mix.ZeroOutputs()
	}

	for _, mixer := range device.mixers {
		mixer.Mix(device.controls) //add in defelctions
	}
}

func (device *Device) GetVehicle(players map[uint32]*player.Player) *thing.Thing {
	if device.ViewingPlayer == nil {
		return nil
	}
	return device.ViewingPlayer.GetVehicle()

}

func (device *Device) startEngine(index int, game *game.Game, response *msg.Msg) *errorplus.Event {
	if device.ViewingPlayer == nil {
		return errorplus.New(nil, errorplus.Info, "Engine start whilst not viewing a player")
	}
	vehicle := device.ViewingPlayer.GetVehicle()
	if vehicle == nil {
		return errorplus.New(nil, errorplus.Info, "Engine start - Player is not in a vehicle")
	}

	if index < 0 || index >= len(vehicle.Engines) {
		return errorplus.New(nil, errorplus.Info, "Engine start - No such engine")
	}

	vehicle.Engines[index].Start(game.Sounds, response)

	return nil
}

func (device *Device) SetControlInputsFromBlob(blobId byte, x float64, y float64) {

	switch blobId {

	case 1:
		device.controls[input.StickX] = float64(x)/128 - 1 //normalise to +/- 1
		device.controls[input.StickY] = float64(y)/128 - 1
		log.Logit("right stick", device.controls[input.StickX], device.controls[input.StickY])

	case 2:
		device.controls[input.Rudder] = float64(x)/128 - 1
		device.controls[input.Throttle] = float64(y)/128 - 1
		log.Logit("left stick", device.controls[input.Rudder], device.controls[input.Throttle])

	case 3:
		device.controls[input.WheelBrakeLeft] = float64(y)/128 - 1

	case 4:
		device.controls[input.WheelBrakeRight] = float64(y)/128 - 1

	default:
		log.Logit("warning - unhandled blob id ", blobId)
	}

}

func (device *Device) BindMixers(vehicle *thing.Thing) {
	// For each mixer, set the mixers, mass and engine (output) the the actuator in the vehicle

	for _, mx := range device.mixers {

		mx.Spring = vehicle.FindSpringActuator(mx.Actuator)
		mx.Engine.Spring = vehicle.FindSpringActuator(mx.Actuator)

		if mx.Spring == nil { //we didnt bind it to a spring - try a mass
			mx.Mass = vehicle.FindMassActuator(mx.Actuator)
		}
	}

}

func (device *Device) RevokeButton() string {
	return ("<button>Kick</button>") //will need to set the devices watchingplayer, primary and secondary controls to it's owner
}

func observers(player *player.Player, response *msg.Msg, globalDevices map[uint32]*Device) {
	//Observers see exactly what the pilot sees (low cost)
	//Independent observers can switch cameras, slew and orbit
	//Flight engineers - occupy the copilot seat and can operate throttles, flaps, gear, water drop, probes etc
	//Co-pilots - have a full set of controls (and an indepent Pov)

	//players request permission (o/io/fe/cp).. if granted their devices are set to watch/control the player
	//                       vieweingPlayer flightcontrols ancillaryControls isIndependent
	//            observer =  target            nil             nil                false
	//independent observer =  target            nil             nil                true
	//flight engineer      =  target            nil             target             true
	//copilot              =  target           target           target            true

	response.Write("<h2>Observers and Co-pilots:</h2>") //observer, independent observer, flight engineer, co-pilot
	//todo - devices where viewing player is me, but I am not the owner
	response.Write("<table>")
	tableHead(response, "ID", "Name", "Device", "Role", "Connected", "Remove")
	for _, d := range globalDevices {
		if d.ViewingPlayer != nil && d.ViewingPlayer == player && d.owner != player {
			tableRow(response, fmt.Sprintf("%d", d.owner.Id), d.owner.Name, d.Name, Role(player, d), d.Status(), d.RevokeButton())
		}
	}
	response.Write("</table>")
}

func welcome(player *player.Player, response *msg.Msg) {
	pid := strconv.FormatUint(uint64(player.Id), 10)
	response.Write("<h1>Hi ", player.Name, " - PID:", pid, "</h1>")
}

func deviceList(player *player.Player, response *msg.Msg, globalDevices map[uint32]*Device) {

	response.Write("<h2>Your devices:</h2>")
	response.Write("<table>")
	tableHead(response, "ID", "Name", "PoV", "Connected", "Action")

	for _, d := range globalDevices {
		if d.ViewingPlayer != nil && d.ViewingPlayer == player && d.owner == player {
			tableRow(response, fmt.Sprintf("%d", d.Id), d.Name, d.pov, d.Status(), d.RevokeButton())
		}
	}

}

func gamesInProgress(response *msg.Msg, globalPlayers map[uint32]*player.Player) {
	response.Write("<h2>Games in progress:</h2>")
	response.Write("<table>")
	tableHead(response, "ID", "Players")

	pbg := playersByGame(globalPlayers)
	for g, players := range pbg {
		tableRow(response, fmt.Sprintf("%d", g.Id), fmt.Sprintf("%d", len(players)))
		response.Write("<tr><td colspan='2'>")
		tableHead(response, "Player ID", "Name")
		for _, p := range players {
			tableRow(response, fmt.Sprintf("%d", p.Id), fmt.Sprintf("%v", p.Name))
		}
		response.Write("</td></tr>")
	}
	response.Write("</table>")
}

func playersByGame(globalPlayers map[uint32]*player.Player) map[*game.Game][]*player.Player {
	pbg := make(map[*game.Game][]*player.Player)
	for _, p := range globalPlayers {
		if p.Game != nil {
			pbg[p.Game] = append(pbg[p.Game], p)
		}
	}
	return pbg
}

func Role(target *player.Player, device *Device) string {

	pfc := device.primaryControls == target
	sfc := device.secondaryControls == target
	indie := device.isIndependent

	if pfc && sfc && indie {
		return "Co-pilot"
	} else if sfc && indie {
		return "Flight Engineer"
	} else if indie {
		return "Independent Observer"
	} else if !pfc && !sfc && !indie {
		return "Observer"
	} else {
		return "Unknown role"
	}

}
func tableHead(response *msg.Msg, cols ...string) {

	response.Write("<tr>")
	for _, c := range cols {
		response.Write("<th>", c, "</th>")
	}
	response.Write("</tr>")

}

func tableRow(msg *msg.Msg, cols ...string) {
	msg.Write("<tr>")
	for _, c := range cols {
		msg.Write("<td>", c, "</td>")
	}
	msg.Write("</tr>")
}
