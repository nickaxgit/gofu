package main

import (
	"bytes"
	"encoding/binary"
	// "math"
)

//coins (which are masses) - can be pushed by other masses, or by the springs of things
//spring (and therefore things) can never collide
//all collsions are either mass-mass or mass-spring
//when coins overlap - the slow coin is 'owned' by the fast coin
//when a mass overlaps a spring - the mass is 'owned' by the spring

type mass struct {
	index       int32
	thing       *thing
	p           *vec3
	transformOf *mass
	r           float64
	fixed       bool
	isCoin      bool
	collideable bool
	op          *vec3
	//	v           *vec3 //"velocity" - the change in position of this mass
	enabled bool
	//selected         bool //needs to be per player - see player.selectedMasses map
	lastThingTouched *thing
	axle             *mass //if the mass is a wheel - the vector to this mass is the axle
	wingRoot         *mass //if the mass is a wing tip - this is the TE root (the axle is the LE root)
	axi              int32 //index of the axle (used only temporarily while loading)
	wri              int32 //index of the wing root (used only temporarily while loading)
	//aoaRads          float64 //(additional) angle of attack
	wingArea float64
	//dihedralDegrees  float64
	lift       *vec3
	thrust     float64
	aoaDegrees float64
	drag       *vec3
	flip       bool
	//correction *vec3
	contribs   float64
	correction *vec3   //used to correct the position of the masses
	section    float64 //airfoil section - actually just a byte but we bind a slider to it so it has to be a float64
	//flightOutput     float64 //outputEnum - what am I ? aileron, elevator, rudder, flap, engine
	//gain             float32 //multiplier/inverter for the (normalised) control input - e.g. convert to radians of aileron deflection
	//isThrust         bool    //throttles create thrust - control surfaces have their AoA changed

}

func (m *mass) mass() float64 {
	//this assumes a density of 10kg/liter

	m3 := 4 / 3 * 3.14159 * m.r * m.r * m.r
	return m3 * 1000 * 1000 //litres in a m^3
}

func newMass(p *vec3, r float64, fixed bool, isCoin bool, collideable bool, thing *thing, transFormOf *mass) *mass {

	return &mass{p: p, r: r, fixed: fixed, isCoin: isCoin, collideable: collideable, thing: thing, enabled: true, op: p.clone(), transformOf: transFormOf, correction: newVec3(0, 0, 0)}
}

func (m *mass) overlaps(masses []*mass) *mass {

	for _, m2 := range masses {
		if m != m2 && m.p.distanceFrom(m2.p) < m.r+m2.r {
			return m2
		}
	}
	return nil

}

func (m *mass) fromByteBuffer(buff *bytes.Buffer, withDetail byte, s *state) {

	binary.Read(buff, le, &m.index)
	m.p.fromByteBuffer(buff)
	m.op = m.p.clone() //todo serialise this to be able to save masses in motion

	if withDetail != 0 {
		r := float32(0)
		binary.Read(buff, le, &r)
		m.r = float64(r)

		binary.Read(buff, le, &m.fixed)
		binary.Read(buff, le, &m.isCoin)
		binary.Read(buff, le, &m.collideable)
		selected := byte(0)
		binary.Read(buff, le, &selected)

		binary.Read(buff, le, &m.axi)
		binary.Read(buff, le, &m.wri)

		if m.axi > -1 {
			m.axle = s.masses[m.axi]
		} else {
			m.axle = nil
		}
		if m.wri > -1 {
			m.wingRoot = s.masses[m.wri]
		} else {
			m.wingRoot = nil
		}

		//remove
		f32 := float32(0)
		//binary.Read(buff, le, &f32)
		//m.aoaRads = float64(f32)

		binary.Read(buff, le, &f32)
		m.wingArea = float64(f32)

		//remove
		//binary.Read(buff, le, &f32)
		//m.dihedralDegrees = float64(f32)

		i32 := int32(0)
		binary.Read(buff, le, &i32)
		if i32 != -1 {
			m.transformOf = s.masses[i32]
		}

		l := byte(0)
		binary.Read(buff, le, &l)
		m.flip = false
		if l > 0 {
			m.flip = true
		}

		sb := byte(0) //section byte
		binary.Read(buff, le, &sb)
		m.section = float64(sb)

	}
}

