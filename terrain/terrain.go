package terrain

import (
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/mesh"

	//"github.com/nickax/gofu/ray"
	"fmt"
	"math"

	"github.com/nickax/gofu/ray"
	"github.com/nickax/gofu/vec"
)

// feels like the collector may be an unnecesssary step
// can't splitifnecessary generate the plants and simplemesh faces directly ? - doing the occlusion and backface culling there too ?
// even writing the faces directly to the message ?
// patching is a problem - may need to integrate

type TriMesh struct {
	name        string
	Root        *Tri
	verts       []*vert //{}
	VertexCount int     //uint32
	//fi          []uint32 //{} //face indices

	midpoints map[uint64]uint32 //compound key of the two endpoints of an edge, map contains the index of its midpoint vertex

	size      float64            //+/- land size
	kinks     []float64          //fractions of the land height to pull down by at each level
	height    float64            //+/- land height
	flooding  bool               // for debug (concurrency check)
	firstLeaf map[uint32]LeafTri //devciceID to first leaf tri seen by that device
}

type fireInfo struct {
	//tri          *tri
	flames       int
	sparkAt      uint //when to spark a new fire
	smokeDensity int
	smokeFloor   int
	smokeCeiling int
	wind         vec.V3
	depth        int
	y            float64 //the y position of the land directly under the centre of this triangle (updated as players see more land)
	landDepth    int     //splitting depth when making the observation of y
	normal       *vec.V3
}

func (tri *Tri) MeshPoints(mesh *TriMesh, subdivisions int) []vec.V3 {
	points := []vec.V3{}
	step := 1.0 / float64(subdivisions)
	for i := 0; i <= subdivisions; i++ {
		for j := 0; j <= subdivisions-i; j++ {
			u := float64(i) * step
			v := float64(j) * step
			w := 1.0 - u - v
			p := tri.BarycentricInterpolate(mesh, u, v, w)
			points = append(points, p)
		}
	}
	return points
}

func (t *Tri) BarycentricInterpolate(m *TriMesh, u, v, w float64) vec.V3 {
	a := m.verts[t.vi[0]].p
	b := m.verts[t.vi[1]].p
	c := m.verts[t.vi[2]].p
	p := vec.NewVec3(0, 0, 0)
	p.X = a.X*u + b.X*v + c.X*w
	p.Y = a.Y*u + b.Y*v + c.Y*w
	p.Z = a.Z*u + b.Z*v + c.Z*w
	return p
}

func (land *TriMesh) GetTreesInto(message *msg.Msg) {

	// 	occludedTrees := dev.ViewingPlayer.Game.PlaceTrees() //place trees on non occluded, level 10, triangles infront of the camera
	// 	log.Logit("Placed trees - occluded:", occludedTrees, "near trees:", dev.nearTreeCount, "far trees:", dev.farTreeCount)
	// 	//near trees (mesh intances)
	// 	message.Write(msg.PositionInstances, uint16(100),
	// 		uint16(0),                 //from
	// 		uint16(dev.nearTreeCount), //to (we position a subset)
	// 		byte(0),                   //padding for dword alignemnt
	// 		dev.nearTreePositions[0:dev.nearTreeCount*3], //Slice of XYZ float32's
	// 	)

	// 	message.Write(msg.PositionInstances, uint16(105),
	// 		uint16(0),                //from
	// 		uint16(dev.farTreeCount), //to (we position a subset)
	// 		byte(0),                  //padding for dword alignemnt
	// 		dev.farTreePositions[0:dev.farTreeCount*3], //Slice of XYZ float32's
	// 	)

	// 	// //far trees (billboards)
	// 	// if dev.farTreeBillBoardMesh.FaceCount() > 0 {
	// 	// 	dev.farTreeBillBoardMesh.WriteTo(message, 1)
	// 	// }
	// }
}

// return the normals of the verts specified in vis (vertices we've added)
func (m *TriMesh) getNormals(asWater bool, touchedVerts *Contacts) []vec.V3 {

	//for every new vertex, reset the normal to zero, then add the normals of the faces it touches
	n := make([]vec.V3, m.VertexCount) //position x,y,z

	if asWater {
		up := vec.NewVec3(0, 1, 0)
		for i := 0; i < m.VertexCount; i++ { //} range m.verts {
			n[i] = up
		}
		return n

	}

	for i := 0; i < m.VertexCount; i++ {
		n[i] = touchedVerts.GetNormal(uint32(i), m)
	}

	return n

}

