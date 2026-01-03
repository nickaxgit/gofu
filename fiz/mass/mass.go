package mass

import (
	"fmt"
	"math"

	"github.com/nickax/gofu/colors"
	"github.com/nickax/gofu/game/actuator"
	"github.com/nickax/gofu/game/aero"
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/ray"
	"github.com/nickax/gofu/terrain"
	"github.com/nickax/gofu/vec"
)

//coins (which are masses) - can be pushed by other masses, or by the springs of things
//spring (and therefore things) can never collide
//all collsions are either mass-mass or mass-spring
//when coins overlap - the slow coin is 'owned' by the fast coin
//when a mass overlaps a spring - the mass is 'owned' by the spring

type Mass struct {
	Index       int32
	Owner       uint32 //device id - penetrations are resolved against the owners version of the terrain
	ThingIndex  uint32 //not persisted - set (from springs) after loading
	P           vec.V3
	transformOf *Mass
	R           float64
	Fixed       bool
	IsCoin      bool
	Collideable bool
	Op          vec.V3
	//	v           *vec.V3 //"velocity" - the change in position of this mass
	//Enabled bool
	//selected         bool //needs to be per player - see player.selectedMasses map
	//lastThingTouched *thing.Thing
	Axle     *Mass //if the mass is a wheel - the vector to this mass is the axle
	WingRoot *Mass //if the mass is a wing tip - this is the TE root (the axle is the LE root)
	axi      int32 //index of the axle (used only temporarily while loading)
	wri      int32 //index of the wing root (used only temporarily while loading)
	//aoaRads          float64 //(additional) angle of attack
	WingArea float64
	//dihedralDegrees  float64
	lift       vec.V3
	thrust     float64
	AoaDegrees float64
	drag       vec.V3
	Flip       bool //?

	Section float64 //airfoil section - actually just a byte but we bind a slider to it so it has to be a float64
	Brake   float64 //brake force (0-1) - 1= full brake used for wheels
	//flightOutput     float64 //outputEnum - what am I ? aileron, elevator, rudder, flap, engine
	//gain             float32 //multiplier/inverter for the (normalised) control input - e.g. convert to radians of aileron deflection
	//isThrust         bool    //throttles create thrust - control surfaces have their AoA changed
	ActuatorTag actuator.ActuatorEnum //float64

}

func VectorsAsMsg(masses []*Mass) *msg.Msg {

	//send the mass index, vector and color - show lift at the wingtips (althoug it is actually shared between the three verts)

	msg := msg.NewMsg(msg.Vectors)

	for _, m := range masses {
		m.WriteVectorsTo(msg)
	}

	terminator := float32(math.Inf(1))
	msg.Write(terminator, terminator, terminator) //Terminator for vectors

	return msg

}

func (m *Mass) Contains(p vec.V3) bool {
	return m.P.DistanceFrom(p) <= m.R
}

func (m *Mass) SetVelocity(v vec.V3) {
	m.Op = m.P.Sub(v)
}

func (m *Mass) GetVelocity() vec.V3 {
	return m.P.Sub(m.Op)
}

func FindAt(masses []*Mass, p vec.V3, tol float64) *Mass {
	for _, m := range masses {
		if m.WithinDistanceOf(p, tol) {
			return m
		}
	}
	return nil
}

// ReReferenceIndices sets the axle and wingroot pointers after loading
func (m *Mass) ReReferenceIndices(masses []*Mass) {
	if m.axi > -1 {
		if masses[m.axi] == nil {
			panic("mass axis not found")
		}
		m.Axle = masses[m.axi]
	}
	if m.wri > -1 {
		if masses[m.wri] == nil {
			panic("mass wri not found")
		}
		m.WingRoot = masses[m.wri]
	}
}

func (m *Mass) CalcWingArea() {
	if m.Axle != nil && m.WingRoot != nil && m.WingArea == 0 {
		//panic("no wing area")
		m.WingArea = m.P.Sub(m.Axle.P).Length() * m.Axle.P.Sub(m.WingRoot.P).Length()
	}
}

func (m *Mass) MassKG() float64 {
	//this assumes a density of 10kg/liter

	m3 := (4 / 3) * 3.14159 * m.R * m.R * m.R
	return m3 * 1000 * 500 //litres in a m^3
}

