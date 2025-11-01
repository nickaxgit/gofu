package server

import (
	"github.com/gorilla/websocket"
	"github.com/nickax/gofu/fiz/thing"
	"github.com/nickax/gofu/game"
	"github.com/nickax/gofu/game/player"
	"github.com/nickax/gofu/global"
	"github.com/nickax/gofu/viewer"

	"sync"
	"time"
)

//var nick = player.New(globalPlayers, uint32(999), "nick", 0) //the nick player

func StepWorldsForever() {

	//gravTest()

	//every 100 ms step all worlds
	for range time.Tick(time.Millisecond * 33) { //<<waits here  //30fps
		//print(".") //<< this is the heartbeat
		for _, game := range global.Games {

			cvs := currentViewers(game) //TODO optimise (cache this)
			//game.Step(5) //<- this is a physics step - it queues stuff for all players
			if game.Running {
				for _, t := range game.Things {
					t.UpdateTelemetry()
				}

				game.Burn()

				msgs := game.MoveAll(5) //<- this is a physics step - it returns a message containing moved masses
				for _, viewer := range cvs {
					viewer.Send(msgs) //send the moved masses

					vehicle := viewer.GetVehicle(global.Players)
					if vehicle != nil {
						//viewer.Send(mass.Vectors())
						viewer.SendVectors()
						viewer.Send(vehicle.GetTelemetry())
					}

					//viewer.FollowVehicleWithCamera()

					viewer.SendCamera()
					viewer.SendLabels()

					if viewer.ViewChangedSignificantly() {

						go viewer.GetFlames(game.fire) //update visible flames for this player
						go func() {
							msgs := game.makeAndSendLandTo(viewer.Camera.Position, viewer.Camera.Direction) //makes and sends new land (different for every viewer)
							viewer.Send(msgs...)
						}()
					}

					viewer.MoveCamera(game) //move the camera according to input
				}
			}

		}
	}

	panic(`stepWorlds() has exited`)

}

func currentViewers(game *game.State) []*viewer.Viewer {
	current := []*viewer.Viewer{}
	for _, v := range global.Viewers {
		player := v.GetPlayer()
		if player != nil {
			if player.Game == game {
				current = append(current, v)
			}
		}
	}
	return current
}

func joinGame(gameId uint32, playerName string, ws *websocket.Conn) *player.Player {

	g, present := Games[gameId]
	if !present {
		//TODO this is very unfinished
		g = game.Load(Games, string(gameId)+".bin")

		for _, p := range g.Players {
			for _, v := range p.Viewers {
				v.Mtx = &sync.Mutex{}
				v.InMtx = &sync.Mutex{}
			}
		}
	}

	p, present := globalPlayers[playerId]

	if !present {
		p = state.AddPlayer(playerId, playerName, state.RandomStartPos(10000), ws)
	} else {
		logit("Player already exists:", playerId)
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

// func ProcessCreateJoinOrControl(m *msg.Msg, ws *websocket.Conn) error {

// 	var playerId uint32
// 	var playerName string

// 	switch m.MsgType {
// 	case msg.CreateGame:

// 		aircraft = nil //gc (eventually)

// 		player.SetVehicle(vehicle)

// 		gm.SceneStart(player, viewer) //sends stuff to the viewer
// 		viewer.SendGameId(gameId)     //game id starts it running
// 		viewer.SendControlPin()

// 		log.Logit("Game created", game.GameId)

// 		return nil

// 	case msg.JoinGame:

// 		m.Read(&gameId, &playerId, &playerName)
// 		joinGame(gameId, playerId, playerName, ws) //returns a player

// 	case msg.JoinAsController:
// 		token := uint32(0)
// 		msg.Read(m.Buff, token)
// 		playerToControl := controlTokens[token]
// 		if playerToControl != nil {
// 			playerToControl.ControllerSocket = ws
// 			return playerToControl //return the player to which this token maps
// 		}
// 		log.Logit("No pin for token", token)
// 		return nil

// 	default:
// 		panic("First message was not create, join or control")
// 	}

// }
