//https://visualpde.com/sim/?options=N4IgxiBcIG4JYCc4BM4GcQBoTIKq6hAEYscA1MwgJlOQHU7CAGWjaIpz0sA6F7MJXakA1oQCGAXiYA6JgGYA7AAI4AO2UBtJpllEAugG4ARtLkA2VRu1yALLpkHDpALbjmMpa7jNSAB0IAYQBXNAAXAHsXfwQI4wBTADFgtUJgmLj4gBl4tQBzMIALKHlOHRAEXhBggD0qACoYZQBaZQAKcQBqYwBKevTsBCEQZtqGps7lcXq2ohblGB7SBEY+UjQA6HlSGChNUHEEWIB3QIiAGwjghChzAFY7+TvsQ5OAEVy0ODCATyhZZ4gV4RY45fJFACy4gAHoQSC8jiCAMpgcTneISYKRUjA44ADUIWQA+tCAPRUeawhEnACahKJP3J8z+LyxESR8TCZ0u1wASuJ8hjIAAzNFoeICcQueIIcQAFUKnPckFKkulsoACoUfCryqj1eIAFoRKL-GQADgEFyuCDcm1giBQ6G41uuxkOUFF53FVrUkWubC9PvAET9Nu5Nv+vv9CAAon4vpdUpBZEwiNGbQA5YLRSCAsChSIuJHXUVgIVBiUgeIuYwRNCBsVVmt1hsAQVrcFyYTNgJb9bQbzgwuFoSFsnM2H7Da1OqIMjuiiYlurtYHSO1aj+kA4U7XDaRLhNRTU8QbZqoe9baCRfniBfOHpTMkvq+vCqVZsU5ru9hAwvOOA-DvZAI2uO1PSbbBANPOgUEhYJzigdMQDcaEwKGNFgiFEBZC8bAXHUDCyCwnD+BACIYBlR9t0rbBKOo8QfgwqMKKohAaPjRNQzNThAQYjimNjaE-BuYR6PYmisnUeI4OQBCkJ3bA-EuMI5R+O9CBUgUMWU2JjHUPJIO9KtRMyZJk2qDIEjBApil1Tg9MydTNOgdQwniPJZSQpyEgJNZfPiOkApANBS3EcsLN8bAigQeJ4jeAAJeI4DyQowhRNFxxkRRsCosBIgQRIu3OZBjODY5CnENSIg1VS0lINQpRwkgAF99GwY4yugAYQGOe1eq+Fw5W+dFCAAcVlH5AGQCFEIjCHtWqAA

package main

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"time"
)

type block struct {
	GameId     int
	PlayerName string
	Msgs       []msg
}

