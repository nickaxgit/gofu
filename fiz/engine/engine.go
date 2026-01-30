package engine

import (
	"math"

	"github.com/nickax/gofu/fiz/spring"
	"github.com/nickax/gofu/game/aero"
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/game/sound"
	"github.com/nickax/gofu/log"
)

type Engine struct {
	index byte //for telemetry display
	name  string
	//vehicle            *Thing
	started            bool
	rpm                float64 //multiple engines
	kw                 float64 //current fuel burn/power output (kw) (per engine)
	kwMax              float64 //max fuel burn/power output (kw) (per engine)
	soundHandle        uint16  // handle for the sounds (per engine) - so we can detune it
	propTotalBladeArea float64 //radius of the propellor (m) - used for engine sound -- see runEngines
	propRadius         float64 //radius of the propellor (m) - used for thrust calculation
	//propChord   float64 //chord of the propellor (m) - used for thrust calculation
	//blades	    int32   //number of blades on the propellor (used for thrust calculation)
	//bladeMass  float64 //mass of each blade (kg) - used for thrust calculation
	pitch         float64 //angle of the propellor (degrees) - used for thrust calculation
	moi           float64 //moment of inertia of the propellor (kg*m^2) - used for engine sound -- see runEngines
	thrustNewtons float64 //current thrust being generated
	lastRpmSent   float64 //last sent rpm (used for sound)
	Spring        *spring.Spring
}

func New(name string, index byte, kwMax float64, propRadius float64, propTotalBladeArea float64, pitch float64, moi float64, spring *spring.Spring) *Engine {
	return &Engine{name: name, index: index, kwMax: kwMax, propRadius: propRadius, propTotalBladeArea: propTotalBladeArea, pitch: pitch, moi: moi, Spring: spring, rpm: 0, lastRpmSent: 0, started: false}

}

func (engine *Engine) GetRPM() float64 {

	return engine.rpm
}

func (engine *Engine) IsStarted() bool {
	return engine.started
}

func (engine *Engine) Start(sounds []*sound.Sound, response *msg.Msg) {
	log.Logit("starting engine", engine.name)
	engine.started = true
	engine.rpm = 100
	position := engine.Spring.M1.P //where will the sound come from
	startToIdle := sound.New(sounds, "startToIdle", position, 0.5, false, nil, 50)
	idleUp := sound.New(sounds, "idleUp", position, 0.5, false, startToIdle, 100)
	engineLoop := sound.New(sounds, "engineLoop", position, 0.5, true, idleUp, 0)

	startToIdle.WriteInto(response)
	idleUp.WriteInto(response)
	engineLoop.WriteInto(response)

}

func (engine *Engine) writePitchInto(response *msg.Msg) {

	response.Write(msg.Detune)
	response.Write(uint16(engine.soundHandle), int16(engine.rpm-1000)) //detune is in cents - can be negative
	response.Write(byte(engine.index), uint16(engine.rpm))

}

// Throttle sets the current power output of the engine (Run() accelerates the prop disc and generates thrust based on this)
func (engine *Engine) Throttle(throttle float64) {
	engine.kw = engine.kwMax * throttle //this is KW - ree Run()
}

func (engine *Engine) Run(activity *msg.Msg) {

	if engine.started { //is the engine started/running
		av := engine.rpm / 60 * 2 * math.Pi //radians per second
		torque := engine.kw * 1000 / av     //watts to torque (Nm)
		av += torque / (engine.moi * 100)   //divide by the time slice (100 cycles per second)

		//generate thrust
		v := av * engine.propRadius * 0.6 //generate thrust at 60% of the prop radius (0.6 is a guess)
		aoa := engine.pitch               //todo - account for forward speed (and the reducing angle of attack)) - although i imagine aircraft systems handle this to keep pitch optimal
		cl := aero.LiftCurves[aero.Symetrical].GetY(aoa)
		lift := v * v * cl * aero.Rho * .5 * engine.propTotalBladeArea
		engine.thrustNewtons = lift

		if engine.thrustNewtons > 10000 { //more than 5000kg of thrust
			log.Logit("excess thrust", engine.thrustNewtons)

		}

		cd := aero.DragCurves[aero.Symetrical].GetY(aoa)
		drag := v * v * cd * engine.propTotalBladeArea

		dragTorque := drag * engine.propRadius * .6 //torque is the drag on the prop (Nm)
		av -= dragTorque / (engine.moi * 100)       //divide by the time slice (100 cycles per second)
		engine.rpm = av * 60 / (2 * math.Pi)        //radians/second to rpm

		if engine.rpm > engine.lastRpmSent+10 || engine.rpm < engine.lastRpmSent-10 {
			//send sound/detune
			engine.writePitchInto(activity)
		}

		svn := engine.Spring.M1.P.Sub(engine.Spring.M2.P).Normalised()

		//acceleration = force / mass
		//mv1 := svn.multiply(e.thrustNewtons / e.spring.m1.mass * 150)
		mv := svn.Multiply(engine.thrustNewtons / (500000 * 150)) //vehicle weight and cycles per second (5*30)
		engine.Spring.M1.P.AddIn(mv)
		engine.Spring.M2.P.AddIn(mv)
	}

}
