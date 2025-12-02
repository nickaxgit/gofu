package terrain

import (
	"github.com/nickax/gofu/colors"
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/poly"
	"github.com/nickax/gofu/ray"
	"github.com/nickax/gofu/vec"
	//"github.com/nickax/gofu/cam"

	"fmt"
	"math"
	"math/rand/v2"
	"time"
)

var up = vec.NewVec3(0, 1, 0)
var nowhereSpecial = vec.NewVec3(0, -99999, 0)

type vert struct {
	p                  *vec.V3
	n                  *vec.V3
	uv                 *vec.V2
	wl                 float64       //water level
	touches            map[*Tri]bool //the triangles that touch this vertex (whos face normals contribute to the vertex normal)
	occluded           bool
	testedForOcclusion bool
}

type Tri struct {
	parent     *Tri
	Depth      int
	vi         []uint32
	children   []*Tri
	childCount int
	//mesh       *TriMesh //a reference to the mesh this tri is part of (that the vi's point into v's of)
	Normal   *vec.V3
	Scorched bool //note this is not part of the fireInfo - it's a cache of which land triangles are burned out
	Culled   bool
	occCount int //number of vertices occluded
	xMin     float64
	xMax     float64
	yMin     float64
	yMax     float64
	zMin     float64
	zMax     float64

	PrismFaces []*poly.ConvexPoly //3 sides plus bottom and top
	poly       *poly.ConvexPoly   // made/cached JIT
	//shadow  *poly.ConvexPoly //the trinagle pojected onto y=0 JIT/cached for vprobe
	centre   *vec.V3
	FireInfo *fireInfo //nil for land triangles

}

func (t *Tri) updateTrianglePoly(mesh *TriMesh) {

	if t.poly == nil {
		t.poly = poly.NewConvexPoly()
	} else {
		t.poly.PointCount = 0 //reset
	}
	for i := 0; i < 3; i++ {
		t.poly.AddPoint(mesh.verts[t.vi[i]].p)
	}

}

func (tri *Tri) reset() {
	tri.childCount = 0
	tri.Culled = false
	tri.occCount = 0
	tri.yMax = -math.MaxFloat64
	tri.yMin = math.MaxFloat64
	tri.xMax = -math.MaxFloat64
	tri.xMin = math.MaxFloat64
	tri.zMax = -math.MaxFloat64
	tri.zMin = math.MaxFloat64

	// for i := 0; i < 5; i++ {
	// 	if tri.PrismFaces[i] != nil {
	// 		tri.PrismFaces[i].PointCount = 0
	// 	}
	// }

	// if tri.poly != nil {
	// 	tri.poly.PointCount = 0 //DONT reset the root poly (it's never re-created)
	// }

	for _, c := range tri.children { //Reset *all* children (not just childcount - which is now zero)
		c.reset()
	}
}

func (tri *Tri) NewBottomCap(mesh *TriMesh, y float64) *poly.ConvexPoly {
	poly := poly.NewConvexPoly()
	//for _, v := range t.vi {
	for i := len(tri.vi) - 1; i >= 0; i-- {
		//poly.AddPoint(tri.mesh.verts[v].p.Add(vec.NewVec3(0, y, 0)))
		p := mesh.verts[tri.vi[i]].p.Clone()
		p.Y = y
		poly.AddPoint(p)
	}
	return poly
}

func (tri *Tri) NewTopCap(mesh *TriMesh, y float64) *poly.ConvexPoly {
	poly := poly.NewConvexPoly()
	for _, v := range tri.vi {
		//poly.AddPoint(tri.mesh.verts[v].p.Add(vec.NewVec3(0, y, 0)))
		p := mesh.verts[v].p.Clone()
		p.Y = y
		poly.AddPoint(p)
	}
	return poly
}

func (tri *Tri) Plough(mesh *TriMesh, runwayStart *vec.V3, runwayEnd *vec.V3, runwayWidth float64) {

	for _, vi := range tri.vi {
		pp := mesh.verts[vi].p.Clone()
		pp.Y = runwayStart.Y //move the vertex to the runway height
		if pp.DistanceFromLineSegment(runwayStart, runwayEnd) < runwayWidth*2 {
			//cp := p.closestPointOnLineSegment(runwayStart, runwayEnd)
			mesh.verts[vi].p.Y = runwayStart.Y
		}
	}
	tri.calcNormal(mesh) //SUPER important !

	//for _, ct := range t.children {
	for i := 0; i < tri.childCount; i++ {
		tri.children[i].Plough(mesh, runwayStart, runwayEnd, runwayWidth)
	}

}

