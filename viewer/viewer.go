package viewer

import (
	"github.com/nickax/gofu/cam"

	"sync"

	"github.com/gorilla/websocket"
	"github.com/nickax/gofu/fiz/mass"
	"github.com/nickax/gofu/fiz/thing"
	"github.com/nickax/gofu/game/label"
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/game/player"
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/vec"
)

type Viewer struct {
	ws      *websocket.Conn
	pov     string      //point of pilot, copilot, instruments, overhead panel, satellite etc
	Camera  *cam.Camera //initially a clone of viewPoint(within the vehicle) - Ongoing, additional position direction and up in vehicle space (our head swivel/slew)
	lastCam *cam.Camera //where were we positioned/looking when we last sent an update
	mtx     *sync.Mutex // a mutex is required to 'lock' access to each users connection (for writing)
	InMtx   *sync.Mutex //inbound mutex, ensure only one command is processed at a time

}

// New a viewer clones a named camera from the vehicle (thing.cameras map[string]*cam.Camera),
// and controls it via a socket
// viewers are sent the entire scene, and all subsequent updates
func New(viewers []*Viewer, vehicle *thing.Thing, pov string, ws *websocket.Conn) *Viewer {

	cam := cam.New(vec.NewVec3(0, 0, 0), vec.NewVec3(1, 0, 0), vec.NewVec3(0, 1, 0)) //default camera if none found
	if vehicle != nil {
		vehicle.FindCam(cam, pov)
	}
	v := &Viewer{
		ws:     ws,
		pov:    pov,
		Camera: cam.Clone(),
	}
	viewers = append(viewers, v)
	return v
}

func (viewer *Viewer) SendCamera() {

	msg := msg.NewMsg(msg.Camera)
	viewer.Camera.WriteTo(msg)

	viewer.Send(msg)

}

func (viewer *Viewer) sendBytes(msg []byte) {

	if viewer.ws == nil {
		log.Logit(viewer.pov + " viewer socket is disconnected")
		return
	}

	viewer.mtx.Lock()         //<<---MUTEX
	defer viewer.mtx.Unlock() //deferred unlock
	messageType := websocket.BinaryMessage

	//logit("sent", len(msg), " binary bytes")
	viewer.ws.WriteMessage(messageType, msg) //write the message (and return any error)
}

func (viewer *Viewer) ViewChangedSignificantly() bool {
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

func (viewer *Viewer) Send(msg ...*msg.Msg) {
	for _, msg := range msg {
		viewer.sendBytes(msg.AllBytes())
	}

}

func (viewer *Viewer) Start(gameId uint32, masses []*mass.Mass, things []*thing.Thing, selectedMasses map[*mass.Mass]bool) {

	viewer.SendCamera()
	viewer.Send(player.Grid.AsMsg())

	player.currentThing = things[0]

	viewer.sendMasses(masses, true, selectedMasses)
	viewer.sendThings(things) //[]*thing.Thing{p.vehicle}) //sends mesh name and springs
	viewer.sendGameId(gameId) //game id starts it running
	viewer.SendLabelSets()
	viewer.vehicle = things[0]
	viewer.vehicle.SetVelocity(aero.TestFlight)
	viewer.Send(viewer.Vectors(masses))
	viewer.sendCentreOfMass(viewer.currentThing)

	viewer.Notify("Loaded", "info")
}

func (viewer *Viewer) sendThings(things []*thing.Thing) {

	msg := msg.NewMsg(msg.Things, uint32(len(things))) //placeholder for number of things

	for _, thing := range things {
		thing.WriteTo(msg)
	}
	viewer.Send(msg)

}

func (viewer *Viewer) SendLabels(labels []*label.Label) {

	msg := msg.NewMsg(msg.Labels)
	msg.Write(uint16(len(labels)))
	for _, l := range labels {
		l.WriteTo(msg)
	}
	viewer.Send(msg)
}

func (viewer *Viewer) sendMasses(masses []*mass.Mass, withDetail bool, selectedMasses map[*mass.Mass]bool) {

	msg := mass.MassesAsMsg(masses, withDetail, selectedMasses)

	viewer.Send(msg)

}

func (player *Player) sendClear() {
	msg := msg.NewMsg(msg.Clear)
	player.Send(msg)
}
