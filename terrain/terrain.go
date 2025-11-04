package terrain

import (
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/vec"
	"math"
)

type TriMesh struct {
	name  string
	Root  *Tri
	verts []*vert  //{}
	fi    []uint32 //{} //face indices

	midpoints map[uint64]uint32 //compound key of the two endpoints of an edge, map contains the index of its midpoint vertex

	size   float64   //+/- land size
	kinks  []float64 //fractions of the land height to pull down by at each level
	height float64   //+/- land height

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
			v.n = vec.NewVec3(0, 0, 0)
			//usually 6 0- can be 5 - or even 3 at edges and 1 in conrners
			for t := range v.touches { //for every face this vertex touches
				if len(t.children) == 0 { //ony include bottom level traingles in the normal calculation
					v.n.AddIn(t.Normal)
				}
			}

			vn := v.n.Normalise()
			n[i*3] = float32(vn.GetX())
			n[i*3+1] = float32(vn.GetY())
			n[i*3+2] = float32(vn.GetZ())

		} else {
			log.Logit(v, " touches no tris")
		}
	}

	return n

}

func NewTriMesh(name string, maxFaces uint16, size float64, kinks []float64, height float64) *TriMesh {

	//we need three entries per face in fis

	//l := int(maxFaces) * 3
	//fis := make([]uint32, l)
	//return &landMesh{name: name, verts: []*vert{}, fi: fis, midpoints: make(map[uint64]uint32, 0), splits: splits, height: height, size: size, kinks: kinks}
	verts := make([]*vert, 0, 65000) //clear the verts
	mesh := &TriMesh{name: name, verts: verts, midpoints: make(map[uint64]uint32, maxFaces*3), size: size, kinks: kinks, height: height}
	mesh.addVert(vec.NewVec3(0, 0, size), 0, 0) //the height of the first vertex is the initial seed for the entire land
	mesh.addVert(vec.NewVec3(size, 0, -size), 0, 0)
	mesh.addVert(vec.NewVec3(-size, 0, -size), 0, 0)

	root := newTri(nil, mesh, 0, 0, 1, 2)
	return &TriMesh{name: name, Root: root, verts: []*vert{}, midpoints: make(map[uint64]uint32, maxFaces*3), size: size, kinks: kinks, height: height}
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

	pa := m.verts[a].p
	pb := m.verts[b].p

	p := pa.Tween(pb, 0.5)

	p.SetY(p.GetY() + dy)

	//maintain a map of edges to midpoints
	// key := uint32(a) + uint32(65536)*uint32(b)
	// existingMid, present := m.midpoints[key]

	vi := m.midpoint(a, b) //will look (in a map) for an existing midpoints a->b or b->a
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
	uvy := (p.GetY() + m.height) / (2 * m.height)
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

	// if reUseVert {
	// 	for i, v := range m.verts { //todo - this is slow - use a map
	// 		if v.p.equals(p) {
	// 			return uint32(i)
	// 		}
	// 	}
	// }

	m.verts = append(m.verts, newVert(p, u, v))

	return uint32(len(m.verts) - 1)
}

// create a shore by dropping the land vertex to the level of
func (m *TriMesh) flowWater(fis []uint16) {

	//flow the water (only along the deepest/visible faces)
	//for iw := 0; iw < 10; iw++ { //iteration of water
	for i := 0; i < len(fis); i += 3 {
		a := m.verts[fis[i]]
		b := m.verts[fis[i+1]]
		c := m.verts[fis[i+2]]
		flow(a, b) //update the accumulators at each vertex
		flow(b, c)
		flow(c, a)
	}
	//update the water levels (from the accumulators)
	for _, v := range m.verts { //note m.verts is a slice of pointers to verts - so we *can* mutate them

		v.wl += v.acc / float64(v.incount)
		v.acc = 0
		v.incount = 0
	}

}

func (m *TriMesh) updateUVxsFromNormals() {
	for _, v := range m.verts {
		v.uv.UpdateUVxFromNormal(v.n)
	}

}

func (m *TriMesh) VertCount() uint32 {
	return uint32(len(m.verts))
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

	for i, j := range m.verts {

		if asWater {
			v := []float32{float32(j.p.GetX()), float32(j.wl), float32(j.p.GetZ())}
			p = append(p, v...)
			p[i*3+1] = float32(j.wl) //y + j.wl*10) //water level * 10
		} else {
			p = append(p, j.p.AsFloat32s()...)
		}

	}
	return p

}

func (land *TriMesh) FloodAndDrain(waterlines []float64) []*msg.Msg {

	msgs := []*msg.Msg{}
	land.Root.shoreLines(waterlines) //snaps the lowest vert of triangles spanning the waterline(s) to the waterline

	for i, wl := range waterlines {
		land.flood(wl) //set the waterlevel of all land below this waterline

		if i != len(waterlines)-1 {
			for lake := 0; lake < 10; lake++ {
				drained := false
				for _, v := range land.verts {
					if v.wl > v.p.Y+300 {
						count := 0
						land.drain(v, wl, &count) //all deel lakes at this WL are drained to -1000000

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

		funcIsUnderwater := func(t *Tri) bool { return t.IsUnderwater(.1) }
		waterMesh := land.Root.ToSimpleMesh(uint16(4+i), land, nil, "water", true, funcIsUnderwater)

		msgs = append(msgs, waterMesh.ToMsg(1))

	}
	return msgs
}

func (tri *Tri) shoreLines(levels []float64) {
	//snap the lowest vert of triangles spanning the waterline(s) to the waterline
	//note verts are pointers to vecs
	if len(tri.children) == 0 {

		yl, yh := tri.getYLowHigh()

		for _, wl := range levels {
			if yl.GetY() < wl && yh.GetY() > wl {
				yl.SetY(wl)
				break
			}
		}

	} else {
		for _, c := range tri.children {
			c.shoreLines(levels)
		}
	}

}

func (m *TriMesh) flood(wl float64) {
	for _, v := range m.verts {
		if v.p.GetY() <= wl {
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

func flow(a *vert, b *vert) {

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

	if a.wl > a.p.GetY() || b.wl > b.p.GetY() {
		diff := a.wl - b.wl //uses the absolute water level
		if diff < 0 {
			diff = -diff
		}

		if a.wl >= b.wl {
			//a is high
			a.acc -= diff //* .8
			//b.acc += diff * .2
		} else {
			b.acc -= diff //* 0.8
			//a.acc += diff * 0.2
		}

		a.incount++
		b.incount++
	}

}
