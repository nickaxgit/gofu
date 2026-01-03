package terrain

import (
	"github.com/nickax/gofu/colors"
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/log"
	//"github.com/nickax/gofu/mesh"
	"github.com/nickax/gofu/plant"
	"github.com/nickax/gofu/poly"
	"github.com/nickax/gofu/ray"

	"github.com/nickax/gofu/vec"

	//"github.com/nickax/gofu/cam"

	"math"
	"math/rand/v2"
	//	"time"
)

var up = vec.NewVec3(0, 1, 0)
var nowhereSpecial = vec.NewVec3(0, -99999, 0)

type TouchedVerts struct {
	touches []map[*LeafTri]bool //Vertex ID to leaf triangles touching that vertex (per device)
}

func (tv *TouchedVerts) GetNormal(vi uint32, mesh *TriMesh) (normal vec.V3, steepness float64) {
	n := vec.NewVec3(0, 0, 0)
	numTouches := len(tv.touches[vi])

	if numTouches == 0 {
		return n, 1
	} //no triangles touch this vertex ?

	for lt := range tv.touches[vi] {
		n.AddIn(lt.CacheNormal(mesh))
	}

	if numTouches > 100 {
		log.Logit(numTouches, " that seems like a lot of triangles touching a vertex")
	}

	//local steepness calculation - a flat area is like a starfish, a mountain peak is a witches hat - but both would have an average normal pointing up
	//a measure of local steepness is the average y component
	yTotal := 3.0 //0.0 //n.Y
	n.DivIn(float64(numTouches))
	return n, yTotal / float64(numTouches)
}

func NewTouchedVerts(numverts int) *TouchedVerts {
	return &TouchedVerts{touches: make([]map[*LeafTri]bool, numverts)}
}
func (tv *TouchedVerts) Reset() {
	for i := range tv.touches {
		clear(tv.touches[i])
	}
}

func (tv *TouchedVerts) Add(ToVertexId uint32, lt *LeafTri) {
	if tv.touches[ToVertexId] == nil {
		tv.touches[ToVertexId] = make(map[*LeafTri]bool)
	}
	tv.touches[ToVertexId][lt] = true
	if len(tv.touches[ToVertexId]) > 100 {
		log.Logit("seems a lot")
	}
}

func (tv *TouchedVerts) remove(fromVertexId uint32, lt *LeafTri) {
	delete(tv.touches[fromVertexId], lt)
	if len(tv.touches[fromVertexId]) == 0 {
		tv.touches[fromVertexId] = nil
	}
}

type vert struct {
	p vec.V3
	//n  vec.V3 - now in "touchedverts"
	uv *vec.V2
	wl float64 //water level
}

type Tri struct {
	Parent     *Tri
	Depth      int
	vi         [3]uint32
	children   []*Tri
	childCount int
	//mesh       *TriMesh //a reference to the mesh this tri is part of (that the vi's point into v's of)
	Normal   vec.V3
	Scorched bool //note this is not part of the fireInfo - it's a cache of which land triangles are burned out
	//Culled   bool
	//occCount int //number of vertices occluded
	xMin float64
	xMax float64
	yMin float64
	yMax float64
	zMin float64
	zMax float64

	PrismFaces []*poly.ConvexPoly //3 sides plus bottom and top
	//poly       *poly.ConvexPoly   // made/cached JIT
	//shadow  *poly.ConvexPoly //the trinagle pojected onto y=0 JIT/cached for vprobe
	Centre    vec.V3              //needed for splitifneeded
	Plants    []plant.Plant       //generated as we split deeper - trees are generated early grass very late
	FireInfo  *fireInfo           //nil for land triangles
	firstLeaf map[uint32]*LeafTri //per device cache of the first leaf triangle below this tri

}

func (t *LeafTri) updateTrianglePoly(mesh *TriMesh) {

	if t.poly == nil {
		t.poly = poly.NewConvexPoly()
	} else {
		t.poly.PointCount = 0 //reset
	}
	for i := 0; i < 3; i++ {
		t.poly.AddPoint(mesh.verts[t.vi[i]].p)
	}

}

// func (tri *Tri) NewBottomCap(mesh *TriMesh, y float64) *poly.ConvexPoly {
// 	poly := poly.NewConvexPoly()
// 	//for _, v := range t.vi {
// 	for i := len(tri.vi) - 1; i >= 0; i-- {
// 		//poly.AddPoint(tri.mesh.verts[v].p.Add(vec.NewVec3(0, y, 0)))
// 		p := mesh.verts[tri.vi[i]].p.Clone()
// 		p.Y = y
// 		poly.AddPoint(p)
// 	}
// 	return poly
// }

// func (tri *Tri) NewTopCap(mesh *TriMesh, y float64) *poly.ConvexPoly {
// 	poly := poly.NewConvexPoly()
// 	for _, v := range tri.vi {
// 		//poly.AddPoint(tri.mesh.verts[v].p.Add(vec.NewVec3(0, y, 0)))
// 		p := mesh.verts[v].p.Clone()
// 		p.Y = y
// 		poly.AddPoint(p)
// 	}
// 	return poly
// }

func (tri *Tri) Plough(mesh *TriMesh, runwayStart vec.V3, runwayEnd vec.V3, runwayWidth float64) {

	for _, vi := range tri.vi {
		pp := mesh.verts[vi].p.Clone()
		pp.Y = runwayStart.Y //move the vertex to the runway height
		if pp.DistanceFromLineSegment(runwayStart, runwayEnd) < runwayWidth*2 {
			//cp := p.closestPointOnLineSegment(runwayStart, runwayEnd)
			mesh.verts[vi].p.Y = runwayStart.Y
		}
	}
	//tri.calcNormal(mesh) //SUPER important !

	//for _, ct := range t.children {
	for i := 0; i < tri.childCount; i++ {
		tri.children[i].Plough(mesh, runwayStart, runwayEnd, runwayWidth)
	}

}

func (t *Tri) MakeMissingPrisms(land *TriMesh, missing *int) {
	if t.childCount > 0 && t.PrismFaces[0] == nil {
		t.MakePrism(land)
		(*missing)++
	}
	for i := 0; i < t.childCount; i++ {
		t.children[i].MakeMissingPrisms(land, missing)
	}
}

// func (tri *Tri) MakePrisms(mesh *TriMesh) {

// 	if tri.PrismFaces[0] == nil {
// 		tri.PrismFaces = make([]*poly.ConvexPoly, 5) //3 sides + top + bottom
// 		for i := 0; i < 5; i++ {
// 			tri.PrismFaces[i] = poly.NewConvexPoly()
// 		}
// 	} else {
// 		for i := 0; i < 5; i++ {
// 			tri.PrismFaces[i].PointCount = 0 //reset/reuse polys
// 		}
// 	}

