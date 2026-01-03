package terrain

import (
	"math"

	// "github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/poly"
	"github.com/nickax/gofu/ray"
	"github.com/nickax/gofu/vec"
)

type TriCollector struct {
	triangles []*Tri
	TriCount  int
}

type LeafTri struct {
	vi         [3]uint32
	children   [2]*LeafTri
	childCount int
	normal     vec.V3
	Scorched   bool
	centre     vec.V3
	poly       *poly.ConvexPoly // made/cached JIT
	WaterPoly  *poly.ConvexPoly // made/cached JIT
}

func (lt *LeafTri) ToConvexPoly(mesh *TriMesh, asWater bool) *poly.ConvexPoly {

	v0 := mesh.verts[lt.vi[0]]
	v1 := mesh.verts[lt.vi[1]]
	v2 := mesh.verts[lt.vi[2]]

	a := v0.p
	b := v1.p
	c := v2.p

	if asWater {
		a.Y = v0.wl
		b.Y = v1.wl
		c.Y = v2.wl
	}
	return poly.NewConvexPolyFromVecs([]vec.V3{a, b, c})
}

func (lt *LeafTri) Probe(ray ray.Ray, mesh *TriMesh, water int) (bool, vec.V3, *LeafTri) {

	if lt.poly == nil {
		lt.poly = lt.ToConvexPoly(mesh, false) //cache it
	}
	if water == 1 {
		if lt.WaterPoly == nil {
			lt.WaterPoly = lt.ToConvexPoly(mesh, true) //cache it
		}
	}

	hit, where := lt.poly.Probe(ray)

	if hit {
		if lt.childCount == 0 {
			return true, where, lt
		}

		for i := range lt.childCount {
			child := lt.children[i]
			childHit, childWhere, childLeaf := child.Probe(ray, mesh, water)
			if childHit {
				return true, childWhere, childLeaf
			}
		}
		//log.Logit("hit parent but missed all children")
		//hit parent but missed all children - can/does happen when a leaf triangle is kinked
		return false, vec.NewVec3(0, 0, 0), nil

		//return hit, where, lt //return the hit on the parent if we somehow missed all children
	}

	return false, vec.NewVec3(0, 0, 0), nil
}

func NewLeafTri(a, b, c uint32, scorched bool, mesh *TriMesh) *LeafTri {

	//initialise normals  pointing down to indicate uncalculated
	lt := LeafTri{vi: [3]uint32{a, b, c}, normal: vec.NewVec3(0, 0, 0), Scorched: scorched}

	//lt.CacheNormal(mesh)
	// if lt.normal.Y < 0 {
	// 	panic("leaf tri with downward normal created")
	// }
	return &lt
}

func (leaf *LeafTri) CacheCentre(mesh *TriMesh) vec.V3 {

	if leaf.centre.X != 0 || leaf.centre.Y != 0 || leaf.centre.Z != 0 {
		return leaf.centre
	}
	leaf.centre = (mesh.verts[leaf.vi[0]].p.Add(mesh.verts[leaf.vi[1]].p).Add(mesh.verts[leaf.vi[2]].p)).Multiply(1 / 3.0)
	return leaf.centre
}

type LeafCollector struct {
	DeviceId  uint32
	leaves    []*LeafTri
	LeafCount int
}

func NewLeafCollector(tris int, deviceId uint32) *LeafCollector {
	return &LeafCollector{LeafCount: 0, leaves: make([]*LeafTri, tris), DeviceId: deviceId}
}

func (in *LeafCollector) Reset() {
	in.LeafCount = 0
	clear(in.leaves)
}

func (in *LeafCollector) AddLeaf(t *LeafTri) {
	in.leaves[in.LeafCount] = t
	in.LeafCount++
}

func NewTriCollector(tris int) *TriCollector {
	return &TriCollector{TriCount: 0, triangles: make([]*Tri, tris)}
}

func (c *TriCollector) BubbleVerticalExtentsFromLeaves(mesh *TriMesh) {

	for i := 0; i < c.TriCount; i++ {
		tri := c.triangles[i]
		if tri.childCount == 0 {
			tri.BubbleVerticalExtents()
		}
	}
}

