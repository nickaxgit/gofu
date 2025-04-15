package main

import (
	"bytes"
	"encoding/binary"
	"math"
)

type thing struct {
	//a thing is really a collection of Springs (which never intersect)
	//to which we pin an image
	state    *state // a reference back to the game/state it belongs to
	index    int32
	om       *mass //origin mass
	fm       *mass //forward mass (defines z axis)
	rm       *mass // right mass (deinfes x axis)
	springs  []*spring
	faces    []*face
	meshName string
	offset   *vec3
	scale    *vec3
	rotation *vec3 //axis/angle
	//masses     map[*mass]bool //all the masses in the thing (once) (used for applying lift)
	visibility   byte
	uniqueMasses map[*mass]bool
	engines      []*engine //multiple engines

}

func newThing(meshName string) *thing {

	t := &thing{meshName: meshName, scale: newVec3(1, 1, 1), offset: newVec3(0, 0, 0), springs: []*spring{}, faces: []*face{}, visibility: 1, rotation: newVec3(0, math.Pi*2, 0), uniqueMasses: make(map[*mass]bool)}

	//moment of inertie of a disc is 1/2 m * r^2
	bladeMass := 30.0
	blades := 4.0
	propChord := 0.4                                          //0.4m chord of the propellor (m) - used for thrust calculation
	propRadius := 2.0                                         //2m radius propellor (4m diameter)
	moi := 0.5 * bladeMass * propRadius * propRadius * blades //moment of inertia of the propellor (kg*m^2) - used for engine sound -- see runEngines
	numEngines := 2
	engines := make([]*engine, numEngines)

	for i := 0; i < numEngines; i++ {
		engines[i] = &engine{
			name:               "No 1 (port)",
			vehicle:            t,
			rpm:                0,
			kwMax:              1775,
			kw:                 0,
			propRadius:         propRadius,
			propTotalBladeArea: propChord * propRadius * blades,
			pitch:              10.0,
			moi:                moi,
		}
	}
	engines[1].name = "No 2 (starboard)" //don't "correct" this - engine are internally numbered from 0
	t.engines = engines
	return t

}

// start an engine and send startup sound to all players

// func (s *state) setVelocity(v *vec3) {
// 	for _, m := range s.masses {
// 		m.v.addIn(v)
// 		m.op = m.p.sub(v)
// 	}
// }

func (t *thing) setVelocity(v *vec3) {

	for m := range t.uniqueMasses {
		//	m.v = v
		m.op = m.p.sub(v)
	}
}

// find the closest point on any face in the thing to the point p
func (thing *thing) distanceFrom(p *vec3) float64 {
	bestDist := 1000000.0
	// for _, f := range thing.faces {
	// 	d := p.distanceFromFace(f) //f.distanceFrom(m, p, false)
	// 	if d < bestDist {
	// 		bestDist = d
	// 	}
	// }
	return bestDist

}

func (t *thing) translate(v *vec3) {
	for m := range t.uniqueMasses {
		m.p.addIn(v)
		m.op.addIn(v) //important

	}
}

// Is the point P the thing
// "draws" a line from the point (to test) to the origin and counts how many springs of this thing are crossed - if the number is odd, then the point is inside the thing
func (thing *thing) contains(p *vec2, m []*mass) bool {

	panic("contains not done")

}

func (t *thing) centreOfMass() (*vec3, float64) {

	c := newVec3(0, 0, 0)
	tm := 0.0
	for m := range t.uniqueMasses {
		mm := m.mass()
		tm += mm
		c.addIn(m.p.multiply(mm))
	}

	return c.multiply(1.0 / float64(tm)), tm

}

func (thing *thing) closestPointOnEdge(masses []*mass, wp *vec3) *vec3 {
	bestDist := float64(1000000)
	bestPoint := newVec3(0, 0, 0)
	for _, s := range thing.springs {
		if s.contains(wp) {
			p := s.closestPointTo(wp)
			d := p.distanceFrom(wp)
			if d < bestDist {
				bestDist = d
				bestPoint = p
			}
		}
	}
	return bestPoint
}
func (t *thing) toByteBuffer(buff *bytes.Buffer) {
	binary.Write(buff, le, t.index)
	binary.Write(buff, le, byte(len(t.meshName)))
	binary.Write(buff, le, []byte(t.meshName))
	//binary.Write(buff, le, make([]byte, len(t.meshname)%4+2)) //padding

	t.offset.toByteBuffer(buff)
	t.scale.toByteBuffer(buff)
	t.rotation.toByteBuffer(buff)

	binary.Write(buff, le, t.om.index)
	binary.Write(buff, le, t.fm.index)
	binary.Write(buff, le, t.rm.index)

	binary.Write(buff, le, t.visibility)

	binary.Write(buff, le, uint32(len(t.springs)))
	for _, spring := range t.springs {
		if spring.m1.index == spring.m2.index {
			panic("degenerate spring whilst serialising")
		}
		binary.Write(buff, le, spring.m1.index)
		binary.Write(buff, le, spring.m2.index)
		binary.Write(buff, le, spring.collideable)
		binary.Write(buff, le, float32(spring.restLength))
		binary.Write(buff, le, byte(spring.flightOutput))

	}
}

func (t *thing) fromByteBuffer(buff *bytes.Buffer, e binary.ByteOrder) {

	mnl := byte(0)
	binary.Read(buff, e, &mnl)
	meshName := make([]byte, mnl)
	binary.Read(buff, e, &meshName)
	t.meshName = string(meshName)

	t.offset.fromByteBuffer(buff)
	t.scale.fromByteBuffer(buff)
	t.rotation.fromByteBuffer(buff)
	if t.rotation.length() == 0 {
		t.rotation = newVec3(0, math.Pi*2, 0)
	}

	var omi, fmi, rmi int32
	binary.Read(buff, e, &omi)
	binary.Read(buff, e, &fmi)
	binary.Read(buff, e, &rmi)
	t.om = t.state.masses[omi]
	t.fm = t.state.masses[fmi]
	t.rm = t.state.masses[rmi]
	binary.Read(buff, e, &t.visibility)

	ns := uint32(0)
	binary.Read(buff, e, &ns)
	t.springs = make([]*spring, 0)
	for i := 0; i < int(ns); i++ {

		m1 := int32(0)
		m2 := int32(0)
		collideable := byte(0)
		restLength := float32(0)
		binary.Read(buff, e, &m1)
		binary.Read(buff, e, &m2)
		binary.Read(buff, e, &collideable)
		binary.Read(buff, e, &restLength)
		fo := byte(0)
		binary.Read(buff, e, &fo)

		spring := t.AddSpring(t.state.masses[m1], t.state.masses[m2], collideable, actuatorEnum(fo))
		spring.restLength = float64(restLength)

	}
}