// 	for i, v := range tri.vi {
// 		vp := mesh.verts[v].p
// 		vpn := mesh.verts[tri.vi[(i+1)%3]].p

// 		poly := tri.PrismFaces[i]
// 		//poly.addPointAt(vp.x, t.yMax, vp.z)

// 		//TODO endcaps and sides could share vec3 verts

// 		tl := vpn.Clone()
// 		tl.Y = tri.yMax

// 		tr := vp.Clone()
// 		tr.Y = tri.yMax

// 		br := vp.Clone()
// 		br.Y = tri.yMin

// 		bl := vpn.Clone()
// 		bl.Y = tri.yMin

// 		poly.AddPoint(tl)
// 		poly.AddPoint(tr)
// 		poly.AddPoint(br)
// 		poly.AddPoint(bl)
// 		tri.PrismFaces[i] = poly

// 		// c2c := poly.Centre().Sub(t.centre).Normalise()
// 		// ppn := poly.Plane.GetNormal()
// 		// if ppn.Dot(c2c) < 0 {
// 		// 	panic("side face normal incorrect")
// 		// }

// 	}

// 	//make endcaps

// 	//top cap
// 	tri.PrismFaces[3].Cap(mesh.verts[tri.vi[0]].p.Clone(), mesh.verts[tri.vi[1]].p.Clone(), mesh.verts[tri.vi[2]].p.Clone(), tri.yMax)

// 	if tri.PrismFaces[3].Plane.GetNormal().Y < 0 {
// 		panic("top cap normal incorrect")
// 	}

// 	tri.PrismFaces[4].Cap(mesh.verts[tri.vi[2]].p.Clone(), mesh.verts[tri.vi[1]].p.Clone(), mesh.verts[tri.vi[0]].p.Clone(), tri.yMin)
// 	if tri.PrismFaces[4].Plane.GetNormal().Y > 0 {
// 		panic("bottom cap normal incorrect")
// 	}

// 	//for _, child := range t.children {
// 	for i := 0; i < tri.childCount; i++ {
// 		tri.children[i].MakePrisms(mesh)
// 	}

// }

// if the water level at ANY vertes - is higher than the Y coords
func (leaf *LeafTri) OnOrUnderWater(mesh *TriMesh) bool {

	a := mesh.verts[leaf.vi[0]]
	b := mesh.verts[leaf.vi[1]]
	c := mesh.verts[leaf.vi[2]]

	epsilon := 0.0001
	//if a.wl >= a.p.Y-epsilon || b.wl >= b.p.Y-epsilon || c.wl >= c.p.Y-epsilon { //if all verts are at or under water level
	if a.wl >= -epsilon || b.wl >= -epsilon || c.wl >= -epsilon { //if all verts are at or under water level
		//if a.p.Y <= a.wl+epsilon || b.p.Y <= b.wl+epsilon || c.p.Y <= c.wl+epsilon { //if all verts are at water level

		return true
	}
	return false
}

func (leaf *LeafTri) IsSubmerged(mesh *TriMesh) bool {

	a := mesh.verts[leaf.vi[0]]
	b := mesh.verts[leaf.vi[1]]
	c := mesh.verts[leaf.vi[2]]
	//if a.p.Y < a.wl && b.p.Y < b.wl && c.p.Y < c.wl { //if all verts are below water
	if a.wl > 0 && b.wl > 0 && c.wl > 0 { //if all verts are below water
		return true
	}
	return false
}

func (child *Tri) BubbleVerticalExtents() {
	parent := child.Parent
	if parent == nil {
		return
	}
	if child.yMax > parent.yMax {
		parent.yMax = child.yMax
	}
	if child.yMin < parent.yMin {
		parent.yMin = child.yMin
	}

	parent.BubbleVerticalExtents()

}

func (tri *Tri) updateExtents(p vec.V3) { //yMin float64, yMax float64) {

	if p.X < tri.xMin {
		tri.xMin = p.X
	}
	if p.X > tri.xMax {
		tri.xMax = p.X
	}
	if p.Y < tri.yMin {
		tri.yMin = p.Y
	}
	if p.Y > tri.yMax {
		tri.yMax = p.Y
	}

	if p.Z < tri.zMin {
		tri.zMin = p.Z
	}
	if p.Z > tri.zMax {
		tri.zMax = p.Z
	}

	if tri.Parent == nil && tri.Depth > 0 {
		log.Logit("ORPHANED TRIANGLE")
	}
}

// // keep track of the deepest (up to) 6 triangles touching this vert -- allows us to recalculate normals quickly
// func (v *vert) touch(t ...*Tri) {

// 	for _, t := range t {
// 		v.touches[t] = true
// 		if len(v.touches) > 6 {
// 			//panic("vertex touched by more than 6 triangles")
// 		}
// 	}
// }

func newVert(p vec.V3, u, v float64, wl float64) *vert {
	return &vert{p: p, wl: wl, uv: vec.NewVec2(u, v)}
}

// func (tri *Tri) flatContains(p *vec.V3) bool {
// 	//cache the flat polygon - particularly useful for fire mesh (which is persistent)
// 	if tri.flat == nil {
// 		tri.flat = &poly.ConvexPoly{}
// 		for _, vi := range tri.vi {
// 			tri.flat.AddPoint(tri.mesh.verts[vi].p)
// 	}
// 	return tri.flat.Contains(p)
// 		}
// }

func (tri *Tri) find2D(p vec.V3, mesh *TriMesh) *Tri {

	if tri.contains2D(p, mesh) {
		if tri.childCount == 0 {
			return tri
		}

		scorched := 0
		//for _, c := range t.children {
		for i := 0; i < tri.childCount; i++ {

			f := tri.children[i].find2D(p, mesh)
			if f != nil {
				return f
			}
		}
		if scorched == tri.childCount {
			tri.FireInfo.flames = -1 //mark parent as scorched
			tri.children = nil
		}
	} else {
		return nil
	}
	log.Logit("warn: fTri find failed to find a tri")
	return nil
}

func (parent *Tri) addChild(mesh *TriMesh, vi [3]uint32) *Tri {
	if parent.childCount < len(parent.children) {

		child := parent.ReUse(parent.childCount, mesh, vi)

		parent.childCount++
		return child

	} else {
		child := newTri(parent, mesh, vi) //t.mesh.makeTri(t.depth+1, fi, a, b, c)
		parent.children = append(parent.children, child)
		parent.childCount++ //= len(t.children)
		return child
	}

}

