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
	p           *Vec3
	r           float64
	fixed       bool
	isCoin      bool
	collideable bool
	op          *Vec3
	v           *Vec3 //"velocity" - the change in position of this mass
	enabled     bool
	//selected         bool //needs to be per player - see player.selectedMasses map
	lastThingTouched *thing
	axle             *mass   //if the mass is a wheel - the vector to this mass is the axle
	wingRoot         *mass   //if the mass is a wing tip - this is the TE root (the axle is the LE root)
	aoa              float64 //(additional) angle of attack
	//grounded         bool
}

func NewMass(p *Vec3, r float64, fixed bool, isCoin bool, collideable bool, thing *thing) *mass {

	return &mass{p: p, r: r, fixed: fixed, isCoin: isCoin, collideable: collideable, thing: thing, enabled: true, op: p, v: newVec3(0, 0, 0)}
}

func (m *mass) readBinary(s *state, buff *bytes.Buffer, e binary.ByteOrder, withDetail byte) {

	floats := make([]float32, 3)
	binary.Read(buff, e, &floats) //reads all 12 bytes of the position (we can't read the float32's directly into float64s)
	m.p.fromByteBuffer(buff, e)

	if withDetail != 0 {
		binary.Read(buff, e, &m.r)
		binary.Read(buff, e, &m.fixed)
		binary.Read(buff, e, &m.isCoin)
		binary.Read(buff, e, &m.collideable)
		selected := byte(0)
		binary.Read(buff, e, &selected)
		wr := int32(-1)
		ax := int32(-1)
		binary.Read(buff, e, &ax)
		binary.Read(buff, e, &wr)
		if wr > -1 {
			m.wingRoot = s.masses[wr]
		}
		if ax > -1 {
			m.axle = s.masses[ax]
		}

	}
}

func (m *mass) writeBinary(buff *bytes.Buffer, e binary.ByteOrder, withDetail bool, selected byte) {

	m.p.toByteBuffer(buff, e)

	if withDetail {
		binary.Write(buff, e, float32(m.r))
		binary.Write(buff, e, m.fixed)
		binary.Write(buff, e, m.isCoin)
		binary.Write(buff, e, m.collideable)
		binary.Write(buff, e, selected) //note highlit mass is not done this way (becuase there must be only one)
		ax := int32(-1)
		if m.axle != nil {
			ax = m.axle.index
		}
		wr := int32(-1)
		if m.wingRoot != nil {
			wr = m.wingRoot.index
		}

		//although the client doesnt need to know about axles, and wingroots - we do need to persist them
		binary.Write(buff, e, ax) //note highlit mass is not done this way (becuase there must be only one)
		binary.Write(buff, e, wr) //note highlit mass is not done this way (becuase there must be only one)

		//todo - mass
	}

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
