package terrain

import (
	"math"

	// "github.com/nickax/gofu/game/msg"
	// "github.com/nickax/gofu/log"
	"github.com/nickax/gofu/poly"
	"github.com/nickax/gofu/ray"
	"github.com/nickax/gofu/vec"
)

type Collector struct {
	triangles  []*Tri
	plants     []Plant
	TriCount   int
	PlantCount int
}

type LeafTri struct {
	vi         [3]uint32
	children   [2]*LeafTri
	childCount int
	normal     vec.V3
	Scorched   bool
	centre     vec.V3
	poly       *poly.ConvexPoly // made/cached JIT
}

func (lt *LeafTri) ToConvexPoly(mesh *TriMesh) *poly.ConvexPoly {
	a := mesh.verts[lt.vi[0]].p
	b := mesh.verts[lt.vi[1]].p
	c := mesh.verts[lt.vi[2]].p
	return poly.NewConvexPolyFromVecs([]vec.V3{a, b, c})
}

func (lt *LeafTri) Probe(ray ray.Ray, mesh *TriMesh) (bool, vec.V3) {

	if lt.poly == nil {
		lt.poly = lt.ToConvexPoly(mesh) //cache it
	}

	hit, where := lt.poly.Probe(ray)

	if hit {
		if lt.childCount == 0 {
			return true, where
		}

		for i := range lt.childCount {
			childHit, childWhere := lt.children[i].Probe(ray, mesh)
			if childHit {
				return true, childWhere
			}
		}
		return hit, where //return the hit on the parent if we somehow missed all children
	}

	return false, vec.NewVec3(0, 0, 0)
}

func NewLeafTri(a, b, c uint32, scorched bool) *LeafTri {

	return &LeafTri{vi: [3]uint32{a, b, c}, normal: vec.NewVec3(0, 0, 0), Scorched: scorched}
}

func (leaf *LeafTri) CacheCentre(mesh *TriMesh) vec.V3 {

	if leaf.centre.X != 0 || leaf.centre.Y != 0 || leaf.centre.Z != 0 {
		return leaf.centre
	}
	leaf.centre = (mesh.verts[leaf.vi[0]].p.Add(mesh.verts[leaf.vi[1]].p).Add(mesh.verts[leaf.vi[2]].p)).Multiply(1 / 3.0)
	return leaf.centre
}

type LeafCollector struct {
	DeviceId     uint32
	TouchedVerts *Contacts
	leaves       []*LeafTri
	LeafCount    int
}

func NewLeafCollector(tris int, deviceId uint32) *LeafCollector {
	return &LeafCollector{LeafCount: 0, leaves: make([]*LeafTri, tris), TouchedVerts: NewContacts(tris), DeviceId: deviceId}
}

func (lc *LeafCollector) Reset() {
	lc.LeafCount = 0
	clear(lc.leaves)
}

func (lc *LeafCollector) AddLeaf(t *LeafTri) {
	lc.leaves[lc.LeafCount] = t
	lc.LeafCount++
}

func NewCollector(tris, plants int) *Collector {
	return &Collector{TriCount: 0, PlantCount: 0, triangles: make([]*Tri, tris), plants: make([]Plant, plants)}
}

func (c *Collector) BubbleVerticalExtentsFromLeaves(mesh *TriMesh) {

	for i := 0; i < c.TriCount; i++ {
		tri := c.triangles[i]
		if tri.childCount == 0 {
			tri.BubbleVerticalExtents()
		}
	}
}

func (c *Collector) CheckPrisms() {

	for i := 0; i < c.TriCount; i++ {
		t := c.triangles[i]
		if t.childCount > 0 {
			if t.PrismFaces[0] == nil {
				panic("prism face missing")
			}
		}
	}
}

func (c *Collector) MakePrisms(mesh *TriMesh) {

	for i := 0; i < c.TriCount; i++ {
		t := c.triangles[i]
		if t.childCount > 0 {
			t.MakePrism(mesh)
		}
	}

}

// func (c *Collector) Flood(mesh *TriMesh, waterLevel float64) {
// 	for i := 0; i < c.TriCount; i++ {
// 		v := mesh.verts[c.triangles[i].vi[0]]
// 		if v.p.Y <= waterLevel {
// 			v.wl = waterLevel

// 		} else {
// 			v.wl = -100000 //high and dry
// 		}
// 	}
// }

// FloodAndDrain - adds the water surface meshes (to the msg)
// func (tris *Collector) FloodAndDrain(land *TriMesh, waterlines []float64, response *msg.Msg, campos vec.V3) {
// 	log.Logit("flood and drain", tris.TriCount)
// 	if land.flooding {
// 		panic("concurrent flooding detected!")
// 	}
// 	defer func() { land.flooding = false }()
// 	land.flooding = true