// scorch - recurse through all land triangles flagging them as scorched by checking their centres in the fire mesh
func (tri *Tri) Scorch(fire *TriMesh) {
	if tri.childCount == 0 {
		if fire.ScorchedAt(fire.Root, tri.Centre) {
			tri.Scorched = true
		}
	}
	//for _, c := range t.children {
	for i := 0; i < tri.childCount; i++ {
		tri.children[i].Scorch(fire)
	}
}

// for every bottom level triangle, look to see if there is a vertex at the midpoint of each edge (caused by a more divided neighbouring tri)
// if so, split in two to the opposite vertex
// func (tri *Tri) Patch(mesh *TriMesh) {

// 	if tri.childCount == 0 {

// 		for i := 0; i < 3; i++ {

// 			ai := tri.vi[i]
// 			bi := tri.vi[(i+1)%3]
// 			ci := tri.vi[(i+2)%3]
// 			//mi := m.midpoint(ai, bi) //looks both ways for a midpoint   -- m.vertAtMidPointXZ(ai, bi) //this could be way faster by recording mid verts
// 			mi := mesh.midpoint(bi, ci) //looks both ways for a midpoint   -- m.vertAtMidPointXZ(ai, bi) //this could be way faster by recording mid verts

// 			if mi != math.MaxUint32 { //is there a midpoint ?

// 				//if t.aspect() < 2 { //is it 'fat'
// 				tri.splitIn2(mesh, ai, bi, ci, mi)
// 				break //only one edge of this tri (becuase it is now multiple child tris)
// 				//} else {
// 				//	t.split()
// 				//}

// 			}

// 		}

// 	}

// 	// no longer recursive
// 	// //for _, c := range t.children {
// 	// for i := 0; i < tri.childCount; i++ {
// 	// 	tri.children[i].Patch(mesh)
// 	// }

// }

// func (tri *Tri) addToTouches(mesh *TriMesh) {

// 	mesh.verts[tri.vi[0]].touch(tri)
// 	mesh.verts[tri.vi[1]].touch(tri)
// 	mesh.verts[tri.vi[2]].touch(tri)

// }

// func (tri *Tri) removeFromTouches(mesh *TriMesh) {
// 	if tri.childCount > 0 {
// 		panic("Tri has children")
// 	}
// 	for _, vi := range tri.vi {
// 		v := mesh.verts[vi]
// 		delete(v.touches, tri) //remove this tri from the list of tris touching this vertex
// 	}
// }

func (tri *Tri) facesTowards(direction vec.V3) bool {
	//the extra -.1 is to account for traingles facing away at less than half the camera vertical FOV
	return tri.Normal.Dot(direction) < -.1 //is the traingle forward facing ? (relative to the camera)

}

func (tri *Tri) allVertsLeftOrRightOfFov(mesh *TriMesh, camPos vec.V3, camDir vec.V3, fov float64) bool {

	onLeft := 0
	behind := 0
	for _, vi := range tri.vi {
		cam2vert := mesh.verts[vi].p.Sub(camPos).Normalise()
		dp := camDir.Dot(cam2vert)
		if dp > fov {
			return false //a vertex is within the FOV EARLY EXIT
		}
		if dp < 0 {
			behind++
		}
		//we're outside the FOV
		cp := cam2vert.Cross(camDir)
		if cp.Y < 0 {
			onLeft++
		}
	}

	if behind == 3 {
		return true //all vertices are behind the camera
	}

	if onLeft == 3 {
		return true //all vertices are outside and on the same side of the FOV

	}
	if onLeft == 0 {
		return true
	}

	return false //vertices straddle the viewing frustum

}

func (tri *Tri) hasVertexWithinFov(mesh *TriMesh, pos vec.V3, focus vec.V3, fov float64) bool {

	camDir := focus.Sub(pos).Normalise()
	for _, vi := range tri.vi {
		cam2vert := mesh.verts[vi].p.Sub(pos).Normalise()
		if camDir.Dot(cam2vert) > fov {
			return true //a vertex is within the FOV
		}
	}

	return false //no vertices are within the FOV

}

func (tri *Tri) countChildren(count *int) {

	*count += tri.childCount //len(t.children)

	//for _, c := range t.children {
	for i := 0; i < tri.childCount; i++ {
		tri.children[i].countChildren(count)
		//
	}

}

func (tri *Tri) Flatten(depth int, into []*Tri, wp *int) {
	if *wp >= len(into) {
		panic("Triangle Flatten overflow")

	}

	if tri.Depth == depth {
		into[*wp] = tri
		*wp++
	}
	for i := 0; i < tri.childCount; i++ {
		if tri.children[i].Depth <= depth {
			tri.children[i].Flatten(depth, into, wp)
		}
	}

}
func (tri *Tri) getLeaves() []*Tri {
	leaves := []*Tri{}
	if tri.childCount == 0 {
		leaves = append(leaves, tri)
	}

	//for _, c := range t.children {
	for i := 0; i < tri.childCount; i++ {
		leaves = append(leaves, tri.children[i].getLeaves()...)
	}

	return leaves
}

func (leaf *LeafTri) occluded(prober *Prober) bool {

	//occlusion cull test
	//if ANY vertex is NOT occluded - return false

	//return false

	target := vec.NewVec3(0, 0, 0) //leaf.CacheCentre(prober.mesh)

	for i := range 3 {
		isOccluded, present := prober.votc[leaf.vi[i]]
		if present && !isOccluded {
			return false
		}

		if !present {
			vp := prober.mesh.verts[leaf.vi[i]]
			target.X = vp.p.X
			target.Z = vp.p.Z
			if vp.p.Y < vp.wl {
				target.Y = vp.wl + 0.02
			} else {
				target.Y = vp.p.Y + 0.1
			}

			prober.Target(target) //we dont want to mutate the actual vertex pos

			if prober.earlyExit == false {
				panic("occlusion prober should have earlyExit true")
			}
			prober.mesh.Root.Probe(prober)
			//save the result in the vertex occlusion test cache
			prober.votc[leaf.vi[i]] = prober.Hit
			if prober.Hit == false {
				return false //vertex IS NOT occluded -- (we *can* see the triangle) so exit early (returning false)
			}
		}

	}

	//none of the vertices were visible
	c := leaf.CacheCentre(prober.mesh)
	c.Y += 0.1
	prober.Target(c)
	prober.mesh.Root.Probe(prober)
	if prober.Hit == false {
		return false //we can see the middle
	}

	return true //occluded

}

// // this might be *much* faster if we checked all 6 points of the prisms for occlusion recursively
// // if a prism is fully occluded - all its children are too
// func (tri *Tri) Occlude(mesh *TriMesh, camPos *vec.V3) {

