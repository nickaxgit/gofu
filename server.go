//https://visualpde.com/sim/?options=N4IgxiBcIG4JYCc4BM4GcQBoTIKq6hAEYscA1MwgJlOQHU7CAGWjaIpz0sA6F7MJXakA1oQCGAXiYA6JgGYA7AAI4AO2UBtJpllEAugG4ARtLkA2VRu1yALLpkHDpALbjmMpa7jNSAB0IAYQBXNAAXAHsXfwQI4wBTADFgtUJgmLj4gBl4tQBzMIALKHlOHRAEXhBggD0qACoYZQBaZQAKcQBqYwBKevTsBCEQZtqGps7lcXq2ohblGB7SBEY+UjQA6HlSGChNUHEEWIB3QIiAGwjghChzAFY7+TvsQ5OAEVy0ODCATyhZZ4gV4RY45fJFACy4gAHoQSC8jiCAMpgcTneISYKRUjA44ADUIWQA+tCAPRUeawhEnACahKJP3J8z+LyxESR8TCZ0u1wASuJ8hjIAAzNFoeICcQueIIcQAFUKnPckFKkulsoACoUfCryqj1eIAFoRKL-GQADgEFyuCDcm1giBQ6G41uuxkOUFF53FVrUkWubC9PvAET9Nu5Nv+vv9CAAon4vpdUpBZEwiNGbQA5YLRSCAsChSIuJHXUVgIVBiUgeIuYwRNCBsVVmt1hsAQVrcFyYTNgJb9bQbzgwuFoSFsnM2H7Da1OqIMjuiiYlurtYHSO1aj+kA4U7XDaRLhNRTU8QbZqoe9baCRfniBfOHpTMkvq+vCqVZsU5ru9hAwvOOA-DvZAI2uO1PSbbBANPOgUEhYJzigdMQDcaEwKGNFgiFEBZC8bAXHUDCyCwnD+BACIYBlR9t0rbBKOo8QfgwqMKKohAaPjRNQzNThAQYjimNjaE-BuYR6PYmisnUeI4OQBCkJ3bA-EuMI5R+O9CBUgUMWU2JjHUPJIO9KtRMyZJk2qDIEjBApil1Tg9MydTNOgdQwniPJZSQpyEgJNZfPiOkApANBS3EcsLN8bAigQeJ4jeAAJeI4DyQowhRNFxxkRRsCosBIgQRIu3OZBjODY5CnENSIg1VS0lINQpRwkgAF99GwY4yugAYQGOe1eq+Fw5W+dFCAAcVlH5AGQCFEIjCHtWqAA

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/gorilla/websocket"
	"maps"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"time"
)

// these must be upper cased or theu don't get unmarshalled
type msg struct {
	Cmd     string    `json:"cmd"`
	Key     string    `json:"key"`
	Payload []float64 `json:"payload"`
}

