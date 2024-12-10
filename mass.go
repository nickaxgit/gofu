package main

import (
	"math"
)

//coins (which are masses) - can be pushed by other masses, or by the springs of things
//spring (and therefore things) can never collide
//all collsions are either mass-mass or mass-spring
//when coins overlap - the slow coin is 'owned' by the fast coin
//when a mass overlaps a spring - the mass is 'owned' by the spring

type Mass struct {
	p           Vector
	r           float64
	fixed       bool
	isCoin      bool
	collideable bool
	thingNum    int //which thing is this mass part of - coins are -1
	op          Vector
	oz          float64
	fallingInto int    //*Thing           //index of thing
	v           Vector //"velocity" - the change in position of this mass
	//angle            float64 //flipping angle (for coins)
	z                float64 //falling depth/spin (/height above ground)
	enabled          bool
	lastThingTouched int //who 'owns' this mass
}

func NewMass(p Vector, r float64, fixed bool, isCoin bool, collideable bool, thingNum int) *Mass {
	return &Mass{p: p, r: r, fixed: fixed, isCoin: isCoin, collideable: collideable, thingNum: thingNum}
}

func (m *Mass) moveTowards(p *Vector, dist float64) {
	//move this mass towards the point p by dist
	delta := p.subtract(&m.p)
	d := delta.normalise()
	m.p.addIn(d.multiply(dist))
}

func (m *Mass) isInside(masses []*Mass, thing *Thing) bool {
	//is this mass inside the thing - "draws" a line from the point (to test) to the origin and counts how many springs of this thing are crossed - if the number is odd, then the point is inside the thing
	//works for convex or concave things - dont know about things with holes
	o := Vector{x: 0, y: 0}
	crossings := 0
	for s := 0; s < len(thing.springs); s++ {
		if thing.springs[s].crosses(masses, &o, &m.p) {
			crossings++
		}
	}

	return crossings%2 == 1
}

func (m *Mass) resolveMasSpringOverlap(masses []*Mass, spring *Spring, pen float64) {
	//pushes masses back out of springs
	//a mass can only penetrate one spring on a given thing at a time
	ratio := 0.5 //how much into the mass vs the spring
	resolve := spring.direction(masses).rotate(math.Pi / 2)
	resolve = resolve.multiply(pen)
	//console.log('resolve penetration of',pen)

	// this.p.addIn(resolve)
	// return

	a := masses[spring.m1] //.p
	b := masses[spring.m2] //.p

	if m.fixed && a.fixed && b.fixed {
		panic(`all masses fixed - but overlap ?`)
	}

	//the spring is fixed the mass is free
	if a.fixed && b.fixed {
		m.p.addIn(resolve)
		return
	} //if both ends of the spring are pinned move the mass - and EXIT

	if m.fixed {
		ratio = 0
	}
	m.p.addIn(resolve.multiply(ratio)) //push the mass out to the left (things are defined clockwise) - so left is outwards

	pol := m.p.closestPointOnLine(&a.p, &b.p)

	share := pol.distanceFrom(&a.p) / a.p.distanceFrom(&b.p)
	if share > 1 || share < 0 {
		panic(`share out of range`)
	}
	if a.fixed {
		share = 1
	}
	if b.fixed {
		share = 0
	}

	a.p.subIn(resolve.multiply((1 - ratio) * (1 - share)))
	b.p.subIn(resolve.multiply((1 - ratio) * share))
}

func (m *Mass) sideof(masses []*Mass, spring *Spring) float64 {
	//return the signed distance of this point from the spring
	a := &masses[spring.m1].p
	b := &masses[spring.m2].p

	return m.p.distanceFromLine(a, b) * -mathSign(b.subtract(a).cross(m.p.subtract(a)))

}

func mathSign(x float64) float64 {
	if x > 0 {
		return 1
	}
	if x < 0 {
		return -1
	}
	return 0
}
