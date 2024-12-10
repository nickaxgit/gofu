//https://visualpde.com/sim/?options=N4IgxiBcIG4JYCc4BM4GcQBoTIKq6hAEYscA1MwgJlOQHU7CAGWjaIpz0sA6F7MJXakA1oQCGAXiYA6JgGYA7AAI4AO2UBtJpllEAugG4ARtLkA2VRu1yALLpkHDpALbjmMpa7jNSAB0IAYQBXNAAXAHsXfwQI4wBTADFgtUJgmLj4gBl4tQBzMIALKHlOHRAEXhBggD0qACoYZQBaZQAKcQBqYwBKevTsBCEQZtqGps7lcXq2ohblGB7SBEY+UjQA6HlSGChNUHEEWIB3QIiAGwjghChzAFY7+TvsQ5OAEVy0ODCATyhZZ4gV4RY45fJFACy4gAHoQSC8jiCAMpgcTneISYKRUjA44ADUIWQA+tCAPRUeawhEnACahKJP3J8z+LyxESR8TCZ0u1wASuJ8hjIAAzNFoeICcQueIIcQAFUKnPckFKkulsoACoUfCryqj1eIAFoRKL-GQADgEFyuCDcm1giBQ6G41uuxkOUFF53FVrUkWubC9PvAET9Nu5Nv+vv9CAAon4vpdUpBZEwiNGbQA5YLRSCAsChSIuJHXUVgIVBiUgeIuYwRNCBsVVmt1hsAQVrcFyYTNgJb9bQbzgwuFoSFsnM2H7Da1OqIMjuiiYlurtYHSO1aj+kA4U7XDaRLhNRTU8QbZqoe9baCRfniBfOHpTMkvq+vCqVZsU5ru9hAwvOOA-DvZAI2uO1PSbbBANPOgUEhYJzigdMQDcaEwKGNFgiFEBZC8bAXHUDCyCwnD+BACIYBlR9t0rbBKOo8QfgwqMKKohAaPjRNQzNThAQYjimNjaE-BuYR6PYmisnUeI4OQBCkJ3bA-EuMI5R+O9CBUgUMWU2JjHUPJIO9KtRMyZJk2qDIEjBApil1Tg9MydTNOgdQwniPJZSQpyEgJNZfPiOkApANBS3EcsLN8bAigQeJ4jeAAJeI4DyQowhRNFxxkRRsCosBIgQRIu3OZBjODY5CnENSIg1VS0lINQpRwkgAF99GwY4yugAYQGOe1eq+Fw5W+dFCAAcVlH5AGQCFEIjCHtWqAA

package main

import (
	"encoding/json"
	"math/rand/v2"
	"strings"
)

type msg struct {
	gameId     int
	playerName string
	cmd        string
	payload    interface{} //Object
}

type inKeyPayload struct {
	key   string
	shift bool
	ctrl  bool
	alt   bool
}

type reply struct {
	cmd     string
	payload interface{} //map[string]interface{} //names to (arbitrarily typed) values
}

type ModeEnum int

const (
	playing      = 1
	editing      = 2
	addingSpring = 3
)

var games map[int]State    //the data of games in progress - by id
var obq map[string][]reply //qued outbound JSON data (replies), per player (new mass index, position triples)

func qSound(sound string, position Vector, volume float32, label string, loop bool) {
	//this is a queue for sounds to be played

	// = (sound:sound,position:position,volume:volume,label:label,loop:loop)

	payload := soundPayload{sound: sound, position: position, volume: volume, label: label, loop: loop}
	q4all(reply{cmd: "sound", payload: payload})

}

func q4all(msg reply) {
	for _, q := range obq { //for every outbound que (player)
		q = append(q, msg)
	}
}

func spotOccupied(props []Prop, p *Vector, r float64) bool {
	for _, prop := range props {
		if prop.position.distanceFrom(p) < prop.radius+r {
			return true
		}
	}
	return false
}

func rx(inJSONbytes []byte) []byte {
	//this is the entry point for the server
	var inq []msg
	err := json.Unmarshal(inJSONbytes, &inq)
	if err != nil {
		panic(err)
	}

	//made an empty request
	if len(inq) == 0 {
		return []byte("{}")
	}

	for _, m := range inq { //process every msg in the queue
		process(m)
	}

	from := inq[0].playerName
	if from == "" {
		panic(`no player name in message`)
	}

	//is there anything in this players outpud queue to reply with
	if len(obq[from]) > 0 {

		jsonReply, err := json.Marshal(obq[from]) //reply to sender
		if err != nil {
			panic(err)
		}

		obq[from] = []reply{} //empty the queue
		return jsonReply      // return this players outbound queue - all changed mass positions and player states
		//and empty the queue
	} else {
		return []byte("{}") //nothing queued for them - reply with an empty object
	}
}