// 	viewpoint := camPos.Clone()
// 	viewpoint.Y += 5 //

// 	occluded := 0
// 	//defining a slice once, and using/resetting a penetration count is faster

// 	totalDepth := 0
// 	maxDepth := 0 //how deep did we go (in any one recursion)
// 	deepestEver := 0
// 	//backfacing := 0

// 	ray := ray.New(viewpoint, nowhereSpecial)

// 	//ts := time.Now()
// 	leaves := tri.getLeaves()

// 	stats := NewProbeStats()

// 	shortTarget := vec.NewVec3(0, 0, 0)
// 	rd := vec.NewVec3(0, 0, 0)
// 	for _, leaf := range leaves {

// 		// if leaf.poly == nil {
// 		// 	leaf.poly = newTrianglePoly(leaf, mesh) //cache the polygon
// 		// }

// 		//backFacecull test
// 		ray.PointAt(leaf.Centre)
// 		if ray.GetDirection().Dot(leaf.Normal) > 0 {
// 			//triangle is backfacing - it will be culled anyway
// 			leaf.Culled = true
// 		} else {
// 			//occlusion cull test

// 			for i := 0; i < 3; i++ {
// 				v := mesh.verts[leaf.vi[i]]
// 				if !v.testedForOcclusion {

// 					ray.PointAt(v.p)
// 					rd = ray.GetDirection()
// 					shortTarget.X = ray.Origin.X + rd.X*.999
// 					shortTarget.Y = ray.Origin.Y + rd.Y*.999
// 					shortTarget.Z = ray.Origin.Z + rd.Z*.999

// 					ray.PointAt(shortTarget)

// 					stats.Hit = false //clear the hit (we acculumulate in the stats object)

// 					tri.Probe(mesh, ray, stats, 0)
// 					if stats.Hit {
// 						v.occluded = true
// 						occluded++
// 					}
// 					v.testedForOcclusion = true
// 				}
// 				if v.occluded {
// 					leaf.occCount++
// 					if leaf.occCount == 3 {
// 						leaf.Culled = true
// 						break
// 					}
// 					if leaf.occCount > 3 {
// 						panic(fmt.Sprintf("occCount >3 on tri %v", leaf.vi))
// 					}
// 				}
// 			}
// 		}

// 		totalDepth += stats.maxDepth

// 		if maxDepth > deepestEver {
// 			deepestEver = maxDepth
// 		}

// 	}

// 	// avgDepth := float64(totalDepth) / float64(mesh.VertCount())
// 	// ms := time.Since(ts).Milliseconds()

// 	// log.Logit("occluded", occluded, " of ", mesh.VertCount(), " verts",
// 	// 	" backfacing:", backfacing,
// 	// 	" max depth:", deepestEver,
// 	// 	" avg depth:", avgDepth,
// 	// 	" time:", ms, "ms",
// 	// )
// 	// log.Logit(stats.String())

// }

func (tri *Tri) MakePrism(mesh *TriMesh) {

	if tri.PrismFaces[0] == nil {
		tri.PrismFaces = make([]*poly.ConvexPoly, 5) //3 sides + top + bottom
		for i := 0; i < 5; i++ {
			tri.PrismFaces[i] = poly.NewConvexPoly()
		}
	} else {
		log.Logit("reusing prism faces")
		for i := 0; i < 5; i++ {
			tri.PrismFaces[i].PointCount = 0 //reset/reuse polys
		}
	}

	for i, v := range tri.vi {
		vp := mesh.verts[v].p
		vpn := mesh.verts[tri.vi[(i+1)%3]].p

		poly := tri.PrismFaces[i]
		if poly == nil {
			panic("nil poly prism face XXX")
		}
		//poly.addPointAt(vp.x, t.yMax, vp.z)

		//TODO endcaps and sides could share vec3 verts

		if tri.yMin == tri.yMax {
			tri.yMax += 0.01
		} //avoid zero height prisms

		tl := vpn.Clone()
		tl.Y = tri.yMax

		tr := vp.Clone()
		tr.Y = tri.yMax

		br := vp.Clone()
		br.Y = tri.yMin

		bl := vpn.Clone()
		bl.Y = tri.yMin

		poly.AddPoint(tl)
		poly.AddPoint(tr)
		poly.AddPoint(br)
		poly.AddPoint(bl)
		tri.PrismFaces[i] = poly

	}

	//make endcaps

	//top cap
	tri.PrismFaces[3].Cap(mesh.verts[tri.vi[0]].p.Clone(), mesh.verts[tri.vi[1]].p.Clone(), mesh.verts[tri.vi[2]].p.Clone(), tri.yMax)

	if tri.PrismFaces[3].Plane.GetNormal().Y < 0 {
		panic("top cap normal incorrect")
	}

	tri.PrismFaces[4].Cap(mesh.verts[tri.vi[2]].p.Clone(), mesh.verts[tri.vi[1]].p.Clone(), mesh.verts[tri.vi[0]].p.Clone(), tri.yMin)
	if tri.PrismFaces[4].Plane.GetNormal().Y > 0 {
		panic("bottom cap normal incorrect")
	}

}

// WritePrismHeirarchyEdgesInto - gathers the edges of this and all ancestor prisms into MSG as vectors for  debugging
func (tri *Tri) WritePrismHeirarchyEdgesInto(msg *msg.Msg) {

	//gather the (three) uprights from the sides
	//if tri.childCount > 0 {
	for i := 0; i < 3; i++ {
		poly := tri.PrismFaces[i]
		a := poly.P[0] //top left
		b := poly.P[3] //bottom left
		msg.Write(a, b, colors.White)
	}

	top := tri.PrismFaces[3]
	bottom := tri.PrismFaces[4]

	for i := 0; i < 3; i++ {
		n := (i + 1) % 3
		msg.Write(top.P[i], top.P[n], colors.Red)
		msg.Write(bottom.P[i], bottom.P[n], colors.Blue)
	}
	//}//

	//gather all ancestor prisms
	if tri.Parent != nil {
		tri.Parent.WritePrismHeirarchyEdgesInto(msg)
	}

}

// // for each triangle T - check if none of its verts can been seen from pos
// func (root *Tri) OccludeVerts(pos *vec.V3) {

// 	occluded := 0
// 	//defining a slice once, and using/resetting a penetration count is faster
// 	//pen := vec.NewVec3(0, 0, 0)

// 	leafHits := 0
// 	leafMisses := 0
// 	prismHits := 0
// 	prismMisses := 0
// 	fourLeafMisses := 0
// 	fourPrismMisses := 0
// 	hct := 0