// 	//snapped := 0
// 	//land.Root.shoreLines(waterlines, &snapped) //snaps the lowest vert of triangles spanning the waterline(s) to the waterline

// 	//snapped = 0
// 	//land.Root.shoreLines(waterlines, &snapped) //snaps the lowest vert of triangles spanning the waterline(s) to the waterline

// 	for _, v := range land.verts {
// 		v.wl = -5000 //reset all waterlevels
// 	}

// 	for i, wl := range waterlines { //work DOWN through the waterlines

// 		//land.Flood(wl) //set the waterlevel of all land below wl to wl (anything above is set to wl=-1000000
// 		tris.Flood(land, wl) //MAYBE AT FAULT ?

// 		if i != len(waterlines)-1 { //Don't drain the sea
// 			for lake := 0; lake < 10; lake++ {
// 				drained := false
// 				for _, v := range land.verts {
// 					//for j := 0; j < tris.TriCount; j++ {
// 					//	v := land.verts[tris.triangles[j].vi[0]]
// 					if v.wl > v.p.Y+300 {
// 						count := 0
// 						//land.drain(v, wl, &count) //all deep lakes at this WL are drained to -1000000
// 						drain(land, v, wl, &count)

// 						log.Logit("drained", count, "verts at ", wl)
// 						drained = true

// 						break
// 					}
// 				}
// 				if !drained {
// 					log.Logit("no more lakes to drain at wl", wl)
// 					break
// 				}
// 			}
// 		}

// 		funcIsWater := func(t *Tri, m *TriMesh) bool { return t.OnOrUnderWater(m) }
// 		waterMesh := land.ToSimpleMesh(tris, uint16(4+i), nil, "water", true, funcIsWater, campos)

// 		if waterMesh.FaceCount() > 0 {
// 			log.Logit("water mesh at wl", wl, " has ", waterMesh.FaceCount(), " faces and ", waterMesh.VertCount(), " verts")
// 			waterMesh.WriteTo(response, 1)
// 		} else {
// 			log.Logit("water mesh at wl", wl, i, " is empty")
// 		}

// 	}

// }

// func drain(m *TriMesh, v *vert, waterLevel float64, count *int) {

// 	deep := -5000.0
// 	if len(v.touches) == 0 {
// 		panic("vert touches no faces")
// 	}

// 	for f := range v.touches {

// 		if f.childCount == 0 {
// 			a := m.verts[f.vi[0]]
// 			b := m.verts[f.vi[1]]
// 			c := m.verts[f.vi[2]]

// 			if a != v && a.wl == waterLevel {
// 				a.wl = deep
// 				*count++
// 				drain(m, a, waterLevel, count)
// 			}
// 			if b != v && b.wl == waterLevel {
// 				b.wl = deep
// 				*count++
// 				drain(m, b, waterLevel, count)
// 			}
// 			if c != v && c.wl == waterLevel {
// 				c.wl = deep
// 				*count++
// 				drain(m, c, waterLevel, count)
// 			}
// 		}

// 	}
// }

func (leaf *LeafTri) PatchInto(final *LeafCollector, mesh *TriMesh, contacts *Contacts) {

	for j := range 3 {
		ai := leaf.vi[j]
		bi := leaf.vi[(j+1)%3]
		ci := leaf.vi[(j+2)%3]

		mi := mesh.midpoint(bi, ci) //looks both ways for a midpoint

		if mi != math.MaxUint32 { //is there a midpoint(on the opposite edge) ?
			leaf.splitIn2(mesh, contacts, ai, bi, ci, mi)
			a := leaf.children[0]
			b := leaf.children[1]
			a.PatchInto(final, mesh, contacts)
			b.PatchInto(final, mesh, contacts)
			return
		}
	}

	//no midpoints found - this is a final triangle
	if leaf.childCount > 0 {
		panic("leaf with children being added to final")
	}
	final.AddLeaf(leaf)

}

// PatchTriangles - Bridges adjoining depths - splitting triangles in two (sometimes recursively)
func (c *LeafCollector) PatchTriangles(mesh *TriMesh) *LeafCollector {

	final := NewLeafCollector(c.LeafCount*3, c.DeviceId)

	for i := range c.LeafCount {

		tri := c.leaves[i]
		tri.addTo(c.TouchedVerts)

		tri.PatchInto(final, mesh, c.TouchedVerts)

	}

	return final
}

func (c *Collector) Reset() {
	c.TriCount = 0
	c.PlantCount = 0
	//clear (c.plants) clearing plants is unnecessary - they are values not pointers
	clear(c.triangles)

}

func (c *Collector) AddTri(t *Tri) {
	c.triangles[c.TriCount] = t
	c.TriCount++
}

func (c *Collector) AddPlants(p []Plant) {
	for _, pl := range p {
		c.plants[c.PlantCount] = pl
		c.PlantCount++
	}
}
