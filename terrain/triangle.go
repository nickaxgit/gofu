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
	//touches []map[*LeafTri]bool //Vertex ID to leaf triangles touching that vertex (per device)
	//Vertex ID to leaf triangles touching that vertex (per device)
	touches [][]*LeafTri // an array of slices of leaf tris
}

func (tv *TouchedVerts) GetAverageVertexNormal(vi uint32, mesh *TriMesh) vec.V3 {
	n := vec.NewVec3(0, 0, 0)
	numTouches := len(tv.touches[vi])

	if numTouches == 0 {
		n := vec.NewVec3(0, -1, 0)
		log.Logit("vertex with no touching triangles - returning DOWN normal", vi, colors.Yellow)
		return n
	} //no triangles touch this vertex ?

	for _, lt := range tv.touches[vi] {
		fn := lt.CacheNormal(mesh)
		n.AddIn(fn)

	}

	if numTouches > 100 {
		log.Logit(numTouches, " that seems like a lot of triangles touching a vertex")
	}

	//local steepness calculation - a flat area is like a starfish, a mountain peak is a witches hat - but both would have an average normal pointing up
	//a measure of local steepness is the average y component
	//yTotal := n.Y
	n.DivIn(float64(numTouches))

	n = n.Normalised() //DON'T be tempted to remove (avergaing normals does not produce a unit length normal)

	ls := n.LengthSq()
	if ls > 1.00001 || ls < 0.99999 {
		panic("non unit normal calculated")
	}
	return n //yTotal / float64(numTouches)
}

func (t *Tri) RemakePrisms(land *TriMesh) {
	if t.ChildCount == 0 { //BLT's don't need prisms
		return
	}

	if t.PrismFaces == nil {
		t.MakePrism(land)
	} else {
		//if there is a mismatch between the vertical extents and the prism endcaps, remake the prism
		if t.PrismFaces[4] == nil || t.PrismFaces[4].P[0].Y != t.yMin || t.PrismFaces[3].P[0].Y != t.yMax {
			t.MakePrism(land)
		}
	}

	for i := 0; i < t.ChildCount; i++ {
		t.children[i].RemakePrisms(land)
	}

}

func NewTouchedVerts(numverts int) *TouchedVerts {
	return &TouchedVerts{touches: make([][]*LeafTri, numverts)}
}
func (tv *TouchedVerts) Reset() {
	for i := range tv.touches {
		//clear(tv.touches[i])
		tv.touches[i] = tv.touches[i][:0] //slice it to zero length( keep the capacity)
	}
}

func (tv *TouchedVerts) Add(ToVertexId uint32, lt *LeafTri) {

	if lt == nil {
		panic("adding nil leaf tri to touched verts")
	}
	tv.touches[ToVertexId] = append(tv.touches[ToVertexId], lt)
	if len(tv.touches[ToVertexId]) > 100 {
		log.Logit("seems a lot")
	}
}

// remove this leaf tri from the list of tris touching this vertex
func (tv *TouchedVerts) remove(fromVertexId uint32, lt *LeafTri) {

	for i, tlt := range tv.touches[fromVertexId] {
		if tlt == lt {
			tv.touches[fromVertexId] = append(tv.touches[fromVertexId][:i], tv.touches[fromVertexId][i+1:]...)
			break
		}
	}

	// delete(tv.touches[fromVertexId], lt)
	// if len(tv.touches[fromVertexId]) == 0 {
	// 	tv.touches[fromVertexId] = nil
	// }
}

type Tri struct {
	Parent *Tri
	Depth  int

	vi         [3]uint32
	children   []*Tri
	ChildCount int
	//mesh       *TriMesh //a reference to the mesh this tri is part of (that the vi's point into v's of)
	//Normal   vec.V3
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
	Centre        vec.V3              //needed for splitifneeded
	Distributions [][]plant.Instance  //generated as we split deeper - trees are generated early grass very late
	FireInfo      *fireInfo           //nil for land triangles
	firstLeaf     map[uint32]*LeafTri //per device cache of the first leaf triangle below this tri

}

