package player

import (
	"crypto/sha256"
	"encoding/base64"
	"github.com/nickax/gofu/fiz/thing"
	"github.com/nickax/gofu/game"
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/log"
	"math/rand/v2"
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
	coins   uint32  //in-game currency
	heading float64 //heading of the vehicle in degrees
	email   string
	hash    string
	salt    string

	//engineSounds        []uint16 //sound ids for the engine sounds (multi-engined aircraft)
}

func (p *Player) GetVehicle() *thing.Thing {
	return p.vehicle //which migh nil
}

func New(globalPlayers map[uint32]*Player, id uint32, name string, email string, hash string, salt string, token string, coins uint32, game *game.Game) *Player {

	existing, present := globalPlayers[id]
	if present {
		log.Logit("player with this ID already exists", id, existing.Name)
		panic("player with this ID already exists")
	}

	player := &Player{Id: id,
		Name:    name,
		Game:    game,
		email:   email,
		coins:   coins,
		heading: 0,
		salt:    salt,
		hash:    hash, //(password, salt),
		Token:   token,
	}

	globalPlayers[id] = player

	return player

}

func NewFromMsg(games map[uint32]*game.Game, players map[uint32]*Player, m *msg.Msg, things []*thing.Thing) *Player {

	playerId := uint32(0)
	gameId := uint32(0)
	playerName, email, hash, salt, token := "", "", "", "", ""
	coins := uint32(0)
	m.Read(&playerId, &playerName, &gameId, &email, &hash, &salt, &token, &coins)

	player := New(players, playerId, playerName, email, hash, salt, token, coins, games[gameId])

	vehicleIndex := int32(0)
	m.Read(&vehicleIndex)
	if vehicleIndex > -1 {
		player.vehicle = things[vehicleIndex]
	}

	return player
}

func (player *Player) WriteTo(msg *msg.Msg) {
	msg.Write(player.Id, //unique player ID (Uint32)
		player.Name, //curent player name (may change)
		player.Game.Id,
		player.vehicle.Index) //thing index of their current vehicle (-1 is none)

}

func (p *Player) SetVehicle(t *thing.Thing) {
	p.vehicle = t
}

func playersToMsg(players map[uint32]*Player) *msg.Msg {

	msg := msg.NewMsg(msg.Players, uint32(len(players)))

	for _, player := range players {
		player.WriteTo(msg)
	}

	return msg
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
