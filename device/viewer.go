package device

import (
	"fmt"
	"maps"
	"math"
	"math/rand"
	"slices"
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
	"github.com/nickax/gofu/terrain"
	"github.com/nickax/gofu/vec"
)

type Device struct {
	Id        int32
	Name      string
	token     string          //used to authenticate a device
	WebSocket *websocket.Conn //if this is nil, they are diconnected
	//viewingPlayerId int32
	Player  *player.Player
	pov     string      //point of pilot, copilot, instruments, overhead panel, satellite etc
	Camera  *cam.Camera //initially a clone of viewPoint(within the vehicle) - Ongoing, additional position direction and up in vehicle space (our head swivel/slew)
	lastCam *cam.Camera //where were we positioned/looking when we last sent an update

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

func (viewer *Device) GetPlayer() *player.Player {
	return viewer.Player
}

// New viewers are initially created watching noone - the socket is bound (so we can respond)
// id is either a known viewer id and correct token, OR -1 which will create a viewer/device and send ID/token back
func New(viewers map[uint32]*Device, id int32, player *player.Player, ws *websocket.Conn) *Device {
	//pov string, ws *websocket.Conn, gridOrigin *vec.V3, controls map[input.ControlInput]float64) *Viewer {

	gridOrigin := vec.NewVec3(0, 0, 0)
	gridX := vec.NewVec3(1, 0, 0)
	gridY := vec.NewVec3(0, 0, 1) //this is a bit confusing but the 2d grid is initialised on the word xz plane

	v := &Device{
		//viewingPlayerId: -1,
		Id:        id,
		Player:    player, //can be nil at the very begining -
		token:     fmt.Sprintf("%06d", rand.Int31n(999999)),
		WebSocket: ws,
		pov:       "none",
		Camera:    cam.New(vec.NewVec3(0, 0, 0), vec.NewVec3(1, 0, 0), vec.NewVec3(0, 1, 0)), //default camera if none found
		gridPos:   vec.NewVec3(0, 0, 0),

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
		mtx:            &sync.Mutex{},
		InMtx:          &sync.Mutex{},
		mixers:         mixer.StandardMixers,
	}

	//create and set control inputs for all 'channels'
	for i, _ := range input.InLabels {
		v.controls[input.ControlInput(i)] = 0
	}

	if v.Id > -1 {
		mutex.Devices.Lock()
		viewers[uint32(v.Id)] = v
		mutex.Devices.Unlock()
	}
	return v
}

// Watch a players vehicle from a given pov (defined within that vehicle)
func (viewer *Device) Watch(player *player.Player, pov string) {

	v := player.GetVehicle()
	if v != nil {
		cam := v.FindCam(pov)
		if cam != nil {
			viewer.Camera = cam
			viewer.pov = pov //the vehicle.cameras[pov] can be used to find the 'home' position
		}
	}

}

func (viewer *Device) clearContextMenu() {
	//clear the context menu (on the client)
	m := msg.NewMsg(msg.ClearContextMenu)
	viewer.Send(m)

}

// collects and sends the flames visible to this viewer
func (viewer *Device) GetFlames(fire *terrain.TriMesh, message *msg.Msg) {

	tcs := mesh.NewTcs(0, 1, 1, 0)                    //texture atlas coordinates
	flameMesh := mesh.New(201, "flame", 10000, 30000) //10k faces, 30k verts

	//these are no in game.lands
	fire.Root.GetFlames(viewer.landTri, fire, flameMesh, viewer.Camera, tcs)

	flameMesh.WriteTo(message, 1)
	//
	// viewer.Send(flameMesh.ToMsg(1))

}

func (viewer *Device) SendCamera() {

	msg := msg.NewMsg(msg.Camera)
	viewer.Camera.WriteTo(msg)
	viewer.Send(msg)

}

func (viewer *Device) GetLandRoot() *terrain.Tri {
	return viewer.landTri
}

func (viewer *Device) processMouseMove(game *game.Game) { //isRunning bool, masses []*mass.Mass, things []*thing.Thing) {

	if game.Running == false {

		//a point on the far plane (where the mouse cursor is pointing)

		if viewer.buttons == 2 { //panning camera

			delta := (viewer.cursor.Sub(viewer.grab)).Mul(2)

			if delta.LengthSq() != 0 {

				//logit("delta", delta.x, delta.y)

				camRight := viewer.downCam.Direction.Cross(viewer.downCam.Up).Normalise()

				viewer.Camera.Up = viewer.downCam.Up.RotateAbout(camRight, delta.Y).Normalise()
				pitched := viewer.downCam.Direction.RotateAbout(camRight, delta.Y)
				yawed := pitched.RotateAbout(viewer.Camera.Up, -delta.X)
				//viewer.camUp = viewer.camUp.rotateAbout(viewer.downCamUp, delta.X).normalise()
				viewer.Camera.Direction = yawed
				viewer.Camera.Up = vec.NewVec3(0, 1, 0) //auto level the camera

				// worldUp := NewVec3(0, 1, 0)
				// camDir := (viewer.camLookAt.sub(viewer.camPosition)).normalise()
				// viewer.camUp = camDir.cross(worldUp).normalise().cross(camDir).normalise()

				viewer.SendCamera()
			}
			return
		}

		if viewer.mode == editing {

			if viewer.buttons == 0 {
				pickRay := ray.New(viewer.Camera.Position, viewer.Camera.FarPos)
				//cm, _ := pickRay.ClosestMass(game.Masses, viewer.springCursor) //state.ClosestMassToRay(viewer.Camera.Position, viewer.Camera.FarPos, viewer.springCursor)
				cm, _ := mass.ClosestMassToRay(game.Masses, viewer.springCursor, pickRay) //state.ClosestMassToRay(viewer.Camera.Position, viewer.Camera.FarPos, viewer.springCursor)

				if cm != viewer.highlit.mass {
					viewer.highlit.mass = cm
					viewer.sendHighlit() //might be nil
				}
			}

			//mutates the viewers gridPos and spacePos (by reference)
			viewer.Grid.UpdateGridPosAndSpacePos(viewer.Camera, viewer.zOff, viewer.gridPos, viewer.spacePos)

			pickRay := ray.New(viewer.Camera.Position, viewer.Camera.FarPos)

			closest := math.MaxFloat64
			for _, thing := range game.Things {
				//spring, d := pickRay.ClosestSpring(thing.Springs)
				spring, d := spring.ClosestSpringToRay(thing.Springs, pickRay)

				if d < closest {
					viewer.highlit.thing = thing
					viewer.highlit.spring = spring
					closest = d
				}
			}

			viewer.sendHighlit()

		} else if viewer.mode == moving {
			viewer.moveSelected(game)
		} else if viewer.mode == stretching {
			viewer.springCursor.P = viewer.spacePos.Clone() //moveSpringCursor()
			viewer.sendMasses([]*mass.Mass{viewer.springCursor}, false)
		}

		if viewer.buttons == 1 && viewer.mode == editing {
			//dragging/panning the camera
			if viewer.downGridPos != nil {
				delta := viewer.gridPos.Sub(viewer.downGridPos).Multiply(.9)
				viewer.Camera.Position = viewer.downCam.Position.Sub(delta)
				viewer.SendCamera()
			}
		}

		viewer.SendCursor()
	}
}

func (viewer *Device) sendBytes(msg []byte) {

	if viewer.WebSocket == nil {
		log.Logit(viewer.pov + " viewer socket is disconnected")
		return
	}

	viewer.mtx.Lock()         //<<---MUTEX
	defer viewer.mtx.Unlock() //deferred unlock
	messageType := websocket.BinaryMessage

	//logit("sent", len(msg), " binary bytes")
	viewer.WebSocket.WriteMessage(messageType, msg) //write the message (and return any error)
}

func (viewer *Device) ViewChangedSignificantly() bool {
	if viewer.lastCam == nil {
		return true
	}

	dist := viewer.Camera.Position.DistanceFrom(viewer.lastCam.Position)
	dir := viewer.Camera.Direction.Dot(viewer.lastCam.Direction)
	if dist > 100 || dir < .95 {
		viewer.lastCam = viewer.Camera.Clone() //store this as the new old position
		return true
	}
	return false
}

func (viewer *Device) Send(msg ...*msg.Msg) {
	for _, msg := range msg {
		viewer.sendBytes(msg.AllBytes())
	}

}

// []**gameId uint32, masses []*mass.Mass, things []*thing.Thing, selectedMasses map[*mass.Mass]bool, grid *grid.Grid) {

func (viewer *Device) SendLabels() {

	msg := msg.NewMsg(msg.Labels)
	msg.Write(uint16(len(viewer.labels)))
	for _, l := range viewer.labels {
		l.WriteTo(msg)
	}
	viewer.Send(msg)
}

func (viewer *Device) sendThings(things []*thing.Thing) {
	msg := thing.ThingsAsMsg(things)
	viewer.Send(msg)
}

func (viewer *Device) sendMasses(masses []*mass.Mass, withDetail bool) {

	msg := mass.MassesAsMsg(masses, withDetail, viewer.selectedMasses)

	viewer.Send(msg)

}

func (viewer *Device) sendClear() {
	msg := msg.NewMsg(msg.Clear)
	viewer.Send(msg)
}

func (viewer *Device) SendLabelSets() { //For options on the sliders

	sendLabelSet(viewer, 1, actuator.MassActuators)
	sendLabelSet(viewer, 2, actuator.SpringActuators)
	sendLabelSet(viewer, 3, aero.SectionNames)
}

func sendLabelSet[E actuator.ActuatorEnum | aero.Section](p *Device, idx byte, valueLabelPairs map[E]string) {

	m := msg.NewMsg(msg.LabelSet)
	m.Write(idx, byte(len(valueLabelPairs))) //index of the label set/ count of labels

	for value, label := range valueLabelPairs {
		m.Write(int32(value), label)
	}
	p.Send(m)

}

func (viewer *Device) sendControlPin() uint32 {

	m := msg.NewMsg(msg.ControlToken)
	token := randomPin()

	m.Write(token)
	viewer.Send(m)

	return token

}

func randomPin() uint32 { //TODO - Check for existing token
	return uint32(math.Round(1000 + rand.Float64()*8999))
}

// SendGameId - causes the client to start the game
func (viewer *Device) sendGameId(gameId uint32) {
	m := msg.NewMsg(msg.GameId)
	m.Write(gameId)
	viewer.Send(m)
}

func (viewer *Device) sendCentreOfMass(t *thing.Thing) {

	cg, weight := t.CentreOfMass()
	viewer.Send(msg.NewMsg(msg.CentreOfMass, t.Index, cg, float32(weight)))

}

func (viewer *Device) moveSelected(game *game.Game) {
	moveDelta := viewer.spacePos.Sub(viewer.moveStart)

	//add any movement normal to the grid to the delta
	gridNormal := viewer.Grid.Xaxis.Cross(viewer.Grid.Yaxis).Normalise()
	camDGN := gridNormal.Multiply(viewer.Camera.Position.Sub(viewer.downCam.Position).Dot(gridNormal))
	moveDelta.AddIn(camDGN)

	for m := range viewer.selectedMasses {
		p, present := viewer.massStartPos[m]
		if present {
			m.P = p.Add(moveDelta)
		} else {
			log.Logit("no startpos present for mass ", m.Index)
		}

	}

	viewer.regenTransformed(game.Masses)

	s := slices.Collect(maps.Keys(viewer.selectedMasses))
	if len(s) > 0 {
		viewer.sendMasses(s, false) //just send the new positions (not details)
	}

}

func (viewer *Device) SetBoundValue(key string, value float64, masses []*mass.Mass, things []*thing.Thing) {
	pointer := viewer.boundValues[key]
	*(*float64)(pointer) = value //cast to *float64 and then dereference/set value

	//TODO - optimise/reduce chatter
	viewer.Send(mass.VectorsAsMsg(masses)) //send the new vectors

	viewer.sendMasses(masses, true) //send the potentially) modified mass
	if viewer.currentThing == nil {
		viewer.currentThing = things[0]
	}
	viewer.sendCentreOfMass(viewer.currentThing)

}

// regenTransformed updates any masses that are transforms of other masses (e.g. on the other side of a mirror)
func (viewer *Device) regenTransformed(masses []*mass.Mass) {
	//reflected := make(map[*mass.Mass]*mass.Mass)
	for _, m := range masses {

		//this could be another transform such as a rotation
		reflect := func(p *vec.V3) *vec.V3 {
			return p.ReflectInPlane(viewer.Grid.Origin, viewer.Grid.Normal())
		}
		m.RegenFromMaster(reflect)

	}

	viewer.sendMasses(masses, true)

}

// used for sending section of the interleaved (often) floating point data that makes up vertex, normal, position and index buffers
// in a format very close to that need by the GPU (or three.js buffers)

func (viewer *Device) SendCursor() {

	msg := msg.NewMsg(msg.Cursor, viewer.cursor, viewer.gridPos, viewer.spacePos)

	if viewer.highlit.mass != nil {
		msg.Write(0) //cursor sphere radius
	} else {
		msg.Write(float32(0.05)) //cursor sphere radius
	}
	viewer.Send(msg)

}

func (viewer *Device) sendHighlit() {

	hm, ht, hs := int32(-1), int32(-1), int32(-1)
	if viewer.highlit.mass != nil {
		hm = viewer.highlit.mass.Index
	}
	if viewer.highlit.thing != nil {
		ht = int32(viewer.highlit.thing.Index)

	}
	if viewer.highlit.spring != nil {
		hs = viewer.highlit.spring.Index
	}

	msg := msg.NewMsg(msg.Highlit, hm, ht, hs)
	viewer.Send(msg)
}

func (viewer *Device) Notify(text string, severity string) {

	msg := msg.NewMsg(msg.Message, text, severity)
	viewer.Send(msg)
	log.Logit(msg, severity)

}

func (viewer *Device) recordMassPositions(masses []*mass.Mass) {
	viewer.massStartPos = make(map[*mass.Mass]*vec.V3) //reset each time
	for _, m := range masses {                         //selectedMasses {
		viewer.massStartPos[m] = m.P.Clone()
	}
}

// MoveCamera moves the camera (and any selected masses) based on key presses
func (viewer *Device) MoveCamera(game *game.Game) {

	dir := vec.NewVec3(0, 0, 0)

	speed := .5
	if viewer.keys["Alt"] {
		speed = 10
	}

	if viewer.keys["w"] {
		dir.SetZ(speed)
	}
	if viewer.keys["s"] {
		dir.SetZ(-speed)
	}
	if viewer.keys["a"] {
		dir.SetX(-speed)
	}
	if viewer.keys["d"] && viewer.keys["Control"] == false {
		dir.SetX(speed)
	}
	if viewer.keys["ArrowUp"] {
		dir.SetY(speed)
	}
	if viewer.keys["ArrowDown"] {
		dir.SetY(-speed)
	}

	camDir := viewer.Camera.Direction

	right := camDir.Cross(viewer.Camera.Up).Normalise()
	up := right.Cross(camDir).Normalise()
	delta := right.Multiply(dir.X).Add(up.Multiply(dir.Y)).Add(camDir.Multiply(dir.Z))

	if delta.LengthSq() > 0 {
		viewer.Camera.Position.AddIn(delta)
		viewer.SendCamera()
		if viewer.mode == moving {
			viewer.moveSelected(game)
		}
	}

}

func (viewer *Device) setMode(mode ModeEnum) {
	viewer.mode = mode
	log.Logit("viewers mode set to", mode)

	msg := msg.NewMsg(msg.Mode)
	msg.Write(mode)
	viewer.Send(msg)

}

func (viewer *Device) checkHighlitMass() bool {
	if viewer.highlit.mass == nil {
		viewer.Notify("Highlight a mass and press the key", "error")
		return false
	}
	return true

}

// func (viewer *Viewer) send(msg *reply) { //this is fo JSON message s- Dperecated

// 	if viewer.Socket != nil {
// 		viewer.mtx.Lock()         //<<---MUTEX
// 		defer viewer.mtx.Unlock() //deferred unlock
// 		messageType := websocket.TextMessage

// 		bytes, err := json.Marshal(msg)
// 		if err != nil {
// 			logit(err.Error())
// 			return
// 		}

// 		viewer.Socket.WriteMessage(messageType, bytes) //write the message (and return any error)
// 	} else {
// 		logit("viewer socket is disco'd")
// 	}

// }

// Tidy Remove masses not attached to a spring
func (viewer *Device) Tidy(game *game.Game) {

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
func (viewer *Device) ProcessStructuredMsg(ibm *jsonmsg.Msg, response *msg.Msg) *errorplus.Event {

	game := viewer.Player.Game

	dx, dy := float64(0), float64(0) //used for sliding skins
	prop := "offset"
	step := float64(3)

	//logit(msg.Cmd)

	switch ibm.Cmd {
	case "keyUp":
		//a key was released
		viewer.keys[ibm.Key] = false

		switch ibm.Key {
		case "ArrowLeft", "ArrowRight":
			dx = 0
		case "ArrowUp", "ArrowDown":
			dy = 0
		}

	case "mw": //mousewheel

		//viewer.Camera.Position.y += msg.Payload[0] * -0.01 //up and down

		viewer.Camera.Position.AddIn(viewer.Grid.Normal().Multiply(ibm.Payload[0] * -0.005))
		viewer.zOff += ibm.Payload[0] * -0.005

		viewer.processMouseMove(game) //*isRunning, masses, things)
		viewer.SendCamera()

	case "mm": //mouse move

		viewer.movedSinceMouseDown = true

		viewer.buttons = byte(ibm.Payload[0])
		viewer.Camera.FarPos = vec.NewVec3(ibm.Payload[1], ibm.Payload[2], ibm.Payload[3])
		viewer.cursor.X = ibm.Payload[4]
		viewer.cursor.Y = ibm.Payload[5]

		viewer.processMouseMove(game) //*isRunning, masses, things)

	case "mu":

		if viewer.buttons == 2 && viewer.movedSinceMouseDown == false {

			viewer.Send(mass.VectorsAsMsg(game.Masses)) //send the new vectors

			if viewer.highlit.mass != nil {

				//degreesToRadians := float32(180.0) / float32(math.Pi)
				viewer.boundValues = make(map[string]*float64, 0)

				//p.bindValue("radius", &p.highlit.mass.r, 0.01, 1.00, .01, 0)
				viewer.bindValue("radius", &viewer.highlit.mass.R, 0.01, 1.00, .01, 0)
				viewer.bindValue("Section", &viewer.highlit.mass.Section, 0, 1, 1, 3)
				//p.bindValue("aoa", p.highlit.mass, &p.highlit.mass.aoaRads, -20, +20, 1, 0)
				viewer.bindValue("wingArea", &viewer.highlit.mass.WingArea, 0.1, 500.00, 1, 0)
				//p.bindValue("dihedral", p.highlit.mass, &p.highlit.mass.dihedralDegrees, -10, 10, 1, 0)
				//p.bindValue("controlSurface", p.highlit.mass, &p.highlit.mass.flightOutput, 0, 10, 1, 1) //use labelt set 1 (outoput flight controls)

				viewer.bindValue("massActuator", (*float64)(unsafe.Pointer(&viewer.highlit.mass.ActuatorTag)), 0, float64(len(actuator.MassActuators)), 1, 1) //use label set 1 (mass actuator labels)
				//p.sendBoundValues() //will pop up a context menu clientside
				viewer.setMode(props)
			} else if viewer.highlit.spring != nil {
				//p.boundValues = make(map[string]boundValue, 0)
				viewer.bindValue("springActuator", (*float64)(unsafe.Pointer(&viewer.highlit.spring.ActuatorTag)), 0, float64(len(actuator.SpringActuators)), 1, 2) //use label set 2 (spring actuator labels)
				//p.bindValue("springActuator", &p.highlit.spring.actuatorTag, 0, float64(len(springActuators)), 1, 1) //use label set 1 (actuator labels)
				//p.sendBoundValues()                                                                      //will pop up a context menu clientside
				viewer.setMode(props)
			}
		}

		viewer.buttons = byte(ibm.Payload[0])

	case "md":

		viewer.buttons = byte(ibm.Payload[0])

		viewer.movedSinceMouseDown = false

		viewer.grab = viewer.cursor.Clone()

		viewer.downGridPos = viewer.gridPos.Clone()

		viewer.downCam = viewer.Camera.Clone()

		if viewer.buttons == 1 {

			if viewer.highlit.mass != nil {
				viewer.moveStart = viewer.highlit.mass.P.Clone()
			} else {
				viewer.moveStart = viewer.spacePos.Clone() //may be snapped
			}

			viewer.recordMassPositions(game.Masses)

			if viewer.mode == editing {
				//toggle selection of highlit mass
				phm := viewer.highlit.mass
				if phm != nil {
					psm := viewer.selectedMasses

					there := psm[phm]
					if there {
						delete(psm, phm)
					} else {
						psm[phm] = true
					}
					viewer.sendMasses([]*mass.Mass{phm}, true)
				}
			}

			if viewer.highlit.mass != nil {
				m := viewer.highlit.mass
				gridPlane := viewer.Grid.Plane()
				viewer.zOff = gridPlane.DistanceFrom(m.P)

				viewer.SendCursor()
			}

			if viewer.mode == startMove {

				viewer.setMode(moving)

			} else if viewer.mode == moving {
				viewer.setMode(editing)
			} else if viewer.mode == grabbingMesh {
				viewer.meshGrab = viewer.spacePos.Clone()
				viewer.setMode(offsettingMesh)
			} else if viewer.mode == offsettingMesh {
				delta := viewer.spacePos.Sub(viewer.meshGrab)
				//delta.x *= -1 //UGLY - but the scenes x axis is inverted
				viewer.currentThing.MeshOffset.AddIn(delta)
				viewer.sendThings([]*thing.Thing{viewer.currentThing})
				viewer.setMode(editing)

			} else if viewer.mode == adding {
				//we will make the spring between the highlit mass and a new mass
				//if there is no highlit mass, then we add one
				//on mouseup - we will collapse the new mass into any we are on top op
				//first (possibly highlit) mass
				viewer.makeNextSpring(game.Masses)

				viewer.setMode(stretching)

			} else if viewer.mode == stretching {

				if viewer.highlit.mass != nil {
					log.Logit("substituting mass")
					viewer.highlit.spring.M2 = viewer.highlit.mass

					//remove it clientside
					viewer.springCursor.R = 0
					viewer.sendMasses([]*mass.Mass{viewer.springCursor}, true)

					game.Masses = game.Masses[:len(game.Masses)] //delete the last mass

					viewer.sendThings([]*thing.Thing{viewer.currentThing}) //sends the new spring (once on mousedown)
				} else {
					viewer.highlit.mass = viewer.springCursor
				}

				viewer.makeNextSpring(game.Masses)

			}

		}

		//viewer.send(&reply{Cmd: "gridPos", Payload: viewer.gridPos.payload()})

	case "keyDown":

		k := ibm.Key
		kl := strings.ToLower(k)
		viewer.keys[k] = true

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
				viewer.controls[input.StickX] -= 0.05
			}
		case "ArrowRight":
			dx = +step
			if game.Running {
				viewer.controls[input.StickX] += 0.05
			}

		case "ArrowUp":
			if game.Running {
				viewer.controls[input.StickY] += 0.05
			}

			dy = +step
		case "ArrowDown":
			if game.Running {
				viewer.controls[input.StickY] -= 0.05
			}

			dy = -step //see the end of the if block for where the transform is send if dx or dy are set

		}

		switch kl {
		case "t":
			if viewer.currentThing == nil {
				viewer.currentThing = game.Things[0]
			}
			viewer.setMode(adding)
		case "y": //Tidy - permanenty snaps reflection halves together and removes unreferenced masses
			viewer.SnapMasses(game.Things)
			viewer.Tidy(game)
			viewer.sendClear()
			log.Logit("tidy")
			viewer.sendMasses(game.Masses, true)
			viewer.sendThings(game.Things)
		case "-":
			viewer.controls[input.Throttle] -= 0.05
		case "+":
			viewer.controls[input.Throttle] += 0.05
		case "b":
			//grow a bush

		case "e":
			viewer.setMode(editing)
		}

		if viewer.keys["Control"] && kl == "d" { //deselect all
			//deselect all masses
			viewer.selectedMasses = make(map[*mass.Mass]bool)
			viewer.sendMasses(game.Masses, true)
		} else if viewer.keys["Control"] && kl == "a" { //select all
			//deselect all masses
			for _, m := range game.Masses {
				viewer.selectedMasses[m] = true
			}
			viewer.sendMasses(game.Masses, true)
		} else if k == "0" { //reset z offset (from the grid)
			viewer.zOff = 0
			viewer.processMouseMove(game)
			viewer.SendCamera()
			viewer.controls[input.Throttle] = 0
		} else if k == "1" {
			//start port engine
			return viewer.startEngine(0, game, response)
		} else if k == "2" {
			//start starboard engine
			return viewer.startEngine(1, game, response)

		} else if kl == "o" {

			if viewer.checkHighlitMass() {
				viewer.currentThing.Om = viewer.highlit.mass
				viewer.sendThings([]*thing.Thing{viewer.currentThing})
			}

		} else if kl == "f" { //set the forward direction mass
			if viewer.checkHighlitMass() {
				viewer.currentThing.Fm = viewer.highlit.mass
				viewer.sendThings([]*thing.Thing{viewer.currentThing})
			} else {
				viewer.follow = !viewer.follow
			}
		} else if kl == "r" {
			if viewer.keys["Control"] {
				//rotate thing 90 degrees more
				viewer.currentThing.MeshRotation.AddIn(viewer.currentThing.MeshRotation.Normalise().Multiply(math.Pi / 2))
			} else {
				//right mass  (x axis mass) of thing mesh
				if viewer.checkHighlitMass() {
					viewer.currentThing.Rm = viewer.highlit.mass
				}
			}
			viewer.sendThings([]*thing.Thing{viewer.currentThing})

		} else if kl == "g" { //align the grid
			if viewer.keys["Control"] { //CTRL-G - toggle gravity
				game.ZeroG = !game.ZeroG
			} else {

			}
		} else if kl == "m" && viewer.keys["Shift"] {
			if viewer.currentThing == nil {
				viewer.currentThing = game.Things[0]
			}
			viewer.currentThing.MeshVisibility = 1 - viewer.currentThing.MeshVisibility
			viewer.sendThings([]*thing.Thing{viewer.currentThing})
		} else if kl == "m" && viewer.keys["Alt"] {
			viewer.setMode(grabbingMesh)

		} else if kl == "m" && viewer.keys["Control"] {
			//mirror the selected masses (in the grid)
			//more generally - we will add the selected masses to the current transformation
			//note - some masses will map the the same position (we will want to discard/reinstate them when hooking up springs)

			sm := maps.Keys(viewer.selectedMasses)
			transformed := make(map[*mass.Mass]*mass.Mass)

			for m := range sm {
				tp := m.P.ReflectInPlane(viewer.Grid.Origin, viewer.Grid.Normal())
				//is there already one at the transformed point?
				transformed[m] = mass.FindAt(game.Masses, tp, 0.01) //some masses (those on the plane) will map to themselves
				if transformed[m] == nil {
					//nop, make a new mass
					transformed[m] = mass.New(game.Masses, int32(len(game.Masses)), tp, m.R, m.Fixed, m.IsCoin, m.Collideable, m) //add 'shadow' mass
				}
			}
			viewer.regenTransformed(game.Masses) //(re)mirror all transformed masses (in the grid plane)

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
			for _, s := range viewer.currentThing.Springs {
				//todo - if only one end is selected (and transformed) we should still create a spring
				tm1 := transformed[s.M1]
				tm2 := transformed[s.M2]
				if tm1 != nil && tm2 != nil {

					if tm1 == tm2 {
						log.Logit("spring to self")
					}
					viewer.currentThing.AddSpring(tm1, tm2, s.RestLength, s.Collideable, actuator.NONE)

				}
			}
			viewer.sendThings([]*thing.Thing{viewer.currentThing})

		} else if k == "Escape" {
			if viewer.mode == stretching {
				viewer.springCursor.R = 0
				viewer.sendMasses([]*mass.Mass{viewer.springCursor}, true)

				game.DeleteLastMass()
				viewer.currentThing.DeleteLastSpring()
				viewer.sendThings([]*thing.Thing{viewer.currentThing}) //one less spring
				viewer.setMode(adding)
			} else if viewer.mode == moving { // cancel a move
				for m := range viewer.selectedMasses {
					m.P = viewer.moveStart
				}

				s := slices.Collect(maps.Keys(viewer.selectedMasses))
				viewer.sendMasses(s, false)
				viewer.setMode(editing)
			} else if viewer.mode == props {
				viewer.boundValues = make(map[string]*float64, 0)
				viewer.clearContextMenu()
				viewer.setMode(editing)

			} else {

				//pressing escape to run
				viewer.SnapMasses(game.Things)
				viewer.sendThings(game.Things)

				viewer.BindMixers(viewer.Player.GetVehicle())

				game.Running = !game.Running
				log.Logit("running", game.Running)
			}

		} else if kl == "m" {
			viewer.setMode(startMove)
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
			selectedMass := slices.Collect(maps.Keys(viewer.selectedMasses))[0]

			if viewer.highlit.mass != nil && len(viewer.selectedMasses) == 1 && viewer.highlit.mass != selectedMass {
				if k == "z" {
					m := selectedMass
					wr := viewer.highlit.mass
					m.WingRoot = wr
					span := m.P.Sub(m.Axle.P).Length()
					chord := m.Axle.P.Sub(m.WingRoot.P).Length()
					m.WingArea = span * chord

					if viewer.currentThing != nil {
						viewer.currentThing.SetVelocity(aero.TestFlight)
						game.FlyMasses() //*pretend* we are flying at 20ms
						viewer.SendVectors(game)
					}

					viewer.Notify("Wing root defined", "info")
				} else if k == "x" {
					if selectedMass.Axle == viewer.highlit.mass {
						selectedMass.Axle = nil //remove the axle
						viewer.Notify("Axle removed", "info")
					} else {
						selectedMass.Axle = viewer.highlit.mass
						viewer.Notify("Axis/Axle defined", "info")
					}
				}
				viewer.sendMasses([]*mass.Mass{selectedMass}, true)

			} else {
				viewer.Notify("Select one mass, and higlight another when setting axes", "error")
			}
		} else if kl == "l" { //flip the lift direction of the highlit mass (wing)

			viewer.SnapMasses(game.Things)
			if viewer.highlit.mass != nil {
				viewer.highlit.mass.Flip = !viewer.highlit.mass.Flip
				viewer.currentThing.SetVelocity(aero.TestFlight)
				game.FlyMasses() //*pretend* we are flying at 20ms
				viewer.SendLabels()
			}
		} else if kl == "i" { //turin on AoA labels on wings, and brake force on brake masses, extension on spring actuators
			viewer.labels = make([]*label.Label, 0)

			for _, m := range game.Masses {
				if m.WingRoot != nil {
					label.New(viewer.labels, "AOA", m, m, 1, 30, &m.AoaDegrees)
				}
				if m.ActuatorTag == actuator.LeftWheelBrake || m.ActuatorTag == actuator.RightWheelBrake {
					label.New(viewer.labels, "BRK", m, m, 5, 20, &m.Brake)
				}
			}
			if viewer.currentThing == nil {
				viewer.currentThing = game.Things[0]
			}
			for _, s := range viewer.currentThing.Springs {
				if s.ActuatorTag > 0 {
					label.New(viewer.labels, actuator.SpringActuators[s.ActuatorTag], s.M1, s.M2, 5, 20, &s.Expansion)
				}
			}

			viewer.SendLabels()

		} else if kl == "p" {
			m := viewer.highlit.mass
			if m != nil {
				m.Fixed = !m.Fixed
				viewer.sendMasses([]*mass.Mass{m}, true)
			}

		} else if k == "Delete" {
			if viewer.highlit.mass == nil && viewer.highlit.spring != nil {
				viewer.highlit.thing.DeleteSpring(viewer.highlit.spring)
				viewer.sendThings([]*thing.Thing{viewer.highlit.thing})
			} else if viewer.highlit.mass != nil {
				//if viewer.highlit.mass.NotAttached() {
				viewer.highlit.mass.R = 0
				viewer.sendMasses([]*mass.Mass{viewer.highlit.mass}, true)

				viewer.highlit.mass.Delete(game.Masses) //less than straightforward
				viewer.sendMasses(game.Masses, true)
				viewer.sendThings(game.Things)
				//}
			}
		}
	}

	//sends any transform of the skin (done with cursor keys and/or shift)
	if dx != 0 || dy != 0 {
		if viewer.currentThing != nil {
			ct := viewer.currentThing
			if prop == "rotation" {
				//ct.Rotation += dx
			} else { //if prop=="scale" {
				ct.MeshScale.X += dx
				ct.MeshScale.Y += dy
			}
			viewer.sendThings([]*thing.Thing{ct}) //will need to send the offset, rotation and scale of the mesh/skin
		}
	}

	return nil
}