// 	depth := 0

// 	totalDepth := 0
// 	maxDepth := 0 //how deep did we go (in any one recursion)
// 	deepestEver := 0

// 	done := false

// 	ray := ray.New(pos, nowhereSpecial)
// 	for _, v := range root.mesh.verts {

// 		ray.PointAt(v.p)
// 		maxDepth = 0

// 		//reursively probe the prisms/faces
// 		v.occluded, _ = root.probe(ray, &leafHits, &prismMisses, &prismHits, &fourLeafMisses, &fourPrismMisses, &leafMisses, &hct, depth, &maxDepth, &done)

// 		totalDepth += maxDepth

// 		if maxDepth > deepestEver {
// 			deepestEver = maxDepth
// 		}

// 		if v.occluded {
// 			occluded++
// 		}

// 	}

// 	avgDepth := float64(totalDepth) / float64(len(root.mesh.verts))

// 	log.Logit("occluded", occluded, " of ", root.mesh.VertCount(), " verts",
// 		" leaf hits:", leafHits,
// 		" leaf misses:", leafMisses,
// 		" prism hits:", prismHits,
// 		" prism misses:", prismMisses,
// 		" four leaf misses:", fourLeafMisses,
// 		" four prism misses:", fourPrismMisses,
// 		" max depth:", deepestEver,
// 		" avg depth:", avgDepth,
// 		" hct:", hct, //missed prism but hit child triangle
// 	)
// }

// func (t *Tri) OcclusionCull(culled *int, kept *int) {

// 	if t.Culled {
// 		panic("already culled")
// 	}
// 	if !t.Culled && len(t.children) == 0 {

// 		hidden := 0
// 		for _, vi := range t.vi {
// 			if t.mesh.verts[vi].occluded {
// 				hidden++
// 			}
// 		}
// 		if hidden == 3 {
// 			t.Culled = true
// 			*culled++

// 		} else {
// 			*kept++
// 		}
// 	}
// 	for _, c := range t.children {
// 		c.OcclusionCull(culled, kept)
// 	}
// }

// drill down from the land root triangle - bubbling up and calculating y extents for all ancestors of all leaf triangles
// func (tri *Tri) CalcVerticalExtents(m *TriMesh) { //called on the root triangle
// 	if tri.childCount == 0 {

// 		for _, vi := range tri.vi {
// 			tri.updateExtents(m.verts[vi].p) //recursively bubble up and update/expand all ancestors extents
// 		}
// 		if tri.yMax-tri.yMin < 0.01 {
// 			//log.Logit("flat triangle detected", t.yMax, t.yMin, t.Depth)
// 		}

// 		//t.updateExtents(yMin, yMax) //recursively bubble up and update all ancestors extents
// 	} else {
// 		//for _, c := range t.children {
// 		for i := 0; i < tri.childCount; i++ {
// 			tri.children[i].CalcVerticalExtents(m) //recursively drill down to leaf triangles
// 		}
// 	}
// }

func (tri *Tri) scatter(mesh *TriMesh, species plant.Species, divisions int) []plant.Plant {

	plants := []plant.Plant{}

	step := 1.0 / float64(divisions+1) // offset by a ha

	for i := 1; i <= divisions; i++ {
		for j := 1; j <= divisions-i; j++ {
			u := float64(i) * step
			v := float64(j) * step
			//w := 1.0 - u - v
			//p := tri.BarycentricInterpolate(mesh, u, v, w)
			plants = append(plants, plant.New(u, v, species, 0.5+rand.Float64(), 0))

		}
	}

	return plants
}

func (tri *Tri) shallowCopy() *LeafTri {
	return &LeafTri{
		vi: tri.vi,
		//normal: tri.Normal,
		// children:   [2]*LeafTri{},
		// childCount: 0,
	}

}

func (tri *Tri) SplitIfNeeded(deviceId uint32, mesh *TriMesh, camPos vec.V3, camDir vec.V3, fov float64, leafTris *LeafCollector, newTris *TriCollector, touchedVerts *TouchedVerts) {

	//      0
	//		/\
	//     /  \
	//  5 /____\ 3
	//   / \  / \
	//  /___\/___\
	// 2     4    1
	if tri.Depth >= len(mesh.kinks) {
		return
	}

	if tri.Depth < 5 || !tri.allVertsLeftOrRightOfFov(mesh, camPos, camDir, fov) {

		//if t.normal.dot((t.centre().sub(pos)).normalise()) < 0.3 { //lower number here cull more backfacing tris

		shouldBeSplitToLevel := 5.0

		if tri.Depth >= 5 {

			centre := tri.Centre
			if tri.shallowCopy().OnOrUnderWater(mesh) {

				v1 := mesh.verts[tri.vi[0]]
				v2 := mesh.verts[tri.vi[1]]
				v3 := mesh.verts[tri.vi[2]]
				centre.Y = (v1.p.Y + v1.wl + v2.p.Y + v2.wl + v3.p.Y + v3.wl) / 3
				// log.Logit("water centre Y", centre.Y, " verts wl:", v1.wl, v2.wl, v3.wl)

				//centre.Y = (mesh.verts[tri.vi[0]].wl  + mesh.verts[tri.vi[1]].wl + mesh.verts[tri.vi[2]].wl) / 3
			}

			distSQ := camPos.DistanceSQ(&centre)
			//we want triangles at 3 metres split to level 15 - and those at 10,000 metres split to level 5
			shouldBeSplitToLevel = 12 - math.Log10(distSQ/2) //)*2 // (dist*dist-9)/(2000*2000) // * (5-15) + 15

			dp := camDir.Dot(centre.Sub(camPos).Normalise())
			if dp < 0.25 { //was 0.1
				dp = 0.25
			} //don't penalise *too* much for being behind
			shouldBeSplitToLevel += dp * 4 //4 //bring front and centre triangles forward up to 4 levels

			if shouldBeSplitToLevel >= float64(len(mesh.kinks)-1) {
				shouldBeSplitToLevel = float64(len(mesh.kinks) - 1)
			}
			if shouldBeSplitToLevel < 5 {
				shouldBeSplitToLevel = 5 //limits the fans created off large trianlges behind the camera
			}

			if !tri.shallowCopy().OnOrUnderWater(mesh) {
				if tri.Depth == 8 {
					if len(tri.Plants) == 0 { //only plant them once
						species := plant.Oak
						if tri.Centre.Y > 3000 {
							species = plant.Pine
						}
						tri.Plants = tri.scatter(mesh, species, 2)

						for _, plant := range tri.Plants {
							u := float64(plant.BcU) / float64(255)
							v := float64(plant.BcV) / float64(255)
							p := tri.BarycentricInterpolate(mesh, u, v)
							if !tri.contains2D(p, mesh) {
								log.Logit("plant outside triangle", tri.vi, p)
							}
						}
					}

				}
				// if tri.Depth == 9 {
				// 	tri.Plants = tri.scatter(mesh, plant.Grass, 3)
				// }
			}
		}

		//does this triangle need splitting
		//&& tri.Normal.Dot(camDir) > -.6 { //triangles leaning away from the camera by more than about 40 degrees are not split
		if tri.Depth < int(shouldBeSplitToLevel) {

			if tri.firstLeaf != nil {
				delete(tri.firstLeaf, deviceId) //pushing the leaves down
			}
			switch tri.childCount { //how many ways is it already split ?

			case 0:

				tri.split(mesh)
				for _, child := range tri.children {
					newTris.AddTri(child)
				}

			case 4:
				//already split (in 4)
			default:
				panic("WTF")
			}

			//if tri.Depth == 6 && tri.SpansWater(mesh) {
			if tri.SpansWater(mesh) {
				tri.DropVertsToWaterLevel(mesh)
			}

			for i := 0; i < tri.childCount; i++ {
				tri.children[i].SplitIfNeeded(deviceId, mesh, camPos, camDir, fov, leafTris, newTris, touchedVerts) //recurse

			}

		} else { // no it doesn't need spliting (or might be already) but it's 'good enough' for the PoV
			leaf := tri.shallowCopy()
			tri.firstLeaf[deviceId] = leaf
			leafTris.AddLeaf(leaf)
		}
	} else {
		//Triangle is split enough and/or all verts are outside the FOV
		//even offscreen triangles need a leaf for tree/flame placement
		if tri.firstLeaf[deviceId] != nil {
			//unpatch it to free space in touchedverts
			tri.firstLeaf[deviceId].removeFrom(touchedVerts)

		}

		leaf := tri.shallowCopy()
		tri.firstLeaf[deviceId] = leaf
		leafTris.AddLeaf(leaf)
		//

	}

}

