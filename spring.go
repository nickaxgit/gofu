package main

import (
	"strconv"
)

type Spring struct {
	length      float64
	M1          int  `json:"m1"`
	M2          int  `json:"m2"`
	collideable bool //collision are not handled client side (so the client doesn't need to know)
}

func NewSpring(masses []*Mass, m1 int, m2 int, collideable bool) *Spring {
	//set rest length at constrcution
	if m1 == m2 {
		panic(`degenerate spring (both ends same mass) at construction ` + strconv.Itoa(m1))
	}
	length := masses[m1].P.distanceFrom(&masses[m2].P)
	if length == 0 {
		panic(`zero length spring at construction`)
	}
	return &Spring{length, m1, m2, collideable}
}

func (s *Spring) crosses(m []*Mass, p1 *Vector, p2 *Vector) bool {
	//do the lines p1,p2 and p3,p4 cross
	return lineSegmentsCross(p1, p2, &m[s.M1].P, &m[s.M2].P) //TODO tidy/refactor

}

func (s *Spring) closestPointTo(masses []*Mass, p *Vector) Vector {
	return p.closestPointOnLine(&masses[s.M1].P, &masses[s.M2].P)

}

func (s *Spring) stretch(masses []*Mass) {
	m1 := masses[s.M1]
	m2 := masses[s.M2]

	if m1.fixed && m2.fixed {
		return
	} //if both ends are pinned, then the spring does not stretch

	delta := m2.P.subtract(&m1.P)
	distance := delta.length()

	if distance == 0 {
		panic(`zero length spring`)
	}

	difference := (s.length - distance) / distance
	move := delta.multiply(difference * 0.5 * 0.6) //stiffness
	if !m1.fixed {
		m1.P.subIn(move)
	} //unless they're pinned
	if !m2.fixed {
		m2.P.addIn(move)
	}
}

func (s *Spring) contains(masses []*Mass, p *Vector) bool {

	m1 := masses[s.M1]
	m2 := masses[s.M2]

	return p.liesBetween(&m1.P, &m2.P)

}

func (s *Spring) direction(m []*Mass) Vector {
	v := m[s.M2].P.subtract(&m[s.M1].P)
	return v.normalise()
}

func lineSegmentsCross(a1 *Vector, a2 *Vector, b1 *Vector, b2 *Vector) bool {

	//returns true if the lines a1-a2 and b1-b2 cross
	d := (a2.X-a1.X)*(b2.Y-b1.Y) - (a2.Y-a1.Y)*(b2.X-b1.X)
	if d == 0 {
		return false
	} //lines are parallel
	u := ((b1.X-a1.X)*(b2.Y-b1.Y) - (b1.Y-a1.Y)*(b2.X-b1.X)) / d
	v := ((b1.X-a1.X)*(a2.Y-a1.Y) - (b1.Y-a1.Y)*(a2.X-a1.X)) / d
	return (u >= 0 && u <= 1 && v >= 0 && v <= 1)

}
