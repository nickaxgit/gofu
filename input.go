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

type ActuatorEnum float64 //do vstab - also give masses mass -
const (
	fcNONE            ActuatorEnum = 0
	fcLeftEngine      ActuatorEnum = 1
	fcRightEngine     ActuatorEnum = 2
	fcLeftAileron     ActuatorEnum = 3
	fcRightAileron    ActuatorEnum = 4
	fcLeftElevator    ActuatorEnum = 5
	fcRightElevator   ActuatorEnum = 6
	fcLeftRudder      ActuatorEnum = 7
	fcRightRudder     ActuatorEnum = 8
	fcLeftFlap        ActuatorEnum = 9
	fcRightFlap       ActuatorEnum = 10
	fcLeftWheelBrake  ActuatorEnum = 11
	fcRightWheelBrake ActuatorEnum = 12
)

var springActuators = map[ActuatorEnum]string{
	fcNONE:          "NONE",
	fcLeftEngine:    "Left Engine",
	fcRightEngine:   "Right Engine",
	fcLeftAileron:   "Left Aileron",
	fcRightAileron:  "Right Aileron",
	fcLeftElevator:  "Left Elevator",
	fcRightElevator: "Right Elevator",
	fcLeftRudder:    "Left Rudder",
	fcRightRudder:   "Right Rudder",
	fcLeftFlap:      "Left Flap",
	fcRightFlap:     "Right Flap",
}

var massActuators = map[ActuatorEnum]string{
	fcNONE:            "NONE",
	fcLeftWheelBrake:  "Left WheelBrake",
	fcRightWheelBrake: "Right WheelBrake",
}

type sectionEnum float64 //do vstab - also give masses mass -
const (
	scCambered   sectionEnum = 0
	scSymetrical sectionEnum = 1
)

var sections = map[sectionEnum]string{scCambered: "Cambered", scSymetrical: "Symetrical"}

type mix struct {
	in controlInput
	//value      float64 //normalised input value (-1 to 1)
	min       float64 //what does "-1" on the stick map to
	max       float64
	engineNum int //1 based engine number (0 = none)
	//conversion float64 //final scaling/conversion at output (mostly to convert degrees to radians)
	actuator ActuatorEnum //float64 //springActuatorEnum //springs/masses are 'tagged' with this - and 'bound' to the spring/mass herein
	spring   *spring      //springs are like hydraulic cylinders (unles they are engines)
	mass     *mass        //masses are brakes or driven wheels (they must have an axle)
	//isBrake  bool //is this a brake (affects mass friction)
}

func newMix(controlIn controlInput, min float64, max float64, engineNum int, actuator ActuatorEnum) *mix { //,spring *spring,mass *mass,isBrake bool) *mix {
	return &mix{in: controlIn, min: min, max: max, engineNum: engineNum, actuator: actuator, spring: nil, mass: nil}
}

func (mix *mix) output(p *player) float64 {
	return ((p.controls[mix.in]+1)/2)*(mix.max-mix.min) + mix.min
}

var throw float64 = 0.2 //full throw of a control surface (in metres of actuator extension)
var standardMixers = []*mix{

	newMix(ciStickX, -throw, throw, 0, fcLeftAileron),
	newMix(ciStickX, throw, -throw, 0, fcRightAileron),

	newMix(ciStickY, throw, -throw, 0, fcLeftElevator),
	newMix(ciStickY, throw, -throw, 0, fcRightElevator),

	newMix(ciRudder, -throw, throw, 0, fcLeftRudder),
	newMix(ciRudder, -throw, throw, 0, fcRightRudder),

	newMix(ciThrottle, 0, 1, 1, fcLeftEngine), // 1000 newtons of reverse thrust 01 10k newtons of forward thrust
	newMix(ciThrottle, 0, 1, 2, fcRightEngine),

	// flaps
	newMix(ciFlaps, -5, 5, 0, fcLeftAileron),
	newMix(ciFlaps, -5, 5, 0, fcRightAileron),

	newMix(ciWheelBrakeLeft, 1, 0, 0, fcRightWheelBrake),
	newMix(ciWheelBrakeRight, 1, 0, 0, fcLeftWheelBrake),

	//{ciRudder, -0.5, 0.5, 0, fcRightWheelBrake, nil, true}, //rudder is a brake on the wheel - not a control surface

}