func (tri *Tri) SpansWater(m *TriMesh) bool {
	above := false
	below := false
	for vi := range tri.vi {
		v := m.verts[tri.vi[vi]]
		if v.wl > 0 {
			above = true
		} else if v.wl < 0 {
			below = true
		}
		if above && below {
			return true
		}
	}
	return false
}

func (tri *Tri) DropVertsToWaterLevel(m *TriMesh) {

	return

	for vi := range tri.vi {
		v := m.verts[tri.vi[vi]]
		if v.wl > 0 {
			v.p.Y -= v.wl
			v.wl = 0
			//	v.p.Y = v.wl
		}
	}
}

func (tri *Tri) CountTris(count *int) {
	*count += tri.childCount
	for i := 0; i < tri.childCount; i++ {
		tri.children[i].CountTris(count)
	}
}

func (tri *Tri) SplitDownTo(mesh *TriMesh, level int) {

	if tri.Depth < level {
		tri.split(mesh)
		//for _, c := range t.children {
		for i := 0; i < tri.childCount; i++ {
			tri.children[i].SplitDownTo(mesh, level) //recurse
		}
	}

}

// remove the references to this leaf triangle from the verts it touches
func (leaf *LeafTri) removeFrom(tv *TouchedVerts) {
	for _, vi := range leaf.vi {
		tv.remove(vi, leaf)
	}
}

func (leaf *LeafTri) addTo(tv *TouchedVerts) {
	for _, vi := range leaf.vi { //for each vertex of the leaf
		tv.Add(vi, leaf) //add the leaf to the list of triangles touching that vertex
	}
}

// touches contains //Vertex ID to leaf triangles touching that vertex
func (leaf *LeafTri) splitIn2(mesh *TriMesh, a, b, c, m uint32) {
	//leaf.removeFrom(touches)                              //remove this leaf from the three verts it touches
	leaf.children[0] = NewLeafTri(a, m, c, leaf.Scorched, mesh) //left (clockwise wound)
	leaf.children[1] = NewLeafTri(a, b, m, leaf.Scorched, mesh) //right
	leaf.childCount = 2

}

func (tri *Tri) split(mesh *TriMesh) {

	//      V2
	//		/\
	//     /  \
	// V4 /____\ V5
	//   / \  / \
	//  /___\/___\
	// V1   V3    V0

	if tri.childCount == 0 {

		//	kink := (t.mesh.height/(float64(t.depth*t.depth)+1) - 1) * .5 //maximum kink in this edge
		//kink := (t.mesh.height/(float64(t.depth*5)+1) - 1) * .5 //maximum kink in this edge
		//kink := -t.mesh.height / math.Pow(2, float64(t.depth))
		kink := mesh.kinks[tri.Depth] * mesh.height //maximum kink in this edge

		m := mesh
		v0, v1, v2 := tri.vi[0], tri.vi[1], tri.vi[2]

		seed := uint64(m.verts[v1].p.Y)
		rnGen := rand.New(rand.NewPCG(seed, seed+1))
		//rnGe§n = &rand.New(rand.NewPCG(m.verts[v0].p.x, m.verts[v0].p.y))

		rn1, rn2, rn3 := rnGen.NormFloat64(), rnGen.NormFloat64(), rnGen.NormFloat64() //random number between -1 and 1

		v3 := m.splitEdge(v0, v1, rn1*kink, tri.Depth)
		v4 := m.splitEdge(v1, v2, rn2*kink, tri.Depth)
		v5 := m.splitEdge(v2, v0, rn3*kink, tri.Depth)

		//	tri.removeFromTouches(mesh) //the list of triangles touching a vertex is used for normal calculation
		//Always put the horizontal edge in first
		//wind clockwise
		tri.addChild(mesh, [3]uint32{v5, v4, v2}) //top
		tri.addChild(mesh, [3]uint32{v0, v3, v5}) //right
		tri.addChild(mesh, [3]uint32{v3, v1, v4}) //left
		tri.addChild(mesh, [3]uint32{v4, v5, v3}) //centre

	} else {
		log.Logit("splitting a triangle that already has children ??")
	}
}

func (tri *Tri) calcCentre(mesh *TriMesh) vec.V3 {
	v := mesh.verts

	// if tri.Centre == nil {
	// 	tri.Centre = &vec.NewVec3(0, 0, 0)
	// }
	tri.Centre.X, tri.Centre.Y, tri.Centre.Z = 0, 0, 0
	tri.Centre.AddInto(v[tri.vi[0]].p, v[tri.vi[1]].p, v[tri.vi[2]].p)
	tri.Centre.MulIn(float64(1.0 / 3.0))

	return tri.Centre
}

