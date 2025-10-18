package main

import (
	"math"
	"math/rand/v2"
)

type fireMesh struct {
	root      *fTri
	p         []*vec3
	midpoints map[uint64]uint32 //used to index vertices on their edges

}
type fTri struct {
	vi       [3]uint32 //vertex indices
	children []*fTri
	//parent   *tri
	flames       int
	sparkAt      uint //when to spark a new fire
	smokeDensity int
	smokeFloor   int
	smokeCeiling int
	wind         vec3
	depth        int
	y            float64 //the y position of the land directly under the centre of this triangle (updated as players see more land)
	landDepth    int     //splitting depth when making the observation of y
	normal       *vec3
}

func (fm *fireMesh) ignite(firePos *vec3) {
	//find the triangle containing this position, and ignite it

	result := fm.root.splitUntil(fm, firePos, 10)
	if result != nil { // are we on the map ?
		if len(result.children) > 0 {
			logit("Igniting a non-leaf triangle at depth", result.depth)
		}

		if result.flames == 0 {
			result.flames = 1
		}

	}
}

func (fm *fireMesh) centre(ft *fTri) *vec3 {
	mid := newVec3(0, 0, 0)
	for _, vi := range ft.vi {
		mid = mid.add(fm.p[vi])
	}
	mid = mid.multiply(1 / 3.0)
	return mid
}

func (fm *fireMesh) burn(ft *fTri) int {
	if ft.flames > 0 {
		ft.flames++
		if ft.flames > int(ft.sparkAt) && ft.flames < int(ft.sparkAt)+10 {
			fm.ignite(fm.centre(ft).add(newVec3((rand.Float64()-0.5)*50, 0, (rand.Float64()-0.5)*50)))
		}
		if ft.flames == 300 {
			ft.flames = -1 //burnt out
		}
		return ft.flames
	}
	tflames := 0
	for _, c := range ft.children {
		tflames += fm.burn(c)
	}
	return tflames
}

// return whether all bottom level 'leaf' triangles are alight
func (fTri *fTri) allBLTsAlight() bool {

	if len(fTri.children) == 0 {
		return fTri.flames > 0
	}
	for _, c := range fTri.children {
		if !c.allBLTsAlight() { //burnt out or not alight
			return false
		}
	}
	return true

}

// collects and sends the flames vsible to this player
func (p *player) getFlames() {
	flameMesh := newSimpleMesh(201, "flame", 10000, 30000) //10k faces, 30k verts

	tcs := &tcs{left: 0, top: 1, right: 1, bottom: 0} //texture atlas coordinates
	p.state.fire.root.getFlames(p.landTri, p.state.fire, flameMesh, p.camera, tcs)

	//flameCount := p.state.fire.root.getFlames(p.landTri, p.state.fire, flameMesh, p.camera) //update flame base heights with this players observable land
	//logit("Player", p.name, "sees", flameCount, "flames")

	flameMesh.sendTo(p, 1)

}

// recurse from fTri to find all triangles with flames, insert them (as billboards) into the simpleMesh
func (fTri *fTri) getFlames(lt *tri, fm *fireMesh, intoMesh *simpleMesh, cam *camera, tcs *tcs) int {

	up := newVec3(0, 1, 0)
	if fTri.flames > 0 { //fTri.allBLTsAlight() { //fTri.flames > 0 {

		mid := fm.centre(fTri)

		if fTri.landDepth < 10 { //sampling at level 12 is 'good enough' for flame base
			p, t := lt.probeLand(mid) //TODO - do once and cache - also normal (for slope)
			if t.depth < 8 {
				return 0
			} //it's either very far away, or behind the camera

			if t.depth > fTri.landDepth {
				fTri.y = p.y //we have a better observation - update the flame base height
				fTri.normal = t.normal
			}
			if t.isUnderwater(0) {
				fTri.flames = -1 //extinguish the flame
				return 0
			} //flames under water do not burn

		}
		mid.y = fTri.y

		//flameTop := p.clone()
		//flameTop.y += 3

		//optimise - remove hidden/backfacing flames
		// 			pop := fTri.probePlane(flameTop, camPos) //fire a 10km ray
		// 			if pop != nil {
		// 				if fTri.contains(pop, fm) {
		// //					*hidden++
		// 					return
		// 				}
		// 			}

		//pens := make([]*vec3, 100)
		//count := int(0)

		//landTri.probe(treeTop, camPos, pens, &count) //can we see the camera from the tree top

		intoMesh.billboard(mid, up, cam.position, 4, 0, 8, 3, tcs) //triangular flame
		return 1

	} else {
		flames := 0
		for _, c := range fTri.children {
			flames += c.getFlames(lt, fm, intoMesh, cam, tcs)
		}
		return flames
	}

}

func (fTri *fTri) splitUntil(fm *fireMesh, firePos *vec3, maxDepth int) *fTri {
	//recursively split triangles until the firePos is contained in a triangle at maxDepth

	if fTri.contains(firePos, fm) {
		if fTri.depth == maxDepth {
			return fTri
		}

		//split only if necessary
		if len(fTri.children) == 0 {
			fTri.split(fm)
		}

		miss := 0
		for _, c := range fTri.children {
			res := c.splitUntil(fm, firePos, maxDepth)
			if res != nil {
				return res
			}
			miss++
		}
		if miss == 4 {
			logit("missed all children")
		}

	}
	return nil

}

