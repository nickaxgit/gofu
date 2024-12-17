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
