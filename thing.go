package main

type Thing struct {
	//a thing is really a collection of Springs (which never intersect)
	//to which we pin an image
	Springs  []*Spring `json:"springs"`
	Offset   Vector    `json:"offset"` //offset of the image from the origin of the thing
	Scale    Vector    `json:"scale"`
	Rotation float64   `json:"rotation"`
	Layer    string    `json:"layer"`
	PicName  string    `json:"picName"`
	IsHole   bool      `json:"isHole"`
}

func NewThing(layer, picName string, isHole bool) *Thing {
	return &Thing{Layer: layer, PicName: picName, IsHole: isHole, Springs: []*Spring{}}
}

//find the closest point on any spring in the thing to the point p
func (thing *Thing) distanceFrom(p *Vector, m []*Mass) float64 {
	bestDist := 1000000.0
	for _, s := range thing.Springs {
		d := s.distanceFrom(p, m)
		if d < bestDist {
			bestDist = d
		}
	}
	return bestDist

}

// Is the point P the thing
// "draws" a line from the point (to test) to the origin and counts how many springs of this thing are crossed - if the number is odd, then the point is inside the thing
func (thing *Thing) contains(p *Vector, m []*Mass) bool {
	if !thing.IsHole {
		panic("contains only works for holes")
	}

	o := Vector{X: -1000, Y: -1000}
	crossings := 0
	for _, s := range thing.Springs {
		if s.crosses(m, &o, p) {
			crossings++
		}
	}

	return crossings%2 == 1

}

func (thing *Thing) closestPointOnEdge(masses []*Mass, wp *Vector) Vector {
	bestDist := float64(1000000)
	bestPoint := Vector{0, 0}
	for _, s := range thing.Springs {
		if s.contains(masses, wp) {
			p := s.closestPointTo(masses, wp)
			d := p.distanceFrom(wp)
			if d < bestDist {
				bestDist = d
				bestPoint = p
			}
		}
	}
	return bestPoint
}