// checks for containment on the x/z plane only - faster than full 3d check
func (fTri *fTri) fastContains(p *vec3, fm *fireMesh) bool {

	a := fm.p[fTri.vi[0]]
	b := fm.p[fTri.vi[1]]
	c := fm.p[fTri.vi[2]]
	a2 := newVec2(a.x, a.z)
	b2 := newVec2(b.x, b.z)
	c2 := newVec2(c.x, c.z)

	e0 := b2.subtract(a2)
	e1 := c2.subtract(b2)
	e2 := a2.subtract(c2)

	p2 := newVec2(p.x, p.z)

	x0 := p2.subtract(a2).cross(e0)
	x1 := p2.subtract(b2).cross(e1)

	if x0 == 0 || x1 == 0 {
		return true //on an edge
	}

	if (x0 > 0 && x1 < 0) || (x0 < 0 && x1 > 0) {
		return false
	}
	x2 := p2.subtract(c2).cross(e2)
	if x2 == 0 {
		return true
	} //on an edge
	if (x0 > 0 && x2 < 0) || (x0 < 0 && x2 > 0) {
		return false
	}

	return true

}

func (fTri *fTri) contains(p *vec3, fm *fireMesh) bool {

	up := newVec3(0, 1, 0)
	return p.isInsideTri(fm.p[fTri.vi[0]], fm.p[fTri.vi[1]], fm.p[fTri.vi[2]], up, true, true)
}

func newFireMesh(size float64) *fireMesh {
	fm := &fireMesh{p: []*vec3{}, midpoints: map[uint64]uint32{}, root: newFtri(0, 0, 1, 2)}

	fm.addVert(newVec3(0, .1, size)) //the height of the first vertex is the initial seed for the entire land
	fm.addVert(newVec3(size, 0, -size))
	fm.addVert(newVec3(-size, 0, -size))

	return fm
}

func (fm *fireMesh) addVert(p *vec3) uint32 {

	fm.p = append(fm.p, p)

	return uint32(len(fm.p) - 1)

}

func (ft *fTri) split(fm *fireMesh) {
	if len(ft.children) == 0 {

		//	kink := (t.mesh.height/(float64(t.depth*t.depth)+1) - 1) * .5 //maximum kink in this edge
		//kink := (t.mesh.height/(float64(t.depth*5)+1) - 1) * .5 //maximum kink in this edge
		//kink := -t.mesh.height / math.Pow(2, float64(t.depth))

		v0 := ft.vi[0]
		v1 := ft.vi[1]
		v2 := ft.vi[2]

		v3 := fm.splitEdge(v0, v1)
		v4 := fm.splitEdge(v1, v2)
		v5 := fm.splitEdge(v2, v0)

		ft.addChild(v0, v3, v5) //top
		ft.addChild(v3, v1, v4) //right
		ft.addChild(v5, v4, v2) //left
		ft.addChild(v3, v4, v5) //centre

	} else {
		logit("splitting a fire triangle that already has children ??")
	}
}

func newFtri(depth int, vi ...uint32) *fTri {
	return &fTri{vi: [3]uint32{vi[0], vi[1], vi[2]}, children: []*fTri{}, depth: depth, sparkAt: uint(rand.Float32()*100) + 50}
}

func (ft *fTri) addChild(vi ...uint32) *fTri {
	child := newFtri(ft.depth+1, vi...) //t.mesh.makeTri(t.depth+1, fi, a, b, c)
	ft.children = append(ft.children, child)
	return child
}

func (fm *fireMesh) splitEdge(a, b uint32) uint32 {

	pa := fm.p[a]
	pb := fm.p[b]

	p := pa.tween(pb, 0.5)

	//maintain a map of edges to midpoints
	// key := uint32(a) + uint32(65536)*uint32(b)
	// existingMid, present := m.midpoints[key]

	vi := fm.midpoint(a, b) //will look (in a map) for an existing midpoints a->b or b->a
	if vi != math.MaxUint32 {
		if vi == 0 {
			logit("zero midpoint")
		}
		return vi
	}

	vi = fm.addVert(p) //OrReuseVertAtXZ(p)

	//add it to the index of midpoints
	key := uint64(a) + uint64(math.MaxUint32)*uint64(b)
	fm.midpoints[key] = vi

	return vi
}

func (fm *fireMesh) midpoint(v1 uint32, v2 uint32) uint32 {
	//return the index of the midpoint of the edge v1,v2
	key := uint64(v1) + uint64(math.MaxUint32)*uint64(v2)
	mid, present := fm.midpoints[key]
	if present {
		return mid
	}

	//look the other way
	key = uint64(v2) + uint64(math.MaxUint32)*uint64(v1)
	mid, present = fm.midpoints[key]
	if present {
		return mid
	}

	return math.MaxUint32
}
