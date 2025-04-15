package main

import (
	"bytes"
	"encoding/binary"
)

type engine struct {
	name               string
	vehicle            *thing
	rpm                float64 //multiple engines
	kw                 float64 //current fuel burn/power output (kw) (per engine)
	kwMax              float64 //max fuel burn/power output (kw) (per engine)
	soundHandle        uint16  // handle for the sounds (per engine)
	propTotalBladeArea float64 //radius of the propellor (m) - used for engine sound -- see runEngines
	propRadius         float64 //radius of the propellor (m) - used for thrust calculation
	//propChord   float64 //chord of the propellor (m) - used for thrust calculation
	//blades	    int32   //number of blades on the propellor (used for thrust calculation)
	//bladeMass  float64 //mass of each blade (kg) - used for thrust calculation
	pitch         float64 //angle of the propellor (degrees) - used for thrust calculation
	moi           float64 //moment of inertia of the propellor (kg*m^2) - used for engine sound -- see runEngines
	thrustNewtons float64 //current thrust being generated
	lastRpmSent   float64 //last sent rpm (used for sound)
	spring        *spring
}

func (e *engine) start() {
	logit("starting engine", e.name)

	e.rpm = 100
	v := e.vehicle
	cg, _ := v.centreOfMass()
	start := v.state.qSound("startToIdle", cg, 0.5, false, 0)
	idleUp := v.state.qSound("idleUp", cg, 0.5, false, start)
	e.soundHandle = v.state.qSound("engineLoop", cg, 0.5, true, idleUp)
}

func (e *engine) sendRpm(engineNo int) {
	s := e.vehicle.state

	buff := new(bytes.Buffer)

	binary.Write(buff, le, msgDetune)
	binary.Write(buff, le, e.soundHandle)
	binary.Write(buff, le, int16(e.rpm-1000)) //write the rpm
	binary.Write(buff, le, byte(engineNo))    //write the rpm
	binary.Write(buff, le, int16(e.rpm))      //write the rpm

	logit("sending engine rpm", e.rpm, e.soundHandle)
	//binary.Write(buff, le, e.rpm) //write the rpm
	s.sendBinary(buff.Bytes())

}