func (c *TriCollector) CheckPrisms() {

	for i := 0; i < c.TriCount; i++ {
		t := c.triangles[i]
		if t.childCount > 0 {
			if t.PrismFaces[0] == nil {
				panic("prism face missing")
			}
		}
	}
}

func (c *TriCollector) MakePrisms(mesh *TriMesh) {

	for i := 0; i < c.TriCount; i++ {
		t := c.triangles[i]
		if t.childCount > 0 {
			t.MakePrism(mesh)
		}
	}

}

func (m *TriMesh) TextureX(tv *TouchedVerts) {

	for v := range m.verts {
		m.verts[v].uv.X = -1 //mark all as untextured
	}

	count := 0
	m.verts[0].uv.X = 0 //seed vert
	textureX(m, 0, tv, 0, &count)
	log.Logit("textured verts", count)
}

//call this on some central onscreen vert and it will wrap the texture x coords outwards from there based on the edge lengths in world space
func textureX(m *TriMesh, vi uint32, touchedVerts *TouchedVerts, tcx float64, count *int) {

	//touchedVerts is an array of maps - it gives us a list of all the leaf tris touching a vert
	touches := touchedVerts.touches[vi]

	v := m.verts[vi]

	pp := v.p
	for lt := range touches {

		if lt.childCount == 0 {
			for _, vti := range lt.vi {
				nv := m.verts[vti] //next vert

				if nv.uv.X == -1 {
					hop := pp.Sub(nv.p)
					hop.Y = 0 //project onto the xz plane

					dx := hop.Dot(lt.CacheNormal(m).Cross(vec.NewVec3(0, 1, 0)))
					tcx += dx / 400
					nv.uv.X = tcx
					*count++
					textureX(m, vti, touchedVerts, tcx, count) //recurse

				}
			}
		}

	}
}

func (leaf *LeafTri) PatchInto(camPos vec.V3, final *LeafCollector, mesh *TriMesh, touchedVerts *TouchedVerts, depth int) {

	for j := range 3 {
		ai := leaf.vi[j]
		bi := leaf.vi[(j+1)%3]
		ci := leaf.vi[(j+2)%3]

		mi := mesh.midpoint(bi, ci) //looks both ways for a midpoint

		if mi != math.MaxUint32 { //is there a midpoint(on the opposite edge) ?
			leaf.removeFrom(touchedVerts)
			leaf.splitIn2(mesh, ai, bi, ci, mi)
			a := leaf.children[0]
			b := leaf.children[1]
			a.addTo(touchedVerts)
			b.addTo(touchedVerts)

			a.PatchInto(camPos, final, mesh, touchedVerts, depth+1)
			b.PatchInto(camPos, final, mesh, touchedVerts, depth+1)
			return
		}
	}

	//no midpoints found - this is a final triangle
	if leaf.childCount > 0 {
		panic("leaf with children being added to final")
	}

	//backface culling (don't ever backface cull under water tris)
	camPos.Y += 3 //pretend we're 10 ft taller
	if leaf.OnOrUnderWater(mesh) || leaf.CacheCentre(mesh).Sub(camPos).Dot(leaf.CacheNormal(mesh)) <= 0 {
		final.AddLeaf(leaf)
	}

}

// PatchTriangles - Bridges adjoining depths - splitting triangles in two (sometimes recursively)
// iterates over top level leaves (bottom level triangle)
// also fills touchedVerts with all verts touched by 'final' patch triangles
func (in *LeafCollector) PatchTriangles(camPos vec.V3, mesh *TriMesh, out *LeafCollector, touchedVerts *TouchedVerts) {

	//	final := NewLeafCollector(c.LeafCount*3, c.DeviceId)

	for i := range in.LeafCount {

		leaf := in.leaves[i]     //these are 'shallow copies' of the BLTs (as leafTris)
		leaf.addTo(touchedVerts) //this will be undone if we split the leaf

		leaf.PatchInto(camPos, out, mesh, touchedVerts, 0) //this splits some leaves into fans

	}

	//	return final //note - also mutates (which persist on the device)
}

func (c *TriCollector) Reset() {
	c.TriCount = 0
	//clear (c.plants) clearing plants is unnecessary - they are values not pointers
	clear(c.triangles)
}

func (c *TriCollector) AddTri(t *Tri) {
	c.triangles[c.TriCount] = t
	c.TriCount++
}
