package terrain

import (
	"math"

	// "github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/colors"
	"github.com/nickax/gofu/game/msg"
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
	owner      *Tri //see shallowCopy
	Vi         [3]uint32
	children   [2]*LeafTri
	childCount int
	normal     vec.V3
	Scorched   bool
	centre     vec.V3
	poly       *poly.ConvexPoly // made/cached JIT
	WaterPoly  *poly.ConvexPoly // made/cached JIT
}

func (lt *LeafTri) WriteEdgesInto(msg *msg.Msg, mesh *TriMesh, color colors.Color) {

	v := mesh.verts
	a := v[lt.Vi[0]].P
	b := v[lt.Vi[1]].P
	c := v[lt.Vi[2]].P

	msg.Write(a, b, color,
		b, c, color,
		c, a, color,
	)

}

func (lt *LeafTri) WriteVertexNormalsInto(msg *msg.Msg, mesh *TriMesh, color colors.Color) {

	v := mesh.verts
	for _, vi := range lt.Vi {
		vv := v[vi]
		start := vv.P
		end := vv.P.Add(lt.CacheNormal(mesh).Multiply(.4)) //scale normal for visibility
		msg.Write(start, end, color)
	}

}

func (lt *LeafTri) ToConvexPoly(mesh *TriMesh, asWater bool) *poly.ConvexPoly {

	v0 := mesh.verts[lt.Vi[0]]
	v1 := mesh.verts[lt.Vi[1]]
	v2 := mesh.verts[lt.Vi[2]]

	a := v0.P
	b := v1.P
	c := v2.P

	if asWater {
		a.Y += v0.Wl
		b.Y += v1.Wl
		c.Y += v2.Wl
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

	p := lt.poly
	if water == 1 {
		p = lt.WaterPoly
	}
	hit, where := p.Probe(ray)

	if hit {
		if lt.childCount == 0 {
			return true, where, lt
		}

		//A leaftri has two or zero children
		if lt.childCount == 2 {

			aHit, aWhere, aLeaf := lt.children[0].Probe(ray, mesh, water)
			bHit, bWhere, bLeaf := lt.children[1].Probe(ray, mesh, water)

			if aHit && bHit {
				if ray.Origin.DistanceSQ(&aWhere) < ray.Origin.DistanceSQ(&bWhere) {
					return true, aWhere, aLeaf
				} else {
					return true, bWhere, bLeaf
				}
			}
			if aHit {
				return true, aWhere, aLeaf
			}
			if bHit {
				return true, bWhere, bLeaf
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
	lt := LeafTri{Vi: [3]uint32{a, b, c}, normal: vec.NewVec3(0, 0, 0), Scorched: scorched}

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
	leaf.centre = (mesh.verts[leaf.Vi[0]].P.Add(mesh.verts[leaf.Vi[1]].P).Add(mesh.verts[leaf.Vi[2]].P)).Multiply(1 / 3.0)
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

func (lc *LeafCollector) BubbleVerticalExtents(land *TriMesh) {
	for i := 0; i < lc.LeafCount; i++ {
		leaf := lc.leaves[i]
		leaf.owner.BubbleVerticalExtents(land)
	}
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

func (c *TriCollector) AsAffectedMap() map[*Tri]int {
	affectedTris := make(map[*Tri]int)
	for i := 0; i < c.TriCount; i++ {
		t := c.triangles[i]
		affectedTris[t] = 1
	}
	return affectedTris
}

// func (c *TriCollector) BubbleVerticalExtentsFromLeaves(land *TriMesh) {

// 	for i := 0; i < c.TriCount; i++ {
// 		tri := c.triangles[i]
// 		if tri.childCount == 0 {
// 			tri.BubbleVerticalExtents(land.Root)
// 		}
// 	}
// }

func (c *TriCollector) CheckPrisms() {

	for i := 0; i < c.TriCount; i++ {
		t := c.triangles[i]
		if t.ChildCount > 0 {
			if t.PrismFaces[0] == nil {
				panic("prism face missing")
			}
		}
	}
}

func (m *TriMesh) TextureX(tv *TouchedVerts, txScale float64, startVertex uint32) {

	tcx := m.verts[startVertex].Uv.X
	tcy := m.verts[startVertex].Uv.Y

	for v := range m.verts {
		m.verts[v].Uv.X = -math.MaxFloat64 //mark all as untextured
	}

	count := 0

	textureX(m, startVertex, tv, tcx, tcy, txScale, &count)
	log.Logit("textured verts", count)
}

//call this on some central onscreen vert and it will wrap the texture x coords outwards from there based on the edge lengths in world space
func textureX(m *TriMesh, vi uint32, touchedVerts *TouchedVerts, tcx float64, tcy float64, txScale float64, count *int) {

	//touchedVerts is an array of slices of leafTris
	//  - it gives us a list of all the leaf tris touching a vert
	TrisTouchingVert := touchedVerts.touches[vi]

	v := m.verts[vi]

	up := vec.NewVec3(0, 1, 0)

	pp := v.P
	for _, lt := range TrisTouchingVert { //for each leaf triagle this vertex touches..

		if lt.childCount == 0 {
			for _, vti := range lt.Vi {

				if vti != vi { //don't go back the way we came
					nv := m.verts[vti] //next vert

					if nv.Uv.X == -math.MaxFloat64 {
						hop := pp.Sub(nv.P)
						hop.Y = 0 //project onto the xz plane

						faceX := lt.CacheNormal(m).Cross(up).Normalised()
						//faceY := faceX.Cross(up).Normalise()
						dx := hop.Dot(faceX)
						//dy := hop.Dot(faceY)

						tcx += dx / txScale
						tcy = nv.P.Y / 300 // += dy / txScale

						nv.Uv.X = tcx
						nv.Uv.Y = tcy

						*count++
						textureX(m, vti, touchedVerts, tcx, tcy, txScale, count) //recurse

					}
				}
			}
		}

	}
}

func (leaf *LeafTri) PatchInto(camPos vec.V3, final *LeafCollector, mesh *TriMesh, touchedVerts *TouchedVerts, depth int) (wasSplit bool) {

	for j := range 3 { //for each edge
		ai := leaf.Vi[j]
		bi := leaf.Vi[(j+1)%3]
		ci := leaf.Vi[(j+2)%3]

		if ai == bi || bi == ci || ai == ci {
			panic("degenerate triangle in PatchInto")
		}

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
			return true
		}
	}

	//no midpoints found - this is a final triangle (It didin't need splitting to patch a hole)
	if leaf.childCount > 0 {
		panic("leaf with children being added to final")
	}

	//backface culling (don't ever backface cull under water tris)
	camPos.Y += 3 //pretend we're 10 ft taller
	if leaf.OnOrUnderWater(mesh) || leaf.CacheCentre(mesh).Sub(camPos).Dot(leaf.CacheNormal(mesh)) <= 0 {
		final.AddLeaf(leaf)
		//affectedTris[leaf.owner]++ //should not be needed (and doesnt help)
	}

	return false

}

// PatchTriangles - Bridges adjoining depths - splitting triangles in two (sometimes recursively)
// iterates over top level leaves (bottom level triangles)
// also fills touchedVerts with all verts touched by 'final' patch triangles
func (in *LeafCollector) PatchTriangles(camPos vec.V3, mesh *TriMesh, out *LeafCollector, touchedVerts *TouchedVerts) {

	//	final := NewLeafCollector(c.LeafCount*3, c.DeviceId)

	for i := range in.LeafCount {

		leaf := in.leaves[i]     //these are 'shallow copies' of the BLTs (as leafTris)
		leaf.addTo(touchedVerts) //this will be undone if we split the leaf

		//this splits SOME leaves into fans (and adds some directly) to out - it extends the prism of the owning triangle
		leaf.PatchInto(camPos, out, mesh, touchedVerts, 0)

	}

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
