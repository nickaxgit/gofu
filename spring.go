package main

import (
	"strconv"
)

type Spring struct {
	length      float64
	m1          int
	m2          int
	collideable bool
}

func NewSpring(masses []*Mass, m1 int, m2 int, collideable bool) *Spring {
	//set rest length at constrcution
	if m1 == m2 {
		panic(`degenerate spring (both ends same mass) at construction ` + strconv.Itoa(m1))
	}
	length := masses[m1].p.distanceFrom(&masses[m2].p)
	if length == 0 {
		panic(`zero length spring at construction`)
	}
	return &Spring{length, m1, m2, collideable}
}

func (s *Spring) crosses(m []*Mass, p1 *Vector, p2 *Vector) bool {
	//do the lines p1,p2 and p3,p4 cross
	return lineSegmentsCross(p1, p2, &m[s.m1].p, &m[s.m2].p) //TODO tidy/refactor

}

func (s *Spring) closestPointTo(masses []*Mass, p *Vector) Vector {
	return p.closestPointOnLine(&masses[s.m1].p, &masses[s.m2].p)

}

func (s *Spring) stretch(masses []*Mass) {
	m1 := masses[s.m1]
	m2 := masses[s.m2]

	if m1.fixed && m2.fixed {
		return
	} //if both ends are pinned, then the spring does not stretch

	delta := m2.p.subtract(&m1.p)
	distance := delta.length()

	if distance == 0 {
		panic(`zero length spring`)
	}

	difference := (s.length - distance) / distance
	move := delta.multiply(difference * 0.5 * 0.6) //stiffness
	if !m1.fixed {
		m1.p.subIn(move)
	} //unless they're pinned
	if !m2.fixed {
		m2.p.addIn(move)
	}
}

func (s *Spring) contains(masses []*Mass, p *Vector) bool {

	m1 := masses[s.m1]
	m2 := masses[s.m2]

	return p.liesBetween(&m1.p, &m2.p)

}

func (s *Spring) direction(m []*Mass) Vector {
	v := m[s.m2].p.subtract(&m[s.m1].p)
	return v.normalise()
}

func lineSegmentsCross(a1 *Vector, a2 *Vector, b1 *Vector, b2 *Vector) bool {

	//returns true if the lines a1-a2 and b1-b2 cross
	d := (a2.x-a1.x)*(b2.y-b1.y) - (a2.y-a1.y)*(b2.x-b1.x)
	if d == 0 {
		return false
	} //lines are parallel
	u := ((b1.x-a1.x)*(b2.y-b1.y) - (b1.y-a1.y)*(b2.x-b1.x)) / d
	v := ((b1.x-a1.x)*(a2.y-a1.y) - (b1.y-a1.y)*(a2.x-a1.x)) / d
	return (u >= 0 && u <= 1 && v >= 0 && v <= 1)

}