func (viewer *Device) SendVectors(game *game.Game) {
	viewer.Send(mass.VectorsAsMsg(game.Masses)) //send the new vectors
}

// Snapmasses - 	where we have mirrored, or rotationally copied springs - collapse the coincident masses and rewire the springs
func (viewer *Device) SnapMasses(things []*thing.Thing) {

	for _, t := range things {
		t.Rewire()
	}

}

func (viewer *Device) makeNextSpring(masses []*mass.Mass) {
	if viewer.highlit.mass == nil {
		viewer.highlit.mass = mass.New(masses, int32(len(masses)), viewer.spacePos, .05, false, false, true, nil)
	}

	m1 := viewer.highlit.mass
	m2 := mass.New(masses, int32(len(masses)), m1.P.Clone().Add(vec.NewVec3(0, .001, 0)), 0.05, false, false, true, nil)

	viewer.springCursor = m2

	viewer.highlit.spring = viewer.currentThing.AddSpring(m1, m2, 1, 1, actuator.NONE)
	log.Logit("made spring", m1.Index, m2.Index)

	//p.recordMassPositions() //we need to (re) do this as we have added masses

	// p.selectedMasses = make(map[*mass.Mass]bool)
	// p.selectedMasses[m2] = true
	viewer.sendMasses([]*mass.Mass{m1, m2}, false)
	viewer.sendThings([]*thing.Thing{viewer.currentThing})
	viewer.sendHighlit()

}