func (mesh *TriMesh) Reset() {

	panic("dont call this")
	clear(mesh.midpoints) //empties the map preserving capacity

	//preserve the first three verts (the root triangle)
	for i := 0; i < 3; i++ {
		v := mesh.verts[i]

		//clear(v.touches)
		v.wl = 0
		//v.occluded = false
		//v.testedForOcclusion = false

	}

	mesh.VertexCount = 3 //beacuse we start again at the root triangle

	mesh.Root.Reset()

}
func NewTriMesh(name string, maxFaces uint16, size float64, kinks []float64, height float64) *TriMesh {

	//we need three entries per face in fis

	//l := int(maxFaces) * 3
	//fis := make([]uint32, l)
	//return &landMesh{name: name, verts: []*vert{}, fi: fis, midpoints: make(map[uint64]uint32, 0), splits: splits, height: height, size: size, kinks: kinks}
	verts := make([]*vert, 0, 65000) //clear the verts
	mesh := &TriMesh{name: name, verts: verts, midpoints: make(map[uint64]uint32, maxFaces*3), size: size, kinks: kinks, height: height}

	seaLevel := height * .2
	mesh.addVert(vec.NewVec3(size, 0, -size), 0, 0, seaLevel)  //near right
	mesh.addVert(vec.NewVec3(-size, 0, -size), 0, 0, seaLevel) //near left
	mesh.addVert(vec.NewVec3(0, 0, size), 0, 0, seaLevel)      //far, far away

	mesh.Root = newTri(nil, mesh, [3]uint32{0, 1, 2}) //make the root triangle
	mesh.Root.MakePrism(mesh)
	return mesh //&TriMesh{name: name, Root: root, verts: []*vert{}, midpoints: make(map[uint64]uint32, maxFaces*3), size: size, kinks: kinks, height: height}
}

func (m *TriMesh) midpoint(v1 uint32, v2 uint32) uint32 {
	//return the index of the midpoint of the edge v1,v2
	key := uint64(v1) + uint64(math.MaxUint32)*uint64(v2)
	mid, present := m.midpoints[key]
	if present {
		return mid
	}

	//look the other way
	key = uint64(v2) + uint64(math.MaxUint32)*uint64(v1)
	mid, present = m.midpoints[key]
	if present {
		return mid
	}

	return math.MaxUint32
}

// creates a new vertex halfway between the indexed verts a and b and returns the index of the new vertex
// depth is used to determin whether to calculate UVs from alitude, or halfway between the two parent verts UVs
// such that geoloiical strata are decided 'early' and later splits distort them - this produces more natural strata, and also beans finer geometry does not have texture coords popping (snow patches appearing out of nowhere for example)
func (m *TriMesh) splitEdge(ai, bi uint32, dy float64, depth int) uint32 {

	if ai == bi {
		panic("degenerate edge in splitEdge")
	}
	a := m.verts[ai]
	b := m.verts[bi]

	p := a.p.Tween(b.p, 0.5)

	p.Y += dy

	depthA := a.wl - a.p.Y
	depthB := b.wl - b.p.Y

	//maintain a map of edges to midpoints
	// key := uint32(a) + uint32(65536)*uint32(b)
	// existingMid, present := m.midpoints[key]

	vi := m.midpoint(ai, bi) //will look (in a map) for an existing midpoints a->b or b->a

	if vi == ai || vi == bi {
		panic("midpoint is one of the endpoints")
	}
	if vi != math.MaxUint32 {

		return vi
	}

	//No vertex here - make one
	//for the first few levels of splitting, calculate UVs from altitude (so large scale features are consistent)
	uvy := (p.Y + m.height) / (2 * m.height)
	if depth > 6 {
		//later splits, preserve ealier UVs (so fine geometry detail does not cause texture popping)
		uvy = (a.uv.Y + b.uv.Y) / 2
	}

	//we don't have normals yet - so must calc UVX's later
	vi = m.addVert(p, 0, uvy, p.Y+(depthA+depthB)/2) //OrReuseVertAtXZ(p)

	//add it to the index of midpoints
	key := uint64(ai) + uint64(math.MaxUint32)*uint64(bi)
	m.midpoints[key] = vi

	return vi
}

