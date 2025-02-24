package main

import (
	"time"
)

func gravTest() {

	y := 10.0
	//oy := y

	gravity := 9.81 * (1 / (100.0 * 100.0))

	v := 0.0
	//ov := 0.0

	ts := time.Now()
	for range time.Tick(time.Millisecond * 10) { //<<waits here  //30fps

		y += v
		v -= gravity

		// oy = y
		// y += v - gravity

		//logit("y:", y, "v:", v, "t:", time.Since(ts).Milliseconds())
		if y < 0 {
			break
		}

		// v = y - oy

	}

	logit("hit the ground @", v*100.0, "m/s", time.Since(ts).Milliseconds())
	panic("")

}
