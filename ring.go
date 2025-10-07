package main

// import "math"

func (ps *penSet) penAt(p *vec3) *pen {
	for _, pen := range ps.pens {
		if pen.p.equals(p) {
			return pen
		}
	}

	return nil
}

// //returns the positive angle (in radians) between a line from 0,0 to p and a line from 0,0 to b
// //todo - write test
// func (p *Vec3) PositiveAngleFrom(b *Vec3) float64 {

// 	aa := b.dot(p) / (b.length() * p.length())
// 	if aa < -1.001 || aa > 1.001 {
// 		panic("aa out of range")
// 	}
// 	if aa < -1 {
// 		aa = -1
// 	}
// 	if aa > 1 {
// 		aa = 1
// 	}

// 	a := math.Acos(aa)
// 	if a < 0 {
// 		a += math.Pi * 2
// 	}

// 	if math.IsNaN(a) {
// 		panic("angle is NaN")
// 	}

// 	return a

// }

// // //return 0, 1 or 2 points of intersection of the edges of triangle b, with the plane of triangle a
// //penetrations will always come in pairs, sometimes two face penetrations, sometimes to edge penetrations, sometimes one of each
type pen struct {
	ps *penSet //hold a reference to the set a belong to
	p  *vec3   //the point of penetration
	//vi	   	 int       //the new vertex of penetration
	v1, v2     uint32 // for face penetrations, the edge of the tool that penetrated
	toolTri    *tri   //the penatrator
	clayTri    *tri   //the penetratee
	used       bool   //has this been incoroporated into a ring
	isBoundary bool   //is on an edge of the clay triangle - v1 and v2 are of the penetrated edge
	next       *pen   //penetrations are formed into rings
}

type penSet struct {
	pens      []*pen
	usedCount int
}

func (ps *penSet) add(p *vec3, v1, v2 uint32, toolTri, clayTri *tri, isBoundary bool) {
	ps.pens = append(ps.pens, &pen{ps, p, v1, v2, toolTri, clayTri, false, isBoundary, nil})
}

func NewPenSet() *penSet {
	return &penSet{pens: []*pen{}, usedCount: 0}
}

// func abs(a float64) float64 {
// 	if a < 0 {
// 		return -a
// 	}
// 	return a
// }

// //write tests for  distance from planeof (which should be signed)

// func oppositeSigns(a, b float64) bool {
// 	return a*b < 0
// }