func (m *Mass) SetIndex(i int32) {
	m.Index = i
}

func New(index int32, p vec.V3, r float64, fixed bool, isCoin bool, collideable bool, transFormOf *Mass, owner uint32) *Mass {

	return &Mass{Index: index,
		P: p, R: r, Fixed: fixed, IsCoin: isCoin,
		Collideable: collideable,
		Op:          p.Clone(),
		transformOf: transFormOf,
		Owner:       owner}
}

func (m *Mass) HasMoved() bool {
	if m.P == m.Op {
		panic("logic error - mass position references are the same")
	}
	return !m.P.Equals(m.Op)
}
func (m *Mass) overlaps(masses []*Mass) *Mass {

	for _, m2 := range masses {
		if m != m2 && m.P.DistanceFrom(m2.P) < m.R+m2.R {
			return m2
		}
	}
	return nil

}
func (m *Mass) WithinDistanceOf(p vec.V3, tol float64) bool {
	return m.P.DistanceFrom(p) <= tol
}
func NewFromMsg(massesSoFar []*Mass, msg *msg.Msg, withDetail byte, owner uint32) *Mass {

	var idx int32 = 0
	msg.Read(&idx)

	p := vec.V3{}
	msg.Read(p) //read the mass position (note p is a pointer)

	m := New(idx, p, 0, false, false, false, nil, owner)

	m.Op = m.P.Clone() //todo serialise this to be able to save masses in motion
	if withDetail != 0 {
		r := float32(0)
		msg.Read(&r)
		m.R = float64(r)
		msg.Read(&m.Fixed, &m.IsCoin, &m.Collideable)

		transformIdx := int32(-1)
		mab := byte(0) //mass actuator tag byte
		sec := byte(0) //section

		//obsoltete mass selected
		selected := false
		//msg.Read(&selected, &m.axi, &m.wri, &m.WingArea, &transformIdx, &m.Flip, &sec, &mab)
		junk := int32(0)
		//f32Junk := float32(0)
		msg.Read(&selected, &m.axi, &m.wri) //, &f32Junk, &f32Junk, &f32Junk)

		msg.Read(&junk)
		msg.Read(&transformIdx, &m.Flip, &sec, &mab)

		if transformIdx != -1 {
			m.transformOf = massesSoFar[transformIdx]
		}

		m.Section = float64(aero.Section(sec))
		m.ActuatorTag = actuator.ActuatorEnum(mab)

	}
	return m
}

func (m *Mass) Fly(running bool) {

	if m.WingRoot != nil {
		if m.Axle == nil {
			log.Logit("no wing axis")
			return
		}

		//wingAxis := (m.p.sub(m.axle.p)).normalise()
		wingAxis := (m.P.Sub(m.WingRoot.P)).Normalise() //TE
		if m.Flip {
			wingAxis = wingAxis.Multiply(-1)
		}

		rootChord := (m.Axle.P.Sub(m.WingRoot.P)).Normalise()

		vms := m.P.Sub(m.Op).Length() * 30.0 * 5.0 //cyles per second * steps per cycle

		if vms > 0.1 {
			direction := (m.P.Sub(m.Op)).Normalise()
			//log.Logit(vms, "m/s")
			v2 := vms * vms

			if vms > 100 {
				log.Logit("Overspeed", vms)
			}

			dd := direction.Dot(wingAxis)
			if dd == 1.0 || dd == -1.0 {
				return //parallel to the wing axis - no lift (for example the fin moving up)
			}

			//the lift direction is always orthogonal to the direction of travel (regardless of the AoA)
			liftDir := (direction.Cross(wingAxis)).Normalise() //.rotateAbout(rootAxis, m.dihedralDegrees)
			wingUp := (rootChord.Cross(wingAxis)).Normalise()  //orthogonal to the chord of the wing (le-te)

			aoa := -math.Asin(direction.Dot(wingUp)) // + math.Pi/2 //+ m.aoaRads
			// if aoa < -math.Pi {
			// 	aoa += math.Pi * 2
			// } else if aoa > math.Pi {
			// 	aoa -= math.Pi * 2
			// }

			m.AoaDegrees = aoa / (math.Pi * 2) * 360

			if m.WingArea > 50 {
				m.AoaDegrees -= 2
			} //reduce incidence of the main wing (what - why ??)

			if m.AoaDegrees < -20 || m.AoaDegrees > 20 {
				log.Logit("aoa", m.AoaDegrees)
			}
			cl := aero.LiftCurves[int(m.Section)].GetY(m.AoaDegrees) //lerp(m.aoaDegrees, state.liftCurves[int(m.section)])
			cd := aero.DragCurves[int(m.Section)].GetY(m.AoaDegrees) //lerp(m.aoaDegrees, state.dragCurves[int(m.section)])
			liftNewtons := v2 * cl * aero.Rho * .5 * m.WingArea      //cycles per second * steps per cycle
			if liftNewtons > 100000 {
				log.Logit("excess lift", liftNewtons)
			}
			lift := liftDir.Multiply(liftNewtons)

			dragNewtons := v2 * cd * m.WingArea
			drag := direction.Multiply(-dragNewtons)

			m.lift = lift //.multiply(0.001) //visualise at 1mm per newton
			m.drag = drag

			if running {
				// if m.thrust != 0 {
				// 	m.p.addIn(rootAxis.multiply(m.thrust * ntm))
				// }

				f := float64(2 * 3 * 15000) // half (acceleration to distance travelled) 1/3rd of the lift distribution  150 steps per second
				d := lift.Divide(m.WingRoot.MassKG() * f)

				m.WingRoot.P.AddIn(d) // ntm * .33))
				d = lift.Divide(m.Axle.MassKG() * f)
				m.Axle.P.AddIn(d)
				d = lift.Divide(m.MassKG() * f)
				m.P.AddIn(d)

				m.P.AddIn(drag.Divide(m.MassKG() * f)) //multiply(ntm * 1))
			}

		}

		//todo - spread across the three masses
	}

}

