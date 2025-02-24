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
	"strconv"
	"sync"
	"time"
)

// type block struct {
// 	//GameId     int
// 	//PlayerName string
// 	Msgs []msg
// }

// these must be upper cased or theu don't get unmarshalled
type msg struct {
	Cmd     string    `json:"cmd"`
	Key     string    `json:"key"`
	Payload []float64 `json:"payload"`
}

// type inKeyPayload struct {
// 	key   string
// 	shift bool
// 	ctrl  bool
// 	alt   bool
// }

// type VectorPayload struct {
// 	x float64
// 	y float64
// }

// type qHolder struct {
// 	mutex *sync.Mutex
// 	q     map[int][]*reply //sets of replies by sqn
// }

// these must be uppercased for marshalling
type reply struct {
	Cmd     string      `json:"cmd"`
	Payload interface{} `json:"payload"`
}

type ModeEnum string

const (
	editing      = "Editing"
	playing      = "Running"
	addingSpring = "Adding spring"
	startMove    = "Select the start point of the move"
	moving       = "Moving - Select the destination"
)

var games map[uint32]*state //the data of games in progress - by id
var accountsByGuid map[string]*account

//var obq map[string]*qHolder //*reply //qued outbound JSON data (replies), per player (new mass index, position triples)

// func spotOccupied(props []Prop, p *Vector, clearance float64) bool {
// 	for _, prop := range props {
// 		if prop.Position.distanceFrom(p) < prop.Radius+clearance {
// 			return true
// 		}
// 	}
// 	return false
// }

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
func (state *state) RandomStartPos(s float64) *Vec3 {

	z := (rand.Float64() - .5) * s * 2
	xr := (1 - (z+s)/(2*s)) * s //range of x at z (to stay on dry land)
	x := (rand.Float64() - .5) * xr * 2

	//x = 5.01
	//z = 2.001
	return newVec3(x, 0, z)

}

// what is distance is the closest thing to P
func (state *state) thingClearance(p *Vec3) float64 {

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
	state := NewState() //asigns a random game id
	games[state.gameId] = state

	s := float64(10000) //+/- 10km land = 200 km^2

	y0pos := state.RandomStartPos(s)
	player := state.AddPlayer(playerId, playerName, y0pos, ws)
	//player.makeLand(y0pos, 10, 2000, s)
	player.makeDozer(y0pos)

	//state.scatterCoins(2000, 2000)

	e := binary.LittleEndian
	player.sendCamera(e)
	player.sendMasses(state.masses, true, e)
	player.sendThings([]*thing{player.vehicle}, e) //sends mesh name and springs
	player.sendGameId(state.gameId, e)             //game id starts it running

	//player.send(&reply{Cmd: "state", Payload: state.payload()})

	state.makeHoles(10, 5000, 5000)

	//state.landMesh.sendToAll("land", state)

	//t := tetra()
	//t.offset(pos)

	//t.sendToAll("tetra", state)

	// campos := pos.add(newVec3(0, 5, -30))
	// state.send(player, &reply{Cmd: "campos", Payload: campos.payload()})
	// state.send(player, &reply{Cmd: "camlookat", Payload: pos.payload()})

	//state.makeWater() //water is flowed and sent every cycle
	//state.sendWater()

	logit("Game created", state.gameId)

	state.qSound("dozer", player.vehicle.centreOfMass(state.masses), 0.1, "revs-"+playerName, true)

	//state.save("game" + fmt.Sprint(state.gameId) + ".bson")

	return player
}