func (tri *Tri) slopeDegrees(mesh *TriMesh) float64 {

	upDot := tri.shallowCopy().CacheNormal(mesh).Dot(up)
	return math.Acos(upDot) * (180 / math.Pi)
}

func (t *LeafTri) updateTrianglePoly(mesh *TriMesh) {

	if t.poly == nil {
		t.poly = poly.NewConvexPoly()
	} else {
		t.poly.PointCount = 0 //reset
	}
	for i := 0; i < 3; i++ {
		t.poly.AddPoint(mesh.verts[t.Vi[i]].P)
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
		pp := mesh.verts[vi].P.Clone()
		pp.Y = runwayStart.Y //move the vertex to the runway height
		if pp.DistanceFromLineSegment(runwayStart, runwayEnd) < runwayWidth*2 {
			//cp := p.closestPointOnLineSegment(runwayStart, runwayEnd)
			mesh.verts[vi].P.Y = runwayStart.Y
		}
	}
	//tri.calcNormal(mesh) //SUPER important !

	//for _, ct := range t.children {
	for i := 0; i < tri.ChildCount; i++ {
		tri.children[i].Plough(mesh, runwayStart, runwayEnd, runwayWidth)
	}

}

func (t *Tri) MakeMissingPrisms(land *TriMesh, missing *int) {

	// if t.yMin != t.PrismFaces[4].P[0].Y {
	// 	log.Logit("triangle prism bottom cap Y does not match triangle ymin")
	// }
	// if t.yMax != t.PrismFaces[3].P[0].Y {
	// 	log.Logit("triangle prism top cap Y does not match triangle ymax")
	// }

	if t.ChildCount > 0 && (t.PrismFaces[4] == nil || t.PrismFaces[4].PointCount == 0) {
		t.MakePrism(land)
		(*missing)++
	}
	for i := 0; i < t.ChildCount; i++ {
		t.children[i].MakeMissingPrisms(land, missing)
	}
}

// if the water level at ANY vertes - is higher than the Y coords
func (leaf *LeafTri) OnOrUnderWater(mesh *TriMesh) bool {

	a := mesh.verts[leaf.Vi[0]]
	b := mesh.verts[leaf.Vi[1]]
	c := mesh.verts[leaf.Vi[2]]

	epsilon := 0.0001
	//if a.wl >= a.p.Y-epsilon || b.wl >= b.p.Y-epsilon || c.wl >= c.p.Y-epsilon { //if all verts are at or under water level
	if a.Wl >= -epsilon || b.Wl >= -epsilon || c.Wl >= -epsilon { //if all verts are at or under water level
		//if a.p.Y <= a.wl+epsilon || b.p.Y <= b.wl+epsilon || c.p.Y <= c.wl+epsilon { //if all verts are at water level

		return true
	}
	return false
}

func (leaf *LeafTri) IsSubmerged(mesh *TriMesh) bool {

	a := mesh.verts[leaf.Vi[0]]
	b := mesh.verts[leaf.Vi[1]]
	c := mesh.verts[leaf.Vi[2]]
	//if a.p.Y < a.wl && b.p.Y < b.wl && c.p.Y < c.wl { //if all verts are below water
	if a.Wl > 0 && b.Wl > 0 && c.Wl > 0 { //if all verts are below water
		return true
	}
	return false
}

func (child *Tri) BubbleVerticalExtents(land *TriMesh) {
	parent := child.Parent
	if parent == nil {
		if child != land.Root {
			panic("orphaned triangle during bubble extents")
		}
		return
	}

	expanded := false
	if child.yMax > parent.yMax {
		parent.yMax = child.yMax

		expanded = true

	}
	if child.yMin < parent.yMin {
		parent.yMin = child.yMin

		expanded = true
	}

	if !expanded {
		return //no change to parent (stop recursion)
	}

	parent.BubbleVerticalExtents(land)
}

