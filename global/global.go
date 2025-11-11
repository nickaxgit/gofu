package global

import (
	"github.com/nickax/gofu/device"
	"github.com/nickax/gofu/game"
	"github.com/nickax/gofu/game/player"
)

var controlTokens = make(map[uint32]*player.Player, 0) //every login adds a random token to here to allow control by another player/device

// initialise the maps including the none objects ("sentinel" values) - these *aren't* put in on construction as it create an import cycle and bootstrap problem
var Players = map[uint32]*player.Player{0: player.None} //all players in all games, by id
var Games = map[uint32]*game.Game{0: game.None}         //the data of games in progress - by id
var Devices = map[uint32]*device.Device{0: device.None} //the viewers in all games, by id (or reconnection by id)

//NOTE there is no "SavAll()" function things are constantly appended to the repository as they change
