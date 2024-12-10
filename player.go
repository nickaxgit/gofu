package main

import ()

type highlitType struct {
	mass   int
	spring int
	thing  int
}

type Player struct {
	name           string
	dozer          int //index of dozer in the things array
	maxDamage      byte
	maxTemperature byte
	worldCursor    Vector
	damage         byte
	temperature    byte

	leftDrive  float64
	rightDrive float64
	//touchControlled:boolean = false //set to true as soon as we get a touch event
	oRevs float32

	coins     int
	stepCoins int

	highlit highlitType //= {mass:-1,spring:-1,thing:-1}

	springStart  int
	currentThing int
	mode         ModeEnum

	killer int //Player
	dying  bool
}

func NewPlayer(name string, dozer int) *Player {
	return &Player{name: name, dozer: dozer, maxDamage: 100, maxTemperature: 100, worldCursor: Vector{0, 0}, damage: 0, temperature: 0, leftDrive: 0, rightDrive: 0, oRevs: 0, coins: 0, stepCoins: 0, highlit: highlitType{-1, -1, -1}, springStart: -1, currentThing: -1, mode: playing, killer: -1, dying: false}

}
