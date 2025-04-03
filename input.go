package main

import (
// "math"
)

type controlInput byte

const (
	ciNONE     controlInput = 0
	ciStickX   controlInput = 1
	ciStickY   controlInput = 2
	ciThrottle controlInput = 3
	ciRudder   controlInput = 4
	ciFlaps    controlInput = 5
)

var inLabels = []string{"NONE", "StickX", "StickY", "Throttle", "Rudder", "Flaps"}

type actuatorEnum float64 //byte

const (
	fcNONE                     = 0
	fcLeftEngine  actuatorEnum = 1
	fcRightEngine actuatorEnum = 2

	fcLeftAileron  actuatorEnum = 3
	fcRightAileron actuatorEnum = 4

	fcLeftElevator  actuatorEnum = 5
	fcRightElevator actuatorEnum = 6

	fcRudder    actuatorEnum = 7
	fcLeftFlap  actuatorEnum = 8
	fcRightFlap actuatorEnum = 9
)

var actLabels = []string{"NONE", "LeftEngine", "RightEngine", "LeftAileron", "RightAileron", "LeftElevator", "RightElevator", "Rudder", "LeftFlap", "RightFlap"}

type sectionEnum float64 //do vstab - also give masses mass -
const (
	scCambered   sectionEnum = 0
	scSymetrical sectionEnum = 1
)

var sectionLabels = []string{"Cambered", "Symetrical"}

type mix struct {
	in controlInput
	//value      float64 //normalised input value (-1 to 1)
	min        float64 //wgat does "-1" on the stick map to
	max        float64
	isThrottle bool
	//conversion float64 //final scaling/conversion at output (mostly to convert degrees to radians)
	actuator actuatorEnum //springs are 'tagged' with this - and 'bound' to the spring herein
	spring   *spring
}

func (mix *mix) output(p *player) float64 {
	return ((p.controls[mix.in]+1)/2)*(mix.max-mix.min) + mix.min
}

var standardMixers = []*mix{
	{ciStickX, .5, -.5, false, fcLeftAileron, nil},
	{ciStickX, -.5, .5, false, fcRightAileron, nil},

	{ciStickY, .5, -.5, false, fcLeftElevator, nil},
	{ciStickY, .5, -.5, false, fcRightElevator, nil},

	{ciRudder, -15, 15, false, fcRudder, nil},
	{ciThrottle, -10000, 10000, true, fcRightEngine, nil},
	{ciThrottle, -10000, 10000, true, fcLeftEngine, nil},

	//flaps
	{ciFlaps, -5, 5, false, fcLeftAileron, nil},
	{ciFlaps, -5, 5, false, fcRightAileron, nil},
}