func joinGame(gameId uint32, playerId uint32, playerName string, ws *websocket.Conn) *player {
	state := games[gameId]
	if state == nil {
		state = load("game" + fmt.Sprint(gameId) + ".bin")
		games[gameId] = state
		for _, p := range state.players {
			p.mtx = &sync.Mutex{}
		}
	}

	var player *player = nil
	if state.players[playerId] != nil { //player already in game - joining to control them from another device
		player = state.players[playerId]
		//i am joining to control an existing player (whos is already in)
		//player.send(&reply{Cmd: "state", Payload: state.payload()}) //inst important that state is sent bedore the change of name
		//from now on the client will prefix its playerName with "control-" to indicate that it is controlling another player
		//player.send(&reply{Cmd: "prefix", Payload: "control-"})
		//p := NewPlayer("control-"+playerName, -1, state, ws)  TODO BROKEN By REFACTORE of THING (to a ref )
		//state.Players[p.Name] = p TODO

	} else {
		player := state.AddPlayer(playerId, playerName, state.RandomStartPos(10000), ws)
		//player.send(&reply{Cmd: "state", Payload: state})
		state.send(nil, &reply{Cmd: "playerJoined", Payload: player})    //tell everyone about the new player
		player.sendMasses(state.masses, true, binary.LittleEndian)       //send all the masses
		player.sendThings([]*thing{player.vehicle}, binary.LittleEndian) //sends mesh name and springs
		state.qSound("dozer", player.vehicle.centreOfMass(state.masses), 0.1, "revs-"+playerName, true)
	}

	return player
}

func processCreateOrJoin(mb []byte, ws *websocket.Conn) *player {

	buff := bytes.NewBuffer(mb)
	e := binary.LittleEndian

	cmd := [1]byte{}
	binary.Read(buff, e, &cmd)

	gameId := uint32(0)
	playerId := uint32(0)
	sl := byte(0)

	binary.Read(buff, e, &gameId)
	binary.Read(buff, e, &playerId)
	binary.Read(buff, e, &sl)
	playerName := make([]byte, sl)
	binary.Read(buff, e, &playerName)

	if cmd[0] == byte(msgCreateGame) {
		return createGame(playerId, string(playerName), ws) //returns a player
	} else if cmd[0] == byte(msgJoinGame) {
		return joinGame(gameId, playerId, string(playerName), ws) //returns a player
	} else {
		panic("first message was not create or join")
	}

}

