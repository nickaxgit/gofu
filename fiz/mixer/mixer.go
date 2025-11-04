package mixer

import (
	"github.com/nickax/gofu/fiz/engine"
	"github.com/nickax/gofu/fiz/mass"
	"github.com/nickax/gofu/fiz/spring"
	"github.com/nickax/gofu/game/actuator"
	"github.com/nickax/gofu/game/input"
)

type Mixer struct {
	in input.ControlInput
	//value      float64 //normalised input value (-1 to 1)
	min         float64 //what does "-1" on the stick map to
	max         float64
	EngineIndex int //this is effectively the 'tag' of the engine - so we can bind the same mixers in multiple aircraft
	//conversion float64 //final scaling/conversion at output (mostly to convert degrees to radians)
	Actuator actuator.ActuatorEnum //float64 //springActuatorEnum //springs/masses are 'tagged' with this - and 'bound' to the spring/mass herein
	Spring   *spring.Spring        //springs are like hydraulic cylinders (unles they are engines)
	Mass     *mass.Mass            //masses are brakes or driven wheels (they must have an axle)
	Engine   *engine.Engine        //once bound - this has a value
	//isBrake  bool //is this a brake (affects mass friction)
}

// func NewMixer(controlIn input.ControlInput, min float64, max float64, engine *engine.Engine, actuator actuator.ActuatorEnum) *Mixer { //,spring *spring,mass *mass,isBrake bool) *mix {
func NewMixer(controlIn input.ControlInput, min float64, max float64, engineIndex int, actuator actuator.ActuatorEnum) *Mixer { //,spring *spring,mass *mass,isBrake bool) *mix {
	return &Mixer{in: controlIn, min: min, max: max, EngineIndex: engineIndex, Actuator: actuator, Spring: nil, Mass: nil}
	//return &Mixer{in: controlIn, min: min, max: max, Engine:engine, Actuator: actuator, Spring: nil, Mass: nil}
}

var throw float64 = 0.2 //full throw of a control surface (in metres of actuator extension)

var StandardMixers = []*Mixer{

	NewMixer(input.StickX, -throw, throw, 0, actuator.LeftAileron),
	NewMixer(input.StickX, throw, -throw, 0, actuator.RightAileron),

	NewMixer(input.StickY, throw, -throw, 0, actuator.LeftElevator),
	NewMixer(input.StickY, throw, -throw, 0, actuator.RightElevator),

	NewMixer(input.Rudder, -throw, throw, 0, actuator.LeftRudder),
	NewMixer(input.Rudder, -throw, throw, 0, actuator.RightRudder),

	NewMixer(input.Throttle, 0, 1, 1, actuator.LeftEngine), // 1000 newtons of reverse thrust 01 10k newtons of forward thrust
	NewMixer(input.Throttle, 0, 1, 2, actuator.RightEngine),

	// flaps
	NewMixer(input.Flaps, -5, 5, 0, actuator.LeftAileron),
	NewMixer(input.Flaps, -5, 5, 0, actuator.RightAileron),

	NewMixer(input.WheelBrakeLeft, 1, 0, 0, actuator.LeftWheelBrake),
	NewMixer(input.WheelBrakeRight, 1, 0, 0, actuator.RightWheelBrake),

	//{ciRudder, -0.5, 0.5, 0, fcRightWheelBrake, nil, true}, //rudder is a brake on the wheel - not a control surface

}

func (mix *Mixer) output(controlInputs map[input.ControlInput]float64) float64 {
	return ((controlInputs[mix.in]+1)/2)*(mix.max-mix.min) + mix.min
}

func (mix *Mixer) ZeroOutputs() {
	if mix.Spring != nil {
		mix.Spring.Expansion = 0
	}
	if mix.Mass != nil {
		mix.Mass.Brake = 0
	}
}

func (mixer *Mixer) Mix(controlInputs map[input.ControlInput]float64) {
	if mixer.Engine != nil { //EngineIndex > -1 { //

		mixer.Engine.Throttle(mixer.output(controlInputs))

		//thrust is genrated and applied to the engines spring in runEngines()

	} else {
		//o := mix.output(p)
		//logit(actLabels[int(mix.actuator)], o)
		if mixer.Mass != nil { //it's a mass actuator (a brake)
			mixer.Mass.Brake += mixer.output(controlInputs) //it's theoretically possible to have more than once control braking a mass (toe brakes,parking brakes for example)
		}

		if mixer.Spring != nil {
			mixer.Spring.Expansion += mixer.output(controlInputs)
		}
	}
}