func (m *TriMesh) addVert(p vec.V3, u float64, v float64, wl float64) uint32 {

	var vert *vert

	if wl == 0 {
		panic("water level not set")
	}

	// Check if we have an existing vert to reuse
	if m.VertexCount < len(m.verts) {
		vert = m.verts[m.VertexCount]
		// Overwrite existing data
		// Note: We assume p is a new vector or we copy it.
		// If p is a pointer to a vector that changes, we might need p.Clone() or *vert.p = *p
		vert.p = p
		vert.uv.X = u
		vert.uv.Y = v
		//vert.occluded = false
		//vert.testedForOcclusion = false
		vert.wl = wl
		//clear(vert.touches)

		//vert.touches = make(map[*Tri]bool, 6)

		//vert.incount = 0
		// vert.touches is already cleared in Reset()
	} else {
		// No free verts, allocate new one and append
		vert = newVert(p, u, v, wl)
		m.verts = append(m.verts, vert)
	}

	idx := uint32(m.VertexCount)
	m.VertexCount++
	return idx
}

// create a shore by dropping the land vertex to the level of
// func (m *TriMesh) flowWater(fis []uint16) {

func (m *TriMesh) Flow() {
	// for _, v := range m.verts {
	// 	v.acc = 0
	// 	v.incount = 0
	// }
	//flow the water (only along the deepest/visible faces)
	for iw := 0; iw < 10; iw++ { //iteration of water
		for _, tri := range m.Root.getLeaves() {
			tri.flow(m)
		}
	}
}

func (t *Tri) flow(m *TriMesh) {
	a := m.verts[t.vi[0]]
	b := m.verts[t.vi[1]]
	c := m.verts[t.vi[2]]
	a.flow(b)
	b.flow(c)
	c.flow(a)
}

func (m *TriMesh) SendWater(tris *LeafCollector, campos vec.V3, response *msg.Msg) {
	funcIsWater := func(t *LeafTri, m *TriMesh) bool { return t.OnOrUnderWater(m) }
	//funcIsWater := func(t *Tri, m *TriMesh) bool { return true }
	waterMesh := m.ToSimpleMesh(tris, uint16(4), nil, "water", true, funcIsWater, campos)

	if waterMesh.FaceCount() > 0 {
		log.Logit("Water mesh has ", waterMesh.FaceCount(), " faces and ", waterMesh.VertCount(), " verts")
		waterMesh.WriteTo(response, 1)
	} else {
		log.Logit("No water")
	}
}

//update the water levels (from the accumulators)
// for _, v := range m.verts { //note m.verts is a slice of pointers to verts - so we *can* mutate them

// 	v.wl += v.acc / float64(v.incount)
// 	v.acc = 0
// 	v.incount = 0
// }

//now done within ToSimpleMesh (only for verts actually used in faces)
// func (m *TriMesh) updateUVxsFromNormals() {

// 	for i := 0; i < m.VertCount(); i++ {
// 		v := m.verts[i]
// 		v.uv.UpdateUVxFromNormal(v.n)
// 	}

// }

func (m *TriMesh) getUVs() []float32 {

	vc := m.VertexCount
	uvs := make([]float32, vc*2)

	//big vertex index (in the huge mesh) to small vertex index (in the new sub mesh)
	for i := 0; i < vc; i++ {
		v := m.verts[i]
		uvs[i*2] = float32(v.uv.X)
		uvs[i*2+1] = float32(v.uv.Y)
	}

	return uvs

}

func (m *TriMesh) getPositions(asWater bool) []float32 {

	vc := m.VertexCount
	p := make([]float32, 0, vc*3) //position x,y,z

	for i := 0; i < vc; i++ {
		j := m.verts[i]
		if asWater {
			v := []float32{float32(j.p.X), float32(j.wl), float32(j.p.Z)}
			p = append(p, v...)
			p[i*3+1] = float32(j.wl) //y + j.wl*10) //water level * 10
		} else {
			p = append(p, j.p.AsFloat32s()...)
		}

	}
	return p

}

func (mesh *TriMesh) getYLowHigh(tri *Tri) (low *vec.V3, high *vec.V3) {
	a := mesh.verts[tri.vi[0]]
	b := mesh.verts[tri.vi[1]]
	c := mesh.verts[tri.vi[2]]

	yh := a
	if b.p.Y > yh.p.Y {
		yh = b
	}
	if c.p.Y > yh.p.Y {
		yh = c
	}

	yl := a
	if b.p.Y < yl.p.Y {
		yl = b
	}
	if c.p.Y < yl.p.Y {
		yl = c
	}

	return &yl.p, &yh.p

}