func processMsg(msg msg, player *player, ws *websocket.Conn) {

	//var fpn string //firstPlayer *Player

	state := player.state

	endian := binary.LittleEndian

	dx, dy := float64(0), float64(0) //used for sliding skins
	prop := "offset"
	step := float64(3)

	if player == nil && msg.Cmd != "createGame" && msg.Cmd != "joinGame" {
		logit("No player for message", msg)
	}

	logit(msg.Cmd)

	if msg.Cmd == "keyUp" {
		//a key was released
		player.keys[msg.Key] = false

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
		player.leftDrive = msg.Payload[0]  //no need to echo them back - local versions are used for knobs only
		player.rightDrive = msg.Payload[1] //no need to echo them back - local versions are used for knobs only
		player.thrust = msg.Payload[2]

	} else if msg.Cmd == "mw" { //mousewheel

		player.camera.position.y += msg.Payload[0] * -0.01 //up and down

		player.sendCamera(binary.LittleEndian)

	} else if msg.Cmd == "mm" { //mouse move

		player.movedSinceMouseDown = true

		player.buttons = byte(msg.Payload[0])
		player.camera.farPos = newVec3(msg.Payload[1], msg.Payload[2], msg.Payload[3])
		player.cursor.X = msg.Payload[4]
		player.cursor.Y = msg.Payload[5]

		if player.mode != playing {

			//a point on the far plane (where the mouse cursor is pointing)

			if player.buttons == 2 {

				delta := (player.cursor.subtract(player.grab)).multiply(2)

				if delta.lengthSq() != 0 {

					logit("delta", delta.X, delta.Y)

					camRight := player.downCamDirection.cross(player.downCamUp).normalise()

					player.camera.up = player.downCamUp.rotateAbout(camRight, delta.Y).normalise()
					pitched := player.downCamDirection.rotateAbout(camRight, delta.Y)
					yawed := pitched.rotateAbout(player.camera.up, -delta.X)
					//player.camUp = player.camUp.rotateAbout(player.downCamUp, delta.X).normalise()
					player.camera.direction = yawed
					player.camera.up = newVec3(0, 1, 0) //auto level the camera

					// worldUp := newVec3(0, 1, 0)
					// camDir := (player.camLookAt.sub(player.camPosition)).normalise()
					// player.camUp = camDir.cross(worldUp).normalise().cross(camDir).normalise()

					player.sendCamera(binary.LittleEndian)
				}
				return
			}

			gn := player.gridXaxis.cross(player.gridYaxis)
			c2g := player.camera.position.closestPointOnPlane(player.gridOrigin, gn).sub(player.camera.position) //vector from the camera pos to the grid
			ttg := player.camera.farPos.closestPointOnPlane(player.gridOrigin, gn).sub(player.camera.farPos)

			if c2g.dot(ttg) < 0 {
				d0 := c2g.length()
				d1 := ttg.length()
				f := d0 / (d0 + d1)
				player.gridPos = player.camera.position.tween(player.camera.farPos, f)
			}

			player.highlit.spring, player.highlit.thing = state.closestSpring(player.gridPos)

			cm := state.closestMassToRay(player.camera.position, player.camera.farPos)

			if cm != player.highlit.mass {
				player.highlit.mass = cm
				player.sendHighlit()
			}

			if player.mode == moving {
				moveDelta := player.gridPos.sub(player.moveStart)
				for m := range player.selectedMasses {
					m.p = player.massStartPos[m].add(moveDelta)
				}
				player.sendMasses(slices.Collect(maps.Keys(player.selectedMasses)), false, endian) //just send the new positions
			}

			if player.buttons == 1 {
				delta := player.gridPos.sub(player.downGridPos).multiply(.9)
				player.camera.position = player.downCamPos.sub(delta)

				player.sendCamera(endian)
			}

			player.sendCursor(player.gridPos)
		}

	} else if msg.Cmd == "mu" {
		player.buttons = byte(msg.Payload[0])
		player.springStart = nil
		if player.mode == moving && !player.movedSinceMouseDown {
			//	player.setMode(editing)
		}

	} else if msg.Cmd == "md" {

		player.movedSinceMouseDown = false
		player.buttons = byte(msg.Payload[0])

		player.grab = &Vector{player.cursor.X, player.cursor.Y} //clone
		player.downGridPos = player.gridPos.clone()
		player.downCamPos = player.camera.position.clone()
		player.downCamDirection = player.camera.direction.clone()
		player.downCamUp = player.camera.up.clone()

		if player.mode == editing {
			//toggle selection of highlit mass
			phm := player.highlit.mass
			if phm != nil {
				psm := player.selectedMasses

				there, _ := psm[phm]
				if there {
					delete(psm, phm)
				} else {
					psm[phm] = true
				}
				player.sendMasses([]*mass{phm}, true, endian)
			}
		}

		if player.mode == startMove {
			player.recordSelectedMassPositions()
			if player.highlit.mass != nil {
				player.moveStart = player.highlit.mass.p.clone()
			} else {
				player.moveStart = player.gridPos.clone() //may be snapped
			}
			player.setMode(moving)

		} else if player.mode == moving {
			player.setMode(editing)

		}

		if player.mode == addingSpring {
			//we are adding a spring to nowhere .. make a new mass
			if player.highlit.mass != nil {
				logit("Spring to/from nowhere - adding a mass")

				m := state.AddMass(NewMass(player.gridPos, 10, false, false, true, player.currentThing))
				//state.send(nil, &reply{Cmd: "mass", Payload: massPayload{I: m.index, P: m.P,}}) //we receive a new thing sfrom someone -

				player.sendMasses([]*mass{m}, false, endian)
				player.highlit.mass = m
			}

			if player.springStart == nil { //starting a new spring
				logit("Starting a new spring")
				player.springStart = player.highlit.mass //closestMass(this.cursor)
				logit("Spring starts at mass", player.springStart)
			} else { //continuing the chain of springs

				s := player.currentThing.AddSpring(player.springStart, player.highlit.mass, true) //this is a spring-like thing (but not an acutal spring.. it has no length for example)
				s.send(nil, state)

				//this.currentThing.springs.push(new Spring(this.state.masses,me.springStart,this.highlit.mass,true))
				logit("Continued chain of springs from mass ", player.springStart, " to ", player.highlit.mass)
				//console.log("Current thing has", t.springs.length, "springs")
				player.springStart = player.highlit.mass
			}
		}

		//player.send(&reply{Cmd: "gridPos", Payload: player.gridPos.payload()})

	} else if msg.Cmd == "pickRay" {
		o := newVec3(msg.Payload[0], msg.Payload[1], msg.Payload[2])
		dir := newVec3(msg.Payload[3], msg.Payload[4], msg.Payload[5])
		if player.landMesh != nil {
			p := player.landMesh.slowProbe(o, o.add(dir.multiply(1000)))
			if p != nil {
				//grow a simple tree here
				m := growTree()
				m.offset(p)
				//	m.sendToAll("tree", state)
			}
		}

	} else if msg.Cmd == "keyDown" {

		k := msg.Key
		player.keys[k] = true

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
			player.aileron--
		} else if k == "ArrowRight" {
			dx = +step
			player.aileron++
		} else if k == "ArrowUp" {
			player.elevator++
			logit("elevator", player.elevator)
			dy = +step
		} else if k == "ArrowDown" {
			player.elevator--
			logit("elevator", player.elevator)
			dy = -step //see the end of the if block for where the transform is send if dx or dy are set
		} else if k == "t" {
			player.setMode(addingSpring)
			player.currentThing = state.addThing("dozer")
			player.sendThings([]*thing{player.currentThing}, endian)
		} else if k == "k" {
			player.setMode(addingSpring)
		} else if k == "m" {
			player.setMode(startMove)
		} else if k == "x" { //define the axle/wing axis
			if len(player.selectedMasses) == 1 && player.highlit.mass != nil {
				for m := range player.selectedMasses {
					m.axle = player.highlit.mass
				}
				player.sendMessage("Wing axis/wheel afle defined", "info")
			} else {
				player.sendMessage("Select one mass, and higlight the axle mass when defining it", "error")
			}
		} else if k == "z" { //define wing root/plane
			if len(player.selectedMasses) == 1 && player.highlit.mass != nil {

				for m := range player.selectedMasses {
					m.wingRoot = player.highlit.mass
				}
				player.sendMasses(slices.Collect(maps.Keys(player.selectedMasses)), true, endian)
				player.sendMessage("Wing root defined", "info")
			} else {
				player.sendMessage("Select one mass, and higlight the root mass when defining it", "error")
			}
		} else if k == "f" {
			player.highlit.mass.aoa += math.Pi
			player.sendMasses([]*mass{player.highlit.mass}, true, endian)

		} else if k == "p" {
			m := player.highlit.mass
			m.fixed = !m.fixed
			player.sendMasses([]*mass{m}, true, endian)
		} else if k == "v" {
			state.save((`game` + fmt.Sprint(state.gameId) + `.bin`), endian, player.selectedMasses)
			player.sendMessage("Game saved "+strconv.Itoa(int(state.gameId)), "info")
			logit("Game saved", state.gameId)
		} else if k == "l" {
			state = load((`game` + fmt.Sprint(state.gameId) + `.bin`))
			logit("Game loaded", state.gameId)
		} else if k == "delete" {
			if player.highlit.mass != nil {
				state.deleteMass(player.highlit.mass)
			} else if player.highlit.spring != nil {
				player.highlit.thing.deleteSpring(player.highlit.spring)
			}
		}
	}

	//sends any tranform of the skin (done with cursor keys and/or shift)
	if dx != 0 || dy != 0 {
		if player.currentThing != nil {
			ct := player.currentThing
			if prop == "rotation" {
				//ct.Rotation += dx
			} else { //if prop=="scale" {
				ct.scale.x += dx
				ct.scale.y += dy
			}
			player.sendThings([]*thing{ct}, endian) //will need to send the offset, rotation and scale of the mesh/skin
		}
	}

}

func logit(s ...interface{}) { //accept an array of any type(s)
	fmt.Println(s...)
}