type msg struct {
	Cmd     string
	Key     string
	Payload []float64 //this could become int32's
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

type qHolder struct {
	mutex *sync.Mutex
	q     map[int][]*reply //sets of replies by sqn
}

type reply struct {
	Cmd     string
	Payload interface{}
}

type ModeEnum int

const (
	playing      = 1
	editing      = 2
	addingSpring = 3
)

var games map[int]*State //the data of games in progress - by id
var accountsByGuid map[string]*account

//var obq map[string]*qHolder //*reply //qued outbound JSON data (replies), per player (new mass index, position triples)

func spotOccupied(props []Prop, p *Vector, clearance float64) bool {
	for _, prop := range props {
		if prop.Position.distanceFrom(p) < prop.Radius+clearance {
			return true
		}
	}
	return false
}

func stepWorlds() {

	//every 100 ms step all worlds
	for range time.Tick(time.Millisecond * 100) { //<<waits here
		print(".") //<< this is the heartbeat
		for _, state := range games {
			state.step() //<- this is a physics step - it queues stuff for all players

			for _, p := range state.Players {
				select { //this is a non-blocking send
				case p.waitChannel <- true: //<- this is a signal to the player that the world has stepped
				default:
				}
			}
		}
	}

	panic(`stepWorlds() has exited`)

}

func processBlock(block block) []byte {
	//this is the entry point for the server - the equivalent of rx() in the TS example

	// var inBlock block // []msg
	// err := json.Unmarshal(inJSONbytes, &inBlock)
	// if err != nil {
	// 	panic(err)
	// }

	from := block.PlayerName
	if from == "" {
		panic(`no player name in message`)
	}

	logit(len(block.Msgs), `messages in block`)
	var state *State
	for _, m := range block.Msgs { //process every msg in the queue
		state = process(block.GameId, from, m)
	}

	if state.Players[from] == nil {
		panic(`player ` + from + ` not found in game ` + fmt.Sprint(block.GameId))
	}

	qh := state.Players[from].qh

	//is there anything in this players outbound queue to reply with
	if len(qh.q) > 0 {

		jsonReply, err := json.Marshal(qh.q) //reply to sender
		if err != nil {
			panic(err)
		}
		//logit(string(jsonReply))

		qh.mutex.Lock()
		qh.q = make(map[int][]*reply) //empty the outbound queue
		qh.mutex.Unlock()

		return jsonReply // return this players outbound queue - all changed mass positions and player states

	} else {
		return []byte("{}") //nothing queued for them - reply with an empty object
	}
}

func (state *State) RandomStartPos() Vector {
	var randomPos Vector
	for {
		randomPos = Vector{5000 * rand.Float64(), 5000 * rand.Float64()}
		//don't spawn in a hole
		if state.thingClearance(&randomPos) > 200 && !state.inHole(randomPos) {
			break
		}
	}
	return randomPos
}

func (state *State) inHole(p Vector) bool {
	for _, thing := range state.Things {
		if thing.IsHole && thing.contains(&p, state.Masses) {
			return true
		}
	}
	return false
}

// what is distance is the closest thing to P
func (state *State) thingClearance(p *Vector) float64 {

	clearance := float64(10000)
	var c float64
	for _, thing := range state.Things {
		c = thing.distanceFrom(p, state.Masses)
		if c < clearance {
			clearance = c
		}
	}

	return c
}

// each player has a que.. but maybe a single shared que of responses by sqn would be better
// when all players have rx'd something it is removed from the queue
func process(gameId int, playerName string, msg msg) *State {

	var player *Player

	var state *State

	if gameId == -1 || gameId == 0 {
		//player = NewPlayer(playerName+"bootstrap", 0)
	} else {
		state = games[gameId]
		player = state.Players[playerName] //may (or may not) be nil for joinGame
	}

	//var fpn string //firstPlayer *Player

	dx, dy := float64(0), float64(0) //used for sliding skins
	prop := "offset"
	step := float64(3)

	if gameId < 1 && msg.Cmd != "createGame" && msg.Cmd != "joinGame" {
		logit("No game id ", msg.Cmd, player)
		return state
	}

	logit("processing", msg.Cmd, "from", playerName)

	if msg.Cmd == "createGame" {
		//create a new game
		state = NewState() //asigns a random game id
		//state.host = playerName //only the host steps the physics

		games[state.GameId] = state
		state.makeHoles(10, 5000, 5000)
		//state.makeHoles(1, 0, 0)

		player := state.AddPlayer(playerName, state.RandomStartPos())
		//state.AddPlayer("bot1", Vector{400, 120})
		state.setupLayers(5000, 5000)
		state.scatterCoins(5000, 5000)

		gs := reply{Cmd: "state", Payload: state}
		state.q4one(state.Players[playerName], &gs)
		logit("Game created", state.GameId)

		state.qSound("dozer", state.Things[player.Dozer].centreOfMass(state.Masses), 0.1, "revs-"+playerName, true)

		state.save("game" + fmt.Sprint(state.GameId) + ".bson")

	} else if msg.Cmd == "joinGame" {
		//join an existing game
		//gameId := int(msg.Payload[0])
		state = games[gameId]
		if state == nil {
			state = load("game" + fmt.Sprint(gameId) + ".bson")
			games[gameId] = state
			for _, p := range state.Players {
				p.qh = &qHolder{mutex: &sync.Mutex{}, q: make(map[int][]*reply, 0)}
			}
		}
		//state.host = playerName //the last person to join becomes the host (and steps the game)

		if player != nil {
			//i am joining to control an existing player (whos is already in)
			state.q4one(player, &reply{Cmd: "state", Payload: state}) //inst important that state is sent bedore the change of name
			//from now on the client will prefix its playerName with "control-" to indicate that it is controlling another player
			state.q4one(player, &reply{Cmd: "prefix", Payload: "control-"})
			p := NewPlayer("control-"+playerName, -1, state)
			state.Players[p.Name] = p

		} else { //player = NewPlayer(playerName, 0)
			joiner := state.AddPlayer(playerName, state.RandomStartPos())
			state.q4one(joiner, &reply{Cmd: "state", Payload: state})
			state.q4all(&reply{Cmd: "playerJoined", Payload: joiner}) //tell everyone about the new player
			sendWholeThing(state, joiner.Dozer)                       //NOTE the joiner will recieve themselves twice
		}

		// } else if msg.Cmd == "keyDown" {
		// 	//a key was pressed
		// 	if msg.Key == "ArrowLeft" {
		// 		dx = -step
		// 	} else if msg.Key == "ArrowRight" {
		// 		dx = +step
		// 	} else if msg.Key == "ArrowUp" {
		// 		dy = +step
		// 	} else if msg.Key == "ArrowDown" {
		// 		dy = -step
		// 	}
	} else if msg.Cmd == "keyUp" {
		//a key was released
		if msg.Key == "ArrowLeft" || msg.Key == "ArrowRight" {
			dx = 0
		} else if msg.Key == "ArrowUp" || msg.Key == "ArrowDown" {
			dy = 0
		}
	} else if msg.Cmd == "step" {
		<-player.waitChannel //see stepworlds fo rthe sending end which releases this

	} else if msg.Cmd == "drive" {

		if strings.HasPrefix(playerName, "control-") {
			player = state.Players[strings.TrimPrefix(playerName, "control-")]
		}
		player.LeftDrive = msg.Payload[0]  //no need to echo them back - local versions are used for knobs only
		player.RightDrive = msg.Payload[1] //no need to echo them back - local versions are used for knobs only

	} else if msg.Cmd == "mm" { //mouse move

		player.worldCursor.X = msg.Payload[0]
		player.worldCursor.Y = msg.Payload[1]

		if player.mode != playing {
			player.Highlit.Mass = state.closestMass(&player.worldCursor)
			player.Highlit.Spring, player.Highlit.Thing = state.closestSpring(player.worldCursor)
		}
	} else if msg.Cmd == "md" {
		if player.mode == addingSpring {
			//we are adding a spring to nowhere .. make a new mass
			if player.Highlit.Mass == -1 {
				logit("Spring to/from nowhere - adding a mass")
				m := NewMass(player.worldCursor, 10, false, false, true, player.currentThing)
				i := state.AddMass(m)
				state.q4all(&reply{Cmd: "mass", Payload: massPayload{I: i, Mass: *state.Masses[i]}}) //we receive a new thing sfrom someone -
				player.Highlit.Mass = i
			}

			if player.springStart == -1 { //starting a new spring
				logit("Starting a new spring")
				player.springStart = player.Highlit.Mass //closestMass(this.cursor)
				logit("Spring starts at mass", player.springStart)
			} else { //continuing the chain of springs
				i := state.AddSpring(player.currentThing, player.springStart, player.Highlit.Mass, true) //this is a spring-like thing (but not an acutal spring.. it has no length for example)
				t := state.Things[player.currentThing]

				s := springPayload{Ti: player.currentThing, Si: i, Spring: *t.Springs[i]}
				state.q4all(&reply{Cmd: "spring", Payload: s})

				//this.currentThing.springs.push(new Spring(this.state.masses,me.springStart,this.highlit.mass,true))
				logit("Continued chain of springs from mass ", player.springStart, " to ", player.Highlit.Mass)
				//console.log("Current thing has", t.springs.length, "springs")
				player.springStart = player.Highlit.Mass
			}
		}
	} else if msg.Cmd == "keyDown" {

		k := msg.Key
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
		} else if k == "ArrowRight" {
			dx = +step
		} else if k == "ArrowUp" {
			dy = +step
		} else if k == "ArrowDown" {
			dy = -step //see the end of the if block for where the transform is send if dx or dy are set
		} else if k == "t" {
			player.mode = addingSpring
			player.currentThing = state.addThing("dozers", "dozer", false)
			sendThing(state, player.currentThing)
		} else if k == "s" {
			player.springStart = -1
			//player.mode = ModeEnum.addingSpring
		} else if k == "p" {
			m := state.Masses[player.Highlit.Mass]
			m.Fixed = !m.Fixed
			state.q4all(&reply{Cmd: "mass", Payload: massPayload{I: player.Highlit.Mass, Mass: *m}})
		} else if k == "v" {
			state.save((`game` + fmt.Sprint(state.GameId) + `.bson`))
			logit("Game saved", state.GameId)
		} else if k == "l" {
			state = load((`game` + fmt.Sprint(state.GameId) + `.bson`))
			logit("Game loaded", state.GameId)
		}
	}

	//sends any tranform of the skin (done with cursor keys and/or shift)
	if dx != 0 || dy != 0 {
		if player.currentThing > -1 {
			ct := state.Things[player.currentThing]
			if prop == "rotation" {
				ct.Rotation += dx
			} else { //if prop=="scale" {
				ct.Scale.X += dx
				ct.Scale.Y += dy
			}
		}
		sendThing(state, player.currentThing) //TODO this could just be the skin
	}

	return state
}

func sendThing(state *State, ti int) {

	state.q4all(&reply{Cmd: "thing", Payload: thingPayload{Ti: ti, Thing: *state.Things[ti]}})
}

func sendWholeThing(state *State, ti int) {
	sendThing(state, ti) //the skin and offsets
	for si, spring := range state.Things[ti].Springs {
		state.q4all(&reply{Cmd: "spring", Payload: springPayload{Ti: ti, Si: si, Spring: *spring}})
		state.q4all(&reply{Cmd: "mass", Payload: massPayload{I: spring.M1, Mass: *state.Masses[spring.M1]}})
		state.q4all(&reply{Cmd: "mass", Payload: massPayload{I: spring.M2, Mass: *state.Masses[spring.M2]}})
	}
}

func logit(s ...interface{}) { //accept an array of any type(s)
	fmt.Println(s...)
}
