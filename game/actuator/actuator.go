package actuator

type ActuatorEnum float64 //do vstab - also give masses mass -
const (
	NONE            ActuatorEnum = 0
	LeftEngine      ActuatorEnum = 1
	RightEngine     ActuatorEnum = 2
	LeftAileron     ActuatorEnum = 3
	RightAileron    ActuatorEnum = 4
	LeftElevator    ActuatorEnum = 5
	RightElevator   ActuatorEnum = 6
	LeftRudder      ActuatorEnum = 7
	RightRudder     ActuatorEnum = 8
	LeftFlap        ActuatorEnum = 9
	RightFlap       ActuatorEnum = 10
	LeftWheelBrake  ActuatorEnum = 11
	RightWheelBrake ActuatorEnum = 12
)

var SpringActuators = map[ActuatorEnum]string{
	NONE:          "NONE",
	LeftEngine:    "Left Engine",
	RightEngine:   "Right Engine",
	LeftAileron:   "Left Aileron",
	RightAileron:  "Right Aileron",
	LeftElevator:  "Left Elevator",
	RightElevator: "Right Elevator",
	LeftRudder:    "Left Rudder",
	RightRudder:   "Right Rudder",
	LeftFlap:      "Left Flap",
	RightFlap:     "Right Flap",
}

var MassActuators = map[ActuatorEnum]string{
	NONE:            "NONE",
	LeftWheelBrake:  "Left WheelBrake",
	RightWheelBrake: "Right WheelBrake",
}
