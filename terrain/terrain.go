package terrain

import (
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/mesh"
	"github.com/nickax/gofu/ray"
	"github.com/nickax/gofu/vec"
	"math"
)

type TriMesh struct {
	name         string
	Root         *Tri
	verts        []*vert //{}
	nextFreeVert uint32
	fi           []uint32 //{} //face indices

	midpoints map[uint64]uint32 //compound key of the two endpoints of an edge, map contains the index of its midpoint vertex

	size     float64   //+/- land size
	kinks    []float64 //fractions of the land height to pull down by at each level
	height   float64   //+/- land height
	flooding bool      // for debug (concurrency check)
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

// return the normals of the verts specified in vis (vertices we've added)
func (m *TriMesh) getNormals(asWater bool) []float32 {

	vc := uint32(len(m.verts))
	//for every new vertex, reset the normal to zero, then add the normals of the faces it touches
	n := make([]float32, vc*3) //position x,y,z

	if asWater {
		for i := range m.verts {
			n[i*3+0] = 0
			n[i*3+1] = 1
			n[i*3+2] = 0
		}
		return n

	}
	//big vertex index (in the huge mesh) to small vertex index (in the new sub mesh)
	for i, v := range m.verts {
		//for i, v := range m.verts {

		if len(v.touches) > 0 {

			v.n.X, v.n.Y, v.n.Z = 0, 0, 0 //reuse (don't re-allocate)

			//usually 6 0- can be 5 - or even 3 at edges and 1 in conrners
			for t := range v.touches { //for every face this vertex touches
				if t.childCount == 0 { //ony include bottom level traingles in the normal calculation
					v.n.AddIn(t.Normal)
				}
			}

			vn := v.n.Normalise()
			n[i*3] = float32(vn.X)
			n[i*3+1] = float32(vn.Y)
			n[i*3+2] = float32(vn.Z)

		} else {
			log.Logit("vert", i, "of", m.name, " touches no tris")
		}
	}

	return n

}

func (mesh *TriMesh) Reset() {

	clear(mesh.midpoints) //empties the map preserving capacity

	for i, v := range mesh.verts {
		clear(v.touches)
		if i < 3 {
			v.wl = 0
			v.occluded = false
			v.testedForOcclusion = false
		}
	}

	mesh.nextFreeVert = 3 //beacuse we start again at the root triangle
	mesh.Root.reset()

	//v.wl

}
func NewTriMesh(name string, maxFaces uint16, size float64, kinks []float64, height float64) *TriMesh {

	//we need three entries per face in fis

	//l := int(maxFaces) * 3
	//fis := make([]uint32, l)
	//return &landMesh{name: name, verts: []*vert{}, fi: fis, midpoints: make(map[uint64]uint32, 0), splits: splits, height: height, size: size, kinks: kinks}
	verts := make([]*vert, 0, 65000) //clear the verts
	mesh := &TriMesh{name: name, verts: verts, midpoints: make(map[uint64]uint32, maxFaces*3), size: size, kinks: kinks, height: height}

	mesh.addVert(vec.NewVec3(size, 0, -size), 0, 0)  //near right
	mesh.addVert(vec.NewVec3(-size, 0, -size), 0, 0) //near left
	mesh.addVert(vec.NewVec3(0, 0, size), 0, 0)      //far, far away

	mesh.Root = newTri(nil, mesh, 0, 1, 2) //make the root triangle
	return mesh                            //&TriMesh{name: name, Root: root, verts: []*vert{}, midpoints: make(map[uint64]uint32, maxFaces*3), size: size, kinks: kinks, height: height}
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
func (m *TriMesh) splitEdge(a, b uint32, dy float64, depth int) uint32 {

	if a == b {
		panic("degenerate edge in splitEdge")
	}
	pa := m.verts[a].p
	pb := m.verts[b].p

	p := pa.Tween(pb, 0.5)

	p.Y += dy

	//maintain a map of edges to midpoints
	// key := uint32(a) + uint32(65536)*uint32(b)
	// existingMid, present := m.midpoints[key]

	vi := m.midpoint(a, b) //will look (in a map) for an existing midpoints a->b or b->a

	if vi == a || vi == b {
		panic("midpoint is one of the endpoints")
	}

	if vi != math.MaxUint32 {
		if vi == 0 {
			log.Logit("zero midpoint")
		}
		return vi
	}

	// if len(m.reuseVerts) > 0 {
	// 	vi = m.reuseVerts[0]
	// 	log.Logit("reusing vert", vi)
	// 	m.verts[vi].p = p
	// 	m.reuseVerts = m.reuseVerts[1:]
	// } else {

	//for the first few levels of splitting, calculate UVs from altitude (so large scale features are consistent)
	uvy := (p.Y + m.height) / (2 * m.height)
	if depth > 6 {
		//later splits, preserve ealier UVs (so fine geometry detail does not cause texture popping)
		uvy = (m.verts[a].uv.GetY() + m.verts[b].uv.GetY()) / 2
	}

	//we don't have normals yet - so must calc UVX's later
	vi = m.addVert(p, 0, uvy) //OrReuseVertAtXZ(p)

	//}

	//add it to the index of midpoints
	key := uint64(a) + uint64(math.MaxUint32)*uint64(b)
	m.midpoints[key] = vi

	return vi
}

func (m *TriMesh) addVert(p *vec.V3, u float64, v float64) uint32 {

	var vert *vert

	// Check if we have an existing vert to reuse
	if m.nextFreeVert < uint32(len(m.verts)) {
		vert = m.verts[m.nextFreeVert]
		// Overwrite existing data
		// Note: We assume p is a new vector or we copy it.
		// If p is a pointer to a vector that changes, we might need p.Clone() or *vert.p = *p
		vert.p = p
		vert.uv.X = u
		vert.uv.Y = v
		vert.occluded = false
		vert.testedForOcclusion = false
		vert.wl = 0
		//vert.touches = make(map[*Tri]bool, 6)

		//vert.incount = 0
		// vert.touches is already cleared in Reset()
	} else {
		// No free verts, allocate new one and append
		vert = newVert(p, u, v)
		m.verts = append(m.verts, vert)
	}

	idx := uint32(m.nextFreeVert)
	m.nextFreeVert++
	return idx
}

// create a shore by dropping the land vertex to the level of
// func (m *TriMesh) flowWater(fis []uint16) {

// 	//flow the water (only along the deepest/visible faces)
// 	//for iw := 0; iw < 10; iw++ { //iteration of water
// 	for i := 0; i < len(fis); i += 3 {
// 		a := m.verts[fis[i]]
// 		b := m.verts[fis[i+1]]
// 		c := m.verts[fis[i+2]]
// 		flow(a, b) //update the accumulators at each vertex
// 		flow(b, c)
// 		flow(c, a)
// 	}
// 	//update the water levels (from the accumulators)
// 	for _, v := range m.verts { //note m.verts is a slice of pointers to verts - so we *can* mutate them

// 		v.wl += v.acc / float64(v.incount)
// 		v.acc = 0
// 		v.incount = 0
// 	}

// }

func (m *TriMesh) updateUVxsFromNormals() {
	for _, v := range m.verts {
		v.uv.UpdateUVxFromNormal(v.n)
	}

}

func (m *TriMesh) VertCount() uint32 {

	return uint32(m.nextFreeVert)

	//return uint32(len(m.verts))
}

func (m *TriMesh) getUVs() []float32 {

	vc := uint32(len(m.verts))
	uvs := make([]float32, vc*2)

	//big vertex index (in the huge mesh) to small vertex index (in the new sub mesh)
	for i, v := range m.verts {
		uvs[i*2] = float32(v.uv.GetX())
		uvs[i*2+1] = float32(v.uv.GetY())
	}

	return uvs

}

func (m *TriMesh) getPositions(asWater bool) []float32 {

	vc := uint32(len(m.verts))
	p := make([]float32, 0, vc*3) //position x,y,z

	for i := 0; i < int(m.nextFreeVert); i++ { //, j := range m.verts {
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

// FloodAndDrain - adds the water surface meshes (to the msg)
func (land *TriMesh) FloodAndDrain(waterlines []float64, response *msg.Msg) {

	if land.flooding {
		panic("concurrent flooding detected!")
	}
	defer func() { land.flooding = false }()
	land.flooding = true

	//snapped := 0
	//land.Root.shoreLines(waterlines, &snapped) //snaps the lowest vert of triangles spanning the waterline(s) to the waterline

	//snapped = 0
	//land.Root.shoreLines(waterlines, &snapped) //snaps the lowest vert of triangles spanning the waterline(s) to the waterline

	for i, wl := range waterlines { //work DOWN through the waterlines
		land.Flood(wl) //set the waterlevel of all land below wl to wl (anything above is set to wl=-1000000

		if i != len(waterlines)-1 { //Don't drain the sea
			for lake := 0; lake < 10; lake++ {
				drained := false
				for _, v := range land.verts {
					if v.wl > v.p.Y+300 {
						count := 0
						land.drain(v, wl, &count) //all deep lakes at this WL are drained to -1000000

						log.Logit("drained", count)
						drained = true

						break
					}
				}
				if !drained {
					log.Logit("no more lakes to drain")
					break
				}
			}
		}

		funcIsWater := func(t *Tri, m *TriMesh) bool { return t.OnOrUnderWater(m) }
		waterMesh := land.ToSimpleMesh(land.Root, uint16(4+i), nil, "water", true, funcIsWater)

		log.Logit("water mesh at wl", wl, " has ", waterMesh.FaceCount(), " faces and ", waterMesh.VertCount(), " verts")
		waterMesh.WriteTo(response, 1)

	}

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

	return yl.p, yh.p

}

func (mesh *TriMesh) scorchedAt(tri *Tri, p *vec.V3) bool {
	if tri.childCount == 0 {
		return tri.Scorched //reached a leaf - return sorched value
	} else {
		//for _, c := range t.children {
		for i := 0; i < tri.childCount; i++ {
			//if c.prismFaces[5].Contains(p) {
			if tri.children[i].contains2D(p) {
				return mesh.scorchedAt(tri.children[i], p)
			}
		}
		return false
	}
}

func (landMesh *TriMesh) FetchTrees(tri *Tri, depth int, positions []float32, billBoardMesh *mesh.SimpleMesh, camPos *vec.V3, camDir *vec.V3, ray *ray.Ray, hidden *int) {

	mid := tri.centre
	tcs := mesh.NewTcs(0, 1, 1, 0)

	treeTop := vec.NewVec3(0, 0, 0) //
	if tri.Depth == depth {
		if !landMesh.scorchedAt(landMesh.Root, mid) && !tri.OnOrUnderWater(landMesh) {

			toTree := mid.Sub(camPos).Normalise()
			dotProd := toTree.Dot(camDir)

			if mid.DistanceFrom(camPos) < 100 {
				if dotProd > -0.2 { //trees in front of, or somewhat behind the camera
					treeTop.SetFrom(mid)
					treeTop.Y += 3

					ray.PointAt(treeTop)

					//check for occlusion (by the triangle it stands on)
					//TODO - check against whole landscape (although these are nearby trees)
					//NOTE Set an occluded flag on triangles and set once (check high and Low) - if the high point is occluded - no need to check low
					//if tri.prismFaces[5].Probe(ray) {
					if tri.poly.Probe(ray) {
						*hidden++
						return
					}

					//place a (instanced mesh) tree here
					positions = append(positions, mid.AsFloat32s()...)
				}
			} else { //it's a faraway tree - only place it if the ground slopes towards the camera
				if dotProd > .25 { //trees generally in front of the camera}
					if toTree.Dot(tri.Normal) < 0 { //if the triangle slopes towards camera
						billBoardMesh.Billboard(mid, up, camPos, 20, 20, 20, 4, tcs) //billboard tree
					}

				}
			}
		}
	} else {
		//for _, c := range t.children {
		for i := 0; i < tri.childCount; i++ {
			landMesh.FetchTrees(tri.children[i], depth, positions, billBoardMesh, camPos, camDir, ray, hidden)
		}
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

func (lm *TriMesh) ToSimpleMesh(tri *Tri, id uint16, fm *TriMesh, material string, asWater bool, faceTest func(face *Tri, mesh *TriMesh) bool) *mesh.SimpleMesh {

	//vc := uint32(len(lm.verts)) //vertex count
	vc := uint32(len(lm.verts))  //vertex count
	fis := make([]uint16, vc*10) //there will actually be many less faces than verts - but we need 3 uints per face

	if vc >= math.MaxUint16 {
		log.Logit("mesh too big - over 65535 verts")
	}

	lm.getNormals(asWater) //we must get normals (becuase it calculates them) before updating UVx's

	wp := uint32(0)

	lm.getFacesInto(tri, fis, &wp, faceTest) //populate Fis (recursivley from the root triangle)
	fis = fis[:wp]                           //truncate at the write pointer

	//kill two birds with one stone - generate a subset of verts just for the face sets - and set all their Y's

	np := make([]float32, lm.VertCount()*3)  // new position
	nn := make([]float32, lm.VertCount()*3)  // new normals
	nuv := make([]float32, lm.VertCount()*2) // new Uvs

	mapping := make(map[uint16]uint16) //map from old vert index to new vert index

	for i := 0; i < int(wp); i++ {
		fi := fis[i]
		tfi, present := mapping[fi]
		if !present {
			v := lm.verts[fi]
			v.uv.UpdateUVxFromNormal(v.n)

			ni := uint16(len(mapping))
			ni2 := ni * 2 //new index (for uv)
			ni3 := ni * 3 //new index (for normal/pos)

			if asWater {
				nn[ni3], nn[ni3+1], nn[ni3+2] = 0, 1, 0
			} else {
				nn[ni3], nn[ni3+1], nn[ni3+2] = float32(v.n.X), float32(v.n.Y), float32(v.n.Z)
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

	np = np[0 : len(mapping)*3] //truncate to actual size
	nn = nn[0 : len(mapping)*3]
	nuv = nuv[0 : len(mapping)*2]

	log.Logit(lm.VertCount(), "verts reduced to", len(mapping), "for mesh", id)

	//normals := lm.getNormals(asWater) //we must get normals (becuase it calculates them) before updating UVx's
	//lm.updateUVxsFromNormals()

	//return mesh.NewFilledSimpleMesh(id, lm.getPositions(asWater), normals, lm.getUVs(), fis, material)
	return mesh.NewFilledSimpleMesh(id, np, nn, nuv, fis, material)

}

func (mesh *TriMesh) getFacesInto(tri *Tri, fis []uint16, p *uint32, test func(face *Tri, m *TriMesh) bool) {
	if tri.childCount == 0 {
		j := *p
		if test(tri, mesh) {
			fis[j] = uint16(tri.vi[0])
			fis[j+1] = uint16(tri.vi[1])
			fis[j+2] = uint16(tri.vi[2])
			*p += 3
		} else {
			//log.Logit("tri rejected at depth", t.Depth)
		}
	}
	//for _, c := range t.children {
	for i := 0; i < tri.childCount; i++ {
		mesh.getFacesInto(tri.children[i], fis, p, test)
	}
}

func (m *TriMesh) Flood(wl float64) {
	for _, v := range m.verts {
		if v.p.Y <= wl {
			v.wl = wl

		} else {
			v.wl = -100000 //high and dry
		}
	}

}

func (m *TriMesh) drain(v *vert, wl float64, count *int) {

	deep := -1000000.0
	for f := range v.touches {

		if len(f.children) == 0 {
			a := m.verts[f.vi[0]]
			b := m.verts[f.vi[1]]
			c := m.verts[f.vi[2]]

			if a != v && a.wl == wl {
				a.wl = deep
				*count++
				m.drain(a, wl, count)
			}
			if b != v && b.wl == wl {
				b.wl = deep
				*count++
				m.drain(b, wl, count)
			}
			if c != v && c.wl == wl {
				c.wl = deep
				*count++
				m.drain(c, wl, count)
			}
		}

	}
}

// func flow(a *vert, b *vert) {

// 	//if diff < 0 {
// 	//rate := float64(1) //free flow - water is above ground at both ends

// 	//surfaceWater := a.p.wl - m.p.y
// 	// if a.wl < a.p.y {
// 	// 	rate -= .3
// 	// } //ground percolation
// 	// if b.wl < b.p.y {
// 	// 	rate -= .3
// 	// }

// 	// if b.wl < b.p.Y {
// 	// 	rate *= .1
// 	// } //ground percolation

// 	//use an accumulator per vertex for the in/out flow

// 	//amount := diff  * rate

// 	if a.wl > a.p.Y || b.wl > b.p.Y {
// 		diff := a.wl - b.wl //uses the absolute water level
// 		if diff < 0 {
// 			diff = -diff
// 		}

// 		if a.wl >= b.wl {
// 			//a is high
// 			a.acc -= diff //* .8
// 			//b.acc += diff * .2
// 		} else {
// 			b.acc -= diff //* 0.8
// 			//a.acc += diff * 0.2
// 		}

// 		a.incount++
// 		b.incount++
// 	}

// }