// // SendToPeers sends messages (masses, vectors etc) to other viewers in the same game
// func (viewer *Viewer) sendToPeers(globalViewers []*Viewer, msgs ...*msg.Msg) {
// 	for _, peer := range globalViewers {
// 		if peer.Player.Game == viewer.Player.Game && peer != viewer {
// 			peer.Send(msgs...)
// 		}
// 	}
// }

func (viewer *Device) bindValue(key string, valuePointer *float64, min float64, max float64, step float64, labelSet byte) {

	viewer.boundValues[key] = valuePointer //store the address of the value

	m := msg.NewMsg(msg.BindValue)
	m.Write(key, *valuePointer, min, max, step, labelSet)
	viewer.Send(m)

}

func (viewer *Device) ReleaseWebSocket() {
	viewer.WebSocket = nil
}

func redact(s string) string {
	// obscure every other character (replace characters at odd rune indices with '*')
	r := []rune(s)
	for i := 1; i < len(r); i += 2 {
		r[i] = '*'
	}
	return string(r)
}

func (viewer *Device) ProcessBinaryMsg(ibm *msg.Msg,
	globalGames map[uint32]*game.Game,
	globalPlayers map[uint32]*player.Player,
	globalDevices map[uint32]*Device,
) *errorplus.Event {

	//  gm *game.State,
	//  games map[uint32]*game.State,
	//  players map[uint32]*player.Player)
	//  error {

	myGame := viewer.Player.Game

	if viewer.Player == nil &&
		ibm.MsgType != msg.CreatePlayer &&
		ibm.MsgType != msg.SignIn &&
		ibm.MsgType != msg.CreateGame {
		return errorplus.New(nil, errorplus.Warn, "A viewer must attach to a player early doors")

	}

	switch ibm.MsgType {

	case msg.ConnectViewer: //sends a viewer id
		viewerId := int32(0)
		vTok := ""
		ibm.Read(&viewerId, vTok)

		if viewerId == -1 {
			// a new unknown device - create a new viewer,
			v := New(globalDevices, int32(next.Id("viewer")), nil, viewer.WebSocket)
			idMsg := msg.NewMsg(msg.DeviceId, v.Id, v.token)
			v.Send(idMsg) //send the id and token to the viewer - they are connected - sign in/up/or watch is next

		} else {
			//we're reconnecting an existing viewer
			mutex.Devices.RLock()
			v, present := globalDevices[uint32(viewerId)]
			mutex.Devices.RUnlock()

			time.Sleep(1000) //don't provide a response instanltly - to make brute forcing harder
			if !present {
				viewer.WebSocket.Close()
				return errorplus.New(nil, errorplus.Warn, "No such viewer to reconnect "+fmt.Sprint(viewerId))
			}
			if v.token != vTok {
				viewer.WebSocket.Close()
				return errorplus.New(nil, errorplus.Warn, fmt.Sprintf("Viewer token does not match got %q, want %q", redact(vTok), redact(v.token)))
			}
			//it's a valid token and known viewer
			viewer = v //important !
		}

	case msg.CreatePlayer:
		player.New(globalPlayers, next.Id("player"), "", nil)

	case msg.SignIn: //signs a player in (so that they can manage devices)
	case msg.CreateGame: //creates a game

		playerId := uint32(0)
		playerName := ""
		ibm.Read(&playerId, &playerName) //who will 'own' this game

		player, present := globalPlayers[playerId]
		if !present {
			return errorplus.New(nil, errorplus.Warn, "No such player "+fmt.Sprint(playerId))
		}
		if player.Name != playerName {
			return errorplus.New(nil, errorplus.Warn, "Player name does not match")
		}

		//will make a new game with a new ID and add the player to it
		newGame := game.New(globalGames)

		runwayPos := vec.NewVec3(0, 0, 0)
		runwayVec := vec.NewVec3(1000, 0, 1000)
		newGame.SetRunway(runwayPos, runwayVec, 40)

		newGame.Ignite(vec.NewVec3(10, 0, -1000)) //note the Y position has no effect

		//Plough the runway and set the start and end heights
		ogm := msg.Empty()
		landPos, _ := newGame.MakeLand(runwayPos, runwayVec.Normalise(), ogm)
		viewer.Send(ogm) //send the land

		log.Logit("Runway land made between", landPos, "and", landPos.Add(runwayVec))

		player.Game = newGame
		viewer.Player = player

		viewer.startIn(newGame)

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
		viewer.SetBoundValue(key, value, myGame.Masses, myGame.Things)

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

		viewer.startIn(gm)
		return errorplus.New(nil, errorplus.Info, "Loaded game")

	case msg.Save:
		filename := ""
		ibm.Read(&filename)
		viewer.Player.Game.Save(filename, viewer.selectedMasses)
		viewer.Notify("Saved OK", "info")
		return errorplus.New(nil, errorplus.Info, "Saved game")

	case msg.ControlPositions:

		blobs := byte(0)
		ibm.Read(&blobs)
		var blobId, x, y = byte(0), byte(0), byte(0)
		for i := 0; i < int(blobs); i++ {
			ibm.Read(&blobId, &x, &y)

			viewer.SetControlInputsFromBlob(blobId, float64(x), float64(y))

		}
		viewer.updateActuators()

	default:
		panic("other Inbound binary messages not implemented")
	}

	return nil

}