// these must be uppercased for marshalling
type reply struct {
	Cmd     string      `json:"cmd"`
	Payload interface{} `json:"payload"`
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

var players = map[uint32]*player{}                                 //all players in all games game - by id
var controlTokens map[uint32]*player = make(map[uint32]*player, 0) //every login adds a random token to here to allow control by another player/device
var games map[string]*state                                        //the data of games in progress - by id
var accountsByGuid map[string]*account

func stepWorlds() {

	//gravTest()

	//every 100 ms step all worlds
	for range time.Tick(time.Millisecond * 33) { //<<waits here  //30fps
		//print(".") //<< this is the heartbeat
		for _, state := range games {
			if state.running {
				state.step() //<- this is a physics step - it queues stuff for all players

				// for _, p := range state.Players {
				// 	select { //this is a non-blocking send
				// 	case p.waitChannel <- true: //<- this is a signal to the player that the world has stepped
				// 	default:
				// 	}
				// }
			}
			state.moveCameras() //in editing mode
		}
	}

	panic(`stepWorlds() has exited`)

}

// find a random start pos on a triangular land of size +/- s
func (state *state) RandomStartPos(s float64) *vec3 {

	z := (rand.Float64() - .5) * s * 2
	xr := (1 - (z+s)/(2*s)) * s //range of x at z (to stay on dry land)
	x := (rand.Float64() - .5) * xr * 2

	//x = 5.01
	//z = 2.001
	return newVec3(x, 0, z)

}

// what is distance is the closest thing to P
func (state *state) thingClearance(p *vec3) float64 {

	clearance := float64(10000)
	var c float64
	for _, thing := range state.things {
		c = thing.distanceFrom(p)
		if c < clearance {
			clearance = c
		}
	}

	return c
}

func createGame(playerId uint32, playerName string, ws *websocket.Conn) *player {
	//create a new game
	id := fmt.Sprintf("%d", uint32(rand.Float32()*1000000))
	state := NewState(id) //asigns a random game id
	games[state.filename] = state

	s := float64(10000) //+/- 10km land = 200 km^2

	y0pos := state.RandomStartPos(s)
	p := state.AddPlayer(playerId, playerName, y0pos, ws)
	p.makeLand(y0pos, 10, 2000, s, nil, nil) //initial make to determin runway height

	state.runwayStart, _ = p.landTri.probeLand(y0pos)
	state.runwayEnd = state.runwayStart.add(newVec3(0, 0, -1200))

	origin := p.makeLand(y0pos, 10, 2000, s, state.runwayStart, state.runwayEnd) //makes and sends land

	//p.makeDozer(y0pos)

	p.grid.origin = origin
	p.grid.send(p)

	aircraft := load("wip22")
	t := state.mergeThing(aircraft.things[0])
	aircraft = nil
	p.vehicle = t

	cg, weight := t.centreOfMass()

	//t.engineSound[0] = state.qSound("engineLoop", cg, 0.2, true)
	//t.engineSound[1] = state.qSound("engineLoop", cg, 0.2, true)

	logit("aircraft weighs", weight)
	t.translate(origin.sub(cg).add(newVec3(0, 10, 0)))

	//state.scatterCoins(2000, 2000)

	p.camera.follow(p.vehicle)

	//p.updateActuators()

	p.sendCamera()

	p.sendMasses(state.masses, true)
	p.sendThings([]*thing{p.vehicle}) //sends mesh name and springs
	p.sendGameId()                    //game id starts it running
	p.sendControlPin()

	//player.send(&reply{Cmd: "state", Payload: state.payload()})

	state.makeHoles(10, 5000, 5000)

	//state.makeWater() //water is flowed and sent every cycle
	//state.sendWater()

	logit("Game created", state.filename)

	//state.save("game" + fmt.Sprint(state.gameId) + ".bson")

	return p
}

func joinGame(gameId string, playerId uint32, playerName string, ws *websocket.Conn) *player {
	state := games[gameId]
	if state == nil {
		state = load(gameId)
		games[gameId] = state
		for _, p := range state.players {
			p.mtx = &sync.Mutex{}
		}
	}

	p, present := state.players[playerId]

	if !present {
		p = state.AddPlayer(playerId, playerName, state.RandomStartPos(10000), ws)
	}

	p.socket = ws //this is important!
	//player.send(&reply{Cmd: "state", Payload: state})
	//state.send(nil, &reply{Cmd: "playerJoined", Payload: player})    //tell everyone about the new player
	p.sendCamera()                    //send the camera position
	p.sendMasses(state.masses, true)  //send all the masses
	p.sendThings([]*thing{p.vehicle}) //sends mesh name and springs
	p.sendGameId()                    //game id starts it running
	p.sendControlPin()

	//cg, _ := p.vehicle.centreOfMass()

	//state.qSound("dozer", cg, 0.1,  true)
	//t.engineSound[0] = state.qSound("engineLoop", cg, 0.2, true)
	//t.engineSound[1] = state.qSound("engineLoop", cg, 0.2, true)

	return p
}

func processCreateJoinOrControl(mb []byte, ws *websocket.Conn) *player {

	buff := bytes.NewBuffer(mb)

	cmd := [1]byte{}
	binary.Read(buff, le, &cmd)

	if cmd[0] == byte(msgCreateGame) {
		playerId := uint32(0)
		gameId := readString(buff)
		logit("gameId", gameId)          //TODO this isnt used (is 0) tidy
		binary.Read(buff, le, &playerId) //gets the player id
		playerName := readString(buff)

		return createGame(playerId, string(playerName), ws) //returns a player
	} else if cmd[0] == byte(msgJoinGame) {

		playerId := uint32(0)
		gameId := readString(buff)
		binary.Read(buff, le, &playerId) //gets the player id
		playerName := readString(buff)

		return joinGame(gameId, playerId, string(playerName), ws) //returns a player
	} else if cmd[0] == byte(msgJoinAsController) {
		token := readUInt32(buff)
		playerToControl := controlTokens[token]
		if playerToControl != nil {
			playerToControl.controllerSocket = ws
			return playerToControl //return the player to which this token maps
		}
		logit("No pin for token", token)
		return nil

	} else {
		panic("first message was not create or join")
	}

}

func processMsg(msg msg, p *player, ws *websocket.Conn) {

	//var fpn string //firstPlayer *Player

	dx, dy := float64(0), float64(0) //used for sliding skins
	prop := "offset"
	step := float64(3)

	if p == nil && msg.Cmd != "createGame" && msg.Cmd != "joinGame" {
		logit("No player for message", msg)
	}

	logit(msg.Cmd)

	if msg.Cmd == "keyUp" {
		//a key was released
		p.keys[msg.Key] = false

		if msg.Key == "ArrowLeft" || msg.Key == "ArrowRight" {
			dx = 0
		} else if msg.Key == "ArrowUp" || msg.Key == "ArrowDown" {
			dy = 0
		}
	} else if msg.Cmd == "step" {
		//<-player.waitChannel //see stepworlds fo rthe sending end which releases this

	} else if msg.Cmd == "drive" {

		// if strings.HasPrefix(player.name, "control-") {
		// 	player = state.players[strings.TrimPrefix(player.name, "control-")]
		// }
		p.leftDrive = msg.Payload[0]  //no need to echo them back - local versions are used for knobs only
		p.rightDrive = msg.Payload[1] //no need to echo them back - local versions are used for knobs only
		p.thrust = msg.Payload[2]

	} else if msg.Cmd == "mw" { //mousewheel

		//player.camera.position.y += msg.Payload[0] * -0.01 //up and down

		p.camera.position.addIn(p.grid.normal().multiply(msg.Payload[0] * -0.005))
		p.zOff += msg.Payload[0] * -0.005

		p.processMouseMove()
		p.sendCamera()

	} else if msg.Cmd == "mm" { //mouse move

		p.movedSinceMouseDown = true

		p.buttons = byte(msg.Payload[0])
		p.camera.farPos = newVec3(msg.Payload[1], msg.Payload[2], msg.Payload[3])
		p.cursor.X = msg.Payload[4]
		p.cursor.Y = msg.Payload[5]

		p.processMouseMove()

	} else if msg.Cmd == "mu" {

		if p.buttons == 2 && p.movedSinceMouseDown == false {

			if p.highlit.mass != nil {

				//degreesToRadians := float32(180.0) / float32(math.Pi)
				p.boundValues = make(map[string]boundValue, 0)
				p.bindValue("radius", &p.highlit.mass.r, 0.01, 1.00, .01, 0)
				p.bindValue("Section", &p.highlit.mass.section, 0, 1, 1, 2)
				//p.bindValue("aoa", p.highlit.mass, &p.highlit.mass.aoaRads, -20, +20, 1, 0)
				p.bindValue("wingArea", &p.highlit.mass.wingArea, 0.1, 500.00, 1, 0)
				//p.bindValue("dihedral", p.highlit.mass, &p.highlit.mass.dihedralDegrees, -10, 10, 1, 0)
				//p.bindValue("controlSurface", p.highlit.mass, &p.highlit.mass.flightOutput, 0, 10, 1, 1) //use labelt set 1 (outoput flight controls)

				p.sendBoundValues() //will pop up a context menu clientside
				p.setMode(props)
			} else if p.highlit.spring != nil {
				p.boundValues = make(map[string]boundValue, 0)
				p.bindValue("actuator", &p.highlit.spring.flightOutput, 0, float32(len(actLabels)), 1, 1) //use label set 1 (actuator labels)
				p.sendBoundValues()                                                                       //will pop up a context menu clientside
				p.setMode(props)
			}
		}

		p.buttons = byte(msg.Payload[0])

	} else if msg.Cmd == "md" {

		p.buttons = byte(msg.Payload[0])

		p.movedSinceMouseDown = false

		p.grab = &vec2{p.cursor.X, p.cursor.Y} //clone

		p.downGridPos = p.gridPos.clone()

		p.downCam = p.camera.clone()

		if p.buttons == 1 {

			if p.highlit.mass != nil {
				p.moveStart = p.highlit.mass.p.clone()
			} else {
				p.moveStart = p.spacePos.clone() //may be snapped
			}

			p.recordMassPositions()

			if p.mode == editing {
				//toggle selection of highlit mass
				phm := p.highlit.mass
				if phm != nil {
					psm := p.selectedMasses

					there, _ := psm[phm]
					if there {
						delete(psm, phm)
					} else {
						psm[phm] = true
					}
					p.sendMasses([]*mass{phm}, true)
				}
			}

			if p.highlit.mass != nil {
				m := p.highlit.mass
				p.zOff = m.p.distanceFrom(m.p.closestPointOnPlane(p.grid.origin, p.grid.normal()))

				p.sendCursor()
			}

			if p.mode == startMove {

				p.setMode(moving)

			} else if p.mode == moving {
				p.setMode(editing)
			} else if p.mode == grabbingMesh {
				p.meshGrab = p.spacePos.clone()
				p.setMode(offsettingMesh)
			} else if p.mode == offsettingMesh {
				delta := p.spacePos.sub(p.meshGrab)
				//delta.x *= -1 //UGLY - but the scenes x axis is inverted
				p.currentThing.offset.addIn(delta)
				p.sendThings([]*thing{p.currentThing})
				p.setMode(editing)

			} else if p.mode == adding {
				//we will make the spring between the highlit mass and a new mass
				//if there is no highlit mass, then we add one
				//on mouseup - we will collapse the new mass into any we are on top op
				//first (possibly highlit) mass
				p.makeNextSpring()

				p.setMode(stretching)

			} else if p.mode == stretching {

				if p.highlit.mass != nil {
					logit("substituting mass")
					p.highlit.spring.m2 = p.highlit.mass

					//remove it clientside
					p.springCursor.r = 0
					p.sendMasses([]*mass{p.springCursor}, true)

					p.state.masses = p.state.masses[:len(p.state.masses)] //delete the last mass

					p.sendThings([]*thing{p.currentThing}) //sends the new spring (once on mousedown)
				} else {
					p.highlit.mass = p.springCursor
				}

				p.makeNextSpring()

			}

		}

		//player.send(&reply{Cmd: "gridPos", Payload: player.gridPos.payload()})

	} else if msg.Cmd == "pickRay" {
		o := newVec3(msg.Payload[0], msg.Payload[1], msg.Payload[2])
		dir := newVec3(msg.Payload[3], msg.Payload[4], msg.Payload[5])
		if p.landMesh != nil {
			p := p.landMesh.slowProbe(o, o.add(dir.multiply(1000)))
			if p != nil {
				//grow a simple tree here
				m := growTree()
				m.offset(p)
				//	m.sendToAll("tree", state)
			}
		}

	} else if msg.Cmd == "keyDown" {

		k := msg.Key
		kl := strings.ToLower(k)

		p.keys[k] = true

		logit("key down", k)
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
			if p.state.running {
				p.controls[ciStickX] -= 0.05
			}

		} else if k == "ArrowRight" {
			dx = +step
			if p.state.running {
				p.controls[ciStickX] += 0.05
			}

		} else if k == "ArrowUp" {
			if p.state.running {
				p.controls[ciStickY] += 0.05
			}

			dy = +step
		} else if k == "ArrowDown" {
			if p.state.running {
				p.controls[ciStickY] -= 0.05
			}

			dy = -step //see the end of the if block for where the transform is send if dx or dy are set
		} else if kl == "t" {
			if p.currentThing == nil {
				p.currentThing = p.state.things[0]
			}
			p.setMode(adding)
		} else if kl == "y" {
			p.snapMasses()
			p.state.tidy()
			p.sendClear()
			logit("tidy")
			p.sendMasses(p.state.masses, true)
			p.sendThings(p.state.things)
		} else if kl == "-" {
			p.controls[ciThrottle] -= 0.05
		} else if kl == "+" {
			p.controls[ciThrottle] += 0.05

		} else if kl == "e" {
			p.setMode(editing)
		} else if p.keys["Control"] && kl == "d" { //deselect all
			//deselect all masses
			p.selectedMasses = make(map[*mass]bool)
			p.sendMasses(p.state.masses, true)
		} else if p.keys["Control"] && kl == "a" { //select all
			//deselect all masses
			for _, m := range p.state.masses {
				p.selectedMasses[m] = true
			}
			p.sendMasses(p.state.masses, true)
		} else if k == "0" {
			p.zOff = 0
			p.processMouseMove()
			p.sendCamera()
			p.controls[ciThrottle] = 0
		} else if k == "1" {
			//start port engine
			p.vehicle.engines[0].start()

		} else if k == "2" {
			//start starboard engine
			p.vehicle.engines[1].start()

		} else if kl == "o" {

			if p.checkHighlitMass() {
				p.currentThing.om = p.highlit.mass
				p.sendThings([]*thing{p.currentThing})
			}

		} else if kl == "f" {
			if p.checkHighlitMass() {
				p.currentThing.fm = p.highlit.mass
				p.sendThings([]*thing{p.currentThing})
			}
		} else if kl == "r" {
			if p.keys["Control"] {
				//rotate thing 90 degrees more
				p.currentThing.rotation.addIn(p.currentThing.rotation.normalise().multiply(math.Pi / 2))
			} else {
				//right mass  (x axis mass) of thing mesh
				if p.checkHighlitMass() {
					p.currentThing.rm = p.highlit.mass
				}
			}
			p.sendThings([]*thing{p.currentThing})

		} else if kl == "g" { //align the grid
			if p.keys["Control"] {
				p.state.zeroG = !p.state.zeroG
			} else {

			}
		} else if kl == "m" && p.keys["Shift"] {
			if p.currentThing == nil {
				p.currentThing = p.state.things[0]
			}
			p.currentThing.visibility = 1 - p.currentThing.visibility
			p.sendThings([]*thing{p.currentThing})
		} else if kl == "m" && p.keys["Alt"] {
			p.setMode(grabbingMesh)

		} else if kl == "m" && p.keys["Control"] {
			//mirror the selected masses (in the grid)
			//more generally - we will add the selected masses to the current transformation
			//note - some masses will map the the same position (we will want to discard/reinstate them when hooking up springs)

			sm := maps.Keys(p.selectedMasses)
			transformed := make(map[*mass]*mass)

			for m := range sm {
				tp := m.p.reflectInPlane(p.grid.origin, p.grid.normal())
				transformed[m] = p.state.massAt(tp, 0.01) //some masses (those on the plane) will map to themselves
				if transformed[m] == nil {
					transformed[m] = p.state.addMass(newMass(tp, m.r, m.fixed, m.isCoin, m.collideable, m.thing, m)) //add 'shadow' mass
				}
			}
			p.regenTransformed() //(re)mirror all transformed masses (in the grid plane)

			for k, m := range transformed {
				m.wingRoot = transformed[k.wingRoot]
				m.axle = transformed[k.axle]
				//v.aoaRads = k.aoaRads + math.Pi
				//v.dihedralDegrees = -k.dihedralDegrees
				m.wingArea = k.wingArea
				m.flip = !k.flip
			}

			//wire up the springs - once. We will need to remove transformed masses which map onto their own point of origin
			//note we are adding to the collection we are iterating over - but that it ok (in Go)
			//mirror the springs
			for _, s := range p.currentThing.springs {
				//todo - if only one end is selected (and transformed) we should still create a spring
				tm1 := transformed[s.m1]
				tm2 := transformed[s.m2]
				if tm1 != nil && tm2 != nil {

					if tm1 == tm2 {
						logit("spring to self")
					}
					p.currentThing.AddSpring(tm1, tm2, s.collideable, fcNONE)
				}
			}
			p.sendThings([]*thing{p.currentThing})

		} else if k == "Escape" {
			if p.mode == stretching {
				p.springCursor.r = 0
				p.sendMasses([]*mass{p.springCursor}, true)
				p.state.masses = p.state.masses[:len(p.state.masses)]                           //delete the last mass
				p.currentThing.springs = p.currentThing.springs[:len(p.currentThing.springs)-1] //delete the last spring
				p.sendThings([]*thing{p.currentThing})                                          //one less spring
				p.setMode(adding)
			} else if p.mode == moving { // cancel a move
				for m := range p.selectedMasses {
					m.p = p.moveStart
				}

				s := slices.Collect(maps.Keys(p.selectedMasses))
				p.sendMasses(s, false)
				p.setMode(editing)
			} else if p.mode == props {
				p.boundValues = make(map[string]boundValue, 0)
				p.sendBoundValues() //sending empty set of bound values clears the popup clientside
				p.setMode(editing)

			} else {

				p.snapMasses()
				p.sendThings(p.state.things)
				//p.vehicle = p.currentThing

				p.sendVectors()
				p.bindMixers(p.vehicle)
				//p.vehicle.setVelocity(testFlight)

				//p.state.setVelocity(newVec3(0, 0, 1/float64(30*5)*50)) //100mph
				//p.labelLiftingMasses()
				logit("running", p.state.running)
				p.state.running = !p.state.running
			}

		} else if kl == "m" {
			p.setMode(startMove)
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
			selectedMass := slices.Collect(maps.Keys(p.selectedMasses))[0]

			if p.highlit.mass != nil && len(p.selectedMasses) == 1 && p.highlit.mass != selectedMass {
				if k == "z" {
					m := selectedMass
					wr := p.highlit.mass
					m.wingRoot = wr
					span := m.p.sub(m.axle.p).length()
					chord := m.axle.p.sub(m.wingRoot.p).length()
					m.wingArea = span * chord
					p.vehicle.setVelocity(testFlight)
					p.state.flyMasses() //*pretend* we are flying at 20ms
					p.sendVectors()
					p.sendMessage("Wing root defined", "info")
				} else if k == "x" {
					selectedMass.axle = p.highlit.mass
					p.sendMessage("Axis/Axle defined", "info")
				}
				p.sendMasses([]*mass{selectedMass}, true)

			} else {
				p.sendMessage("Select one mass, and higlight another when setting axes", "error")
			}
		} else if kl == "l" {

			p.snapMasses()
			if p.highlit.mass != nil {
				p.highlit.mass.flip = !p.highlit.mass.flip
				p.vehicle.setVelocity(testFlight)
				p.state.flyMasses() //*pretend* we are flying at 20ms
				p.sendVectors()     //does a fake flyMasses()
				p.sendLabels()
			}
		} else if kl == "i" {
			for _, m := range p.state.masses {
				if m.wingRoot != nil {
					p.state.addLabel(NewLabel("AOA", m, m, 1, 30, &m.aoaDegrees))
				}
			}
			for _, m := range p.mixers {
				if m.spring != nil {
					p.state.addLabel(NewLabel(actLabels[int(m.actuator)], m.spring.m1, m.spring.m2, 4, 30, &m.spring.flightOutput))
				}
			}
			p.sendLabels()

		} else if kl == "p" {
			m := p.highlit.mass
			if m != nil {
				m.fixed = !m.fixed
				p.sendMasses([]*mass{m}, true)
			}

		} else if k == "Delete" {
			if p.highlit.mass == nil && p.highlit.spring != nil {
				p.highlit.thing.deleteSpring(p.highlit.spring)
				p.sendThings([]*thing{p.highlit.thing})
			} else if p.highlit.mass != nil {
				if p.state.massFree(p.highlit.mass) {
					p.highlit.mass.r = 0
					p.sendMasses([]*mass{p.highlit.mass}, true)

					p.state.deleteMass(p.highlit.mass) //less than straightforward
					p.sendMasses(p.state.masses, true)
					p.sendThings(p.state.things)
				}
			}
		}
	}

	//sends any transform of the skin (done with cursor keys and/or shift)
	if dx != 0 || dy != 0 {
		if p.currentThing != nil {
			ct := p.currentThing
			if prop == "rotation" {
				//ct.Rotation += dx
			} else { //if prop=="scale" {
				ct.scale.x += dx
				ct.scale.y += dy
			}
			// - may be required in future (aliging tex/mesh/masses) p.sendThings([]*thing{ct}) //will need to send the offset, rotation and scale of the mesh/skin
		}
	}

}

func logit(s ...interface{}) { //accept an array of any type(s)
	fmt.Println(s...)
}
