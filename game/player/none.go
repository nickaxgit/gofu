package player

import (
	"github.com/nickax/gofu/game"
)

var junk = make(map[uint32]*Player)
var None = New(junk, 0, "nobody", "nobody@nowhere.com", "hash", "salt", "token", 0, 0, 0, game.None)
