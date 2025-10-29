package thing

import (
	"fmt"
	"github.com/nickax/gofu/cam"
	"github.com/nickax/gofu/game/actuator"
	"math"

	"github.com/nickax/gofu/fiz/engine"
	"github.com/nickax/gofu/fiz/mass"
	"github.com/nickax/gofu/fiz/mixer"
	"github.com/nickax/gofu/fiz/spring"
	"github.com/nickax/gofu/game/msg"
	//"github.com/nickax/gofu/geom"
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/poly"
	"github.com/nickax/gofu/vec"
)

type Thing struct {
	//a thing is really a collection of Springs (which never intersect)
	//to which we pin an image
	//state    *game.State // a reference back to the game/state it belongs to
	Index        uint32
	Om           *mass.Mass //origin mass
	Fm           *mass.Mass //forward mass (defines z axis)
	Rm           *mass.Mass // right mass (deinfes x axis)
	Springs      []*spring.Spring
	faces        []*poly.ConvexPoly //face
	meshName     string
	MeshOffset   *vec.V3
	MeshScale    *vec.V3
	MeshRotation *vec.V3 //axis/angle
	//masses     map[*mass.Mass]bool //all the masses in the thing (once) (used for applying lift)
	Visibility byte

	Engines []*engine.Engine //multiple engines
}

func New(things []*Thing, meshName string, numEngines int) *Thing {

	t := &Thing{Index: uint32(len(things)), meshName: meshName, MeshScale: vec.NewVec3(1, 1, 1), MeshOffset: vec.NewVec3(0, 0, 0), Springs: []*spring.Spring{}, faces: []*poly.ConvexPoly{}, Visibility: 1, MeshRotation: vec.NewVec3(0, math.Pi*2, 0)}
	things = append(things, t)

	//moment of inertia of a disc is 1/2 m * r^2
	bladeMass := 30.0
	blades := 4.0
	propChord := 0.4                                          //0.4m chord of the propellor (m) - used for thrust calculation
	propRadius := 2.0                                         //2m radius propellor (4m diameter)
	moi := 0.5 * bladeMass * propRadius * propRadius * blades //moment of inertia of the propellor (kg*m^2) - used for engine sound -- see runEngines

	t.Engines = make([]*engine.Engine, numEngines)

	for i := 0; i < numEngines; i++ {
		engine.New(t.Engines, "No "+fmt.Sprint(i+1), byte(i), 1775, propRadius, propChord*propRadius*blades, 10.0, moi, nil)
	}

	return t

}

func (thing *Thing) FollowWith(cam *cam.Camera) {

	cam.Position = thing.Focus().Sub(thing.Forward().Multiply(50))
	cam.Position.Y += 10
	cam.Direction = thing.Focus().Sub(cam.Position).Normalise()

}

func (thing *Thing) StretchSprings() {

	for _, s := range thing.Springs {
		s.Stretch()
	}

}

// For each mixer, set the mixers, mass and engine (output) the the actuator in the vehicle
func (thing *Thing) BindMixers(mixers []*mixer.Mixer) {

	for _, mx := range mixers {

		mx.Spring = thing.findSpringActuator(mx.Actuator)
		mx.Engine.Spring = thing.findSpringActuator(mx.Actuator)

		if mx.Spring == nil { //we didnt bind it to a spring - try a mass
			mx.Mass = thing.findMassActuator(mx.Actuator)
		}
	}

}

func (thing *Thing) DeleteLastSpring() {
	ss := thing.Springs //alias (reference) to things springs

	if len(ss) == 0 {
		return
	}
	last := len(ss) - 1
	ss[last] = nil //remove reference to last spring (for GC)
	ss = ss[:last] //delete the last spring

}

func (thing *Thing) DeleteSpring(s *spring.Spring) {

	//TODO sanity check/test

	if thing.Springs[s.Index] != s {
		panic("spring does not belong to thing")
	}
	thing.Springs = append(thing.Springs[:s.Index], thing.Springs[s.Index+1:]...)
	for _, rs := range thing.Springs {
		if rs.Index > s.Index {
			rs.Index--
		}
	}

}

func (thing *Thing) Focus() *vec.V3 {
	return thing.Springs[0].M2.P
}

func (thing *Thing) Forward() *vec.V3 {

	o := thing.Springs[0].M1.P
	f := thing.Springs[1].M1.P
	return f.Sub(o).Normalise()

}

