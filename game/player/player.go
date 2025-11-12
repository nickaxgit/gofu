package player

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/rand/v2"

	"github.com/nickax/gofu/errorplus"
	"github.com/nickax/gofu/fiz/thing"
	"github.com/nickax/gofu/game"
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/mutex"
	"github.com/nickax/gofu/persist"
)

type Player struct {
	Id      uint32 //used in the players map
	Token   string
	Game    *game.Game //current game (can be nil)
	Name    string
	vehicle *thing.Thing //can be nil
	//reduntant - viewers have a ViewingPlayerId
	//Viewers     []*viewer.Viewer  //cameras watching this player (viewers are entirely 'anonymous')
	//controllers []*websocket.Conn ///devices controlling this player/vehicle
	coins uint32 //in-game currency
	xp    uint32 //experience points
	rp    uint32 //reputation points

	heading float64 //heading of the vehicle in degrees
	email   string
	hash    string
	salt    string

	//engineSounds        []uint16 //sound ids for the engine sounds (multi-engined aircraft)
}

func (p *Player) GetVehicle() *thing.Thing {
	return p.vehicle //which migh nil
}

func New(globalPlayers map[uint32]*Player, id uint32, name string, email string, hash string, salt string, token string, coins uint32, xp uint32, rp uint32, game *game.Game) *Player {

	player := &Player{Id: id,
		Name:    name,
		Game:    game,
		email:   email,
		coins:   coins,
		heading: 0,
		salt:    salt,
		hash:    hash, //(password, salt),
		Token:   token,
		xp:      xp,
		rp:      rp,
	}

	if game == nil {
		panic("game nil in player constructor (use game.None")
	}

	mutex.Players.Lock()
	globalPlayers[id] = player
	mutex.Players.Unlock()

	return player

}

// Checkpassword checks a plaintext password against the stored hash
func (p *Player) CheckPassword(pw string) bool {
	hash := Hash(pw, p.salt)
	return hash == p.hash
}

func (p *Player) Persist() *errorplus.Event {

	if p.Id == 0 {
		return errorplus.New(nil, errorplus.Critical, "cannot persist player with id 0")
	}
	pmsg := msg.NewMsg(msg.P_Player)
	p.WriteTo(pmsg) //write the player into the msg
	return persist.Append("repo.bin", pmsg)

}

func (player *Player) WriteTo(m *msg.Msg) {

	gid := uint32(0)
	if player.Game != nil {
		gid = player.Game.Id
	}

	vind := uint32(0)
	if player.vehicle != nil {
		vind = player.vehicle.Index
	}

	m.Write(player.Id, player.Name, gid,
		player.email, player.hash, player.salt, player.Token,
		player.coins, player.xp, player.rp, vind, msg.EndOfRecord) //thing index of their current vehicle (-1 is none)

}

func NewFromMsg(m *msg.Msg, games map[uint32]*game.Game, players map[uint32]*Player, things []*thing.Thing) (*Player, *errorplus.Event) {

	msgType := m.MsgType
	eor := byte(0)
	playerId, gameId := uint32(0), uint32(0)

	playerName, email, hash, salt, token := "", "", "", "", ""
	coins, xp, rp, vind := uint32(0), uint32(0), uint32(0), uint32(0)
	m.Read(&msgType, &playerId, &playerName, &gameId,
		&email, &hash, &salt, &token, &coins, &xp, &rp, &vind, &eor)

	if eor != byte(msg.EndOfRecord) {
		return nil, errorplus.New(nil, errorplus.Error, "Player msg missing EOR")
	}

	player := New(players, playerId, playerName, email, hash, salt, token, coins, xp, rp, nil)

	var game *game.Game = nil
	var present bool = false
	if gameId != 0 {
		game, present = games[gameId]
		if !present {
			return player, errorplus.New(nil, errorplus.Error, fmt.Sprintf("Player %s references missing game id %d", playerName, gameId))
		}

	}
	player.Game = game

	//TODO - vehicles are a posession..
	//they should be allowed in one game at a time, and only once

	// vehicleIndex := int32(0)
	// m.Read(&vehicleIndex)
	// if vehicleIndex > 0 {
	// 	player.vehicle = player.game.things[vehicleIndex]
	// }

	return player, nil
}

func (p *Player) SetVehicle(t *thing.Thing) {
	p.vehicle = t
}

func Salt() string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, 16)
	for i := range b {
		b[i] = letterBytes[rand.Int32N(int32(len(letterBytes)))]
	}
	return string(b)
}

func Hash(pw string, salt string) string {

	hasher := sha256.New()
	hasher.Write([]byte(pw + salt + "$~pepper3n3ss!"))
	sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))

	return sha
}
