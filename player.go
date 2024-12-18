package main

import (
	"sync"
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

	Coins     int `json:"coins"`
	stepCoins int

	Highlit highlitType `json:"highlit"` //= {mass:-1,spring:-1,thing:-1}

	springStart  int
	currentThing int
	mode         ModeEnum

	Killer int `json:"killer"`
	dying  bool
	dead   bool
	lives  int
	qh     *qHolder //pointer to the queue of messages for this player
}

func NewPlayer(name string, dozer int) *Player {

	p := Player{Name: name, Dozer: dozer, MaxDamage: 100, MaxTemperature: 100, worldCursor: Vector{0, 0}, Damage: 0, Temperature: 0, LeftDrive: 0, RightDrive: 0, oRevs: 0, Coins: 0, stepCoins: 0, Highlit: highlitType{-1, -1, -1}, springStart: -1, currentThing: -1, mode: playing, Killer: -1, dying: false, dead: false, lives: 3}
	p.qh = &qHolder{mutex: &sync.Mutex{}, q: make([]*reply, 0)} //initialise their outbound queue
	return &p

}
