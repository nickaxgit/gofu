package server

import (
	"github.com/nickax/gofu/game"

	"github.com/nickax/gofu/device"
	"github.com/nickax/gofu/game/msg"

	"time"
)

func StepWorldsForever() {

	//gravTest()

	//every 100 ms step all worlds
	for range time.Tick(time.Millisecond * 33) { //<<waits here  //30fps
		//print(".") //<< this is the heartbeat
		for _, game := range game.AllRunning() {

			cvs, lands := device.ViewersOf(game) //game.ViewerscurrentViewers(game) //TODO optimise (cache this) - also shouldn't need to collect/pass lands
			//game.UpdateCurrentPlayersAndViewers() //don't need to do this every cycle
			//game.Step(5) //<- this is a physics step - it queues stuff for all players

			for _, t := range game.Things {
				t.UpdateTelemetry() //each thing has a telemetry property (which is an msg.Msg)
			}

			game.Burn()

			response := game.MoveAll(5, lands) //<- this is a physics step - it returns a message containing moved masses
			for _, viewer := range cvs {
				viewer.Send(response) //send the moved masses (and engine sounds)

				vehicle := viewer.GetVehicle()
				if vehicle != nil {
					//viewer.Send(mass.Vectors())
					viewer.SendVectors(game)
					viewer.Send(vehicle.GetTelemetry())
				}

				//viewer.FollowVehicleWithCamera()

				viewer.SendCamera()
				viewer.SendLabels()

				if viewer.ViewChangedSignificantly() {

					go func() {
						message := msg.Empty()
						viewer.GetFlames(game.GetFire(), message) //update visible flames for this player
						viewer.Send(message)
					}()
					go func() {
						message := msg.Empty()
						game.MakeLand(viewer.Camera.Position, viewer.Camera.Direction, message)
						viewer.Send(message)
					}()
				}

				viewer.MoveCamera(game) //move the camera according to input
			}

		}
	}

	panic(`stepWorlds() has exited`)

}

// func joinGame(gameId uint32, playerName string, ws *websocket.Conn) *player.Player {

// 	g, present := Games[gameId]
// 	if !present {
// 		//TODO this is very unfinished
// 		g = game.Load(Games, string(gameId)+".bin")

// 		for _, p := range g.Players {
// 			for _, v := range p.Viewers {
// 				v.Mtx = &sync.Mutex{}
// 				v.InMtx = &sync.Mutex{}
// 			}
// 		}
// 	}

// 	p, present := globalPlayers[playerId]

// 	if !present {
// 		p = state.AddPlayer(playerId, playerName, state.RandomStartPos(10000), ws)
// 	} else {
// 		logit("Player already exists:", playerId)
// 	}

// }

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
