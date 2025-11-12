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
	"github.com/nickax/gofu/html"

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
	mixers      []*mixer.Mixer     //scales the output of a control (channles 1-7) from (-1 to +1) to a mass/spring - see updateActuators()
	warning     []*errorplus.Event //we collect warnings per request/device
	NumWarnings uint32             //current count of warnings - we reuse the warnings in the slice to avoid allocations

}

func (d *Device) ClearWarnings() {
	d.NumWarnings = 0
}

func (d *Device) GetWarnings() []*errorplus.Event {
	return d.warning[:d.NumWarnings]
}

func (d *Device) Warn(msg string, severity errorplus.Severity) {

	if d.NumWarnings < 10 {
		if d.warning[d.NumWarnings] == nil {
			d.warning[d.NumWarnings] = errorplus.New(nil, severity, msg)
		} else {
			d.warning[d.NumWarnings].Msg = msg
			d.warning[d.NumWarnings].Severity = severity
		}
		d.NumWarnings++
	}
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

	if owner == nil || viewing == nil || primaryControls == nil || secondaryControls == nil {
		log.Logit("Device New called with nil player(s): owner", owner, " viewing ", viewing, " primary ", primaryControls, " secondary ", secondaryControls)
		panic("Device New called with nil player(s) - use player.None")
	}

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
		warning:        make([]*errorplus.Event, 10),
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

func (dev *Device) Persist() *errorplus.Event {

	deviceMsg := msg.NewMsg(msg.P_Device)
	dev.WriteTo(deviceMsg)
	return persist.Append("repo.bin", deviceMsg)
}

func NewFromMsg(m *msg.Msg, devices map[uint32]*Device, players map[uint32]*player.Player) *Device {

	did, name, token, oid, vpid, pcid, scid, ii, pov := uint32(0), "", "", uint32(0), uint32(0), uint32(0), uint32(0), true, ""
	m.Read(&did, &name, &token, &oid, &vpid, &pcid, &scid, &ii, &pov)
	return New(devices, did, name, players[oid], players[vpid], players[pcid], players[scid], nil)

}

func (dev *Device) WriteTo(m *msg.Msg) {

	oid, vpid, pcid, scid := uint32(0), uint32(0), uint32(0), uint32(0)
	if dev.owner != nil {
		oid = dev.owner.Id
	}
	if dev.ViewingPlayer != nil {
		vpid = dev.ViewingPlayer.Id
	}
	if dev.primaryControls != nil {
		pcid = dev.primaryControls.Id
	}
	if dev.secondaryControls != nil {
		scid = dev.secondaryControls.Id
	}
	m.Write(dev.Id, dev.Name, dev.token, oid, vpid, pcid, scid, dev.isIndependent, dev.pov)
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

func (dev *Device) GetPlayer() *player.Player {
	return dev.ViewingPlayer
}

func (dev *Device) Status() string {
	if dev.WebSocket != nil {
		return "Connected"
	}
	return "Disconnected"
}

// Watch a players vehicle from a given pov (defined within that vehicle)
func (dev *Device) Watch(player *player.Player, pov string) {

	v := player.GetVehicle()
	if v != nil {
		cam := v.FindCam(pov)
		if cam != nil {
			dev.Camera = cam
			dev.pov = pov //the vehicle.cameras[pov] can be used to find the 'home' position
		}
	}

}

func (dev *Device) clearContextMenu() {
	//clear the context menu (on the client)
	m := msg.NewMsg(msg.ClearContextMenu)
	dev.Send(m)

}

// collects and sends the flames visible to this viewer
func (dev *Device) GetFlames(fire *terrain.TriMesh, message *msg.Msg) {

	tcs := mesh.NewTcs(0, 1, 1, 0)                    //texture atlas coordinates
	flameMesh := mesh.New(201, "flame", 10000, 30000) //10k faces, 30k verts

	fire.Root.GetFlames(dev.landTri, fire, flameMesh, dev.Camera, tcs)

	flameMesh.WriteTo(message, 1)

}

func (dev *Device) SendCamera() {

	msg := msg.NewMsg(msg.Camera)
	dev.Camera.WriteTo(msg)
	dev.Send(msg)

}

func (dev *Device) GetLandRoot() *terrain.Tri {
	return dev.landTri
}

func (dev *Device) processMouseMove(game *game.Game) { //isRunning bool, masses []*mass.Mass, things []*thing.Thing) {

	if !game.Running {

		//a point on the far plane (where the mouse cursor is pointing)

		if dev.buttons == 2 { //panning camera

			delta := (dev.cursor.Sub(dev.grab)).Mul(2)

			if delta.LengthSq() != 0 {

				//logit("delta", delta.x, delta.y)

				camRight := dev.downCam.Direction.Cross(dev.downCam.Up).Normalise()

				dev.Camera.Up = dev.downCam.Up.RotateAbout(camRight, delta.Y).Normalise()
				pitched := dev.downCam.Direction.RotateAbout(camRight, delta.Y)
				yawed := pitched.RotateAbout(dev.Camera.Up, -delta.X)

				dev.Camera.Direction = yawed
				dev.Camera.Up = vec.NewVec3(0, 1, 0) //auto level the camera

				dev.SendCamera()
			}
			return
		}

		switch dev.mode {

		case editing:

			if dev.buttons == 0 {
				pickRay := ray.New(dev.Camera.Position, dev.Camera.FarPos)

				cm, _ := mass.ClosestMassToRay(game.Masses, dev.springCursor, pickRay)

				if cm != dev.highlit.mass {
					dev.highlit.mass = cm
					dev.sendHighlit() //might be nil
				}
			}

			//mutates the viewers gridPos and spacePos (by reference)
			dev.Grid.UpdateGridPosAndSpacePos(dev.Camera, dev.zOff, dev.gridPos, dev.spacePos)

			pickRay := ray.New(dev.Camera.Position, dev.Camera.FarPos)

			closest := math.MaxFloat64
			for _, thing := range game.Things {
				//spring, d := pickRay.ClosestSpring(thing.Springs)
				spring, d := spring.ClosestSpringToRay(thing.Springs, pickRay)

				if d < closest {
					dev.highlit.thing = thing
					dev.highlit.spring = spring
					closest = d
				}
			}

			dev.sendHighlit()

		case moving:
			dev.moveSelected(game)
		case stretching:
			dev.springCursor.P = dev.spacePos.Clone() //moveSpringCursor()
			dev.sendMasses([]*mass.Mass{dev.springCursor}, false)
		}

		if dev.buttons == 1 && dev.mode == editing {
			//dragging/panning the camera
			if dev.downGridPos != nil {
				delta := dev.gridPos.Sub(dev.downGridPos).Multiply(.9)
				dev.Camera.Position = dev.downCam.Position.Sub(delta)
				dev.SendCamera()
			}
		}

		dev.SendCursor()
	}
}

func (dev *Device) sendBytes(msg []byte) {

	if dev.WebSocket == nil {
		log.Logit(dev.pov + " viewer socket is disconnected")
		return
	}

	dev.mtx.Lock()         //<<---MUTEX
	defer dev.mtx.Unlock() //deferred unlock
	messageType := websocket.BinaryMessage

	//logit("sent", len(msg), " binary bytes")
	dev.WebSocket.WriteMessage(messageType, msg) //write the message (and return any error)
}

func (dev *Device) ViewChangedSignificantly() bool {
	if dev.lastCam == nil {
		return true
	}

	dist := dev.Camera.Position.DistanceFrom(dev.lastCam.Position)
	dir := dev.Camera.Direction.Dot(dev.lastCam.Direction)
	if dist > 100 || dir < .95 {
		dev.lastCam = dev.Camera.Clone() //store this as the new old position
		return true
	}
	return false
}

func (dev *Device) Send(msg ...*msg.Msg) {
	for _, msg := range msg {
		dev.sendBytes(msg.AllBytes())
	}

}

// []**gameId uint32, masses []*mass.Mass, things []*thing.Thing, selectedMasses map[*mass.Mass]bool, grid *grid.Grid) {

func (dev *Device) SendLabels() {

	msg := msg.NewMsg(msg.Labels)
	msg.Write(uint16(len(dev.labels)))
	for _, l := range dev.labels {
		l.WriteTo(msg)
	}
	dev.Send(msg)
}

func (dev *Device) sendThings(things []*thing.Thing) {
	msg := thing.ThingsAsMsg(things)
	dev.Send(msg)
}

func (dev *Device) sendMasses(masses []*mass.Mass, withDetail bool) {

	msg := mass.MassesAsMsg(masses, withDetail, dev.selectedMasses)

	dev.Send(msg)

}

func (dev *Device) sendClear() {
	msg := msg.NewMsg(msg.Clear)
	dev.Send(msg)
}

func (dev *Device) SendLabelSets() { //For options on the sliders

	sendLabelSet(dev, 1, actuator.MassActuators)
	sendLabelSet(dev, 2, actuator.SpringActuators)
	sendLabelSet(dev, 3, aero.SectionNames)
}

func sendLabelSet[E actuator.ActuatorEnum | aero.Section](p *Device, idx byte, valueLabelPairs map[E]string) {

	m := msg.NewMsg(msg.LabelSet)
	m.Write(idx, byte(len(valueLabelPairs))) //index of the label set/ count of labels

	for value, label := range valueLabelPairs {
		m.Write(int32(value), label)
	}
	p.Send(m)

}

func (dev *Device) sendControlPin() uint32 {

	m := msg.NewMsg(msg.ControlToken)
	token := randomPin()

	m.Write(token)
	dev.Send(m)

	return token

}

func randomPin() uint32 { //TODO - Check for existing token
	return uint32(math.Round(1000 + rand.Float64()*8999))
}

// SendGameId - causes the client to start the game
func (dev *Device) sendGameId(gameId uint32) {
	m := msg.NewMsg(msg.GameId)
	m.Write(gameId)
	dev.Send(m)
}

func (dev *Device) sendCentreOfMass(t *thing.Thing) {

	cg, weight := t.CentreOfMass()
	dev.Send(msg.NewMsg(msg.CentreOfMass, t.Index, cg, float32(weight)))

}

func (dev *Device) moveSelected(game *game.Game) {
	moveDelta := dev.spacePos.Sub(dev.moveStart)

	//add any movement normal to the grid to the delta
	gridNormal := dev.Grid.Xaxis.Cross(dev.Grid.Yaxis).Normalise()
	camDGN := gridNormal.Multiply(dev.Camera.Position.Sub(dev.downCam.Position).Dot(gridNormal))
	moveDelta.AddIn(camDGN)

	for m := range dev.selectedMasses {
		p, present := dev.massStartPos[m]
		if present {
			m.P = p.Add(moveDelta)
		} else {
			log.Logit("no startpos present for mass ", m.Index)
		}

	}

	dev.regenTransformed(game.Masses)

	s := slices.Collect(maps.Keys(dev.selectedMasses))
	if len(s) > 0 {
		dev.sendMasses(s, false) //just send the new positions (not details)
	}

}

func (dev *Device) SetBoundValue(key string, value float64, masses []*mass.Mass, things []*thing.Thing) {
	pointer := dev.boundValues[key]
	*(*float64)(pointer) = value //cast to *float64 and then dereference/set value

	//TODO - optimise/reduce chatter
	dev.Send(mass.VectorsAsMsg(masses)) //send the new vectors

	dev.sendMasses(masses, true) //send the potentially) modified mass
	if dev.currentThing == nil {
		dev.currentThing = things[0]
	}
	dev.sendCentreOfMass(dev.currentThing)

}

// regenTransformed updates any masses that are transforms of other masses (e.g. on the other side of a mirror)
func (dev *Device) regenTransformed(masses []*mass.Mass) {
	//reflected := make(map[*mass.Mass]*mass.Mass)
	for _, m := range masses {

		//this could be another transform such as a rotation
		reflect := func(p *vec.V3) *vec.V3 {
			return p.ReflectInPlane(dev.Grid.Origin, dev.Grid.Normal())
		}
		m.RegenFromMaster(reflect)

	}

	dev.sendMasses(masses, true)

}

// used for sending section of the interleaved (often) floating point data that makes up vertex, normal, position and index buffers
// in a format very close to that need by the GPU (or three.js buffers)

func (dev *Device) SendCursor() {

	msg := msg.NewMsg(msg.Cursor, dev.cursor, dev.gridPos, dev.spacePos)

	if dev.highlit.mass != nil {
		msg.Write(0) //cursor sphere radius
	} else {
		msg.Write(float32(0.05)) //cursor sphere radius
	}
	dev.Send(msg)

}

func (dev *Device) sendHighlit() {

	hm, ht, hs := int32(-1), int32(-1), int32(-1)
	if dev.highlit.mass != nil {
		hm = dev.highlit.mass.Index
	}
	if dev.highlit.thing != nil {
		ht = int32(dev.highlit.thing.Index)

	}
	if dev.highlit.spring != nil {
		hs = dev.highlit.spring.Index
	}

	msg := msg.NewMsg(msg.Highlit, hm, ht, hs)
	dev.Send(msg)
}

func (dev *Device) Notify(text string, severity string) {

	msg := msg.NewMsg(msg.Notify, text, severity)
	dev.Send(msg)
	log.Logit(text, severity)

}

func (dev *Device) recordMassPositions(masses []*mass.Mass) {
	dev.massStartPos = make(map[*mass.Mass]*vec.V3) //reset each time
	for _, m := range masses {                      //selectedMasses {
		dev.massStartPos[m] = m.P.Clone()
	}
}

// MoveCamera moves the camera (and any selected masses) based on key presses
func (dev *Device) MoveCamera(game *game.Game) {

	dir := vec.NewVec3(0, 0, 0)

	speed := .5
	if dev.keys["Alt"] {
		speed = 10
	}

	if dev.keys["w"] {
		dir.SetZ(speed)
	}
	if dev.keys["s"] {
		dir.SetZ(-speed)
	}
	if dev.keys["a"] {
		dir.SetX(-speed)
	}
	if dev.keys["d"] && !dev.keys["Control"] {
		dir.SetX(speed)
	}
	if dev.keys["ArrowUp"] {
		dir.SetY(speed)
	}
	if dev.keys["ArrowDown"] {
		dir.SetY(-speed)
	}

	camDir := dev.Camera.Direction

	right := camDir.Cross(dev.Camera.Up).Normalise()
	up := right.Cross(camDir).Normalise()
	delta := right.Multiply(dir.X).Add(up.Multiply(dir.Y)).Add(camDir.Multiply(dir.Z))

	if delta.LengthSq() > 0 {
		dev.Camera.Position.AddIn(delta)
		dev.SendCamera()
		if dev.mode == moving {
			dev.moveSelected(game)
		}
	}

}

func (dev *Device) setMode(mode ModeEnum) {
	dev.mode = mode
	log.Logit("viewers mode set to", mode)

	msg := msg.NewMsg(msg.Mode)
	msg.Write(mode)
	dev.Send(msg)

}

func (dev *Device) checkHighlitMass() bool {
	if dev.highlit.mass == nil {
		dev.Notify("Highlight a mass and press the key", "red")
		return false
	}
	return true

}

// Tidy Remove masses not attached to a spring
func (dev *Device) Tidy(game *game.Game) {

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
func (dev *Device) ProcessStructuredMsg(ibm *jsonmsg.Msg, response *msg.Msg) *errorplus.Event {

	if dev == None { //device.None
		return errorplus.New(nil, errorplus.Warn, "structuredmessage (probably a keystroke) from disconnected device "+ibm.Cmd)

	}
	if dev.ViewingPlayer == player.None {
		return errorplus.New(nil,
			errorplus.Warn, "device has no viewing player: "+ibm.Cmd)

	}

	if dev.ViewingPlayer.Game == game.None {
		return errorplus.New(nil,
			errorplus.Warn, "device, viewing player game is none: "+ibm.Cmd)

	}

	gm := dev.ViewingPlayer.Game

	dx, dy := float64(0), float64(0) //used for sliding skins
	prop := "offset"
	step := float64(3)

	//logit(msg.Cmd)

	switch ibm.Cmd {
	case "keyUp":
		//a key was released
		dev.keys[ibm.Key] = false

		switch ibm.Key {
		case "ArrowLeft", "ArrowRight":
			dx = 0
		case "ArrowUp", "ArrowDown":
			dy = 0
		}

	case "mw": //mousewheel

		dev.Camera.Position.AddIn(dev.Grid.Normal().Multiply(ibm.Payload[0] * -0.005))
		dev.zOff += ibm.Payload[0] * -0.005

		dev.processMouseMove(gm) //*isRunning, masses, things)
		dev.SendCamera()

	case "mm": //mouse move

		dev.movedSinceMouseDown = true

		dev.buttons = byte(ibm.Payload[0])
		dev.Camera.FarPos = vec.NewVec3(ibm.Payload[1], ibm.Payload[2], ibm.Payload[3])
		dev.cursor.X = ibm.Payload[4]
		dev.cursor.Y = ibm.Payload[5]

		dev.processMouseMove(gm) //*isRunning, masses, things)

	case "mu":

		if dev.buttons == 2 && !dev.movedSinceMouseDown {

			dev.Send(mass.VectorsAsMsg(gm.Masses)) //send the new vectors

			if dev.highlit.mass != nil {

				//degreesToRadians := float32(180.0) / float32(math.Pi)
				dev.boundValues = make(map[string]*float64, 0)

				//p.bindValue("radius", &p.highlit.mass.r, 0.01, 1.00, .01, 0)
				dev.bindValue("radius", &dev.highlit.mass.R, 0.01, 1.00, .01, 0)
				dev.bindValue("Section", &dev.highlit.mass.Section, 0, 1, 1, 3)
				//p.bindValue("aoa", p.highlit.mass, &p.highlit.mass.aoaRads, -20, +20, 1, 0)
				dev.bindValue("wingArea", &dev.highlit.mass.WingArea, 0.1, 500.00, 1, 0)
				//p.bindValue("dihedral", p.highlit.mass, &p.highlit.mass.dihedralDegrees, -10, 10, 1, 0)
				//p.bindValue("controlSurface", p.highlit.mass, &p.highlit.mass.flightOutput, 0, 10, 1, 1) //use labelt set 1 (outoput flight controls)

				dev.bindValue("massActuator", (*float64)(unsafe.Pointer(&dev.highlit.mass.ActuatorTag)), 0, float64(len(actuator.MassActuators)), 1, 1) //use label set 1 (mass actuator labels)
				//p.sendBoundValues() //will pop up a context menu clientside
				dev.setMode(props)
			} else if dev.highlit.spring != nil {
				//p.boundValues = make(map[string]boundValue, 0)
				dev.bindValue("springActuator", (*float64)(unsafe.Pointer(&dev.highlit.spring.ActuatorTag)), 0, float64(len(actuator.SpringActuators)), 1, 2) //use label set 2 (spring actuator labels)
				//p.bindValue("springActuator", &p.highlit.spring.actuatorTag, 0, float64(len(springActuators)), 1, 1) //use label set 1 (actuator labels)
				//p.sendBoundValues()                                                                      //will pop up a context menu clientside
				dev.setMode(props)
			}
		}

		dev.buttons = byte(ibm.Payload[0])

	case "md":

		dev.buttons = byte(ibm.Payload[0])

		dev.movedSinceMouseDown = false

		dev.grab = dev.cursor.Clone()

		dev.downGridPos = dev.gridPos.Clone()

		dev.downCam = dev.Camera.Clone()

		if dev.buttons == 1 {

			if dev.highlit.mass != nil {
				dev.moveStart = dev.highlit.mass.P.Clone()
			} else {
				dev.moveStart = dev.spacePos.Clone() //may be snapped
			}

			dev.recordMassPositions(gm.Masses)

			if dev.mode == editing {
				//toggle selection of highlit mass
				phm := dev.highlit.mass
				if phm != nil {
					psm := dev.selectedMasses

					there := psm[phm]
					if there {
						delete(psm, phm)
					} else {
						psm[phm] = true
					}
					dev.sendMasses([]*mass.Mass{phm}, true)
				}
			}

			if dev.highlit.mass != nil {
				m := dev.highlit.mass
				gridPlane := dev.Grid.Plane()
				dev.zOff = gridPlane.DistanceFrom(m.P)

				dev.SendCursor()
			}

			switch dev.mode {
			case startMove:
				dev.setMode(moving)
			case moving:
				dev.setMode(editing)
			case grabbingMesh:

				dev.meshGrab = dev.spacePos.Clone()
				dev.setMode(offsettingMesh)
			case offsettingMesh:
				delta := dev.spacePos.Sub(dev.meshGrab)
				//delta.x *= -1 //UGLY - but the scenes x axis is inverted
				dev.currentThing.MeshOffset.AddIn(delta)
				dev.sendThings([]*thing.Thing{dev.currentThing})
				dev.setMode(editing)

			case adding:
				//we will make the spring between the highlit mass and a new mass
				//if there is no highlit mass, then we add one
				//on mouseup - we will collapse the new mass into any we are on top op
				//first (possibly highlit) mass
				dev.makeNextSpring(gm.Masses)

				dev.setMode(stretching)

			case stretching:

				if dev.highlit.mass != nil {
					log.Logit("substituting mass")
					dev.highlit.spring.M2 = dev.highlit.mass

					//remove it clientside
					dev.springCursor.R = 0
					dev.sendMasses([]*mass.Mass{dev.springCursor}, true)

					gm.Masses = gm.Masses[:len(gm.Masses)] //delete the last mass

					dev.sendThings([]*thing.Thing{dev.currentThing}) //sends the new spring (once on mousedown)
				} else {
					dev.highlit.mass = dev.springCursor
				}

				dev.makeNextSpring(gm.Masses)
			default:
				return errorplus.New(nil, errorplus.Error, "Unknown mode on mouse down: "+string(dev.mode))
			}

		}

		//viewer.send(&reply{Cmd: "gridPos", Payload: viewer.gridPos.payload()})

	case "keyDown":

		k := ibm.Key
		kl := strings.ToLower(k)
		dev.keys[k] = true

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
			if gm.Running {
				dev.controls[input.StickX] -= 0.05
			}
		case "ArrowRight":
			dx = +step
			if gm.Running {
				dev.controls[input.StickX] += 0.05
			}

		case "ArrowUp":
			if gm.Running {
				dev.controls[input.StickY] += 0.05
			}

			dy = +step
		case "ArrowDown":
			if gm.Running {
				dev.controls[input.StickY] -= 0.05
			}

			dy = -step //see the end of the if block for where the transform is send if dx or dy are set

		}

		switch kl {
		case "t":
			if dev.currentThing == nil {
				dev.currentThing = gm.Things[0]
			}
			dev.setMode(adding)
		case "y": //Tidy - permanenty snaps reflection halves together and removes unreferenced masses
			dev.SnapMasses(gm.Things)
			dev.Tidy(gm)
			dev.sendClear()
			log.Logit("tidy")
			dev.sendMasses(gm.Masses, true)
			dev.sendThings(gm.Things)
		case "-":
			dev.controls[input.Throttle] -= 0.05
		case "+":
			dev.controls[input.Throttle] += 0.05
		case "b":
			//grow a bush

		case "e":
			dev.setMode(editing)
		}

		if dev.keys["Control"] && kl == "d" { //deselect all
			//deselect all masses
			dev.selectedMasses = make(map[*mass.Mass]bool)
			dev.sendMasses(gm.Masses, true)
		} else if dev.keys["Control"] && kl == "a" { //select all
			//deselect all masses
			for _, m := range gm.Masses {
				dev.selectedMasses[m] = true
			}
			dev.sendMasses(gm.Masses, true)
		} else if k == "0" { //reset z offset (from the grid)
			dev.zOff = 0
			dev.processMouseMove(gm)
			dev.SendCamera()
			dev.controls[input.Throttle] = 0
		} else if k == "1" {
			//start port engine
			return dev.startEngine(0, gm, response)
		} else if k == "2" {
			//start starboard engine
			return dev.startEngine(1, gm, response)

		} else if kl == "o" {

			if dev.checkHighlitMass() {
				dev.currentThing.Om = dev.highlit.mass
				dev.sendThings([]*thing.Thing{dev.currentThing})
			}

		} else if kl == "f" { //set the forward direction mass
			if dev.checkHighlitMass() {
				dev.currentThing.Fm = dev.highlit.mass
				dev.sendThings([]*thing.Thing{dev.currentThing})
			} else {
				dev.follow = !dev.follow
			}
		} else if kl == "r" {
			if dev.keys["Control"] {
				//rotate thing 90 degrees more
				dev.currentThing.MeshRotation.AddIn(dev.currentThing.MeshRotation.Normalise().Multiply(math.Pi / 2))
			} else {
				//right mass  (x axis mass) of thing mesh
				if dev.checkHighlitMass() {
					dev.currentThing.Rm = dev.highlit.mass
				}
			}
			dev.sendThings([]*thing.Thing{dev.currentThing})

		} else if kl == "g" { //align the grid
			if dev.keys["Control"] { //CTRL-G - toggle gravity
				gm.ZeroG = !gm.ZeroG
			} else {

			}
		} else if kl == "m" && dev.keys["Shift"] {
			if dev.currentThing == nil {
				dev.currentThing = gm.Things[0]
			}
			dev.currentThing.MeshVisibility = 1 - dev.currentThing.MeshVisibility
			dev.sendThings([]*thing.Thing{dev.currentThing})
		} else if kl == "m" && dev.keys["Alt"] {
			dev.setMode(grabbingMesh)

		} else if kl == "m" && dev.keys["Control"] {
			//mirror the selected masses (in the grid)
			//more generally - we will add the selected masses to the current transformation
			//note - some masses will map the the same position (we will want to discard/reinstate them when hooking up springs)

			sm := maps.Keys(dev.selectedMasses)
			transformed := make(map[*mass.Mass]*mass.Mass)

			for m := range sm {
				tp := m.P.ReflectInPlane(dev.Grid.Origin, dev.Grid.Normal())
				//is there already one at the transformed point?
				transformed[m] = mass.FindAt(gm.Masses, tp, 0.01) //some masses (those on the plane) will map to themselves
				if transformed[m] == nil {
					//nop, make a new mass
					transformed[m] = mass.New(gm.Masses, int32(len(gm.Masses)), tp, m.R, m.Fixed, m.IsCoin, m.Collideable, m) //add 'shadow' mass
				}
			}
			dev.regenTransformed(gm.Masses) //(re)mirror all transformed masses (in the grid plane)

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
			for _, s := range dev.currentThing.Springs {
				//todo - if only one end is selected (and transformed) we should still create a spring
				tm1 := transformed[s.M1]
				tm2 := transformed[s.M2]
				if tm1 != nil && tm2 != nil {

					if tm1 == tm2 {
						log.Logit("spring to self")
					}
					dev.currentThing.AddSpring(tm1, tm2, s.RestLength, s.Collideable, actuator.NONE)

				}
			}
			dev.sendThings([]*thing.Thing{dev.currentThing})

		} else if k == "Escape" {
			if dev.mode == stretching {
				dev.springCursor.R = 0
				dev.sendMasses([]*mass.Mass{dev.springCursor}, true)

				gm.DeleteLastMass()
				dev.currentThing.DeleteLastSpring()
				dev.sendThings([]*thing.Thing{dev.currentThing}) //one less spring
				dev.setMode(adding)
			} else if dev.mode == moving { // cancel a move
				for m := range dev.selectedMasses {
					m.P = dev.moveStart
				}

				s := slices.Collect(maps.Keys(dev.selectedMasses))
				dev.sendMasses(s, false)
				dev.setMode(editing)
			} else if dev.mode == props {
				dev.boundValues = make(map[string]*float64, 0)
				dev.clearContextMenu()
				dev.setMode(editing)

			} else {

				//pressing escape to run
				dev.SnapMasses(gm.Things)
				dev.sendThings(gm.Things)

				dev.BindMixers(dev.ViewingPlayer.GetVehicle())

				gm.Running = !gm.Running
				log.Logit("running", gm.Running)
			}

		} else if kl == "m" {
			dev.setMode(startMove)
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
			selectedMass := slices.Collect(maps.Keys(dev.selectedMasses))[0]

			if dev.highlit.mass != nil && len(dev.selectedMasses) == 1 && dev.highlit.mass != selectedMass {
				switch k {
				case "z":
					m := selectedMass
					wr := dev.highlit.mass
					m.WingRoot = wr
					span := m.P.Sub(m.Axle.P).Length()
					chord := m.Axle.P.Sub(m.WingRoot.P).Length()
					m.WingArea = span * chord

					if dev.currentThing != nil {
						dev.currentThing.SetVelocity(aero.TestFlight)
						gm.FlyMasses() //*pretend* we are flying at 20ms
						dev.SendVectors(gm)
					}

					dev.Notify("Wing root defined", "green")
				case "x":
					if selectedMass.Axle == dev.highlit.mass {
						selectedMass.Axle = nil //remove the axle
						dev.Notify("Axle removed", "green")
					} else {
						selectedMass.Axle = dev.highlit.mass
						dev.Notify("Axis/Axle defined", "green")
					}
				}
				dev.sendMasses([]*mass.Mass{selectedMass}, true)

			} else {
				dev.Notify("Select one mass, and higlight another when setting axes", "red")
			}
		} else if kl == "l" { //flip the lift direction of the highlit mass (wing)

			dev.SnapMasses(gm.Things)
			if dev.highlit.mass != nil {
				dev.highlit.mass.Flip = !dev.highlit.mass.Flip
				dev.currentThing.SetVelocity(aero.TestFlight)
				gm.FlyMasses() //*pretend* we are flying at 20ms
				dev.SendLabels()
			}
		} else if kl == "i" { //turin on AoA labels on wings, and brake force on brake masses, extension on spring actuators
			dev.labels = make([]*label.Label, 0)

			for _, m := range gm.Masses {
				if m.WingRoot != nil {
					label.New(dev.labels, "AOA", m, m, 1, 30, &m.AoaDegrees)
				}
				if m.ActuatorTag == actuator.LeftWheelBrake || m.ActuatorTag == actuator.RightWheelBrake {
					label.New(dev.labels, "BRK", m, m, 5, 20, &m.Brake)
				}
			}
			if dev.currentThing == nil {
				if gm != game.None {
					dev.currentThing = gm.Things[0]
				}
			}
			for _, s := range dev.currentThing.Springs {
				if s.ActuatorTag > 0 {
					label.New(dev.labels, actuator.SpringActuators[s.ActuatorTag], s.M1, s.M2, 5, 20, &s.Expansion)
				}
			}

			dev.SendLabels()

		} else if kl == "p" {
			m := dev.highlit.mass
			if m != nil {
				m.Fixed = !m.Fixed
				dev.sendMasses([]*mass.Mass{m}, true)
			}

		} else if k == "Delete" {
			if dev.highlit.mass == nil && dev.highlit.spring != nil {
				dev.highlit.thing.DeleteSpring(dev.highlit.spring)
				dev.sendThings([]*thing.Thing{dev.highlit.thing})
			} else if dev.highlit.mass != nil {
				//if viewer.highlit.mass.NotAttached() {
				dev.highlit.mass.R = 0
				dev.sendMasses([]*mass.Mass{dev.highlit.mass}, true)

				dev.highlit.mass.Delete(gm.Masses) //less than straightforward
				dev.sendMasses(gm.Masses, true)
				dev.sendThings(gm.Things)
				//}
			}
		}
	}

	//sends any transform of the skin (done with cursor keys and/or shift)
	if dx != 0 || dy != 0 {
		if dev.currentThing != nil {
			ct := dev.currentThing
			if prop == "rotation" {
				//ct.Rotation += dx
			} else { //if prop=="scale" {
				ct.MeshScale.X += dx
				ct.MeshScale.Y += dy
			}
			dev.sendThings([]*thing.Thing{ct}) //will need to send the offset, rotation and scale of the mesh/skin
		}
	}

	return nil
}

func (dev *Device) SendVectors(game *game.Game) {
	dev.Send(mass.VectorsAsMsg(game.Masses)) //send the new vectors
}

// Snapmasses - 	where we have mirrored, or rotationally copied springs - collapse the coincident masses and rewire the springs
func (dev *Device) SnapMasses(things []*thing.Thing) {

	for _, t := range things {
		t.Rewire()
	}

}

func (dev *Device) makeNextSpring(masses []*mass.Mass) {
	if dev.highlit.mass == nil {
		dev.highlit.mass = mass.New(masses, int32(len(masses)), dev.spacePos, .05, false, false, true, nil)
	}

	m1 := dev.highlit.mass
	m2 := mass.New(masses, int32(len(masses)), m1.P.Clone().Add(vec.NewVec3(0, .001, 0)), 0.05, false, false, true, nil)

	dev.springCursor = m2

	dev.highlit.spring = dev.currentThing.AddSpring(m1, m2, 1, 1, actuator.NONE)
	log.Logit("made spring", m1.Index, m2.Index)

	//p.recordMassPositions() //we need to (re) do this as we have added masses

	// p.selectedMasses = make(map[*mass.Mass]bool)
	// p.selectedMasses[m2] = true
	dev.sendMasses([]*mass.Mass{m1, m2}, false)
	dev.sendThings([]*thing.Thing{dev.currentThing})
	dev.sendHighlit()

}

// // SendToPeers sends messages (masses, vectors etc) to other viewers in the same game
// func (viewer *Viewer) sendToPeers(globalViewers []*Viewer, msgs ...*msg.Msg) {
// 	for _, peer := range globalViewers {
// 		if peer.Player.Game == viewer.Player.Game && peer != viewer {
// 			peer.Send(msgs...)
// 		}
// 	}
// }

func (dev *Device) bindValue(key string, valuePointer *float64, min float64, max float64, step float64, labelSet byte) {

	dev.boundValues[key] = valuePointer //store the address of the value to be updated

	m := msg.NewMsg(msg.BindValue, key, *valuePointer, min, max, step, labelSet)
	dev.Send(m) //we will receive msg.ValueChange messages back

}

func (dev *Device) ReleaseWebSocket() {
	dev.WebSocket = nil
}

func redact(s string) string {
	// obscure every other character (replace characters at odd rune indices with '*')
	r := []rune(s)
	for i := 1; i < len(r); i += 2 {
		r[i] = '*'
	}
	return string(r)
}

func ReConnect(m *msg.Msg, globalDevices map[uint32]*Device, globalPlayers map[uint32]*player.Player, ws *websocket.Conn) (*errorplus.Event, *Device) {

	device, evt := NewFromConnectDeviceMsg(m, globalDevices, player.None, ws)

	if device == nil {
		return evt, nil
	}

	if device != None { //this is device.none (it's just were' in the device namespace)
		if device.owner != player.None {
			response := msg.Empty()
			device.homeScreen(response, globalDevices, globalPlayers)
			device.Send(response)
		} else {
			//send sign in/up options
			response := msg.Empty()
			device.signInUpScreen(response, globalDevices, globalPlayers)
			device.Send(response)
		}
	}

	return evt, device
}
func NewFromConnectDeviceMsg(ibm *msg.Msg, globalDevices map[uint32]*Device, nobody *player.Player, ws *websocket.Conn) (device *Device, error *errorplus.Event) {

	if ibm.MsgType != msg.ConnectDevice {
		return nil, errorplus.New(nil, errorplus.Error, fmt.Sprintf("First message must be connectdevice was %T %v", ibm.MsgType, ibm.MsgType)) //connects an existing or new device (viewer/controller)
	}

	deviceId := uint32(0)
	vTok := ""
	ibm.Read(&deviceId, &vTok)

	if deviceId == 0 {
		// a new unknown device - create a new device,

		newDevice, err := makeNewDevice(globalDevices, ws)
		if vTok != "" {
			newDevice.Warn(fmt.Sprintf("New device provided non empty token %v - ignoring", redact(vTok)), errorplus.Warn)
		}

		return newDevice, err

	} else {
		//we're reconnecting an existing device
		mutex.Devices.RLock()
		d, present := globalDevices[uint32(deviceId)]
		mutex.Devices.RUnlock()

		time.Sleep(time.Millisecond * 500) //don't provide a response instanltly - to make brute forcing harder
		if !present {
			//device.WebSocket.Close()
			remade, _ := makeNewDevice(globalDevices, ws)

			remade.Warn(fmt.Sprintf("No such device (%v) to reconnect.. remade as %v", deviceId, remade.Id), errorplus.Warn)
			return remade, nil
		}
		if d.token != vTok {
			d.WebSocket = ws
			d.Notify("Device token mismatch on reconnect - please refresh", "red")
			d.Warn(fmt.Sprintf("Device token does not match got %v, want %v", redact(vTok), redact(d.token)), errorplus.Warn)
			//Mismatch could be hacking, or loss of database
			//either way - reset the device (send it a new ID and token)
			makeNewDevice(globalDevices, ws) //reset the device ready for adoption on signin/up

			d.ReleaseWebSocket()
			return nil, errorplus.New(nil, errorplus.Warn, fmt.Sprintf("Device token does not match got %v, want %v", redact(vTok), redact(d.token)))
		}
		//it's a valid token and known viewer
		d.WebSocket = ws

		return d, nil
	}
}

func makeNewDevice(globalDevices map[uint32]*Device, ws *websocket.Conn) (*Device, *errorplus.Event) {

	ndid := next.Id("device")
	_, present := globalDevices[ndid]
	if present {
		return nil, errorplus.New(nil, errorplus.Error, fmt.Sprintf("Generated next device ID %v already present", ndid))
	}

	nobody := player.None
	newDevice := New(globalDevices, ndid, "Name me!", nobody, nobody, nobody, nobody, ws)
	err := newDevice.Persist()
	if err != nil {
		return newDevice, err
	}

	idMsg := msg.NewMsg(msg.DeviceId, newDevice.Id, newDevice.token)
	newDevice.Send(idMsg) //send the device id and token to the viewer - they are connected - sign in/up/or watch is next
	newDevice.Notify(fmt.Sprintf("Connected as new device: %v", newDevice.Id), "green")
	return newDevice, nil
}

func (dev *Device) ProcessBinaryMsg(ibm *msg.Msg, globalGames map[uint32]*game.Game, globalPlayers map[uint32]*player.Player, globalDevices map[uint32]*Device) (*errorplus.Event, *Device) {

	var myGame *game.Game = nil
	if dev.ViewingPlayer != nil {
		myGame = dev.ViewingPlayer.Game
	}

	if dev.Id == 0 && ibm.MsgType != msg.ConnectDevice {
		return errorplus.New(nil, errorplus.Warn, fmt.Sprintf("Device must connect first, anonymous/nobody device tried to send %T %v", ibm.MsgType, ibm.MsgType)), dev
	}

	if dev.ViewingPlayer == nil && !ibm.IsOneOf(msg.SignUp, msg.SignIn, msg.ConnectDevice) {
		return errorplus.New(nil, errorplus.Warn, "A viewer must sign up or sign in first"), dev

	}

	switch ibm.MsgType {

	case msg.ConnectDevice:
		return errorplus.New(nil, errorplus.Critical, "connect device should be processed elsewhere"), dev

	case msg.SignOut:
		dev.owner = player.None
		dev.Persist()
		response := msg.Empty()
		dev.signInUpScreen(response, globalDevices, globalPlayers)
		dev.Send(response)
		return errorplus.New(nil, errorplus.Info, "Signed out"), dev

	case msg.SignUp:

		playerName, email, password := "", "", ""
		ibm.Read(&playerName, &email, &password)
		salt := player.Salt()               //generate a new salt
		hash := player.Hash(password, salt) //hash the password with the (additonal) salt
		token := player.Salt()              //generate a new token

		npid := next.Id("player")
		_, present := globalPlayers[npid]
		if present {
			return errorplus.New(nil, errorplus.Error, "Generated player ID already present (signUp)"), dev
		}
		newPlayer := player.New(globalPlayers, npid, playerName, email, hash, salt, token, 0, 0, 0, nil)
		err := newPlayer.Persist()
		if err != nil {
			return err, dev
		}

		//adopt the device
		dev.owner = newPlayer
		dev.ViewingPlayer = newPlayer
		dev.primaryControls = newPlayer
		dev.secondaryControls = newPlayer
		dev.Persist()

		response := msg.NewMsg(msg.PlayerId, newPlayer.Id, newPlayer.Token)

		dev.homeScreen(response, globalDevices, globalPlayers)

		dev.Send(response)
		dev.Notify("Player "+playerName+" created OK", "green")
		dev.Warn("Player "+playerName+" created OK", errorplus.Info)

	case msg.SignIn: //signs a Player in (so that they can manage devices) - generally players remain signed in
		//when signed out a devices owner is set to player.None
		playerName, password := "", ""
		ibm.Read(&playerName, &password)
		playerName = strings.ToLower(playerName)

		for _, p := range globalPlayers { //TODO - index players by name
			if strings.ToLower(p.Name) == playerName {
				//found the player - check the password

				if p.CheckPassword(password) {
					//adopt the device
					dev.owner = p
					dev.Persist()
					response := msg.Empty()
					dev.homeScreen(response, globalDevices, globalPlayers)
					dev.Send(response)
					return nil, dev
				} else {
					time.Sleep(time.Millisecond * 1500) //delay to make brute forcing harder
					dev.Notify("Wrong password", "red")
					dev.Warn("Password incorrect", errorplus.Warn)
					return nil, dev

				}
			}
		}

		dev.Notify("No such player "+playerName, "red")
		return errorplus.New(nil, errorplus.Warn, "no such player "+playerName), dev

	case msg.CreateGame: //creates a game

		playerId := uint32(0)
		playerName := ""
		gameName := ""
		ibm.Read(&playerId, &playerName, &gameName) //who will 'own' this game

		player, present := globalPlayers[playerId]
		if !present {
			return errorplus.New(nil, errorplus.Warn, "No such player "+fmt.Sprint(playerId)), dev
		}
		if player.Name != playerName {
			return errorplus.New(nil, errorplus.Warn, "Player name does not match"), dev
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
		dev.Send(ogm) //send the land

		log.Logit("Runway land made between", landPos, "and", landPos.Add(runwayVec))

		player.Game = newGame
		dev.ViewingPlayer = player

		dev.startIn(newGame)

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
			return errorplus.New(nil, errorplus.Warn, "Received a valuechange outside of a game"), dev
		}
		dev.SetBoundValue(key, value, myGame.Masses, myGame.Things)

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
		dev.Notify("loaded "+filename, "green")
		dev.startIn(gm)
		return errorplus.New(nil, errorplus.Info, "Loaded game"), dev

	case msg.Save:
		filename := ""
		ibm.Read(&filename)
		dev.ViewingPlayer.Game.Save(filename, dev.selectedMasses)
		dev.Notify("Saved OK", "green")
		return errorplus.New(nil, errorplus.Info, "Saved game"), dev

	case msg.ControlPositions:

		blobs := byte(0)
		ibm.Read(&blobs)
		var blobId, x, y = byte(0), byte(0), byte(0)
		for i := 0; i < int(blobs); i++ {
			ibm.Read(&blobId, &x, &y)

			dev.SetControlInputsFromBlob(blobId, float64(x), float64(y))

		}
		dev.updateActuators()

	default:
		return errorplus.New(nil, errorplus.Error, fmt.Sprintf("Inbound binary message not implemented: %v", ibm.MsgType)), dev

	}

	return nil, dev

}

func (dev *Device) startIn(game *game.Game) {
	//state.send(nil, &reply{Cmd: "playerJoined", Payload: player})    //tell everyone about the new player
	dev.SendCamera()                  //send the camera position
	dev.sendMasses(game.Masses, true) //send all the masses
	dev.sendThings(game.Things)
	dev.SendLabelSets()

	dev.sendGameId(game.Id) //game id starts it running

	//	controlTokens[p] = p.sendControlPin() //send a PIN to them so they can take control from another device
}

func (dev *Device) updateActuators() {
	//note that the mixer contains the bound actuators (springs/masses/engines)
	//  so need no knowledge of the vehicle
	for _, mix := range dev.mixers {
		mix.ZeroOutputs()
	}

	for _, mixer := range dev.mixers {
		mixer.Mix(dev.controls) //add in defelctions
	}
}

func (dev *Device) GetVehicle(players map[uint32]*player.Player) *thing.Thing {
	if dev.ViewingPlayer == nil {
		return nil
	}
	return dev.ViewingPlayer.GetVehicle()

}

func (dev *Device) signInUpScreen(response *msg.Msg, globalDevices map[uint32]*Device, globalPlayers map[uint32]*player.Player) {
	response.Write(msg.ReplaceDiv, "content")
	//If they sign out of their account,
	//They can sign in to another and this device will be adopted
	//A device can only be owned by one player at a time
	// (used when several players share a machine, such as a web cafe)
	response.Write("<h1>Welcome Hero!</h1>")
	response.Write("<h2>Please sign in if you have an account, or sign up to create one</h2>")
	response.Write("<h3>You fly a quick training mission, or spectate games without an account</h3>")
	deviceList(dev.owner, response, globalDevices)
	gamesInProgress(response, globalPlayers)

	html.InputBox(response, "un", "Enter your username")
	html.InputBox(response, "pw", "Enter your password")
	html.Button(response, "si", "Sign In", `sm(mt.SignIn,'un','pw','em')`)
	html.Literal(response, " or ")
	html.Button(response, "su", "Sign up", `sm(mt.CreatePlayer,'un','em','pw')`)

}

func (dev *Device) homeScreen(response *msg.Msg, globalDevices map[uint32]*Device, globalPlayers map[uint32]*player.Player) {
	response.Write(msg.ReplaceDiv, "content")
	welcome(dev.owner, response)
	deviceList(dev.owner, response, globalDevices)
	observers(dev.owner, response, globalDevices)
	gamesInProgress(response, globalPlayers)
	html.Button(response, "so", "Sign Out", `sm(mt.SignOut)`)
}

func (dev *Device) startEngine(index int, game *game.Game, response *msg.Msg) *errorplus.Event {
	if dev.ViewingPlayer == nil {
		return errorplus.New(nil, errorplus.Info, "Engine start whilst not viewing a player")
	}
	vehicle := dev.ViewingPlayer.GetVehicle()
	if vehicle == nil {
		return errorplus.New(nil, errorplus.Info, "Engine start - Player is not in a vehicle")
	}

	if index < 0 || index >= len(vehicle.Engines) {
		return errorplus.New(nil, errorplus.Info, "Engine start - No such engine")
	}

	vehicle.Engines[index].Start(game.Sounds, response)

	return nil
}

func (dev *Device) SetControlInputsFromBlob(blobId byte, x float64, y float64) {

	switch blobId {

	case 1:
		dev.controls[input.StickX] = float64(x)/128 - 1 //normalise to +/- 1
		dev.controls[input.StickY] = float64(y)/128 - 1
		log.Logit("right stick", dev.controls[input.StickX], dev.controls[input.StickY])

	case 2:
		dev.controls[input.Rudder] = float64(x)/128 - 1
		dev.controls[input.Throttle] = float64(y)/128 - 1
		log.Logit("left stick", dev.controls[input.Rudder], dev.controls[input.Throttle])

	case 3:
		dev.controls[input.WheelBrakeLeft] = float64(y)/128 - 1

	case 4:
		dev.controls[input.WheelBrakeRight] = float64(y)/128 - 1

	default:
		log.Logit("warning - unhandled blob id ", blobId)
	}

}

func (dev *Device) BindMixers(vehicle *thing.Thing) {
	// For each mixer, set the mixers, mass and engine (output) the the actuator in the vehicle

	for _, mx := range dev.mixers {

		mx.Spring = vehicle.FindSpringActuator(mx.Actuator)
		mx.Engine.Spring = vehicle.FindSpringActuator(mx.Actuator)

		if mx.Spring == nil { //we didnt bind it to a spring - try a mass
			mx.Mass = vehicle.FindMassActuator(mx.Actuator)
		}
	}

}

func (dev *Device) RevokeButton() string {
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
	html.TableHead(response, "ID", "Name", "Device", "Role", "Connected", "Remove")
	for _, d := range globalDevices {
		if d.ViewingPlayer != nil && d.ViewingPlayer == player && d.owner != player {
			html.TableRow(response, fmt.Sprintf("%d", d.owner.Id), d.owner.Name, d.Name, Role(player, d), d.Status(), d.RevokeButton())
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
	html.TableHead(response, "ID", "Name", "PoV", "Connected", "Action")

	for _, d := range globalDevices {
		if d.ViewingPlayer != nil && d.ViewingPlayer == player && d.owner == player {
			html.TableRow(response, fmt.Sprintf("%d", d.Id), d.Name, d.pov, d.Status(), d.RevokeButton())
		}
	}

}

func gamesInProgress(response *msg.Msg, globalPlayers map[uint32]*player.Player) {
	response.Write("<h2>Games in progress:</h2>")
	response.Write("<table>")
	html.TableHead(response, "ID", "Players")

	pbg := playersByGame(globalPlayers)
	for g, players := range pbg {
		html.TableRow(response, fmt.Sprintf("%d", g.Id), fmt.Sprintf("%d", len(players)))
		response.Write("<tr><td colspan='2'>")
		html.TableHead(response, "Player ID", "Name")
		for _, p := range players {
			html.TableRow(response, fmt.Sprintf("%d", p.Id), fmt.Sprintf("%v", p.Name))
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
