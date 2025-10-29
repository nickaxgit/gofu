package server

import (
	"github.com/gorilla/websocket"
	"github.com/nickax/gofu/fiz/thing"
	"github.com/nickax/gofu/game"
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/game/player"

	"github.com/nickax/gofu/jsonmsg"
	"github.com/nickax/gofu/log"

	"github.com/nickax/gofu/vec"

	"errors"
	"sync"
	"time"
)

var globalPlayers = map[string]*player.Player{}        //all players in all games game - by id
var controlTokens = make(map[uint32]*player.Player, 0) //every login adds a random token to here to allow control by another player/device
var Games = make(map[uint32]*game.State)               //the data of games in progress - by id

var nick = player.New(globalPlayers, 999, "nick", "", nil, vec.NewVec3(0, 0, 0)) //the nick player

func StepWorldsForever() {

	//gravTest()

	//every 100 ms step all worlds
	for range time.Tick(time.Millisecond * 33) { //<<waits here  //30fps
		//print(".") //<< this is the heartbeat
		for _, game := range Games {

			game.Step(5) //<- this is a physics step - it queues stuff for all players

			game.MoveCameras() //manual movement (keys and mouse)

		}
	}

	panic(`stepWorlds() has exited`)

}

func ProcessStructuredMsg(player *player.Player, jsonMessage *jsonmsg.Msg, ws *websocket.Conn) {
	game := Games[player.GameId]
	game.ProcessStructuredMsg(player, jsonMessage, ws)
}

func joinGame(gameId uint32, playerName string, ws *websocket.Conn) *player.Player {

	g, present := Games[gameId]
	if !present {
		//TODO this is very unfinished
		g = game.Load(gameId)
		Games[gameId] = state
		for _, p := range state.players {
			p.mtx = &sync.Mutex{}
		}
	}

	p, present := globalPlayers[playerId]

	if !present {
		p = state.AddPlayer(playerId, playerName, state.RandomStartPos(10000), ws)
	}

	p.socket = ws //this is important!
	//player.send(&reply{Cmd: "state", Payload: state})
	//state.send(nil, &reply{Cmd: "playerJoined", Payload: player})    //tell everyone about the new player
	p.sendCamera()                          //send the camera position
	p.sendMasses(state.masses, true)        //send all the masses
	p.sendThings([]*thing.Thing{p.vehicle}) //sends mesh name and springs
	p.sendGameId()                          //game id starts it running
	p.SendLabelSets()

	controlTokens[p] = p.sendControlPin() //send a PIN to them so they can take control from another device

	//cg, _ := p.vehicle.centreOfMass()

	//state.qSound("dozer", cg, 0.1,  true)
	//t.engineSound[0] = state.qSound("engineLoop", cg, 0.2, true)
	//t.engineSound[1] = state.qSound("engineLoop", cg, 0.2, true)

	return p
}

func ProcessCreateJoinOrControl(m *msg.Msg, ws *websocket.Conn) error {

	var playerId uint32
	var playerName string

	if m.MsgType == msg.CreateGame {

		m.Read(&playerId, &playerName)

		if playerName == "" {
			return errors.New("playerName cannot be empty for a Create")
		}

		player, present := globalPlayers[playerName]
		if !present {
			return errors.New("No such player")
		}
		if player.Id != playerId {
			return errors.New("Player ID does not match name")
		}
		player.Socket = ws //Bind the websocket to the player (as early as we can)

		game.NewGame(Games, player) //will make a game with a new ID and add the player to it

		return nil

	} else if m.MsgType == msg.JoinGame {

		m.Read(&gameId, &playerId, &playerName)
		joinGame(gameId, playerId, playerName, ws) //returns a player

	} else if m.MsgType == msg.JoinAsController {
		token := uint32(0)
		msg.Read(m.Buff, token)
		playerToControl := controlTokens[token]
		if playerToControl != nil {
			playerToControl.ControllerSocket = ws
			return playerToControl //return the player to which this token maps
		}
		log.Logit("No pin for token", token)
		return nil

	} else {
		panic("First message was not create, join or control")
	}

}