func (tri *Tri) area(mesh *TriMesh) float64 {
	v := mesh.verts
	a := v[tri.vi[0]].p
	b := v[tri.vi[1]].p
	c := v[tri.vi[2]].p
	return a.Sub(b).Cross(a.Sub(c)).Length() / 2
}

// returns the deepest (i.e. childless/leaf) triangle intersected by the ray from p0 to p1
// maintaining a count, and populating the slice of penetrations by refererence is easier to get your head around than appending slices (possibly faster too)
// func (tri *Tri) Probe(deviceId uint32, mesh *TriMesh, ray ray.Ray, stats *ProbeInfo, depth int) {

// 	if depth > stats.maxDepth {
// 		stats.maxDepth = depth
// 	}

// 	leaf := tri.firstLeaf[deviceId]
// 	if leaf != nil {

// 		//hit, _ := tri.poly.Probe(ray)
// 		hit := leaf.probe(mesh, ray)
// 		if hit {
// 			stats.leafHits++
// 			stats.Hit = true //exit signal
// 			//pen := ray.Intersect.Clone()
// 			//*done = true
// 		} else {
// 			stats.leafMisses++
// 		}

// 	} else {

// 		if ray.Origin.Y > tri.yMax && ray.End.Y > tri.yMax { //|| ray.Origin.Y < t.yMin && ray.End.Y < t.yMin {
// 			stats.skips++
// 		}

// 		if tri.prismContains(ray.Origin) || tri.prismContains(ray.End) || tri.probePrism(ray) {
// 			stats.prismHits++
// 			//for _, ct := range t.children {
// 			for i := 0; i < tri.childCount; i++ {
// 				ct := tri.children[i]

// 				if ray.End.Y > ct.yMax && ray.Origin.Y > ct.yMax ||
// 					ray.End.Y < ct.yMin && ray.Origin.Y < ct.yMin ||
// 					ray.End.X > ct.xMax && ray.Origin.X > ct.xMax ||
// 					ray.End.X < ct.xMin && ray.Origin.X < ct.xMin ||
// 					ray.End.Z > ct.zMax && ray.Origin.Z > ct.zMax ||
// 					ray.End.Z < ct.zMin && ray.Origin.Z < ct.zMin {
// 					stats.skips++
// 					continue
// 				}

// 				ct.Probe(mesh, ray, stats, depth+1)
// 				if stats.Hit {
// 					return
// 				}
// 			}

// 		} else {
// 			//none of the triangles in this prism need checking
// 			stats.prismMisses++

// 		}
// 	}

// }

// probeAll - find all intersections along the ray, returning the nearest hit point
func (tri *Tri) Probe(prober *Prober) bool {

	prober.Hit = false

	firstLeaf := tri.firstLeaf[prober.DeviceId]
	if firstLeaf != nil {

		for water := range 2 {
			if water == 0 || (water == 1 && firstLeaf.OnOrUnderWater(prober.mesh)) {
				hit, pen, leaf := firstLeaf.Probe(prober.ray, prober.mesh, water)
				if hit {
					prober.Hit = true
					prober.leafHits++
					if water == 1 {
						prober.HitWater = true
					}
					if prober.earlyExit {
						return prober.Hit
					}
					d := pen.DistanceFrom(prober.ray.Origin)
					if d < prober.SDist {
						prober.HitWater = (water == 1)
						prober.SDist = d
						prober.nearestHit = pen
						prober.NearestTri = tri
						prober.NearestLeaf = leaf
						prober.ray.PointAt(pen) //move the ray end to the hit point
					}
				} else {
					prober.leafMisses++
				}
			}
		}

	} else {
		if tri.childCount == 0 {
			log.Logit("Should have found a firstLeaf for ", prober.DeviceId, "depth:", tri.Depth)
		}
		if tri.prismContains(prober.ray.Origin, prober.mesh) || tri.prismContains(prober.ray.End, prober.mesh) || tri.probePrism(prober.ray) {
			prober.prismHits++
			//for _, ct := range t.children {
			for i := 0; i < tri.childCount; i++ {
				ct := tri.children[i]

				if prober.ray.End.Y > ct.yMax && prober.ray.Origin.Y > ct.yMax || prober.ray.End.Y < ct.yMin && prober.ray.Origin.Y < ct.yMin {
					prober.skips++
					continue
				}
				ct.Probe(prober)
				if prober.Hit && prober.earlyExit {
					return prober.Hit
				}
			}
		} else {
			//none of the triangles in this prism need checking
			prober.prismMisses++
		}
	}

	return prober.Hit

}

// EdgesAsMsg - return the edges of this triangle as a msg for clientside rendering/debugging
func (tri *Tri) WriteEdgesInto(msg *msg.Msg, mesh *TriMesh, color colors.Color) {

	v := mesh.verts
	a := v[tri.vi[0]].p
	b := v[tri.vi[1]].p
	c := v[tri.vi[2]].p

	msg.Write(a, b, color,
		b, c, color,
		c, a, color,
	)

}

func (tri *Tri) prismContains(p vec.V3, mesh *TriMesh) bool {

	if p.Y < tri.yMin || p.Y > tri.yMax {
		return false //outside the vertical extents of the prism
	}

	//we are within the vertical extents - we now do a 2d check agains the footprint
	return tri.contains2D(p, mesh)

}

// test if the ray from p0 to p1 penetrates the volume of triangular based 'prism' extending between t.ymin and t.ymax
func (tri *Tri) probePrism(ray ray.Ray) bool {

	epsilon := 0.001
	//for all five sides of the prism - check for a penetration
	for i, s := range tri.PrismFaces {
		if s == nil {
			log.Logit("nil prism face in probePrism", i, "depth:", tri.Depth, " children:", tri.childCount)
			return false
			//panic("nil prism face")
		}
		hit, pen := s.Probe(ray)
		if hit {
			if pen.Y > tri.yMax+epsilon {

				log.Logit("penetration is above the vertical extents of the prism", i, "by", pen.Y-tri.yMax, "depth:", tri.Depth)
			}
			if pen.Y < tri.yMin-epsilon {
				log.Logit("penetration is below the vertical extents of the prism", i, "by", tri.yMin-pen.Y)
			}
			return true
		}
	}

	return false
}

func (tri *Tri) vProbe(p vec.V3, mesh *TriMesh, deviceId uint32) *Tri {

	if tri.contains2D(p, mesh) {
		if tri.firstLeaf[deviceId] != nil {
			return tri
		}

		//for _, ct := range t.children {
		for i := 0; i < tri.childCount; i++ {

			tt := tri.children[i].vProbe(p, mesh, deviceId)
			if tt != nil {
				return tt
			}
		}

		log.Logit("No child contained point - but parent did", "depth:", tri.Depth, " children:", tri.childCount)

	}

	return nil

	//panic("vProbe failed to find a tri")

}