func (thing *Thing) findMassActuator(act actuator.ActuatorEnum) *mass.Mass {
	for _, s := range thing.Springs {
		if s.M1.IsActuator(act) {
			return s.M1
		}
		if s.M2.IsActuator(act) {
			return s.M2
		}
	}

	log.Logit(thing.meshName, " has no mass actuator for", actuator.MassActuators[act])
	return nil
}

func (thing *Thing) findSpringActuator(act actuator.ActuatorEnum) *spring.Spring {
	for _, s := range thing.Springs {
		if s.ActuatorTag > 0 {
			if s.ActuatorTag == act {
				return s
			}
		}
	}

	log.Logit(thing.meshName, " has no spring actuator for", actuator.SpringActuators[act])
	return nil
}

func (thing *Thing) SetVelocity(v *vec.V3) {
	for _, s := range thing.Springs {
		s.SetVelocity(v)
	}
}

// find the closest point on any face in the thing to the point p
func (thing *Thing) distanceFrom(p *vec.V3) float64 {
	bestDist := 1000000.0
	// for _, f := range thing.faces {
	// 	d := p.distanceFromFace(f) //f.distanceFrom(m, p, false)
	// 	if d < bestDist {
	// 		bestDist = d
	// 	}
	// }
	return bestDist

}

func (thing *Thing) Translate(v *vec.V3) {
	for _, s := range thing.Springs {
		s.Translate(v)

	}
}

// Is the point P the thing
// "draws" a line from the point (to test) to the origin and counts how many springs of this thing are crossed - if the number is odd, then the point is inside the thing
func (thing *Thing) contains(p *vec.V2, m []*mass.Mass) bool {

	panic("contains not done")

}

func ThingsFromMsg(m *msg.Msg, masses []*mass.Mass) []*Thing {

	msgType := msg.MsgEnum(0)
	m.Read(&msgType)

	if msgType != msg.Things {
		panic("Things are not next")
	}

	numThings := uint32(0)
	m.Read(&numThings)
	if numThings > 1000 {
		panic("Too many things + " + fmt.Sprint(numThings))
	}

	things := make([]*Thing, 0, numThings)

	for i := 0; i < int(numThings); i++ {
		//create a new thing from the byte buffer, add it to the things slice (indexing it upon creation)
		NewFromMsg(things, m, masses)
	}

	return things
}

func (thing *Thing) CentreOfMass() (*vec.V3, float64) {

	cg := vec.NewVec3(0, 0, 0)
	tm := 0.0
	for _, s := range thing.Springs {
		tm += s.M1.MassKG()
		cg.AddIn(s.M1.P.Multiply(s.M1.MassKG()))
		tm += s.M2.MassKG()
		cg.AddIn(s.M2.P.Multiply(s.M2.MassKG()))
	}

	return cg.Divide(tm), tm //return centre of mass and total mass

}

func (thing *Thing) addFace(p ...*vec.V3) {
	f := poly.NewConvexPolyFromVecs(p)
	thing.faces = append(thing.faces, f)
}

// func (thing *Thing) closestPointOnEdge(masses []*mass.Mass, wp *vec.V3) *vec.V3 {
// 	bestDist := float64(1000000)
// 	bestPoint := vec.NewVec3(0, 0, 0)
// 	for _, s := range thing.Springs {
// 		if s.contains(wp) {
// 			p := s.closestPointTo(wp)
// 			d := p.DistanceFrom(wp)
// 			if d < bestDist {
// 				bestDist = d
// 				bestPoint = p
// 			}
// 		}
// 	}
// 	return bestPoint
// }

// func (thing *Thing) WriteTo(msg *msg.Msg) {

// 	msg.WriteUInt32(thing.Index)
// 	msg.WriteByte(byte(len(thing.meshName)))
// 	msg.WriteString(thing.meshName)
// 	//binary.Write(buff, le, make([]byte, len(t.meshname)%4+2)) //padding
// 	msg.WriteVec3(thing.MeshOffset)
// 	msg.WriteVec3(thing.MeshScale)
// 	msg.WriteVec3(thing.MeshRotation)

// 	msg.WriteInt32(thing.Om.Index)
// 	msg.WriteInt32(thing.Fm.Index)
// 	msg.WriteInt32(thing.Rm.Index)
// 	msg.WriteByte(thing.Visibility)
// 	msg.WriteUInt32(uint32(len(thing.Springs)))
// 	for _, spring := range thing.Springs {
// 		spring.WriteTo(msg)

// 	}
// }

func ThingsAsMsg(things []*Thing, masses []*mass.Mass) *msg.Msg {
	m := msg.NewMsg(msg.Things)
	m.Write(uint32(len(things)))

	for _, thing := range things {
		thing.WriteTo(m)
	}

	return m
}