func (tri *Tri) MakePrisms(mesh *TriMesh) {

	if tri.PrismFaces[0] == nil {
		tri.PrismFaces = make([]*poly.ConvexPoly, 5) //3 sides + top + bottom
		for i := 0; i < 5; i++ {
			tri.PrismFaces[i] = poly.NewConvexPoly()
		}
	} else {
		for i := 0; i < 5; i++ {
			tri.PrismFaces[i].PointCount = 0 //reset/reuse
		}
	}

	for i, v := range tri.vi {
		vp := mesh.verts[v].p
		vpn := mesh.verts[tri.vi[(i+1)%3]].p

		poly := tri.PrismFaces[i]
		//poly.addPointAt(vp.x, t.yMax, vp.z)

		//TODO endcaps and sides could share vec3 verts

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

		// c2c := poly.Centre().Sub(t.centre).Normalise()
		// ppn := poly.Plane.GetNormal()
		// if ppn.Dot(c2c) < 0 {
		// 	panic("side face normal incorrect")
		// }

	}

	//make endcaps

	tri.PrismFaces[3] = tri.NewTopCap(mesh, tri.yMax) //top
	if tri.PrismFaces[3].Plane.GetNormal().Y < 0 {
		panic("top cap normal incorrect")
	}
	//tri.prismFaces[5] = newTrianglePoly(tri)

	tri.PrismFaces[4] = tri.NewBottomCap(mesh, tri.yMin) //bottom
	if tri.PrismFaces[4].Plane.GetNormal().Y > 0 {
		panic("bottom cap normal incorrect")
	}

	//for _, child := range t.children {
	for i := 0; i < tri.childCount; i++ {
		tri.children[i].MakePrisms(mesh)
	}

}

// if the water level at ANY vertes - is higher than the Y coords
func (tri *Tri) OnOrUnderWater(mesh *TriMesh) bool {

	a := mesh.verts[tri.vi[0]]
	b := mesh.verts[tri.vi[1]]
	c := mesh.verts[tri.vi[2]]

	epsilon := 0.0001
	if a.wl >= a.p.Y-epsilon || b.wl >= b.p.Y-epsilon || c.wl >= c.p.Y-epsilon { //if all verts are at or under water level
		//if a.p.Y <= a.wl+epsilon || b.p.Y <= b.wl+epsilon || c.p.Y <= c.wl+epsilon { //if all verts are at water level

		return true
	}
	return false
}

func (tri *Tri) IsSubmerged(mesh *TriMesh) bool {

	a := mesh.verts[tri.vi[0]]
	b := mesh.verts[tri.vi[1]]
	c := mesh.verts[tri.vi[2]]

	if a.p.Y < a.wl && b.p.Y < b.wl && c.p.Y < c.wl { //if all verts are below water
		return true
	}
	return false
}

func (tri *Tri) updateExtents(p *vec.V3) { //yMin float64, yMax float64) {

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

	if tri.parent == nil && tri.Depth > 0 {
		log.Logit("ORPHANED TRIANGLE")
	}
	if tri.parent != nil {
		tri.parent.updateExtents(p) //<bubble up and update all ancestors
	}
}

// keep track of the deepest (up to) 6 triangles touching this vert -- allows us to recalculate normals quickly
func (v *vert) touch(t ...*Tri) {

	for _, t := range t {
		v.touches[t] = true
		if len(v.touches) > 6 {
			//panic("vertex touched by more than 6 triangles")
		}
	}
}

