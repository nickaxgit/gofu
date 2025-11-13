package player

import (
	"strings"
	"sync"

	"github.com/nickax/gofu/crypto"
	"github.com/nickax/gofu/errorplus"
	"github.com/nickax/gofu/fiz/thing"
	"github.com/nickax/gofu/game"
	"github.com/nickax/gofu/game/msg"

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

var players = make(map[uint32]*Player, 0)
var playerMutex = sync.RWMutex{}

var playersByName = make(map[string]*Player, 0)
var pbnMutex = sync.RWMutex{}

func GetByName(name string) *Player {
	name = strings.ToLower(name)
	pbnMutex.RLock()
	p, present := playersByName[name]
	pbnMutex.RUnlock()
	if !present {
		return None //player.none
	}
	return p //happy path
}
func Get(id uint32) *Player {
	playerMutex.RLock()
	p, present := players[id]
	playerMutex.RUnlock()
	if !present {
		return None //player.none
	}
	return p
}

func Set(p *Player) {
	playerMutex.Lock()
	players[p.Id] = p
	playerMutex.Unlock()

	pbnMutex.Lock()
	playersByName[strings.ToLower(p.Name)] = p
	pbnMutex.Unlock()
}

func PlayersByGame() map[*game.Game][]*Player {
	result := make(map[*game.Game][]*Player, 0)
	playerMutex.RLock()
	defer playerMutex.RUnlock()
	for _, p := range players {
		g := p.Game

		plist, present := result[g]
		if !present {
			plist = make([]*Player, 0)
			result[g] = plist
		}
		result[g] = append(plist, p)
	}
	return result
}

func (p *Player) GetVehicle() *thing.Thing {
	return p.vehicle //which migh nil
}

func New(id uint32, name string, email string, hash string, salt string, token string, coins uint32, xp uint32, rp uint32, game *game.Game) *Player {

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

	return player

}

// Checkpassword checks a plaintext password against the stored hash
func (p *Player) CheckPassword(pw string) bool {
	hash := crypto.Hash(pw, p.salt)
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

func NewFromMsg(m *msg.Msg) (*Player, *errorplus.Event) {

	//msgType := msg.MsgEnum(0)
	eor := byte(0)
	playerId, gameId := uint32(0), uint32(0)

	playerName, email, hash, salt, token := "", "", "", "", ""
	coins, xp, rp, vind := uint32(0), uint32(0), uint32(0), uint32(0)
	//DONT read the msgType (we've done that already!)

	m.Read(&playerId)
	m.Read(&playerName)
	m.Read(&gameId)
	m.Read(&email, &hash, &salt, &token,
		&coins, &xp, &rp, &vind)

	m.Read(&eor)

	if eor != byte(msg.EndOfRecord) {
		return nil, errorplus.New(nil, errorplus.Error, "Player msg missing EOR")
	}

	player := New(playerId, playerName, email, hash, salt, token, coins, xp, rp, game.Get(gameId))

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
