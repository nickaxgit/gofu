package device

import (
	"github.com/nickax/gofu/game/player"
)

var owner *player.Player = player.None
var viewing *player.Player = player.None
var primarycontrols *player.Player = player.None
var secondaryControls *player.Player = player.None

var None = New(0, "no device", owner, viewing, primarycontrols, secondaryControls, "notoken", nil)