func (m *Mass) WriteInto(msg *msg.Msg, withDetail bool, selected byte) {

	msg.Write(m.Index, m.P)

	if withDetail {
		msg.Write(m.R, m.Fixed, m.IsCoin, m.Collideable, selected == 1)

		//note highlit mass is not done this way (becuase there must be only one)
		ax := int32(-1)
		if m.Axle != nil {
			ax = m.Axle.Index
		}
		wr := int32(-1)
		if m.WingRoot != nil {
			wr = m.WingRoot.Index
		}

		msg.Write(ax, wr, m.WingArea)

		tmi := int32(-1)
		if m.transformOf != nil {
			tmi = m.transformOf.Index
		}
		msg.Write(tmi)

		msg.Write(m.Flip, byte(m.Section), byte(m.ActuatorTag))
	}

}

func (m *Mass) RegenFromMaster(transform func(p vec.V3) vec.V3) {
	if m.transformOf != nil {
		m.P = transform(m.transformOf.P)
	}

}

// selectedmasses are used when sending the client to implement the player-specific selection set

//func (m *Mass) resolveMassSpringOverlap(masses []*Mass, spring *spring.Spring, pen float64) {
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
//}

func (m *Mass) ResolvePenetration(mesh *terrain.TriMesh, depth float64, impact vec.V3, surface *terrain.LeafTri) {
	
	
	v := m.P.Sub(m.Op)
	n:=surface.CacheNormal(mesh)
	vr := v.Reflect(n)

	if depth > 0.1 {
		log.Logit("deep penetration", v.Length()*30, "m/s")
	}

	m.P.Y = impact.Y + m.R

	vr = vr.Sub(n.Multiply(vr.Dot(n) * .8)) //kill 80% of the vertical velocity (20% bounce)

	if m.Axle != nil {
		axle := m.Axle.P.Sub(m.P).Normalise()
		vr = vr.Sub(axle.Multiply(vr.Dot(axle) * .85)) //.95)) //kill (95% of the) sideways velocity of the wheel
		vr = vr.Multiply(0.95)                         //some wheel friciton

		m.Fixed = false
		if m.Brake > .01 {
			maxBrakeForce := 0.03                 //metres per cycle
			brakeForce := m.Brake * maxBrakeForce //brake force in metres per cycle
			vrl := vr.Length()
			if vrl > brakeForce {
				vr.SubIn(vr.Normalise().Multiply(brakeForce)) //some wheel friciton
			} else {
				//vr = NewVec3(0, 0, 0) //vr.multiply(-0.001) //dead stop
				//m.fixed = true
				//we have enough brake force - the brakes are holding
				m.P.X = m.Op.X
				m.P.Z = m.Op.Z

				return
			}
		}

	} else { //not a wheel
		//vr = vr.multiply(.8) //kill 80% of the velocity
	}

	m.Op = m.P.Sub(vr)
}