func (m *mass) toByteBuffer(buff *bytes.Buffer, withDetail bool, selected byte) {

	binary.Write(buff, le, m.index)
	m.p.toByteBuffer(buff)

	if withDetail {
		binary.Write(buff, le, float32(m.r))
		binary.Write(buff, le, m.fixed)
		binary.Write(buff, le, m.isCoin)
		binary.Write(buff, le, m.collideable)
		binary.Write(buff, le, selected) //note highlit mass is not done this way (becuase there must be only one)
		ax := int32(-1)
		if m.axle != nil {
			ax = m.axle.index
		}
		wr := int32(-1)
		if m.wingRoot != nil {
			wr = m.wingRoot.index
		}

		//although the client doesnt need to know about axles, and wingroots - we do need to persist them
		binary.Write(buff, le, ax) //note highlit mass is not done this way (becuase there must be only one)
		binary.Write(buff, le, wr) //note highlit mass is not done this way (becuase there must be only one)
		//binary.Write(buff, le, float32(m.aoaRads))
		binary.Write(buff, le, float32(m.wingArea))
		//binary.Write(buff, le, float32(m.dihedralDegrees))

		if m.transformOf != nil {
			binary.Write(buff, le, m.transformOf.index)
		} else {
			binary.Write(buff, le, int32(-1))
		}

		binary.Write(buff, le, m.flip)
		binary.Write(buff, le, byte(m.section))

	}

}

// selectedmasses are used when sending the client to implement the player-specific selection set
func massesToBytes(masses []*mass, withDetail bool, playerSelected map[*mass]bool) []byte {

	buff := new(bytes.Buffer)

	binary.Write(buff, le, byte(msgMasses)) //masses
	binary.Write(buff, le, uint32(len(masses)))

	if withDetail {
		binary.Write(buff, le, byte(1))
	} else {
		binary.Write(buff, le, byte(0))
	}

	//send the masses
	for _, m := range masses {
		selected := byte(0)

		present, isSelected := playerSelected[m]
		if present && isSelected {
			selected = byte(1)
		}

		m.toByteBuffer(buff, withDetail, selected) //only send the position
	}

	return buff.Bytes()

}

// func (m *mass) send(player *player, state *state) {

// 	msg := &reply{Cmd: "mass", Payload: massPayload{I: m.index, P: *m.p, R: m.r, Fixed: m.fixed, isCoin: m.isCoin}}

// 	state.sendBinary(player, msg)

// }

// func (m *mass) moveTowards(p *Vec3, dist float64) {
// 	//move this mass towards the point p by dist
// 	delta := p.sub(m.p)
// 	d := delta.normalise()
// 	m.p.addIn(d.multiply(dist))
// }

func (m *mass) resolveMasSpringOverlap(masses []*mass, spring *spring, pen float64) {
	// //pushes masses back out of springs
	// //a mass can only penetrate one spring on a given thing at a time
	// ratio := 0.5 //how much into the mass vs the spring
	// resolve := spring.direction(masses).rotate(math.Pi / 2)
	// resolve = resolve.multiply(pen)
	// //console.log('resolve penetration of',pen)

	// // this.p.addIn(resolve)
	// // return

	// a := masses[spring.M1] //.p
	// b := masses[spring.M2] //.p

	// if m.Fixed && a.Fixed && b.Fixed {
	// 	panic(`all masses fixed - but overlap ?`)
	// }

	// //the spring is fixed the mass is free
	// if a.Fixed && b.Fixed {
	// 	m.P.addIn(resolve)
	// 	return
	// } //if both ends of the spring are pinned move the mass - and EXIT

	// if m.Fixed {
	// 	ratio = 0
	// }
	// m.P.addIn(resolve.multiply(ratio)) //push the mass out to the left (things are defined clockwise) - so left is outwards

	// pol := m.P.closestPointOnLine(&a.P, &b.P)

	// share := pol.distanceFrom(&a.P) / a.P.distanceFrom(&b.P)
	// if share > 1 || share < 0 {
	// 	panic(`share out of range`)
	// }
	// if a.Fixed {
	// 	share = 1
	// }
	// if b.Fixed {
	// 	share = 0
	// }

	// a.P.subIn(resolve.multiply((1 - ratio) * (1 - share)))
	// b.P.subIn(resolve.multiply((1 - ratio) * share))
}

func (m *mass) sideOf(f *face) float64 {
	//return the signed distance of centre of the mass from the plane of the face
	a := f.m[0].p
	b := f.m[1].p
	c := f.m[2].p //..note a face may have more than three verts

	return m.p.distanceFromTriPlane(a, b, c)

}

// func mathSign(x float64) float64 {
// 	if x > 0 {
// 		return 1
// 	}
// 	if x < 0 {
// 		return -1
// 	}
// 	return 0
// }
