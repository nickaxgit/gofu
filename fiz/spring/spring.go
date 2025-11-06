package spring

import (
	"github.com/nickax/gofu/fiz/mass"
	"github.com/nickax/gofu/game/actuator"
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/ray"
	"github.com/nickax/gofu/vec"
	//"github.com/nickax/gofu/ray"
)

type Spring struct {
	Index       int32
	RestLength  float64 `json:"-"` //length is not needed clientside
	M1          *mass.Mass
	M2          *mass.Mass
	Collideable byte
	ActuatorTag actuator.ActuatorEnum //float64
	Expansion   float64
	//thrust       float64
	lastError float64 //used for Derivative calculation

}

// func (s *Spring) rest() float64 {
// 	s.RestLength = s.M1.P.DistanceFrom(s.M2.P)
// 	return s.RestLength
// }

func New(springs []*Spring, m1 *mass.Mass, m2 *mass.Mass, collideable byte, restLength float64, actuatorTag actuator.ActuatorEnum) *Spring {
	//set rest length at constrcution
	if m1 == m2 {
		panic(`degenerate spring (both ends same mass) at construction `)
	}

	if restLength == 0 {
		log.Logit("warning: zero length spring at construction - setting rest length to current distance between masses")
		restLength = m1.P.DistanceFrom(m2.P)
	}

	s := &Spring{Index: int32(len(springs)), M1: m1, M2: m2, Collideable: collideable, RestLength: restLength, ActuatorTag: actuatorTag}

	springs = append(springs, s)
	return s

}

// func (s *Spring) crosses(m []*Mass, p1 *Vector, p2 *Vector) bool {
// 	//do the lines p1,p2 and p3,p4 cross
// 	return lineSegmentsCross(p1, p2, &m[s.M1].P, &m[s.M2].P) //TODO tidy/refactor

// }

func (s *Spring) closestPointTo(p *vec.V3) *vec.V3 {
	return p.ClosestPointOnLine(s.M1.P, s.M2.P)

}

func (s *Spring) Stretch() {

	if s.M1.Fixed && s.M2.Fixed {
		return
	} //if both ends are pinned, then the spring does not stretch

	springVector := s.M2.P.Sub(s.M1.P)
	currentLength := springVector.Length()

	if currentLength == 0 {
		panic(`zero length spring`)
	}

	err := ((s.RestLength + s.Expansion) - currentLength) / currentLength //currentLength //what is the error as a fraction of the current vector

	if err > -0.0001 && err < 0.0001 {
		return
	}

	d := 0.0
	if s.lastError != 0 {
		d = err - s.lastError
	}
	s.lastError = err                              //derivative of the error
	move := springVector.Multiply(err*0.6 + d*0.0) //0.1) //stiffness

	m1 := s.M1.MassKG()
	m2 := s.M2.MassKG()
	if m1 == 0 || m2 == 0 {
		panic("zero mass ")
	}

	f1 := move.Multiply(m2 / (m1 + m2))
	f2 := move.Sub(f1)

	if !s.M1.Fixed {

		s.M1.P.SubIn(f1)
		s.M1.Op.SubIn(f1.Multiply(0.1))

	} //unless they're pinned
	if !s.M2.Fixed {
		s.M2.P.AddIn(f2)
		s.M2.Op.AddIn(f2.Multiply(.1))

	}

}

func (s *Spring) contains(p *vec.V3) bool {
	return p.LiesBetween(s.M1.P, s.M2.P)
}

func NewFromMsg(springs []*Spring, m *msg.Msg, masses []*mass.Mass) *Spring {

	m1, m2, collideable, restLength, actuatorTag := int32(0), int32(0), byte(0), float32(0), byte(0)

	m.Read(&m1, &m2, &collideable, &restLength, &collideable, &restLength, &actuatorTag)

	return New(springs, masses[m1], masses[m2], collideable, float64(restLength), actuator.ActuatorEnum(actuatorTag))

}

func (s *Spring) distanceFrom(p *vec.V3) float64 {

	if s.contains(p) {
		return p.DistanceFromLine(s.M1.P, s.M2.P)
	} else {
		d1 := p.DistanceFrom(s.M1.P)
		d2 := p.DistanceFrom(s.M2.P)
		if d1 < d2 {
			return d1
		} else {
			return d2
		}
	}

}

func (s *Spring) direction() *vec.V3 {
	v := s.M2.P.Sub(s.M1.P)
	return v.Normalise()
}

func (s *Spring) SetVelocity(v *vec.V3) {

	if s.M1.Fixed || s.M2.Fixed {
		if v.LengthSq() > 0 {
			log.Logit("warning: trying to set velocity on spring with fixed mass")
		}
	}
	s.M1.SetVelocity(v)
	s.M2.SetVelocity(v)
}
func (s *Spring) Translate(v *vec.V3) {
	s.M1.P.AddIn(v)
	s.M2.P.AddIn(v)
	s.M1.Op.AddIn(v) //important
	s.M2.Op.AddIn(v) //important
}

func (s *Spring) Rewire() {

	s.M1 = s.M1.SwitchToOriginal()
	s.M2 = s.M2.SwitchToOriginal()

}

func (spring *Spring) WriteTo(msg *msg.Msg) {

	//note - we never write the springs index (it's never used)
	if spring.M1.Index == spring.M2.Index {
		panic("degenerate spring whilst serialising")
	}
	msg.Write(spring.M1.Index, spring.M2.Index, spring.Collideable, float32(spring.RestLength), byte(spring.ActuatorTag))

}

// func lineSegmentsCross(a1 *vec2, a2 *vec2, b1 *vec2, b2 *vec2) bool {

// 	//returns true if the lines a1-a2 and b1-b2 cross
// 	d := (a2.x-a1.x)*(b2.y-b1.y) - (a2.y-a1.y)*(b2.x-b1.x)
// 	if d == 0 {
// 		return false
// 	} //lines are parallel
// 	u := ((b1.x-a1.x)*(b2.y-b1.y) - (b1.y-a1.y)*(b2.x-b1.x)) / d
// 	v := ((b1.x-a1.x)*(a2.y-a1.y) - (b1.y-a1.y)*(a2.x-a1.x)) / d
// 	return (u >= 0 && u <= 1 && v >= 0 && v <= 1)

// }

func ClosestSpringToRay(springs []*Spring, ray *ray.Ray) (*Spring, float64) {

	closestDistance := float64(1000)
	var closestSpring *Spring = nil

	for _, s := range springs {
		d := ray.DistanceFromLineSegment(s.M1.P, s.M2.P)
		if d < closestDistance {
			closestDistance = d
			closestSpring = s
		}
	}

	return closestSpring, closestDistance

}
