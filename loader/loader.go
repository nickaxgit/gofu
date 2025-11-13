package loader

import (
	"github.com/nickax/gofu/device"
	"github.com/nickax/gofu/errorplus"
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/game/player"
	"github.com/nickax/gofu/next"
	"github.com/nickax/gofu/persist"
	"os"
)

func initIfAbsent(filename string) {
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		msg := msg.NewMsg(msg.Repository)
		persist.Append(filename, msg)
	}
}

func LoadAll(filename string) *errorplus.Event {

	erp := next.LoadCounters(filename + ".counters")

	if erp.Err != nil {
		return erp
	}

	initIfAbsent(filename)

	file, err := os.Open(filename)
	if err != nil {
		return errorplus.New(err, errorplus.Error, "Failed to open file for reading: "+filename)
	}
	defer file.Close()

	allBytes := make([]byte, 100000)

	n, err := file.Read(allBytes)
	if err != nil {
		return errorplus.New(err, errorplus.Error, "Failed to read from file: "+filename)
	}

	if n == len(allBytes) {
		return errorplus.New(nil, errorplus.Critical, "File too large to read fully: "+filename)
	}

	repo := msg.NewFromBytes(allBytes[:n])

	if repo.MsgType != msg.Repository {
		return errorplus.New(nil, errorplus.Critical, "File is not a repository: "+filename)
	}

	var msgType msg.MsgEnum
	for repo.Remaining() > 0 {
		repo.Read(&msgType)

		switch msgType {
		case msg.P_Player:
			p, _ := player.NewFromMsg(repo) //, store.GlStoreImpl)
			player.Set(p)

		case msg.P_Device:
			d := device.NewFromMsg(repo)
			device.Set(d)
		case msg.P_Asset:
		case msg.P_Possesion:
		case msg.P_Transaction:
		case msg.P_DeviceChange:
			// 		ViewingPlayer     *player.Player
			// isIndependent     bool           //this device has its own camera (otherwise it sees exaclty what the player sees)
			// primaryControls   *player.Player //this is set to a target player when permission is granted, and set back to the owner if it is revoked, expires, or the controller leaves
			// secondaryControls *player.Player //this is set to a target player when permission is granted, and set back to the owner if it is revoked, expires, or the controller leaves
			// Owner             *player.Player
		case msg.P_Game: //snapshot of game - including things,masses,land (players and devices not included)

		}
	}

	return errorplus.New(nil, errorplus.Info, "Loaded repository from "+filename)

}
