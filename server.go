//https://visualpde.com/sim/?options=N4IgxiBcIG4JYCc4BM4GcQBoTIKq6hAEYscA1MwgJlOQHU7CAGWjaIpz0sA6F7MJXakA1oQCGAXiYA6JgGYA7AAI4AO2UBtJpllEAugG4ARtLkA2VRu1yALLpkHDpALbjmMpa7jNSAB0IAYQBXNAAXAHsXfwQI4wBTADFgtUJgmLj4gBl4tQBzMIALKHlOHRAEXhBggD0qACoYZQBaZQAKcQBqYwBKevTsBCEQZtqGps7lcXq2ohblGB7SBEY+UjQA6HlSGChNUHEEWIB3QIiAGwjghChzAFY7+TvsQ5OAEVy0ODCATyhZZ4gV4RY45fJFACy4gAHoQSC8jiCAMpgcTneISYKRUjA44ADUIWQA+tCAPRUeawhEnACahKJP3J8z+LyxESR8TCZ0u1wASuJ8hjIAAzNFoeICcQueIIcQAFUKnPckFKkulsoACoUfCryqj1eIAFoRKL-GQADgEFyuCDcm1giBQ6G41uuxkOUFF53FVrUkWubC9PvAET9Nu5Nv+vv9CAAon4vpdUpBZEwiNGbQA5YLRSCAsChSIuJHXUVgIVBiUgeIuYwRNCBsVVmt1hsAQVrcFyYTNgJb9bQbzgwuFoSFsnM2H7Da1OqIMjuiiYlurtYHSO1aj+kA4U7XDaRLhNRTU8QbZqoe9baCRfniBfOHpTMkvq+vCqVZsU5ru9hAwvOOA-DvZAI2uO1PSbbBANPOgUEhYJzigdMQDcaEwKGNFgiFEBZC8bAXHUDCyCwnD+BACIYBlR9t0rbBKOo8QfgwqMKKohAaPjRNQzNThAQYjimNjaE-BuYR6PYmisnUeI4OQBCkJ3bA-EuMI5R+O9CBUgUMWU2JjHUPJIO9KtRMyZJk2qDIEjBApil1Tg9MydTNOgdQwniPJZSQpyEgJNZfPiOkApANBS3EcsLN8bAigQeJ4jeAAJeI4DyQowhRNFxxkRRsCosBIgQRIu3OZBjODY5CnENSIg1VS0lINQpRwkgAF99GwY4yugAYQGOe1eq+Fw5W+dFCAAcVlH5AGQCFEIjCHtWqAA

package main

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"sync"
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
	q     []*reply
}

type reply struct {
	Cmd     string
	Payload interface{} //map[string]interface{} //names to (arbitrarily typed) values
}

type ModeEnum int

const (
	playing      = 1
	editing      = 2
	addingSpring = 3
)

var games map[int]*State //the data of games in progress - by id
//var obq map[string]*qHolder //*reply //qued outbound JSON data (replies), per player (new mass index, position triples)

func spotOccupied(props []Prop, p *Vector, r float64) bool {
	for _, prop := range props {
		if prop.Position.distanceFrom(p) < prop.Radius+r {
			return true
		}
	}
	return false
}

func processBlock(block block) []byte {
	//this is the entry point for the server - the equivalent of rx() in the TS example

	// var inBlock block // []msg
	// err := json.Unmarshal(inJSONbytes, &inBlock)
	// if err != nil {
	// 	panic(err)
	// }

	if block.PlayerName == `` {
		logit(block)
		panic(`no player name in message`)
	}

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
		qh.q = []*reply{} //empty the queue
		qh.mutex.Unlock()

		return jsonReply // return this players outbound queue - all changed mass positions and player states

	} else {
		return []byte("{}") //nothing queued for them - reply with an empty object
	}
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
		player = state.Players[playerName]
	}

	dx, dy := float64(0), float64(0) //used for sliding skins
	prop := "offset"
	step := float64(3)

	if gameId < 1 && msg.Cmd != "createGame" {
		logit("No game id ", msg.Cmd, player)
		return state
	}

	logit("processing", msg.Cmd, "from", playerName)

	if msg.Cmd == "createGame" {
		//create a new game
		state = NewState()      //asigns a random game id
		state.host = playerName //only the host steps the physics

		games[state.GameId] = state
		state.makeHoles(10, 5000, 5000)
		//state.makeHoles(1, 0, 0)
		state.AddPlayer(playerName, Vector{500, 100})
		//state.AddPlayer("bot1", Vector{400, 120})
		state.setupLayers(5000, 5000)
		state.scatterCoins(5000, 5000)

		gs := reply{Cmd: "state", Payload: state}
		q4one(state.Players[playerName], &gs)
		logit("Game created", state.GameId)
		state.qSound("dozer", Vector{100, 100}, 0.1, "revs-"+playerName, true)

	} else if msg.Cmd == "joinGame" {
		//join an existing game
		gameId := int(msg.Payload[0])
		state = games[gameId]

		randomPos := Vector{1000 * rand.Float64(), 1000 * rand.Float64()}
		joiner := state.AddPlayer(playerName, randomPos)
		state.q4all(&reply{Cmd: "playerJoined", Payload: joiner}) //tell everyone about the new player
		q4one(joiner, &reply{Cmd: "state", Payload: state})
		sendWholeThing(state, joiner.Dozer) //NOTE the joiner will recieve themselves twice

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

		if playerName == state.host {
			newPositions := state.moveAll() //<- this is a physics step
			if len(newPositions) > 0 {
				state.q4all(&reply{Cmd: "mps", Payload: newPositions})
			}

			//count money and send to player
			for _, p := range state.Players {

				if p.stepCoins > 0 { //did we win any coins this step
					p.Coins += p.stepCoins
					q4one(p, &reply{Cmd: "coins", Payload: p.Coins})
				}
				p.stepCoins = 0 //reset for next step
			}
		}

	} else if msg.Cmd == "lt" {
		player.LeftDrive = msg.Payload[0] //no need to echo them back - local versions are used for knobs only
	} else if msg.Cmd == "rt" {
		player.RightDrive = msg.Payload[0]
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
				//console.log("Spring to/from nowhere - adding a mass")
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
			player.currentThing = state.addThing("holes", "hole", true)
			player.currentThing = state.addThing("dozers", "dozer", false)
			sendThing(state, player.currentThing)
		} else if k == "s" {
			player.springStart = -1
			//player.mode = ModeEnum.addingSpring
		} else if k == "p" {
			m := state.Masses[player.Highlit.Mass]
			m.fixed = !m.fixed
			state.q4all(&reply{Cmd: "mass", Payload: massPayload{I: player.Highlit.Mass, Mass: *m}})
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
