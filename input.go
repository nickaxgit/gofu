package main

import (
// "math"
)

type controlInput byte

const (
	ciNONE            controlInput = 0
	ciStickX          controlInput = 1
	ciStickY          controlInput = 2
	ciThrottle        controlInput = 3
	ciRudder          controlInput = 4
	ciFlaps           controlInput = 5
	ciWheelBrakeLeft  controlInput = 6
	ciWheelBrakeRight controlInput = 7
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

	fcLeftRudder  actuatorEnum = 7
	fcRightRudder actuatorEnum = 8

	fcLeftFlap  actuatorEnum = 9
	fcRightFlap actuatorEnum = 10

	fcLeftWheelBrake  actuatorEnum = 11
	fcRightWheelBrake actuatorEnum = 12

	//remember to add actlabels if adding here
)

var actLabels = []string{"NONE", "LeftEngine", "RightEngine", "LeftAileron", "RightAileron", "LeftElevator", "RightElevator", "LeftRudder", "RightRudder", "LeftFlap", "RightFlap", "LeftWheelBrake", "RightWheelBrake"}

type sectionEnum float64 //do vstab - also give masses mass -
const (
	scCambered   sectionEnum = 0
	scSymetrical sectionEnum = 1
)

var sectionLabels = []string{"Cambered", "Symetrical"}

type mix struct {
	in controlInput
	//value      float64 //normalised input value (-1 to 1)
	min       float64 //what does "-1" on the stick map to
	max       float64
	engineNum int //1 based engine number (0 = none)
	//conversion float64 //final scaling/conversion at output (mostly to convert degrees to radians)
	actuator actuatorEnum //springs are 'tagged' with this - and 'bound' to the spring herein
	spring   *spring
	isBrake  bool //is this a brake (affects mass friction)
}

func (mix *mix) output(p *player) float64 {
	return ((p.controls[mix.in]+1)/2)*(mix.max-mix.min) + mix.min
}

var throw float64 = 0.2 //full throw of a control surface (in metres of actuator extension)
var standardMixers = []*mix{

	{ciStickX, -throw, throw, 0, fcLeftAileron, nil, false},
	{ciStickX, throw, -throw, 0, fcRightAileron, nil, false},

	{ciStickY, throw, -throw, 0, fcLeftElevator, nil, false},
	{ciStickY, throw, -throw, 0, fcRightElevator, nil, false},

	{ciRudder, -throw, throw, 0, fcLeftRudder, nil, false},
	{ciRudder, -throw, throw, 0, fcRightRudder, nil, false},

	{ciThrottle, 0, 1, 1, fcLeftEngine, nil, false}, //1000 newtons of reverse thrust 01 10k newtons of forward thrust
	{ciThrottle, 0, 1, 2, fcRightEngine, nil, false},

	//flaps
	{ciFlaps, -5, 5, 0, fcLeftAileron, nil, false},
	{ciFlaps, -5, 5, 0, fcRightAileron, nil, false},

	{ciWheelBrakeLeft, 0, 1, 0, fcLeftWheelBrake, nil, true},
	{ciWheelBrakeRight, 0, 1, 0, fcRightWheelBrake, nil, true},
}