func (tri *Tri) updateVerticalExtents(y float64) {
	if y > tri.yMax {
		tri.yMax = y
	}
	if y < tri.yMin {
		tri.yMin = y
	}
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
		if tri.ChildCount == 0 {
			return tri
		}

		scorched := 0
		//for _, c := range t.children {
		for i := 0; i < tri.ChildCount; i++ {

			f := tri.children[i].find2D(p, mesh)
			if f != nil {
				return f
			}
		}
		if scorched == tri.ChildCount {
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
	if parent.ChildCount < len(parent.children) {

		child := parent.ReUse(parent.ChildCount, mesh, vi)

		parent.ChildCount++
		return child

	} else {
		child := newTri(parent, mesh, vi) //t.mesh.makeTri(t.depth+1, fi, a, b, c)
		parent.children = append(parent.children, child)
		parent.ChildCount++ //= len(t.children)
		return child
	}

}

// scorch - recurse through all land triangles flagging them as scorched by checking their centres in the fire mesh
func (tri *Tri) Scorch(fire *TriMesh) {
	if tri.ChildCount == 0 {
		if fire.ScorchedAt(fire.Root, tri.Centre) {
			tri.Scorched = true
		}
	}
	//for _, c := range t.children {
	for i := 0; i < tri.ChildCount; i++ {
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

func (tri *Tri) allVertsLeftOrRightOfFov(mesh *TriMesh, camPos vec.V3, camDir vec.V3, fov float64) bool {

	onLeft := 0
	behind := 0
	for _, vi := range tri.vi {
		cam2vert := mesh.verts[vi].P.Sub(camPos).Normalised()
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

	camDir := focus.Sub(pos).Normalised()
	for _, vi := range tri.vi {
		cam2vert := mesh.verts[vi].P.Sub(pos).Normalised()
		if camDir.Dot(cam2vert) > fov {
			return true //a vertex is within the FOV
		}
	}

	return false //no vertices are within the FOV

}

func (tri *Tri) countChildren(count *int) {

	*count += tri.ChildCount //len(t.children)

	//for _, c := range t.children {
	for i := 0; i < tri.ChildCount; i++ {
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
	for i := 0; i < tri.ChildCount; i++ {
		if tri.children[i].Depth <= depth {
			tri.children[i].Flatten(depth, into, wp)
		}
	}

}
func (tri *Tri) getLeaves() []*Tri {
	leaves := []*Tri{}
	if tri.ChildCount == 0 {
		leaves = append(leaves, tri)
	}

	//for _, c := range t.children {
	for i := 0; i < tri.ChildCount; i++ {
		leaves = append(leaves, tri.children[i].getLeaves()...)
	}

	return leaves
}

func (leaf *LeafTri) occluded(prober *Prober) bool {

	//occlusion cull test
	//if ANY vertex is NOT occluded - return false

	target := vec.NewVec3(0, 0, 0) //leaf.CacheCentre(prober.mesh)

	for i := range 3 {
		isOccluded, present := prober.votc[leaf.Vi[i]]
		if present && !isOccluded {
			return false
		}

		if !present {
			vp := prober.mesh.verts[leaf.Vi[i]]
			target.X = vp.P.X
			target.Z = vp.P.Z
			if vp.P.Y < vp.Wl {
				target.Y = vp.Wl + 0.02
			} else {
				target.Y = vp.P.Y + 0.1
			}

			prober.Target(target) //we dont want to mutate the actual vertex pos

			if prober.earlyExit == false {
				panic("occlusion prober should have earlyExit true")
			}
			prober.mesh.Root.Probe(prober)
			//save the result in the vertex occlusion test cache
			prober.votc[leaf.Vi[i]] = prober.Hit
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

	//all prisms have a bottom (even if they have no 'height') - so that's what we check on

	//3 is the top, 4 is the bottom
	if tri.PrismFaces[4] == nil {
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

	//only create sides if there is a height difference
	if tri.yMax != tri.yMin {

		for i, v := range tri.vi {
			vp := mesh.verts[v].P
			vpn := mesh.verts[tri.vi[(i+1)%3]].P

			poly := tri.PrismFaces[i]
			if poly == nil {
				panic("nil poly prism face XXX")
			}
			//poly.addPointAt(vp.x, t.yMax, vp.z)

			//TODO endcaps and sides could share vec3 verts

			// if tri.yMin == tri.yMax {
			// 	tri.yMax += 0.01
			// } //avoid zero height prisms

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
		tri.PrismFaces[3].Cap(mesh.verts[tri.vi[0]].P.Clone(), mesh.verts[tri.vi[1]].P.Clone(), mesh.verts[tri.vi[2]].P.Clone(), tri.yMax)

		if tri.PrismFaces[3].Plane.GetNormal().Y < 0 {
			panic("top cap normal incorrect")
		}
	}

	//bottom cap (we *always* have one of these)
	tri.PrismFaces[4].Cap(mesh.verts[tri.vi[2]].P.Clone(), mesh.verts[tri.vi[1]].P.Clone(), mesh.verts[tri.vi[0]].P.Clone(), tri.yMin)
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
		if poly.PointCount != 0 {
			a := poly.P[0] //top left
			b := poly.P[3] //bottom left
			msg.Write(a, b, colors.White)
		}
	}

	top := tri.PrismFaces[3]
	bottom := tri.PrismFaces[4]

	for i := 0; i < 3; i++ {
		n := (i + 1) % 3
		if top.PointCount != 0 {
			msg.Write(top.P[i], top.P[n], colors.Red)
		}
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

func (tri *Tri) scatter(mesh *TriMesh, distribution *plant.Distribution) []plant.Instance {

	instances := []plant.Instance{}

	//use distribution.Density to adjust number of plants per triangle

	divisions := distribution.Density
	step := 1.0 / float64(divisions+1) // offset by a ha

	for i := 1; i <= int(divisions); i++ {
		for j := 1; j <= int(divisions)-i; j++ {

			u := float64(i) * step
			v := float64(j) * step
			//w := 1.0 - u - v
			p := tri.BarycentricInterpolate(mesh, u, v) //, w)
			chance := distribution.Altitude.GetY(p.Y)   //P.Y is the x 'input' (to altitude)
			if rand.Float64() > chance {
				continue
			}
			chance = distribution.Slope.GetY(tri.slopeDegrees(mesh))
			if rand.Float64() > chance {
				continue
			}

			scale := 0.5 + rand.Float64()
			rotation := byte(0) //0 / 255)
			instances = append(instances, plant.New(u, v, scale, rotation))

		}
	}

	return instances
}

func (tri *Tri) shallowCopy() *LeafTri {
	return &LeafTri{
		owner: tri, //hold a reference to the BLT this leaf/patch belongs to
		Vi:    tri.vi,
		//normal: tri.Normal,
		// children:   [2]*LeafTri{},
		// childCount: 0,
	}

}

func (tri *Tri) SplitIfNeeded(deviceId uint32, mesh *TriMesh, camPos vec.V3, camDir vec.V3, fov float64, leafTris *LeafCollector, affectedTris map[*Tri]int, touchedVerts *TouchedVerts) {

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
				centre.Y = (v1.P.Y + v1.Wl + v2.P.Y + v2.Wl + v3.P.Y + v3.Wl) / 3
				// log.Logit("water centre Y", centre.Y, " verts wl:", v1.wl, v2.wl, v3.wl)

				//centre.Y = (mesh.verts[tri.vi[0]].wl  + mesh.verts[tri.vi[1]].wl + mesh.verts[tri.vi[2]].wl) / 3
			}

			distSQ := camPos.DistanceSQ(&centre)
			//we want triangles at 3 metres split to level 15 - and those at 10,000 metres split to level 5
			shouldBeSplitToLevel = 12 - math.Log10(distSQ/2) //)*2 // (dist*dist-9)/(2000*2000) // * (5-15) + 15

			dp := camDir.Dot(centre.Sub(camPos).Normalised())
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
					for idx, distribution := range plant.Distributions {
						if len(tri.Distributions) == 0 { //only plant them once

							//based on the species distribution and density .. scatter plants

							tri.Distributions[idx] = tri.scatter(mesh, distribution)

							// for _, distrubution := range tri.Distributions {
							// 	u := float64(plant.BcU) / float64(255)
							// 	v := float64(plant.BcV) / float64(255)
							// 	p := tri.BarycentricInterpolate(mesh, u, v)
							// 	if !tri.contains2D(p, mesh) {
							// 		log.Logit("plant outside triangle", tri.vi, p)
							// 	}
							// }
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
			switch tri.ChildCount { //how many ways is it already split ?

			case 0:

				tri.split(mesh)
				delete(affectedTris, tri.Parent)
				affectedTris[tri]++
				//for _, child := range tri.children {
				//	newTris.AddTri(child)
				//}

			case 4:
				//already split (in 4)
			default:
				panic("WTF")
			}

			//if tri.Depth == 6 && tri.SpansWater(mesh) {
			if tri.SpansWater(mesh) {
				tri.DropVertsToWaterLevel(mesh)
			}

			for i := 0; i < tri.ChildCount; i++ {
				tri.children[i].SplitIfNeeded(deviceId, mesh, camPos, camDir, fov, leafTris, affectedTris, touchedVerts) //recurse

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
		if v.Wl > 0 {
			above = true
		} else if v.Wl < 0 {
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
		if v.Wl > 0 {
			v.P.Y -= v.Wl
			v.Wl = 0
			//	v.p.Y = v.wl
		}
	}
}

func (tri *Tri) CountTris(count *int) {
	*count += tri.ChildCount
	for i := 0; i < tri.ChildCount; i++ {
		tri.children[i].CountTris(count)
	}
}

func (tri *Tri) SplitDownTo(mesh *TriMesh, level int) {

	if tri.Depth < level {
		tri.split(mesh)
		//for _, c := range t.children {
		for i := 0; i < tri.ChildCount; i++ {
			tri.children[i].SplitDownTo(mesh, level) //recurse
		}
	}

}

// remove the references to this leaf triangle from the verts it touches
func (leaf *LeafTri) removeFrom(tv *TouchedVerts) {
	for _, vi := range leaf.Vi {
		tv.remove(vi, leaf)
	}
}

func (leaf *LeafTri) addTo(tv *TouchedVerts) {
	for _, vi := range leaf.Vi { //for each vertex of the leaf
		tv.Add(vi, leaf) //add the leaf to the list of triangles touching that vertex
	}
}

// touches contains //Vertex ID to leaf triangles touching that vertex
func (leaf *LeafTri) splitIn2(mesh *TriMesh, a, b, c, m uint32) {
	//leaf.removeFrom(touches)                              //remove this leaf from the three verts it touches
	v := mesh.verts[m]

	leaf.owner.updateVerticalExtents(v.P.Y)        //expand the extents of the owner triangle to include the new midpoint vertex
	leaf.owner.updateVerticalExtents(v.P.Y + v.Wl) //Important to do with and without water

	//affectedTris[leaf.owner]++ //how many times has this tri been affected - no that it matters, once is enough

	leaf.children[0] = NewLeafTri(a, m, c, leaf.Scorched, mesh) //left (clockwise wound)
	leaf.children[1] = NewLeafTri(a, b, m, leaf.Scorched, mesh) //right
	leaf.children[0].owner = leaf.owner
	leaf.children[1].owner = leaf.owner

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

	if tri.ChildCount == 0 {

		//	kink := (t.mesh.height/(float64(t.depth*t.depth)+1) - 1) * .5 //maximum kink in this edge
		//kink := (t.mesh.height/(float64(t.depth*5)+1) - 1) * .5 //maximum kink in this edge
		//kink := -t.mesh.height / math.Pow(2, float64(t.depth))
		kink := mesh.kinks[tri.Depth] * mesh.height //maximum kink in this edge

		m := mesh
		v0, v1, v2 := tri.vi[0], tri.vi[1], tri.vi[2]

		seed := uint64(m.verts[v1].P.Y)
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
	tri.Centre.AddInto(v[tri.vi[0]].P, v[tri.vi[1]].P, v[tri.vi[2]].P)
	tri.Centre.MulIn(float64(1.0 / 3.0))

	return tri.Centre
}

func (tri *Tri) area(mesh *TriMesh) float64 {
	v := mesh.verts
	a := v[tri.vi[0]].P
	b := v[tri.vi[1]].P
	c := v[tri.vi[2]].P
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
						//prober.ray.PointAt(pen) //move the ray end to the hit point (to only find points closer than this next time)
					}

				} else {
					prober.leafMisses++
				}
			}
		}

	} else {
		if tri.ChildCount == 0 {
			log.Logit("Should have found a firstLeaf for ", prober.DeviceId, "depth:", tri.Depth)
		}
		if tri.prismContains(prober.ray.Origin, prober.mesh) || tri.prismContains(prober.ray.End, prober.mesh) || tri.probePrism(prober.ray) {
			prober.prismHits++
			//for _, ct := range t.children {
			for i := 0; i < tri.ChildCount; i++ {
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
	a := v[tri.vi[0]].P
	b := v[tri.vi[1]].P
	c := v[tri.vi[2]].P

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
	if tri.PrismFaces[4] == nil {
		panic("all prisms should at least have a bottom face")
	}
	for i, s := range tri.PrismFaces {
		if s != nil {
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
	}

	return false
}

func (tri *Tri) vProbe(p vec.V3, mesh *TriMesh, deviceId uint32) *Tri {

	if tri.contains2D(p, mesh) {
		if tri.firstLeaf[deviceId] != nil {
			return tri
		}

		//for _, ct := range t.children {
		for i := 0; i < tri.ChildCount; i++ {

			tt := tri.children[i].vProbe(p, mesh, deviceId)
			if tt != nil {
				return tt
			}
		}

		log.Logit("No child contained point - but parent did", "depth:", tri.Depth, " children:", tri.ChildCount)

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

	epsilon := 0.001
	for i := range 3 {
		this := mesh.verts[tri.vi[i]].P
		next := mesh.verts[tri.vi[(i+1)%3]].P

		thisToNext := next.Sub(this)
		thisToP := p.Sub(this)

		if thisToNext.Cross(thisToP).Y < -epsilon {
			return false
		}
	}

	return true

}

func (leaf *LeafTri) CacheNormal(mesh *TriMesh) vec.V3 {

	// if leaf.normal.X != 0 || leaf.normal.Y != 0 || leaf.normal.Z != 0 {
	// 	return leaf.normal
	// }

	v := mesh.verts

	numMeshVerts := uint32(mesh.VertexCount)
	if leaf.Vi[0] >= numMeshVerts || leaf.Vi[1] >= numMeshVerts || leaf.Vi[2] >= numMeshVerts {
		panic("index out of range in tri.normal")
	}

	ab := v[leaf.Vi[1]].P.Sub(v[leaf.Vi[0]].P) //.Normalised()
	if ab.LengthSq() == 0 {
		panic("zero length edge in leaf tri")
	}

	ac := v[leaf.Vi[2]].P.Sub(v[leaf.Vi[0]].P) //.Normalised()
	if ac.LengthSq() == 0 {
		panic("zero length edge in leaf tri")
	}
	leaf.normal = (ab.Cross(ac)).Normalised()

	if leaf.normal.Y < 0 {
		panic("downward facing normal on leaf tri")
	}

	return leaf.normal

}

func (parent *Tri) ReUse(childIndex int, mesh *TriMesh, vi [3]uint32) *Tri {

	child := parent.children[childIndex]

	if child.yMax > -math.MaxFloat64 {
		log.Logit("child triangle yMax not reset")
	}

	child.vi = vi //set new vertices
	child.Init(mesh)

	return child
}

func (t *Tri) Init(m *TriMesh) {
	t.ChildCount = 0
	t.yMax = -math.MaxFloat64
	t.yMin = math.MaxFloat64
	t.xMax = -math.MaxFloat64
	t.xMin = math.MaxFloat64
	t.zMax = -math.MaxFloat64
	t.zMin = math.MaxFloat64

	t.PrismFaces = []*poly.ConvexPoly{nil, nil, nil, nil, nil}

	for i := 0; i < 3; i++ {
		v := m.verts[t.vi[i]]
		if v.Wl < 0 {
			//it's OK, that's a thing
			//panic("vertex with negative water level in newTri")
		}

		p := v.P
		t.updateExtents(p)
		p.Y += v.Wl //water surface
		t.updateExtents(p)

	}

	t.calcCentre(m)

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
	t := &Tri{Parent: parent,
		Depth: depth,
		vi:    vi, children: []*Tri{},
		firstLeaf: make(map[uint32]*LeafTri),
	}

	t.Init(m)

	return t
}
