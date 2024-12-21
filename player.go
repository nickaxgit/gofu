package main

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
)

type highlitType struct {
	Mass   int `json:"mass"`
	Spring int `json:"spring"`
	Thing  int `json:"thing"`
}

type Player struct {
	Name           string `json:"playerName"`
	Dozer          int    `json:"dozer"` //index of dozer in the things array
	MaxDamage      byte   `json:"maxDamage"`
	MaxTemperature byte   `json:"maxTemperature"`
	worldCursor    Vector
	Damage         byte `json:"damage"`
	Temperature    byte `json:"temperature"`

	LeftDrive  float64 `json:"leftDrive"` //used as scratchpad clientside - so we need to init them
	RightDrive float64 `json:"rightDrive"`
	//touchControlled:boolean = false //set to true as soon as we get a touch event
	oRevs float32

	Coins          int `json:"coins"`
	stepCoinsValue int
	stepCoinCount  int

	Highlit highlitType `json:"highlit"` //= {mass:-1,spring:-1,thing:-1}

	springStart  int
	currentThing int
	mode         ModeEnum

	Killer int `json:"killer"`
	dying  bool
	dead   bool
	lives  int
	//qh          *qHolder //pointer to the queue of messages for this player
	//waitChannel chan bool
	mtx *sync.Mutex // a mutex is required to 'lock' access to each users connection (for writing)
	// many calls (to wsEndpoint) can be running in paralell - and more than one of them may attempt to write to a single users socket at the same time (not allowed!)
	socket *websocket.Conn // a pointer to the socket
}

func NewPlayer(name string, dozer int, state *State, socket *websocket.Conn) *Player {

	p := Player{Name: name, Dozer: dozer, MaxDamage: 100, MaxTemperature: 100, worldCursor: Vector{0, 0}, Damage: 0, Temperature: 0, LeftDrive: 0, RightDrive: 0, oRevs: 0, Coins: 0, stepCoinsValue: 0, stepCoinCount: 0, Highlit: highlitType{-1, -1, -1}, springStart: -1, currentThing: -1, mode: playing, Killer: -1, dying: false, dead: false, lives: 3, socket: socket}
	//p.qh = &qHolder{mutex: &sync.Mutex{}, q: make(map[int][]*reply, 0)} //initialise their outbound queue
	//p.waitChannel = make(chan bool)
	p.mtx = &sync.Mutex{}
	state.Tracks[p.Name] = &track{Pointer: 0, Points: make([]float64, 800)}

	return &p

}

func (p *Player) Send(msg *reply) {
	p.mtx.Lock() //<<---MUTEX
	defer p.mtx.Unlock()
	messageType := websocket.TextMessage

	bytes, err := json.Marshal(msg)
	if err != nil {
		logit(err.Error())
		return
	}

	p.socket.WriteMessage(messageType, bytes) //write the message (and return any error)
}