func (tri *Tri) VprobeLand(p vec.V3, mesh *TriMesh, deviceId uint32) (bool, vec.V3, *Tri, *LeafTri) {

	t := tri.vProbe(p, mesh, deviceId) //recursively find the tri that holds the leftTri structure for this device

	//fire a ray through that plane
	ray := ray.New(vec.NewVec3(p.X, 100000, p.Z), vec.NewVec3(p.X, -100000, p.Z))
	//if t.prismFaces[5].Probe(ray) {
	firstLeaf := t.firstLeaf[deviceId]

	if firstLeaf == nil {
	}
	hit, where, leaf := firstLeaf.Probe(ray, mesh, 0) //recursively probe leafs (not as water)

	return hit, where, t, leaf

}

func (tri *Tri) contains2D(p vec.V3, mesh *TriMesh) bool {

	//TODO optimise initialise for first edge, (reuse for 2,3rd edges)
	for i := range 3 {
		this := mesh.verts[tri.vi[i]].p
		next := mesh.verts[tri.vi[(i+1)%3]].p

		thisToNext := next.Sub(this)
		thisToP := p.Sub(this)

		if thisToNext.Cross(thisToP).Y < 0 {
			return false
		}
	}

	return true

}

func (leaf *LeafTri) CacheNormal(mesh *TriMesh) vec.V3 {

	//if leaf.normal.Y != -1 { //X != 0 || leaf.normal.Y != 0 || leaf.normal.Z != 0 {
	if leaf.normal.X != 0 || leaf.normal.Y != 0 || leaf.normal.Z != 0 {
		return leaf.normal
	}

	v := mesh.verts

	numMeshVerts := uint32(mesh.VertexCount)
	if leaf.vi[0] >= numMeshVerts || leaf.vi[1] >= numMeshVerts || leaf.vi[2] >= numMeshVerts {
		panic("index out of range in tri.normal")
	}

	ab := v[leaf.vi[1]].p.Sub(v[leaf.vi[0]].p) //.normalise()
	if ab.LengthSq() == 0 {
		panic("zero length edge in leaf tri")
	}

	ac := v[leaf.vi[2]].p.Sub(v[leaf.vi[0]].p) //.normalise()
	if ac.LengthSq() == 0 {
		panic("zero length edge in leaf tri")
	}
	leaf.normal = (ab.Cross(ac)).Normalise()

	if leaf.normal.Y < 0 {
		panic("downward facing normal on leaf tri")
	}

	return leaf.normal

}

func (parent *Tri) ReUse(childIndex int, mesh *TriMesh, vi [3]uint32) *Tri {

	// done in reset()
	// 	tri.childCount = 0
	// tri.Culled = false
	// tri.occCount = 0
	// tri.yMax = -math.MaxFloat64
	// tri.yMin = math.MaxFloat64
	// tri.xMax = -math.MaxFloat64
	// tri.xMin = math.MaxFloat64
	// tri.zMax = -math.MaxFloat64
	// tri.zMin = math.MaxFloat64

	child := parent.children[childIndex]

	if child.yMax > -math.MaxFloat64 {
		log.Logit("child triangle yMax not reset")
	}

	child.Reset()
	child.vi = vi //set new vertices
	//recalc extents
	for i := range 3 {
		child.updateExtents(mesh.verts[vi[i]].p)
	}

	if child.Depth != parent.Depth+1 {
		panic("reused child triangle has wrong depth")
	}

	//child.poly.PointCount=0 // = newTrianglePoly(child, mesh)

	child.calcCentre(mesh)
	//child.updateTrianglePoly(mesh)
	//add this traingle to its verts list of triangles
	//child.addToTouches(mesh)

	//child.calcNormal(mesh)
	if child.Normal.Y < 0 {
		panic("triangle with downward normal")
	}
	return child
}

// reset the triangle heirarchy for reuse - see also tri.ReUse()
func (t *Tri) Reset() {
	t.childCount = 0
	//t.Culled = false
	//t.occCount = 0
	t.yMax = -math.MaxFloat64
	t.yMin = math.MaxFloat64
	t.xMax = -math.MaxFloat64
	t.xMin = math.MaxFloat64
	t.zMax = -math.MaxFloat64
	t.zMin = math.MaxFloat64

	t.PrismFaces = []*poly.ConvexPoly{nil, nil, nil, nil, nil}
	// for i := range t.children {
	// 	t.children[i].Reset()
	// }

}

func newTri(parent *Tri, m *TriMesh, vi [3]uint32) *Tri {

	if vi[0] == vi[1] || vi[0] == vi[2] || vi[1] == vi[2] {
		panic("degenerate triangle")
	}

	//t := Tri{depth: depth, vi: vi, children: []*Tri{}, mesh: m, faceIndex: fi}
	depth := 0
	if parent != nil {
		depth = parent.Depth + 1
	}
	t := &Tri{Parent: parent, Depth: depth, vi: vi, children: []*Tri{},
		xMin: math.MaxFloat64, xMax: -math.MaxFloat64,
		yMin: math.MaxFloat64, yMax: -math.MaxFloat64,
		zMin: math.MaxFloat64, zMax: -math.MaxFloat64,
		PrismFaces: []*poly.ConvexPoly{nil, nil, nil, nil, nil},
		firstLeaf:  make(map[uint32]*LeafTri),
	}

	v0 := m.verts[vi[0]]
	v1 := m.verts[vi[1]]
	v2 := m.verts[vi[2]]

	p0 := v0.p
	p1 := v1.p
	p2 := v2.p
	//water surface (extend prism)
	if v0.wl > p0.Y {
		p0.Y = v0.wl
	}
	if v1.wl > p1.Y {
		p1.Y = v1.wl
	}
	if v2.wl > p2.Y {
		p2.Y = v2.wl
	}

	if p0.Equals(p1) || p0.Equals(p2) || p1.Equals(p2) {
		panic("infinitely thin triangle")
	}

	t.updateExtents(p0)
	t.updateExtents(p1)
	t.updateExtents(p2)

	//t.poly = newTrianglePoly(t, m)
	//t.updateTrianglePoly(m)

	t.calcCentre(m)
	//add this traingle to its verts list of triangles
	//t.addToTouches(m)

	// t.calcNormal(m)
	// if t.Normal.Y < 0 {
	// 	panic("triangle with downward normal")
	// }

	return t
}
