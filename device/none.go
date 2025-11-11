package device

import (
	"github.com/nickax/gofu/game/player"
)

var junk = make(map[uint32]*Device)

var owner *player.Player = player.None
var viewing *player.Player = player.None
var primarycontrols *player.Player = player.None
var secondaryControls *player.Player = player.None

var None = New(junk, 0, "no device", owner, viewing, primarycontrols, secondaryControls, nil)