func (viewer *Device) startIn(game *game.Game) {
	//state.send(nil, &reply{Cmd: "playerJoined", Payload: player})    //tell everyone about the new player
	viewer.SendCamera()                  //send the camera position
	viewer.sendMasses(game.Masses, true) //send all the masses
	viewer.sendThings(game.Things)
	viewer.SendLabelSets()

	viewer.sendGameId(game.Id) //game id starts it running

	//	controlTokens[p] = p.sendControlPin() //send a PIN to them so they can take control from another device
}

func (viewer *Device) updateActuators() {
	//note that the mixer contains the bound actuators (springs/masses/engines)
	//  so need no knowledge of the vehicle
	for _, mix := range viewer.mixers {
		mix.ZeroOutputs()
	}

	for _, mixer := range viewer.mixers {
		mixer.Mix(viewer.controls) //add in defelctions
	}
}

func (viewer *Device) GetVehicle(players map[uint32]*player.Player) *thing.Thing {
	if viewer.Player == nil {
		return nil
	}
	return viewer.Player.GetVehicle()

}

func (viewer *Device) startEngine(index int, game *game.Game, response *msg.Msg) *errorplus.Event {
	if viewer.Player == nil {
		return errorplus.New(nil, errorplus.Info, "Engine start whilst not viewing a player")
	}
	vehicle := viewer.Player.GetVehicle()
	if vehicle == nil {
		return errorplus.New(nil, errorplus.Info, "Engine start - Player is not in a vehicle")
	}

	if index < 0 || index >= len(vehicle.Engines) {
		return errorplus.New(nil, errorplus.Info, "Engine start - No such engine")
	}

	vehicle.Engines[index].Start(game.Sounds, response)

	return nil
}

