package main

type thing struct {
	//a thing is really a collection of Springs (which never intersect)
	//to which we pin an image
	state    *state // a reference back to the game/state it belongs to
	index    int32
	omi      int32 // *mass //origin mass
	fmi      int32 // *mass // forward mass (defines z axis)
	rmi      int32 // *mass // right mass (deinfes x axis)
	springs  []*spring
	faces    []*face
	meshName string
	offset   *Vec3
	scale    *Vec3
	masses   map[*mass]bool //all the masses in the thing (once) (used for applying lift)
}

func NewThing(meshName string) *thing {
	return &thing{meshName: meshName, scale: newVec3(1, 1, 1), offset: newVec3(0, 0, 0), springs: []*spring{}, faces: []*face{}, masses: make(map[*mass]bool)}
}

// func (t *thing) payload() thingPayload {
// 	return thingPayload{
// 		Ti:         t.index,
// 		MeshName:   t.meshName,
// 		Offset:     t.offset.payload(),
// 		Scale:      t.scale.payload(),
// 		Omi:        t.omi,
// 		Fmi:        t.fmi,
// 		Rmi:        t.rmi,
// 		SpringEnds: t.springEnds()} //mass index pairs
// }

//a compact from of the sping mass indices for transmission
// func (t *thing) springEnds() []int {
// 	ends := make([]int, len(t.springs)*2)
// 	for i, s := range t.springs {
// 		ends[i*2] = s.m1.index
// 		ends[i*2+1] = s.m2.index
// 	}
// 	return ends
// }

// func (t *thing) send(player *player) {

// 	player.sendThing(t.index, t.meshName, t.offset, t.scale, t.omi, t.fmi, t.rmi, smi)

// 	//msg := &reply{Cmd: "thing", Payload: t.payload()}

// 	//t.state.send(player, msg)

// }

//find the closest point on any face in the thing to the point p
func (thing *thing) distanceFrom(p *Vec3) float64 {
	bestDist := 1000000.0
	// for _, f := range thing.faces {
	// 	d := p.distanceFromFace(f) //f.distanceFrom(m, p, false)
	// 	if d < bestDist {
	// 		bestDist = d
	// 	}
	// }
	return bestDist

}

// Is the point P the thing
// "draws" a line from the point (to test) to the origin and counts how many springs of this thing are crossed - if the number is odd, then the point is inside the thing
func (thing *thing) contains(p *Vector, m []*mass) bool {

	panic("contains not done")

}

func (thing *thing) centreOfMass(m []*mass) *Vec3 {
	//return the centre of mass of the thing
	//this is the average of the masses of the springs
	//TODO - account for weights
	c := newVec3(0, 0, 0)
	for _, s := range thing.springs {
		h := s.m1.p.add(s.m2.p)
		c.addIn(h.multiply(0.5))
	}
	return c.multiply(1.0 / float64(len(thing.springs)))
}

func (thing *thing) closestPointOnEdge(masses []*mass, wp *Vec3) *Vec3 {
	bestDist := float64(1000000)
	bestPoint := newVec3(0, 0, 0)
	for _, s := range thing.springs {
		if s.contains(wp) {
			p := s.closestPointTo(wp)
			d := p.distanceFrom(wp)
			if d < bestDist {
				bestDist = d
				bestPoint = p
			}
		}
	}
	return bestPoint
}
