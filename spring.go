package main

import ()

type spring struct {
	index       int32
	restLength  float64 `json:"-"` //length is not needed clientside
	m1          *mass
	m2          *mass
	collideable byte
}

func NewSpring(m1 *mass, m2 *mass, collideable byte) *spring {
	//set rest length at constrcution
	if m1 == m2 {
		panic(`degenerate spring (both ends same mass) at construction `)
	}

	restLength := m1.p.distanceFrom(m2.p)
	if restLength == 0 {
		panic(`zero length spring at construction`)
	}
	return &spring{-1, restLength, m1, m2, collideable}
}

// func (s *Spring) crosses(m []*Mass, p1 *Vector, p2 *Vector) bool {
// 	//do the lines p1,p2 and p3,p4 cross
// 	return lineSegmentsCross(p1, p2, &m[s.M1].P, &m[s.M2].P) //TODO tidy/refactor

// }

func (s *spring) closestPointTo(p *vec3) *vec3 {
	return p.closestPointOnLine(s.m1.p, s.m2.p)

}

func (s *spring) stretch(masses []*mass) {

	if s.m1.fixed && s.m2.fixed {
		return
	} //if both ends are pinned, then the spring does not stretch

	springVector := s.m2.p.sub(s.m1.p)
	currentLength := springVector.length()

	if currentLength == 0 {
		panic(`zero length spring`)
	}

	err := (s.restLength - currentLength) / currentLength //what is the error as a fraction of the current vector
	move := springVector.multiply(err * 0.4)              //0.6) //stiffness
	if !s.m1.fixed {
		s.m1.p.subIn(move)
	} //unless they're pinned
	if !s.m2.fixed {
		s.m2.p.addIn(move)
	}

}

func (s *spring) contains(p *vec3) bool {
	return p.liesBetween(s.m1.p, s.m2.p)
}

func (s *spring) distanceFrom(p *vec3) float64 {

	if s.contains(p) {
		return p.distanceFromLine(s.m1.p, s.m2.p)
	} else {
		d1 := p.distanceFrom(s.m1.p)
		d2 := p.distanceFrom(s.m2.p)
		if d1 < d2 {
			return d1
		} else {
			return d2
		}
	}

}

func (s *spring) direction() *vec3 {
	v := s.m2.p.sub(s.m1.p)
	return v.normalise()
}

func (s *spring) send(player *player, state *state) {
	state.send(player, &reply{Cmd: "spring", Payload: s})
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