func (mesh *TriMesh) ScorchedAt(tri *Tri, p vec.V3) bool {
	if tri.childCount == 0 {
		return tri.Scorched //reached a leaf - return sorched value
	} else {
		//for _, c := range t.children {
		for i := 0; i < tri.childCount; i++ {
			//if c.prismFaces[5].Contains(p) {
			if tri.children[i].contains2D(p, mesh) {
				return mesh.ScorchedAt(tri.children[i], p)
			}
		}
		return false
	}
}

func (mesh *TriMesh) shoreLines(tri *Tri, levels []float64, snapped *int) {
	//snap the lowest vert of triangles spanning the waterline(s) to the waterline
	//note verts are pointers to vecs

	if tri.childCount == 0 {

		yl, yh := mesh.getYLowHigh(tri)

		for _, wl := range levels {
			if yl.Y < wl && yh.Y > wl {

				//f := (wl - yl.Y) / (yh.Y - yl.Y)
				//yl.TweenInto(yl, yh, f) //move this vertex to the waterline
				yl.Y = wl

				if yl.Y == yh.Y {
					log.Logit("hh??")
				}
				*snapped++
				break
			}
		}

	} else {
		//for _, c := range t.children {
		for i := 0; i < tri.childCount; i++ {
			mesh.shoreLines(tri.children[i], levels, snapped)
		}
	}
}

func (lm *TriMesh) ToSimpleMesh(leaves *LeafCollector, id uint16, fm *TriMesh, material string, asWater bool, faceTest func(face *LeafTri, mesh *TriMesh) bool, camPos vec.V3) *mesh.SimpleMesh {

	vc := lm.VertexCount

	if vc >= math.MaxUint16 {
		panic("mesh too big - over 65535 verts")
	}

	allNormals := lm.getNormals(asWater, leaves.TouchedVerts) //we must get normals (becuase it calculates them) before updating UVx's

	viewpoint := camPos.Clone()
	viewpoint.Y += 5

	prober := NewProber(leaves.DeviceId, lm, viewpoint, true)

	wp := uint32(0)
	fis := make([]uint16, 65535*3)                   //there will actually be many less faces than verts - but we need 3 uints per face
	leaves.getFinalFaces(fis, &wp, faceTest, prober) //populate Fis (recursivley from the root triangle)
	fis = fis[:wp]
	numLeaves := wp //truncate at the write pointer

	//kill two birds with one stone - generate a subset of verts just for the face sets - and set all their Y's

	np := make([]float32, lm.VertexCount*3)  // new position
	nn := make([]float32, lm.VertexCount*3)  // new normals
	nuv := make([]float32, lm.VertexCount*2) // new Uvs

	mapping := make(map[uint16]uint16) //map from old vert index to new vert index

	for i := range numLeaves {
		fi := fis[i]
		tfi, present := mapping[fi]
		if !present {
			v := lm.verts[fi]
			n := allNormals[fi]
			v.uv.UpdateUVxFromNormal(n)

			ni := uint16(len(mapping))
			ni2 := ni * 2 //new index (for uv)
			ni3 := ni * 3 //new index (for normal/pos)

			if asWater {
				nn[ni3], nn[ni3+1], nn[ni3+2] = 0, 1, 0
			} else {

				nn[ni3], nn[ni3+1], nn[ni3+2] = float32(n.X), float32(n.Y), float32(n.Z) //float32(v.n.X), float32(v.n.Y), float32(v.n.Z)
			}

			nuv[ni2], nuv[ni2+1] = float32(v.uv.X), float32(v.uv.Y)

			np[ni3] = float32(v.p.X)
			if asWater {
				np[ni3+1] = float32(v.wl)
			} else {
				np[ni3+1] = float32(v.p.Y)
			}
			np[ni3+2] = float32(v.p.Z)

			mapping[fi] = ni

			fis[i] = ni //update the face index to point to the new vert index
		} else {
			fis[i] = uint16(tfi) //update the face index to point to the new vert index
		}
	}

	if len(mapping) > 60000 {
		panic("large mesh:" + fmt.Sprint(len(mapping)) + "verts for mesh" + fmt.Sprint(id))
	}
	np = np[0 : len(mapping)*3] //truncate to actual size
	nn = nn[0 : len(mapping)*3]
	nuv = nuv[0 : len(mapping)*2]

	log.Logit(lm.VertexCount, "verts reduced to", len(mapping), "for mesh", id)

	//normals := lm.getNormals(asWater) //we must get normals (becuase it calculates them) before updating UVx's
	//lm.updateUVxsFromNormals()

	//return mesh.NewFilledSimpleMesh(id, lm.getPositions(asWater), normals, lm.getUVs(), fis, material)
	return mesh.NewFilledSimpleMesh(id, np, nn, nuv, fis, material)

}