func (viewer *Device) SetControlInputsFromBlob(blobId byte, x float64, y float64) {

	switch blobId {

	case 1:
		viewer.controls[input.StickX] = float64(x)/128 - 1 //normalise to +/- 1
		viewer.controls[input.StickY] = float64(y)/128 - 1
		log.Logit("right stick", viewer.controls[input.StickX], viewer.controls[input.StickY])

	case 2:
		viewer.controls[input.Rudder] = float64(x)/128 - 1
		viewer.controls[input.Throttle] = float64(y)/128 - 1
		log.Logit("left stick", viewer.controls[input.Rudder], viewer.controls[input.Throttle])

	case 3:
		viewer.controls[input.WheelBrakeLeft] = float64(y)/128 - 1

	case 4:
		viewer.controls[input.WheelBrakeRight] = float64(y)/128 - 1

	default:
		log.Logit("warning - unhandled blob id ", blobId)
	}

}

func (viewer *Device) BindMixers(vehicle *thing.Thing) {
	// For each mixer, set the mixers, mass and engine (output) the the actuator in the vehicle

	for _, mx := range viewer.mixers {

		mx.Spring = vehicle.FindSpringActuator(mx.Actuator)
		mx.Engine.Spring = vehicle.FindSpringActuator(mx.Actuator)

		if mx.Spring == nil { //we didnt bind it to a spring - try a mass
			mx.Mass = vehicle.FindMassActuator(mx.Actuator)
		}
	}

}
