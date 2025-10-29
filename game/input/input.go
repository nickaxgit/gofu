package input

type ControlInput byte

const (
	NONE            ControlInput = 0
	StickX          ControlInput = 1
	StickY          ControlInput = 2
	Throttle        ControlInput = 3
	Rudder          ControlInput = 4
	Flaps           ControlInput = 5
	WheelBrakeLeft  ControlInput = 6
	WheelBrakeRight ControlInput = 7
)

var InLabels = []string{"NONE", "StickX", "StickY", "Throttle", "Rudder", "Flaps", "WheelBrakeLeft", "WheelBrakeRight"}