func newVert(p *vec.V3, u, v float64) *vert {
	return &vert{p: p, uv: vec.NewVec2(u, v), n: vec.NewVec3(0, 0, 0), touches: make(map[*Tri]bool, 6)}
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

func (tri *Tri) find2D(p *vec.V3) *Tri {

	if tri.contains2D(p) {
		if tri.childCount == 0 {
			return tri
		}

		scorched := 0
		//for _, c := range t.children {
		for i := 0; i < tri.childCount; i++ {

			f := tri.children[i].find2D(p)
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

func (parent *Tri) addChild(mesh *TriMesh, vi ...uint32) *Tri {
	if parent.childCount < len(parent.children) {

		child := parent.ReUse(parent.childCount, mesh, vi...)

		parent.childCount++
		return child

	} else {
		child := newTri(parent, mesh, vi...) //t.mesh.makeTri(t.depth+1, fi, a, b, c)
		parent.children = append(parent.children, child)
		parent.childCount++ //= len(t.children)
		return child
	}

}

// scorch - recurse through all land triangles flagging them as scorched by checking their centres in the fire mesh
func (tri *Tri) Scorch(fire *TriMesh) {
	if tri.childCount == 0 && !tri.Culled {
		if fire.scorchedAt(fire.Root, tri.centre) {
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
func (tri *Tri) Patch(mesh *TriMesh) {

	if tri.Culled {
		panic("Culled triangle in Patch")
	}

	if tri.childCount == 0 {

		for i := 0; i < 3; i++ {

			ai := tri.vi[i]
			bi := tri.vi[(i+1)%3]
			ci := tri.vi[(i+2)%3]
			//mi := m.midpoint(ai, bi) //looks both ways for a midpoint   -- m.vertAtMidPointXZ(ai, bi) //this could be way faster by recording mid verts
			mi := mesh.midpoint(bi, ci) //looks both ways for a midpoint   -- m.vertAtMidPointXZ(ai, bi) //this could be way faster by recording mid verts

			if mi != math.MaxUint32 { //is there a midpoint ?

				//if t.aspect() < 2 { //is it 'fat'
				tri.splitIn2(mesh, ai, bi, ci, mi)
				break //only one edge of this tri (becuase it is now multiple child tris)
				//} else {
				//	t.split()
				//}

			}

		}

	}

	//for _, c := range t.children {
	for i := 0; i < tri.childCount; i++ {
		tri.children[i].Patch(mesh)
	}

}

func (tri *Tri) addToTouches(mesh *TriMesh) {

	mesh.verts[tri.vi[0]].touch(tri)
	mesh.verts[tri.vi[1]].touch(tri)
	mesh.verts[tri.vi[2]].touch(tri)

}

func (tri *Tri) removeFromTouches(mesh *TriMesh) {
	if tri.childCount > 0 {
		panic("Tri has children")
	}
	for _, vi := range tri.vi {
		v := mesh.verts[vi]
		delete(v.touches, tri) //remove this tri from the list of tris touching this vertex
	}
}

func (tri *Tri) facesTowards(direction *vec.V3) bool {
	//the extra -.1 is to account for traingles facing away at less than half the camera vertical FOV
	return tri.Normal.Dot(direction) < -.1 //is the traingle forward facing ? (relative to the camera)

}

func (tri *Tri) allVertsLeftOrRightOfFov(mesh *TriMesh, camPos *vec.V3, camDir *vec.V3, fov float64) bool {

	onLeft := 0
	behind := 0
	for _, vi := range tri.vi {
		cam2vert := mesh.verts[vi].p.Sub(camPos).Normalise()
		dp := camDir.Dot(cam2vert)
		if dp > fov {
			return false //a vertex is within the FOV
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

func (tri *Tri) hasVertexWithinFov(mesh *TriMesh, pos *vec.V3, focus *vec.V3, fov float64) bool {

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

// this might be *much* faster if we checked all 6 points of the prisms for occlusion recrsively
// if a prism is fully occluded - all its children are too
func (tri *Tri) Occlude(mesh *TriMesh, camPos *vec.V3) {

	viewpoint := camPos.Clone()
	viewpoint.Y += 5 //

	occluded := 0
	//defining a slice once, and using/resetting a penetration count is faster

	totalDepth := 0
	maxDepth := 0 //how deep did we go (in any one recursion)
	deepestEver := 0
	backfacing := 0

	ray := ray.New(viewpoint, nowhereSpecial)

	ts := time.Now()
	leaves := tri.getLeaves()

	stats := NewProbeStats()

	shortTarget := vec.NewVec3(0, 0, 0)
	rd := vec.NewVec3(0, 0, 0)
	for _, leaf := range leaves {

		// if leaf.poly == nil {
		// 	leaf.poly = newTrianglePoly(leaf, mesh) //cache the polygon
		// }

		//backFacecull test
		ray.PointAt(leaf.centre)
		if ray.GetDirection().Dot(leaf.Normal) > 0 {
			//triangle is backfacing - it will be culled anyway
			leaf.Culled = true
		} else {
			//occlusion cull test

			for i := 0; i < 3; i++ {
				v := mesh.verts[leaf.vi[i]]
				if !v.testedForOcclusion {

					ray.PointAt(v.p)
					rd = ray.GetDirection()
					shortTarget.X = ray.Origin.X + rd.X*.999
					shortTarget.Y = ray.Origin.Y + rd.Y*.999
					shortTarget.Z = ray.Origin.Z + rd.Z*.999

					ray.PointAt(shortTarget)

					stats.hit = false //clear the hit (we acculumulate in the stats object)

					tri.probe(mesh, ray, stats, 0)
					if stats.hit {
						v.occluded = true
						occluded++
					}
					v.testedForOcclusion = true
				}
				if v.occluded {
					leaf.occCount++
					if leaf.occCount == 3 {
						leaf.Culled = true
						break
					}
					if leaf.occCount > 3 {
						panic(fmt.Sprintf("occCount >3 on tri %v", leaf.vi))
					}
				}
			}
		}

		totalDepth += stats.maxDepth

		if maxDepth > deepestEver {
			deepestEver = maxDepth
		}

	}

	avgDepth := float64(totalDepth) / float64(mesh.VertCount())
	ms := time.Since(ts).Milliseconds()

	log.Logit("occluded", occluded, " of ", mesh.VertCount(), " verts",
		" backfacing:", backfacing,
		" max depth:", deepestEver,
		" avg depth:", avgDepth,
		" time:", ms, "ms",
	)
	log.Logit(stats.String())

}

// PrismEdges - gathers the edges of this and all ancestor prisms into MSG as vectors for  debugging
func (tri *Tri) PrismEdges(msg *msg.Msg) {

	//gather the (three) uprights from the sides
	if tri.childCount > 0 {
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
	}

	//gather all ancestor prisms
	if tri.parent != nil {
		tri.parent.PrismEdges(msg)
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
func (tri *Tri) CalcVerticalExtents(mesh *TriMesh) { //called on the root triangle
	if tri.childCount == 0 {
		m := mesh
		//yMin := m.verts[t.vi[0]].p.Y
		//yMax := yMin
		for _, vi := range tri.vi {
			//yMin = math.Min(yMin, m.verts[vi].p.Y)
			//yMax = math.Max(yMax, m.verts[vi].p.Y)
			tri.updateExtents(m.verts[vi].p) //recursively bubble up and update all ancestors extents
		}
		if tri.yMax-tri.yMin < 0.01 {
			//log.Logit("flat triangle detected", t.yMax, t.yMin, t.Depth)
		}

		//t.updateExtents(yMin, yMax) //recursively bubble up and update all ancestors extents
	} else {
		//for _, c := range t.children {
		for i := 0; i < tri.childCount; i++ {
			tri.children[i].CalcVerticalExtents(mesh) //recursively drill down to leaf triangles
		}
	}
}

func (tri *Tri) SplitIfNeeded(mesh *TriMesh, camPos *vec.V3, camDir *vec.V3, fov float64) {

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

	//inFov :=
	if tri.Depth < 5 || !tri.allVertsLeftOrRightOfFov(mesh, camPos, camDir, fov) {
		//if t.Depth < 5 || t.Normal.Dot(camDir) < .2 { //is the traingle forward facing ? (relative to the camera)
		//if t.normal.dot((t.centre().sub(pos)).normalise()) < 0.3 { //lower number here cull more backfacing tris

		shouldBeSplitToLevel := 5.0

		if tri.Depth >= 5 {
			distSQ := camPos.DistanceSQ(tri.centre)
			//dist := camPos.DistanceFrom(t.centre)

			//apud := (2 * t.area()) / (0.0002 * (distSQ))
			//splitPressure := distSQ / float64((t.Depth+1)*(t.Depth+1)*(t.Depth+1)*(t.Depth+1)) //area per unit distance squared
			//splitPressure = float64(t.Depth/15) / ((dist+10)/(t.mesh.size *2)) // (t.mesh.size * 2)) //*(t.Depth)) //area per unit distance squared

			//we want triangles at 3 metres split to level 15 - and those at 10,000 metres split to level 5
			shouldBeSplitToLevel = 12 - math.Log10(distSQ/2) //)*2 // (dist*dist-9)/(2000*2000) // * (5-15) + 15

			dp := camDir.Dot(tri.centre.Sub(camPos).Normalise())
			if dp < 0.1 {
				dp = 0.1
			} //don't penalise *too* much for being behind
			shouldBeSplitToLevel += dp * 4 //bring front and centre triangles forward up to 4 levels

			//splitPressure *= (.1 + dp) // / distSQ //.Normalise())
		}
		//at a value of 1 (area per unit distance), a notional 100 square metre square, would require splitting when it was 10 metres away
		//if apud > 8-(dp*4) || t.Depth < 5 { //.001 is a about 1cm triangles at the horizon
		if tri.Depth < int(shouldBeSplitToLevel) {
			tri.split(mesh)
			//for _, c := range t.children {
			for i := 0; i < tri.childCount; i++ {
				tri.children[i].SplitIfNeeded(mesh, camPos, camDir, fov) //recurse
			}
		}

	}

}

func (tri *Tri) splitDownTo(mesh *TriMesh, level int) {

	if tri.Depth < level {
		tri.split(mesh)
		//for _, c := range t.children {
		for i := 0; i < tri.childCount; i++ {
			tri.children[i].splitDownTo(mesh, level) //recurse
		}
	}

}

func (tri *Tri) splitIn2(mesh *TriMesh, a, b, c, m uint32) {

	tri.removeFromTouches(mesh) //remove this tri from the list tris touching this vertex
	tri.addChild(mesh, a, m, c) //left (clockwise wound)
	tri.addChild(mesh, a, b, m) //right

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

		tri.removeFromTouches(mesh) //the list of triangles touching a vertex is used for normal calculation
		//Always put the horizontal edge in first
		//wind clockwise
		tri.addChild(mesh, v5, v4, v2) //top
		tri.addChild(mesh, v0, v3, v5) //right
		tri.addChild(mesh, v3, v1, v4) //left
		tri.addChild(mesh, v4, v5, v3) //centre

	} else {
		log.Logit("splitting a triangle that already has children ??")
	}
}

func (tri *Tri) calcCentre(mesh *TriMesh) *vec.V3 {
	v := mesh.verts
	tri.centre = vec.NewVec3(0, 0, 0)
	tri.centre.AddInto(v[tri.vi[0]].p, v[tri.vi[1]].p, v[tri.vi[2]].p)
	tri.centre.MulIn(float64(1.0 / 3.0))
	//t.centre.Y += 0.1

	return tri.centre
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
func (tri *Tri) probe(mesh *TriMesh, ray *ray.Ray, stats *ProbeStats, depth int) {

	if depth > stats.maxDepth {
		stats.maxDepth = depth
	}

	if tri.childCount == 0 {

		//if tri.poly == nil {
		//	tri.poly = newTrianglePoly(tri, mesh) //cache the polygon
		//}

		if tri.Culled {
			stats.skippedCulled++
			return
		}

		// if ray.End == t.target {
		// 	*done = true      //check if this has any effect on occlusion counts
		// 	hitsomereturn false, nil //dont let triangles self occlude
		// }

		if tri.poly.Probe(ray) {
			stats.leafHits++
			stats.hit = true //exit signal
			//pen := ray.Intersect.Clone()
			//*done = true
		} else {
			stats.leafMisses++
		}

	} else {

		if ray.Origin.Y > tri.yMax && ray.End.Y > tri.yMax { //|| ray.Origin.Y < t.yMin && ray.End.Y < t.yMin {
			stats.skips++
		}

		if tri.prismContains(ray.Origin) || tri.prismContains(ray.End) || tri.probePrism(ray) {
			stats.prismHits++
			//for _, ct := range t.children {
			for i := 0; i < tri.childCount; i++ {
				ct := tri.children[i]

				if ray.End.Y > ct.yMax && ray.Origin.Y > ct.yMax ||
					ray.End.Y < ct.yMin && ray.Origin.Y < ct.yMin ||
					ray.End.X > ct.xMax && ray.Origin.X > ct.xMax ||
					ray.End.X < ct.xMin && ray.Origin.X < ct.xMin ||
					ray.End.Z > ct.zMax && ray.Origin.Z > ct.zMax ||
					ray.End.Z < ct.zMin && ray.Origin.Z < ct.zMin {
					stats.skips++
					continue
				}

				ct.probe(mesh, ray, stats, depth+1)
				if stats.hit {
					return
				}
			}

		} else {
			//none of the triangles in this prism need checking
			stats.prismMisses++

		}
	}

}

type ProbeStats struct {
	hit           bool //used in occlusiuon cull
	leafHits      int
	leafMisses    int
	prismHits     int
	prismMisses   int
	skips         int
	nearestHit    *vec.V3
	NearestTri    *Tri
	skippedCulled int
	maxDepth      int
	SDist         float64 //smallest distance found sofar (start big) - used if ProbeAll()
}

func NewProbeStats() *ProbeStats {
	return &ProbeStats{
		SDist: math.MaxFloat64,
	}
}

func (S *ProbeStats) String() string {
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

// probeAll - find all intersections along the ray, returning the nearest hit point
func (tri *Tri) ProbeAll(mesh *TriMesh, ray *ray.Ray, stats *ProbeStats) {

	if tri.childCount == 0 {
		//if tri.poly == nil {
		//	tri.poly = newTrianglePoly(tri, mesh) //cache the polygon
		//}

		if tri.Culled {
			stats.skippedCulled++
		} else {
			if tri.poly.Probe(ray) {
				stats.leafHits++
				pen := ray.Intersect.Clone()
				d := pen.DistanceFrom(ray.Origin)
				if d < stats.SDist {
					stats.SDist = d
					stats.nearestHit = pen
					stats.NearestTri = tri
					ray.PointAt(pen) //move the ray end to the hit point
				}
			} else {
				stats.leafMisses++
			}
		}
	} else {

		if tri.prismContains(ray.Origin) || tri.prismContains(ray.End) || tri.probePrism(ray) {
			stats.prismHits++
			//for _, ct := range t.children {
			for i := 0; i < tri.childCount; i++ {
				ct := tri.children[i]

				if ray.End.Y > ct.yMax && ray.Origin.Y > ct.yMax || ray.End.Y < ct.yMin && ray.Origin.Y < ct.yMin {
					stats.skips++
					continue
				}
				ct.ProbeAll(mesh, ray, stats)
			}
		} else {
			//none of the triangles in this prism need checking
			stats.prismMisses++
		}
	}

}

// EdgesAsMsg - return the edges of this triangle as a msg for clientside rendering/debugging
func (tri *Tri) EdgesAsMsg(mesh *TriMesh) *msg.Msg {

	v := mesh.verts
	a := v[tri.vi[0]].p
	b := v[tri.vi[1]].p
	c := v[tri.vi[2]].p

	return msg.NewMsg(
		msg.Vectors,
		a, b, colors.Magenta,
		b, c, colors.Magenta,
		c, a, colors.Magenta,
	)

}

func (tri *Tri) prismContains(p *vec.V3) bool {

	if p.Y < tri.yMin || p.Y > tri.yMax {
		return false //outside the vertical extents of the prism
	}

	//we are within the vertical extents - we now do a 2d check agains the footprint
	return tri.contains2D(p)

}

// test if the ray from p0 to p1 penetrates the volume of triangular based 'prism' extending between t.ymin and t.ymax
func (tri *Tri) probePrism(ray *ray.Ray) bool {

	epsilon := 0.001
	//for all five sides of the prism - check for a penetration
	for i, s := range tri.PrismFaces {
		if s == nil {
			log.Logit("nil prism face in probePrism", i)
			return false
			//panic("nil prism face")
		}
		if s.Probe(ray) {
			if ray.Intersect.Y > tri.yMax+epsilon || ray.Intersect.Y < tri.yMin-epsilon {
				log.Logit("penetration is outside the vertical extents of the prism", i)
			}
			return true
		}
	}

	return false
}

func (tri *Tri) VprobeLand(p *vec.V3) (surfacePoint *vec.V3, surfaceTri *Tri) {

	t := tri.vProbe(p) //recursively find the leaf tri that contains the point

	if t == nil {
		return nil, nil
	}

	//fire a ray through that plane
	ray := ray.New(vec.NewVec3(p.X, 100000, p.Z), vec.NewVec3(p.X, -100000, p.Z))
	//if t.prismFaces[5].Probe(ray) {
	if t.poly.Probe(ray) {
		return ray.Intersect, t
	}

	t.poly.Probe(ray)
	return nil, nil
}

func (tri *Tri) contains2D(p *vec.V3) bool {

	if tri.poly == nil {
		panic("No poly) cached for tri in contains2D")
	}

	return tri.poly.Contains2D(p)
}

func (tri *Tri) vProbe(p *vec.V3) *Tri {

	if tri.contains2D(p) {
		if tri.childCount == 0 {
			return tri
		}

		//for _, ct := range t.children {
		for i := 0; i < tri.childCount; i++ {

			tt := tri.children[i].vProbe(p)
			if tt != nil {
				return tt
			}
		}

		log.Logit("No child contained point - but parent did")

	}

	return nil

	//panic("vProbe failed to find a tri")

}

func (tri *Tri) calcNormal(mesh *TriMesh) *vec.V3 {

	v := mesh.verts

	numMeshVerts := uint32(mesh.VertCount())
	if tri.vi[0] >= numMeshVerts || tri.vi[1] >= numMeshVerts || tri.vi[2] >= numMeshVerts {
		panic("index out of range in tri.normal")
	}

	//n1 := v[t.Vi[1]].p.sub(v[t.Vi[0]].p).cross(v[t.Vi[2]].p.sub(v[t.Vi[0]].p)).normalise()
	//log.Logit(n1.X, n1.Y, n1.Z)
	ab := v[tri.vi[1]].p.Sub(v[tri.vi[0]].p) //.normalise()
	ac := v[tri.vi[2]].p.Sub(v[tri.vi[0]].p) //.normalise()
	n2 := (ab.Cross(ac)).Normalise()
	ln := n2.Length()
	if ln < .999 || ln > 1.00001 {
		panic("normal is not unit length")
	}

	tri.Normal = n2
	return n2
}

func (parent *Tri) ReUse(childIndex int, mesh *TriMesh, vi ...uint32) *Tri {
	child := parent.children[childIndex]
	child.vi = vi

	if child.Depth != parent.Depth+1 {
		panic("reused child triangle has wrong depth")
	}

	//child.poly.PointCount=0 // = newTrianglePoly(child, mesh)

	child.calcCentre(mesh)
	child.updateTrianglePoly(mesh)
	//add this traingle to its verts list of triangles
	child.addToTouches(mesh)

	child.calcNormal(mesh)
	if child.Normal.Y < 0 {
		panic("triangle with downward normal")
	}
	return child
}

func newTri(parent *Tri, m *TriMesh, vi ...uint32) *Tri {

	if vi[0] == vi[1] || vi[0] == vi[2] || vi[1] == vi[2] {
		panic("degenerate triangle")
	}

	p0 := m.verts[vi[0]].p
	p1 := m.verts[vi[1]].p
	p2 := m.verts[vi[2]].p

	if p0.Equals(p1) || p0.Equals(p2) || p1.Equals(p2) {
		panic("infinitely thin triangle")
	}

	//t := Tri{depth: depth, vi: vi, children: []*Tri{}, mesh: m, faceIndex: fi}
	depth := 0
	if parent != nil {
		depth = parent.Depth + 1
	}
	t := &Tri{parent: parent, Depth: depth, vi: vi, children: []*Tri{},
		xMin: math.MaxFloat64, xMax: -math.MaxFloat64,
		yMin: math.MaxFloat64, yMax: -math.MaxFloat64,
		zMin: math.MaxFloat64, zMax: -math.MaxFloat64,
		PrismFaces: []*poly.ConvexPoly{nil, nil, nil, nil, nil},
	}

	//t.poly = newTrianglePoly(t, m)
	t.updateTrianglePoly(m)

	t.calcCentre(m)
	//add this traingle to its verts list of triangles
	t.addToTouches(m)

	t.calcNormal(m)
	if t.Normal.Y < 0 {
		panic("triangle with downward normal")
	}

	return t
}
