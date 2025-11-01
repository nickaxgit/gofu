package player

import (
	"github.com/gorilla/websocket"
	"github.com/nickax/gofu/fiz/mixer"
	"github.com/nickax/gofu/fiz/thing"
	"github.com/nickax/gofu/game"
	"github.com/nickax/gofu/game/msg"
)

type Player struct {
	Id      uint32      //used in the players map
	Game    *game.State //current game (can be nil)
	Name    string
	vehicle *thing.Thing //can be nil
	//reduntant - viewers have a ViewingPlayerId
	//Viewers     []*viewer.Viewer  //cameras watching this player (viewers are entirely 'anonymous')
	controllers []*websocket.Conn ///devices controlling this player/vehicle

	//each mixer (of the player) adds a contribution to to one mass (e.g. an aileron)
	Mixers  []*mixer.Mixer
	heading float64 //heading of the vehicle in degrees

	//engineSounds        []uint16 //sound ids for the engine sounds (multi-engined aircraft)
}

func (p *Player) RunEngines() []*msg.Msg {
	msgs := []*msg.Msg{}
	if p.vehicle != nil {
		for _, engine := range p.vehicle.Engines {
			msgs = append(msgs, engine.Run()...) //Moves the masses (of the engines spring)
		}
	}
	return msgs
}

func SendToMany(players []*Player, msg ...*msg.Msg) {
	if len(msg) == 0 {
		return
	}
	for _, p := range players {
		p.Send(msg...)
	}
}

func IdsAsMsg(players []*Player) *msg.Msg {

	msg := msg.NewMsg(msg.PlayerIds, uint32(len(players)))

	for _, p := range players {
		msg.Write(p.Id)
	}

	return msg
}
func (p *Player) GetVehicle() *thing.Thing {
	return p.vehicle //which migh nil
}

func New(globalPlayers map[uint32]*Player, id uint32, name string, game *game.State) *Player {

	player := &Player{Id: id,
		Name:   name,
		Game:   game,
		Mixers: mixer.StandardMixers,
	}

	globalPlayers[id] = player

	return player

}

func NewFromMsg(games map[uint32]*game.State, players map[uint32]*Player, m *msg.Msg, things []*thing.Thing) *Player {

	playerId := uint32(0)
	gameId := uint32(0)
	playerName := ""
	m.Read(&playerId, &playerName, &gameId)

	player := New(players, playerId, playerName, games[gameId])

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
		player.Game.GameId,
		player.vehicle.Index) //thing index of their current vehicle (-1 is none)

}

func (p *Player) SetVehicle(t *thing.Thing) {
	p.vehicle = t
}

// func (p *Player) AddViewer(pov string, ws *websocket.Conn) *viewer.Viewer {

// 	viewer := viewer.NewViewer(p.Viewers, p.Id, pov, ws)
// 	globalViewers[viewer.Id] = viewer
// 	p.Viewers = append(p.Viewers, viewer)
// 	return viewer.New(p.Viewers, p.vehicle, pov, ws)

// }

func playersToMsg(players map[uint32]*Player) *msg.Msg {

	msg := msg.NewMsg(msg.Players, uint32(len(players)))

	for _, player := range players {
		player.WriteTo(msg)
	}

	return msg
}