func process(msg msg) {

	var player *Player

	var state State

	if msg.gameId == -1 {
		player = NewPlayer(msg.playerName+"bootstrap", 0)
	} else {
		state = games[msg.gameId]
		player = state.Players[msg.playerName]
	}

	dx, dy := float64(0), float64(0) //used for sliding skins
	prop := "offset"
	step := float64(3)

	if msg.cmd == "createGame" {
		//create a new game
		state := NewState() //asigns a random game id
		games[state.gameId] = state
		state.makeHoles(10, 5000, 5000)
		state.AddPlayer(msg.playerName, Vector{100, 100})
		state.AddPlayer("bot1", Vector{400, 120})
		state.setupLayers(5000, 5000)
		state.scatterCoins(5000, 5000)

		obq[msg.playerName] = []reply{{cmd: "state", payload: state}}

		qSound("dozer", Vector{100, 100}, 0.1, "revs-"+msg.playerName, true)
	} else if msg.cmd == "step" {
		newPositions := state.moveAll() //<- this is a physics step
		for pn, p := range state.Players {
			if !strings.HasPrefix(pn, "bot") {
				p.coins += p.stepCoins
				obq[pn] = append(obq[pn], reply{cmd: "mps", payload: newPositions})
				obq[pn] = append(obq[pn], reply{cmd: "coins", payload: player.coins})
				p.stepCoins = 0 //reset for next step
			}
		}
	} else if msg.cmd == "joinGame" {
		randomPos := Vector{1000 * rand.Float64(), 1000 * rand.Float64()}
		joiner := state.AddPlayer(msg.playerName, randomPos)
		//obq[msg.playerName]=[{cmd:"state",payload:state.gameId}   ]
		q4all(reply{cmd: "playerJoined", payload: joiner})
	} else if msg.cmd == "lt" {
		player.leftDrive = msg.payload.(float64) //no need to echo them back - local versions are used for knobs only
	} else if msg.cmd == "rt" {
		player.rightDrive = msg.payload.(float64)
	} else if msg.cmd == "mm" { //mouse move
		pl := msg.payload.(Vector)
		player.worldCursor = Vector{pl.x, pl.y}

		if player.mode != playing {
			player.highlit.mass = state.closestMass(player.worldCursor)
			player.highlit.spring, player.highlit.thing = state.closestSpring(player.worldCursor)

		}
	} else if msg.cmd == "md" {
		if player.mode == addingSpring {
			//we are adding a spring to nowhere .. make a new mass
			if player.highlit.mass == -1 {
				//console.log("Spring to/from nowhere - adding a mass")
				m := NewMass(player.worldCursor, 10, false, false, true, player.currentThing)
				i := state.AddMass(m)
				q4all(reply{cmd: "mass", payload: massPayload{i: i, mass: *state.Masses[i]}}) //we receive a new thing sfrom someone -
				player.highlit.mass = i
			}

			if player.springStart == -1 { //starting a new spring
				logit("Starting a new spring")
				player.springStart = player.highlit.mass //closestMass(this.cursor)
				logit("Spring starts at mass", player.springStart)
			} else { //continuing the chain of springs
				i := state.AddSpring(player.currentThing, player.springStart, player.highlit.mass, false, true) //this is a spring-like thing (but not an acutal spring.. it has no length for example)
				t := state.Things[player.currentThing]

				s := springPayload{ti: player.currentThing, si: i, spring: *t.springs[i]}
				q4all(reply{cmd: "spring", payload: s})

				//this.currentThing.springs.push(new Spring(this.state.masses,me.springStart,this.highlit.mass,true))
				logit("Continued chain of springs from mass ", player.springStart, " to ", player.highlit.mass)
				//console.log("Current thing has", t.springs.length, "springs")
				player.springStart = player.highlit.mass
			}
		}
	} else if msg.cmd == "keyDown" {
		pl := msg.payload.(inKeyPayload) //type assert the payload
		k := pl.key
		shift := pl.shift
		ctrl := pl.ctrl

		if shift {
			prop = "scale"
			step = .1
		}
		if ctrl {
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
			sendCurrentThing(&state, player)
		} else if k == "s" {
			player.springStart = -1
			//player.mode = ModeEnum.addingSpring
		} else if k == "p" {
			m := state.Masses[player.highlit.mass]
			m.fixed = !m.fixed
			q4all(reply{cmd: "mass", payload: massPayload{i: player.highlit.mass, mass: *m}})
		}
	}

	//sends any tranform of the skin (done with cursor keys and/or shift)
	if dx != 0 || dy != 0 {
		if player.currentThing > -1 {
			ct := state.Things[player.currentThing]
			if prop == "rotation" {
				ct.rotation += dx
			} else { //if prop=="scale" {
				ct.scale.x += dx
				ct.scale.y += dy
			}
		}
		sendCurrentThing(&state, player) //TODO this could just be the skin
	}
}

func sendCurrentThing(state *State, player *Player) {
	ti := player.currentThing
	q4all(reply{cmd: "mass", payload: thingPayload{ti: ti, thing: *state.Things[ti]}})
}

func logit(s ...interface{}) {
	println(s)
}
