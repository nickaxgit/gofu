package global

import (
	"github.com/nickax/gofu/game"
	"github.com/nickax/gofu/game/player"
	"github.com/nickax/gofu/viewer"
)

var Players = map[uint32]*player.Player{}              //all players in all games, by id
var controlTokens = make(map[uint32]*player.Player, 0) //every login adds a random token to here to allow control by another player/device
var Games = make(map[uint32]*game.Game)                //the data of games in progress - by id
var Viewers = make(map[uint32]*viewer.Viewer)          //the viewers in all games, by id (or reconnection by id)
