package global

import (
	"github.com/nickax/gofu/game/player"
)

var controlTokens = make(map[uint32]*player.Player, 0) //every login adds a random token to here to allow control by another player/device

//var success = device.SetStore(globalStore{}) //register the global store implementation

//NOTE there is no "SaveAll()" function things are constantly appended to the repository as they change
