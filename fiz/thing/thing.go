package thing

import (
	"fmt"
	"math"

	"github.com/nickax/gofu/cam"
	"github.com/nickax/gofu/game/actuator"

	"github.com/nickax/gofu/fiz/engine"
	"github.com/nickax/gofu/fiz/mass"

	"github.com/nickax/gofu/fiz/spring"
	"github.com/nickax/gofu/game/msg"

	//"github.com/nickax/gofu/geom"
	"time"

	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/poly"
	"github.com/nickax/gofu/vec"
)

type Thing struct {
	//a thing is really a collection of Springs (which never intersect)
	//to which we pin an image
	//state    *game.State // a reference back to the game/state it belongs to
	Index   uint32
	Name    string
	AssetId uint32
	Om      *mass.Mass //origin mass
	Fm      *mass.Mass //forward mass (defines z axis)
	Rm      *mass.Mass // right mass (deinfes x axis)
	Springs []*spring.Spring
	faces   []*poly.ConvexPoly     //face
	cameras map[string]*cam.Camera //Additional *initial* (named) position, direction and up in vehicle space

	meshName     string
	MeshOffset   *vec.V3
	MeshScale    *vec.V3
	MeshRotation *vec.V3 //axis/angle
	//masses     map[*mass.Mass]bool //all the masses in the thing (once) (used for applying lift)
	MeshVisibility byte

	Engines []*engine.Engine //multiple engines

	heading       float64 //heading of the thing (for turn rate calculation)
	lastTelemPos  *vec.V3
	lastTelemTime time.Time
	telemetry     *msg.Msg
}

type ThingList []*Thing

func (tl ThingList) References(m *mass.Mass) bool {
	for _, thing := range tl {
		for _, s := range thing.Springs {
			if s.M1 == m || s.M2 == m {
				return true
			}
		}
	}
	return false
}

func (thing *Thing) GetTelemetry() *msg.Msg {
	return thing.telemetry
}

// TelemetryAsMsg returns a msg containing telemetry data for this vehicle (since last call, over that timespan)
func (thing *Thing) UpdateTelemetry() {

	distance := thing.Om.P.Sub(thing.lastTelemPos).Length()
	dy := thing.Om.P.Y - thing.lastTelemPos.Y

	direction := thing.Forward()
	newHeading := math.Atan2(direction.X, direction.Z) / (math.Pi * 2) * 360 //angle in degrees

	now := time.Now()
	time := now.Sub(thing.lastTelemTime).Seconds()
	thing.lastTelemTime = now

	omega := newHeading - thing.heading/time*60 //degrees per minute

	thing.heading = newHeading

	thing.telemetry = msg.NewMsg(msg.Telemetry,
		uint16(4), //4 name value pairs
		"vel", float32(distance/time),
		"climb", float32(dy/time),
		"turn", float32(omega),
		"head", float32(thing.heading),
	)

}

func (thing *Thing) AddSpring(m1 *mass.Mass, m2 *mass.Mass, restLength float64, collideable byte, actuatorTag actuator.ActuatorEnum) *spring.Spring {

	s := spring.New(int32(len(thing.Springs)), m1, m2, collideable, restLength, actuatorTag)
	thing.Springs = append(thing.Springs, s)
	return s
}
func (thing *Thing) FindCam(pov string) *cam.Camera {

	povCam, present := thing.cameras[pov]
	if present {
		return povCam
	}

	log.Logit("thing", thing.meshName, "has no camera for ", pov)
	return nil

}

func New(index uint32, meshName string, numEngines int) *Thing {

	t := &Thing{Index: index,
		meshName:  meshName,
		MeshScale: vec.NewVec3(1, 1, 1), MeshOffset: vec.NewVec3(0, 0, 0), MeshRotation: vec.NewVec3(0, math.Pi*2, 0),
		Springs:        []*spring.Spring{},
		faces:          []*poly.ConvexPoly{},
		MeshVisibility: 1,
		cameras:        make(map[string]*cam.Camera),
	}

	//moment of inertia of a disc is 1/2 m * r^2
	bladeMass := 30.0
	blades := 4.0
	propChord := 0.4                                          //0.4m chord of the propellor (m) - used for thrust calculation
	propRadius := 2.0                                         //2m radius propellor (4m diameter)
	moi := 0.5 * bladeMass * propRadius * propRadius * blades //moment of inertia of the propellor (kg*m^2) - used for engine sound -- see runEngines

	t.Engines = make([]*engine.Engine, numEngines)

	for i := 0; i < numEngines; i++ {

		ne := engine.New("No "+fmt.Sprint(i+1), byte(i), 1775, propRadius, propChord*propRadius*blades, 10.0, moi, nil)
		t.Engines = append(t.Engines, ne)
	}

	return t

}