func (m *Mass) WriteVectorsTo(msg *msg.Msg) {

	//velocity vector
	pp := m.P.Sub((m.P.Sub(m.Op)).Multiply(10))
	msg.Write(m.P, pp, colors.Magenta)

	if m.Axle != nil {
		//Axle/leading edge
		quarter := m.P.Add(m.Axle.P.Sub(m.P).Multiply(0.25))
		msg.Write(m.P, quarter, colors.Orange)
	}

	if m.lift.IsNonZero() {

		//trailing edge

		msg.Write(m.P, m.WingRoot.P, colors.Blue)

		if m.Axle != nil {
			centreOfLift := m.P.Add(m.WingRoot.P).Add(m.Axle.P).Multiply(1.0 / 3.0)

			if m.Axle.P == m.WingRoot.P {
				log.Logit("axle and wingroot are the same")
			}
			liftEnd := centreOfLift.Add(m.lift.Multiply(0.0001))
			msg.Write(centreOfLift, liftEnd, colors.Red)

		}

		//todo - acutal lift and drag vectors
	}
}

func (m *Mass) IsActuator(act actuator.ActuatorEnum) bool {
	return m.ActuatorTag == act
}

// SwitchToOriginal for a mass whos transform does not move it
// (it is on the mirror plane, or at the centre of rotation)
func (m *Mass) SwitchToOriginal() *Mass {
	if m.transformOf != nil {
		if m.P.DistanceFrom(m.transformOf.P) < 0.001 {
			return m.transformOf
		}
	}
	return m
}

// Delete removes a mass(by index) from a slice and updates the indices of the remaining masses
func (m *Mass) Delete(masses []*Mass) {

	masses = append(masses[:m.Index], masses[m.Index+1:]...)

	for i, j := range masses {
		if j.Index > m.Index {
			j.Index--
		}
		if j.Index != int32(i) {
			panic("mass index mismatch")
		}

	}
}

func MassesFromMsg(message *msg.Msg, owner uint32) []*Mass {

	if message.MsgType != msg.Masses {
		panic("Masses are not next")
	}

	nm := uint32(0)
	message.Read(&nm)

	massBatch := make([]*Mass, 0, nm)
	withDetail := byte(0)
	message.Read(&withDetail)

	for i := 0; i < int(nm); i++ {
		mass := NewFromMsg(massBatch, message, withDetail, owner) //creates them 'into' the mass slice

		massBatch = append(massBatch, mass)
		if int(mass.Index) != i {
			panic(fmt.Sprintf("mass index mismatch i:%v, mass.index:%v", i, mass.Index))
		}
	}

	//now they're all here ..
	for _, m := range massBatch {
		m.ReReferenceIndices(massBatch) //sets Axles and wingRoots
	}

	return massBatch
}

func MassesAsMsg(masses []*Mass, withDetail bool, playerSelected map[*Mass]bool) *msg.Msg {

	msg := msg.NewMsg(msg.Masses, uint32(len(masses)), withDetail)

	//send the masses
	for i, mass := range masses {
		selected := byte(0)

		present, isSelected := playerSelected[mass]
		if present && isSelected {
			selected = byte(1)
		}

		if i != int(mass.Index) {
			panic(fmt.Sprintf("mass index mismatch i:%v, index:%v", i, mass.Index))
		}
		mass.WriteInto(msg, withDetail, selected) //writes the masses into one message

	}

	return msg

}

func ClosestMassToRay(masses []*Mass, exclude *Mass, ray ray.Ray) (m *Mass, distance float64) {

	closestDistance := float64(1000)
	var closestMass *Mass = nil

	for _, m := range masses {
		if m == exclude {
			continue
		} //skip the excluded mass
		d := m.P.DistanceFromLine(ray.Origin, ray.End)
		if d < m.R && d <= closestDistance {
			closestDistance = d
			closestMass = m
		}
	}

	return closestMass, closestDistance
}
