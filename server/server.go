package server

import (
	"github.com/nickax/gofu/game"

	"github.com/nickax/gofu/device"
	"github.com/nickax/gofu/game/msg"
	//	"github.com/nickax/gofu/log"

	"time"
)

func StepWorldsForever() {

	//gravTest()

	//every 100 ms step all worlds
	for range time.Tick(time.Millisecond * 33) { //<<waits here  //30fps
		//print(".") //<< this is the heartbeat
		ar := game.AllRunning()
		for _, game := range ar {

			cvs, lands := device.ViewersOf(game) //game.ViewerscurrentViewers(game) //TODO optimise (cache this) - also shouldn't need to collect/pass lands
			//game.UpdateCurrentPlayersAndViewers() //don't need to do this every cycle
			//game.Step(5) //<- this is a physics step - it queues stuff for all players

			for _, t := range game.Things {
				t.UpdateTelemetry() //each thing has a telemetry property (which is an msg.Msg)
			}

			game.Burn()

			//response := game.MoveAll(5, lands) //<- this is a physics step - it returns a message containing moved masses
			game.MoveAll(5, lands) //<- this is a physics step - it returns a message containing moved masses
			//log.Logit(response)

			for _, viewer := range cvs {
				//	viewer.Send(response) //send the moved masses (and engine sounds)

				vehicle := viewer.GetVehicle()
				if vehicle != nil {
					//viewer.Send(mass.Vectors())
					viewer.SendVectors(game)
					viewer.Send(vehicle.GetTelemetry())
				}

				//viewer.FollowVehicleWithCamera()

				viewer.SendCamera()
				//viewer.SendLabels()

				if viewer.Land == nil || viewer.ViewChangedSignificantly() {

					// go func() {
					// 	if game.GetFire().Root.FireInfo == nil {
					// 		log.Logit("no fire info on root")
					// 		return
					// 	}
					// 	message := msg.Empty()
					// 	viewer.GetFlames(game.GetFire(), message) //update visible flames for this player
					// 	viewer.Send(message)
					// }()
					//go func() {

					message := msg.Empty()
					//use copies of the camera position/direction (as camera is potentially mutated on the main thread)
					viewer.MakeLand(game, message)

					viewer.Send(message)
					//}()
				}

				viewer.MoveCamera(game) //move the camera (and any selected masses)according to keyboard input
			}

		}

	}

	panic(`stepWorlds() has exited`)

}