func (thing *Thing) WriteTo(msg *msg.Msg) {

	msg.Write(
		thing.Index, thing.meshName,
		thing.MeshOffset, thing.MeshScale, thing.MeshRotation,
		thing.Om.Index, thing.Fm.Index, thing.Rm.Index,
		thing.Visibility, uint32(len(thing.Springs)),
	)

	for _, spring := range thing.Springs {
		spring.WriteTo(msg)
	}

	// msg.GenericWrite[uint32](buff, thing.Index)
	// msg.WriteUInt32(thing.Index)
	// msg.WriteByte(byte(len(thing.meshName)))
	// msg.WriteString(thing.meshName)
	// //binary.Write(buff, le, make([]byte, len(t.meshname)%4+2)) //padding
	// msg.WriteVec3(thing.MeshOffset)
	// msg.WriteVec3(thing.MeshScale)
	// msg.WriteVec3(thing.MeshRotation)

	// msg.WriteInt32(thing.Om.Index)
	// msg.WriteInt32(thing.Fm.Index)
	// msg.WriteInt32(thing.Rm.Index)
	// msg.WriteByte(thing.Visibility)
	// msg.WriteUInt32(uint32(len(thing.Springs)))
	// for _, spring := range thing.Springs {
	// 	spring.WriteTo(msg)

	// }
}

// func (t *Thing) toByteBuffer(buff *bytes.Buffer) {

// 	le := binary.LittleEndian
// 	binary.Write(buff, le, t.Index)
// 	binary.Write(buff, le, byte(len(t.meshName)))
// 	binary.Write(buff, le, []byte(t.meshName))
// 	//binary.Write(buff, le, make([]byte, len(t.meshname)%4+2)) //padding

// 	t.MeshOffset.WriteTo(buff)
// 	t.MeshScale.WriteTo(buff)
// 	t.MeshRotation.WriteTo(buff)

// 	binary.Write(buff, le, t.om.Index)
// 	binary.Write(buff, le, t.fm.Index)
// 	binary.Write(buff, le, t.rm.Index)

// 	binary.Write(buff, le, t.visibility)

// 	binary.Write(buff, le, uint32(len(t.springs)))
// 	for _, spring := range t.springs {
// 		if spring.M1.Index == spring.M2.Index {
// 			panic("degenerate spring whilst serialising")
// 		}
// 		binary.Write(buff, le, spring.M1.Index)
// 		binary.Write(buff, le, spring.M2.Index)
// 		binary.Write(buff, le, spring.Collideable)
// 		binary.Write(buff, le, float32(spring.restLength))
// 		binary.Write(buff, le, byte(spring.ActuatorTag))

// 	}
// }

func (thing *Thing) Rewire() {
	for _, s := range thing.Springs {
		s.Rewire()
	}
}
func (t *Thing) PushAway(m *mass.Mass) bool {

	penetrated := false

	for _, face := range t.faces {

		pen := face.penetration(m.p, m.r)
		if pen > 0 && pen < 1 { //we're on the wrong side

			//m.lastThingTouched = thing
			m.P.SubIn(face.plane.normal.multiply(pen)) //TODO  CONSIDER EDGES PROPERLY
			penetrated = true

		}

	}

	return penetrated

}
func NewFromMsg(things []*Thing, m msg.Msg, masses []*mass.Mass) *Thing {

	idx := int32(0)
	m.Read(&idx)

	meshName := ""
	m.Read(&meshName)

	thing := New(things, meshName, 2)

	om, fm, rm := int32(0), int32(0), int32(0) //origin, forward and right masses
	m.Read(&thing.MeshOffset, &thing.MeshScale, &thing.MeshRotation, &om, &fm, &rm, &thing.Visibility)
	if thing.MeshRotation.Length() == 0 {
		log.Logit("warning: thing", thing.meshName, "has zero length mesh rotation - fixing")
		thing.MeshRotation = vec.NewVec3(0, math.Pi*2, 0)
	}

	thing.Om = masses[om]
	thing.Fm = masses[fm]
	thing.Rm = masses[rm]

	thing.Springs = make([]*spring.Spring, 0)

	numSprings := uint32(0)
	m.Read(&numSprings)

	thing.Springs = make([]*spring.Spring, 0, numSprings)

	for i := 0; i < int(numSprings); i++ {
		spring.NewFromMsg(thing.Springs, m, masses)

	}

	return thing
}