func (thing *Thing) StretchSprings() {

	for _, s := range thing.Springs {
		s.Stretch()
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

	o := thing.Om.P
	f := thing.Fm.P
	return f.Sub(o).Normalise()

}

func (thing *Thing) Right() *vec.V3 {
	o := thing.Om.P
	r := thing.Rm.P
	return r.Sub(o).Normalise()
}

func (thing *Thing) FindMassActuator(act actuator.ActuatorEnum) *mass.Mass {
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

func (thing *Thing) FindSpringActuator(act actuator.ActuatorEnum) *spring.Spring {
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
		things = append(things, NewFromMsg(m, masses)) //the things contain (generate) the springs - to we don't need to pass them in
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

func ThingsAsMsg(things []*Thing, withSprings bool) *msg.Msg {
	m := msg.NewMsg(msg.Things)
	m.Write(uint32(len(things)))

	for _, thing := range things {
		thing.WriteTo(m, withSprings)
	}

	return m
}

func (thing *Thing) WriteTo(msg *msg.Msg, withSprings bool) {

	msg.Write(
		thing.Index, thing.meshName,
		thing.MeshOffset, thing.MeshScale, thing.MeshRotation,
		thing.Om.Index, thing.Fm.Index, thing.Rm.Index,
		thing.MeshVisibility)

	if withSprings {
		msg.Write(uint32(len(thing.Springs)))
		for _, spring := range thing.Springs {
			spring.WriteTo(msg)
		}
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
func (thing *Thing) PushAway(m *mass.Mass) bool {

	penetrated := false

	for _, face := range thing.faces {

		pen := face.Penetration(m.P, m.R)
		if pen > 1 {
			log.Logit("deep penetration of thing", thing.meshName, "by mass", m.Index, "pen", pen)
		}
		if pen > 0 { //we're on the wrong side

			//m.lastThingTouched = thing
			m.P.SubIn(face.Plane.GetNormal().Multiply(pen)) //TODO  CONSIDER EDGES PROPERLY
			penetrated = true

		}

	}

	return penetrated

}

func (thing *Thing) velocity() float32 {

	return float32(thing.Om.GetVelocity().Length()) //m/s
}

func (thing *Thing) climbRate() float32 {

	return float32(thing.Om.GetVelocity().Y) //m/s

}

func NewFromMsg(m *msg.Msg, masses []*mass.Mass) *Thing {

	idx := int32(0)
	m.Read(&idx)

	//meshName := ""
	//m.Read(&meshName)

	thing := New(uint32(idx), "CL415", 2)

	m.Read(thing.MeshOffset, thing.MeshScale, thing.MeshRotation)
	junk := int16(0)
	m.Read(&junk) //read padding

	om, fm, rm := int32(0), int32(0), int32(0) //origin, forward and right masses
	m.Read(&om, &fm, &rm)

	thing.Om = masses[om]
	thing.Fm = masses[fm]
	thing.Rm = masses[rm]

	vis := byte(0)
	m.Read(&vis)

	thing.MeshVisibility = vis

	junk32 := int32(0)
	m.Read(&junk32) //read padding

	if thing.MeshRotation.Length() == 0 {
		log.Logit("warning: thing", thing.meshName, "has zero length mesh rotation - fixing")
		thing.MeshRotation = vec.NewVec3(0, math.Pi*2, 0)
	}

	thing.Springs = make([]*spring.Spring, 0)

	numSprings := uint32(0)
	m.Read(&numSprings)

	thing.Springs = make([]*spring.Spring, 0, numSprings)

	for i := 0; i < int(numSprings); i++ {
		s := spring.NewFromMsg(int32(i), m, masses)
		thing.Springs = append(thing.Springs, s)
	}

	return thing
}