func (leaves *LeafCollector) getFinalFaces(fis []uint16, wp *uint32, test func(face *LeafTri, m *TriMesh) bool, prober *Prober) {

	for i := range leaves.LeafCount {
		leaf := leaves.leaves[i]

		if leaf.childCount != 0 {
			panic("There should be no leaves with children here")
		}

		if test(leaf, prober.mesh) {
			if !leaf.occludedOrBackFacing(prober) {

				fis[*wp] = uint16(leaf.vi[0])
				fis[*wp+1] = uint16(leaf.vi[1])
				fis[*wp+2] = uint16(leaf.vi[2])
				*wp += 3
			}
		}

	}
}

func (m *TriMesh) Flood(wl float64) {

	for i := 0; i < m.VertexCount; i++ {
		v := m.verts[i]
		if v.p.Y <= wl {
			v.wl = wl

		} else {
			v.wl = -100000 //high and dry
		}
	}
}

func (m *TriMesh) Rain(rainFall float64) {
	for _, v := range m.verts {
		v.wl += rainFall
	}
}

// func (m *TriMesh) drain(v *vert, wl float64, count *int) {

// 	deep := -1000000.0
// 	for f := range v.touches {

// 		if f.childCount == 0 {
// 			a := m.verts[f.vi[0]]
// 			b := m.verts[f.vi[1]]
// 			c := m.verts[f.vi[2]]

// 			if a != v && a.wl == wl {
// 				a.wl = deep
// 				*count++
// 				m.drain(a, wl, count)
// 			}
// 			if b != v && b.wl == wl {
// 				b.wl = deep
// 				*count++
// 				m.drain(b, wl, count)
// 			}
// 			if c != v && c.wl == wl {
// 				c.wl = deep
// 				*count++
// 				m.drain(c, wl, count)
// 			}
// 		}

// 	}
// }

func (a *vert) flow(b *vert) {

	//if diff < 0 {
	//rate := float64(1) //free flow - water is above ground at both ends

	//surfaceWater := a.p.wl - m.p.y
	// if a.wl < a.p.y {
	// 	rate -= .3
	// } //ground percolation
	// if b.wl < b.p.y {
	// 	rate -= .3
	// }

	// if b.wl < b.p.Y {
	// 	rate *= .1
	// } //ground percolation

	//use an accumulator per vertex for the in/out flow

	//amount := diff  * rate

	if a.wl > a.p.Y || b.wl > b.p.Y {

		diff := a.wl - b.wl //uses the absolute water level
		a.wl -= diff * .3
		b.wl += diff * .3

		// if diff < 0 {
		// 	diff = -diff
		// }

		// if a.wl >= b.wl {
		// 	//a is high
		// 	a.acc -= diff //* .8
		// 	//b.acc += diff * .2
		// } else {
		// 	b.acc -= diff //* 0.8
		// 	//a.acc += diff * 0.2
		// }

		// a.incount++
		// b.incount++
	}

}

// Probes a triMesh with a ray - finding leaf triangle hits
type Prober struct {
	DeviceId      uint32
	mesh          *TriMesh
	votc          map[uint32]bool //vertex occlusion test cache
	ray           ray.Ray
	earlyExit     bool //set to true to exit on first hit
	Hit           bool //used in occlusiuon cull
	leafHits      int
	leafMisses    int
	prismHits     int
	prismMisses   int
	skips         int
	nearestHit    vec.V3
	NearestTri    *Tri
	skippedCulled int
	maxDepth      int
	SDist         float64 //smallest distance found sofar (start big) - used if ProbeAll()
}

func NewProber(deviceId uint32, mesh *TriMesh, camPos vec.V3, earlyExit bool) *Prober {
	return &Prober{
		DeviceId:  deviceId,
		ray:       ray.New(camPos, nowhereSpecial),
		mesh:      mesh,
		earlyExit: earlyExit,
		SDist:     math.MaxFloat64,
		votc:      make(map[uint32]bool), //vertex occlusion test cache TODO reuse/clear (place on device)
	}
}

func (S *Prober) String() string {
	return fmt.Sprintf(`
		leaf hits: %d
		leaf misses: %d
		prism hits: %d
		prism misses: %d 
		skips over/unders: %d 
		nearest hit dist: %.2f
		skipped culled: %d
		`,
		S.leafHits,
		S.leafMisses,
		S.prismHits,
		S.prismMisses,
		S.skips,
		S.SDist,
		S.skippedCulled,
	)

}
