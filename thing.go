package main

type Thing struct {
	//a thing is really a collection of springs (which never intersect)
	//to which we pin an image

	enabled  bool
	springs  []*Spring
	offset   Vector
	scale    Vector
	rotation float64
	layer    string
	picname  string
	isHole   bool
}

func NewThing(layer, picName string, isHole bool) *Thing {
	return &Thing{layer: layer, picname: picName, isHole: isHole}
}

func (thing *Thing) closestPointOnEdge(masses []*Mass, wp *Vector) Vector {
	bestDist := float64(1000000)
	bestPoint := Vector{0, 0}
	for _, s := range thing.springs {
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
