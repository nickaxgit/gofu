package global

import (
	"github.com/nickax/gofu/device"
	"github.com/nickax/gofu/game"
	"github.com/nickax/gofu/game/player"
)

var Players = map[uint32]*player.Player{}              //all players in all games, by id
var controlTokens = make(map[uint32]*player.Player, 0) //every login adds a random token to here to allow control by another player/device
var Games = make(map[uint32]*game.Game)                //the data of games in progress - by id
var Devices = make(map[uint32]*device.Device)          //the viewers in all games, by id (or reconnection by id)

var Nobody = player.New(Players, uint32(0), "nobody", "", "", "", "", 0, 0, 0, nil) //the nobody player
var Nothing = device.New(Devices, uint32(0), "nothing", Nobody, nil, nil, nil, nil) //the nothing device
var NoGame = game.New(Games, uint32(0), "nogame")                                   //the nogame game

//NOTE there is no "SavAll()" function things are constantly appended to the repository as they change
