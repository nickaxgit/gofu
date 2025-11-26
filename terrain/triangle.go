package terrain

import (
	"github.com/nickax/gofu/colors"
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/mesh"
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

type tcs struct {
	left, top, right, bottom float64
}

type vert struct {
	p                  *vec.V3
	n                  *vec.V3
	uv                 *vec.V2
	wl                 float64       //water level
	acc                float64       //accumulated water (during a pass)
	incount            int           //number of ferts flowing into this vert
	touches            map[*Tri]bool //the triangles that touch this vertex (whos face normals contribute to the vertex normal)
	occluded           bool
	testedForOcclusion bool
}

type Tri struct {
	parent   *Tri
	Depth    int
	vi       []uint32
	children []*Tri
	mesh     *TriMesh //a reference to the mesh this tri is part of (that the vi's point into v's of)
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
	centre *vec.V3
	target *vec.V3
	//flat       *poly.ConvexPoly //cache of the flat polygon for 2d contains testing
	FireInfo *fireInfo //nil for land triangles
	isPatch  bool      //whether this triangle was created as part of patching
}

func newTrianglePoly(t *Tri) *poly.ConvexPoly {
	poly := poly.NewConvexPoly()
	for i := 0; i < 3; i++ {
		poly.AddPoint(t.mesh.verts[t.vi[i]].p)
	}
	return poly
}

func (t *Tri) NewBottomCap(y float64) *poly.ConvexPoly {
	poly := poly.NewConvexPoly()
	//for _, v := range t.vi {
	for i := len(t.vi) - 1; i >= 0; i-- {
		//poly.AddPoint(tri.mesh.verts[v].p.Add(vec.NewVec3(0, y, 0)))
		p := t.mesh.verts[t.vi[i]].p.Clone()
		p.Y = y
		poly.AddPoint(p)
	}
	return poly
}

func (t *Tri) NewTopCap(y float64) *poly.ConvexPoly {
	poly := poly.NewConvexPoly()
	for _, v := range t.vi {
		//poly.AddPoint(tri.mesh.verts[v].p.Add(vec.NewVec3(0, y, 0)))
		p := t.mesh.verts[v].p.Clone()
		p.Y = y
		poly.AddPoint(p)
	}
	return poly
}

func (t *Tri) Plough(runwayStart *vec.V3, runwayEnd *vec.V3, runwayWidth float64) {

	m := t.mesh

	for _, vi := range t.vi {
		pp := m.verts[vi].p.Clone()
		pp.Y = runwayStart.Y //move the vertex to the runway height
		if pp.DistanceFromLineSegment(runwayStart, runwayEnd) < runwayWidth*2 {
			//cp := p.closestPointOnLineSegment(runwayStart, runwayEnd)
			m.verts[vi].p.Y = runwayStart.Y
		}
	}
	t.calcNormal() //SUPER important !

	for _, ct := range t.children {
		ct.Plough(runwayStart, runwayEnd, runwayWidth)
	}

}

func (t *Tri) MakePrisms() {

	//let the Leaves have prisms (for debugginh)
	if len(t.children) == 0 {
		return //leaf triangles don't need prisms (it's cheaper and more accurate to test against the triangle itself)
	}

	//check winding !
	for i, v := range t.vi {
		vp := t.mesh.verts[v].p
		vpn := t.mesh.verts[t.vi[(i+1)%3]].p
		poly := poly.NewConvexPoly()
		//poly.addPointAt(vp.x, t.yMax, vp.z)

		//TODO endcaps and sides could share vec3 verts

		tl := vpn.Clone()
		tl.Y = t.yMax

		tr := vp.Clone()
		tr.Y = t.yMax

		br := vp.Clone()
		br.Y = t.yMin

		bl := vpn.Clone()
		bl.Y = t.yMin

		poly.AddPoint(tl)
		poly.AddPoint(tr)
		poly.AddPoint(br)
		poly.AddPoint(bl)
		t.PrismFaces[i] = poly

		// c2c := poly.Centre().Sub(t.centre).Normalise()
		// ppn := poly.Plane.GetNormal()
		// if ppn.Dot(c2c) < 0 {
		// 	panic("side face normal incorrect")
		// }

	}

	//make endcaps

	t.PrismFaces[3] = t.NewTopCap(t.yMax) //top
	if t.PrismFaces[3].Plane.GetNormal().Y < 0 {
		panic("top cap normal incorrect")
	}
	//tri.prismFaces[5] = newTrianglePoly(tri)

	t.PrismFaces[4] = t.NewBottomCap(t.yMin) //bottom
	if t.PrismFaces[4].Plane.GetNormal().Y > 0 {
		panic("bottom cap normal incorrect")
	}

	for _, child := range t.children {
		child.MakePrisms()
	}

}

// if the water level at ANY vertes - is higher than the Y coords
func (t *Tri) OnOrUnderWater() bool {
	m := t.mesh
	a := m.verts[t.vi[0]]
	b := m.verts[t.vi[1]]
	c := m.verts[t.vi[2]]

	epsilon := 0.0001
	if a.wl >= a.p.Y-epsilon || b.wl >= b.p.Y-epsilon || c.wl >= c.p.Y-epsilon { //if all verts are at or under water level
		//if a.p.Y <= a.wl+epsilon || b.p.Y <= b.wl+epsilon || c.p.Y <= c.wl+epsilon { //if all verts are at water level

		return true
	}
	return false
}

func (t *Tri) IsSubmerged() bool {

	m := t.mesh
	a := m.verts[t.vi[0]]
	b := m.verts[t.vi[1]]
	c := m.verts[t.vi[2]]

	if a.p.Y < a.wl && b.p.Y < b.wl && c.p.Y < c.wl { //if all verts are below water
		return true
	}
	return false
}

func (t *Tri) getFacesInto(fis []uint16, p *uint32, test func(face *Tri) bool) {
	if len(t.children) == 0 {
		j := *p
		if test(t) {
			fis[j] = uint16(t.vi[0])
			fis[j+1] = uint16(t.vi[1])
			fis[j+2] = uint16(t.vi[2])
			*p += 3
		} else {
			//log.Logit("tri rejected at depth", t.Depth)
		}
	}
	for _, c := range t.children {
		c.getFacesInto(fis, p, test)
	}
}

func (t *Tri) updateExtents(p *vec.V3) { //yMin float64, yMax float64) {

	if p.X < t.xMin {
		t.xMin = p.X
	}
	if p.X > t.xMax {
		t.xMax = p.X
	}
	if p.Y < t.yMin {
		t.yMin = p.Y
	}
	if p.Y > t.yMax {
		t.yMax = p.Y
	}

	if p.Z < t.zMin {
		t.zMin = p.Z
	}
	if p.Z > t.zMax {
		t.zMax = p.Z
	}

	if t.parent == nil && t.Depth > 0 {
		log.Logit("ORPHANED TRIANGLE")
	}
	if t.parent != nil {
		t.parent.updateExtents(p) //<bubble up and update all ancestors
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
// 		}
// 	}
// 	return tri.flat.Contains(p)
// }

func (t *Tri) find2D(p *vec.V3) *Tri {

	if t.contains2D(p) {
		if len(t.children) == 0 {
			return t
		}

		scorched := 0
		for _, c := range t.children {

			f := c.find2D(p)
			if f != nil {
				return f
			}
		}
		if scorched == len(t.children) {
			t.FireInfo.flames = -1 //mark parent as scorched
			t.children = nil
		}
	} else {
		return nil
	}
	log.Logit("warn: fTri find failed to find a tri")
	return nil
}

func (t *Tri) addChild(isPatch bool, vi ...uint32) *Tri {
	child := newTri(t, t.mesh, t.Depth+1, vi...) //t.mesh.makeTri(t.depth+1, fi, a, b, c)
	child.isPatch = isPatch
	t.children = append(t.children, child)
	return child
}

func (t *Tri) scorchedAt(p *vec.V3) bool {
	if len(t.children) == 0 {
		return t.Scorched
	} else {
		for _, c := range t.children {
			//if c.prismFaces[5].Contains(p) {
			if c.contains2D(p) {
				return c.scorchedAt(p)
			}
		}
		return false
	}
}

func (t *Tri) FetchTrees(depth int, positions []float32, bbm *mesh.SimpleMesh, camPos *vec.V3, camDir *vec.V3, ray *ray.Ray, hidden *int) {

	mid := t.centre
	tcs := mesh.NewTcs(0, 1, 1, 0)

	treeTop := vec.NewVec3(0, 0, 0) //
	if t.Depth == depth {
		if !t.scorchedAt(mid) && !t.OnOrUnderWater() {

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
					if t.poly.Probe(ray) {
						*hidden++
						return
					}

					//place a (instanced mesh) tree here
					positions = append(positions, mid.AsFloat32s()...)
				}
			} else { //it's a faraway tree - only place it if the ground slopes towards the camera
				if dotProd > .25 { //trees generally in front of the camera}
					if toTree.Dot(t.Normal) < 0 { //if the triangle slopes towards camera
						bbm.Billboard(mid, up, camPos, 20, 20, 20, 4, tcs) //billboard tree
					}

				}
			}
		}
	} else {
		for _, c := range t.children {
			c.FetchTrees(depth, positions, bbm, camPos, camDir, ray, hidden)
		}
	}

}

// scorch - recurse through all land triangles flagging them as scorched by checking their centres in the fire mesh
func (t *Tri) Scorch(fire *TriMesh) {
	if len(t.children) == 0 && !t.Culled {
		if fire.scorchedAt(t.centre) {
			t.Scorched = true
		}
	}
	for _, c := range t.children {
		c.Scorch(fire)
	}
}

// for every bottom level triangle, look to see if there is a vertex at the midpoint of each edge (caused by a more divided neighbouring tri)
// if so, split in two to the opposite vertex
func (t *Tri) Patch() {

	if t.Culled {
		panic("Culled triangle in Patch")
	}

	m := t.mesh
	if len(t.children) == 0 {

		for i := 0; i < 3; i++ {

			ai := t.vi[i]
			bi := t.vi[(i+1)%3]
			ci := t.vi[(i+2)%3]
			//mi := m.midpoint(ai, bi) //looks both ways for a midpoint   -- m.vertAtMidPointXZ(ai, bi) //this could be way faster by recording mid verts
			mi := m.midpoint(bi, ci) //looks both ways for a midpoint   -- m.vertAtMidPointXZ(ai, bi) //this could be way faster by recording mid verts

			if mi != math.MaxUint32 { //is there a midpoint ?

				//if t.aspect() < 2 { //is it 'fat'
				t.splitIn2(ai, bi, ci, mi)
				break //only one edge of this tri (becuase it is now multiple child tris)
				//} else {
				//	t.split()
				//}

			}

		}

	}

	for _, c := range t.children {
		c.Patch()
	}

}

func (t *Tri) addToTouches() {

	m := t.mesh
	m.verts[t.vi[0]].touch(t)
	m.verts[t.vi[1]].touch(t)
	m.verts[t.vi[2]].touch(t)

}

func (t *Tri) removeFromTouches() {
	if len(t.children) > 0 {
		panic("Tri has children")
	}
	for _, vi := range t.vi {
		v := t.mesh.verts[vi]
		delete(v.touches, t) //remove this tri from the list of tris touching this vertex
	}
}

func (t *Tri) facesTowards(direction *vec.V3) bool {
	//the extra -.1 is to account for traingles facing away at less than half the camera vertical FOV
	return t.Normal.Dot(direction) < -.1 //is the traingle forward facing ? (relative to the camera)

}

func (t *Tri) allVertsLeftOrRightOfFov(camPos *vec.V3, camDir *vec.V3, fov float64) bool {

	onLeft := 0
	behind := 0
	for _, vi := range t.vi {
		cam2vert := t.mesh.verts[vi].p.Sub(camPos).Normalise()
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

func (t *Tri) hasVertexWithinFov(pos *vec.V3, focus *vec.V3, fov float64) bool {

	camDir := focus.Sub(pos).Normalise()
	for _, vi := range t.vi {
		cam2vert := t.mesh.verts[vi].p.Sub(pos).Normalise()
		if camDir.Dot(cam2vert) > fov {
			return true //a vertex is within the FOV
		}
	}

	return false //no vertices are within the FOV

}

func (t *Tri) countChildren(count *int) {

	*count += len(t.children)

	for _, c := range t.children {
		c.countChildren(count)
	}

}

func (t *Tri) getLeaves() []*Tri {
	leaves := []*Tri{}
	if len(t.children) == 0 {
		leaves = append(leaves, t)
	}

	for _, c := range t.children {
		leaves = append(leaves, c.getLeaves()...)
	}

	return leaves
}

// this might be *much* faster if we checked all 6 points of the prisms for occlusion recrsively
// if a prism is fully occluded - all its children are too
func (root *Tri) Occlude(camPos *vec.V3) {

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
	leaves := root.getLeaves()

	stats := NewProbeStats()

	shortTarget := vec.NewVec3(0, 0, 0)
	rd := vec.NewVec3(0, 0, 0)
	for _, leaf := range leaves {

		if leaf.poly == nil {
			leaf.poly = newTrianglePoly(leaf) //cache the polygon
		}

		//backFacecull test
		ray.PointAt(leaf.centre)
		if ray.GetDirection().Dot(leaf.Normal) > 0 {
			//triangle is backfacing - it will be culled anyway
			leaf.Culled = true
		} else {
			//occlusion cull test

			for i := 0; i < 3; i++ {
				v := leaf.mesh.verts[leaf.vi[i]]
				if !v.testedForOcclusion {

					ray.PointAt(v.p)
					rd = ray.GetDirection()
					shortTarget.X = ray.Origin.X + rd.X*.999
					shortTarget.Y = ray.Origin.Y + rd.Y*.999
					shortTarget.Z = ray.Origin.Z + rd.Z*.999

					ray.PointAt(shortTarget)

					stats.hit = false //clear the hit (we acculumulate in the stats object)

					root.probe(ray, stats, 0)
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

	avgDepth := float64(totalDepth) / float64(len(root.mesh.verts))
	ms := time.Since(ts).Milliseconds()

	log.Logit("occluded", occluded, " of ", len(root.mesh.verts), " verts",
		" backfacing:", backfacing,
		" max depth:", deepestEver,
		" avg depth:", avgDepth,
		" time:", ms, "ms",
	)
	log.Logit(stats.String())

}

// PrismEdges - gathers the edges of this and all ancestor prisms into MSG as vectors for  debugging
func (t *Tri) PrismEdges(msg *msg.Msg) {

	//gather the (three) uprights from the sides
	if len(t.children) > 0 {
		for i := 0; i < 3; i++ {
			poly := t.PrismFaces[i]
			a := poly.P[0] //top left
			b := poly.P[3] //bottom left
			msg.Write(a, b, colors.White)
		}

		top := t.PrismFaces[3]
		bottom := t.PrismFaces[4]

		for i := 0; i < 3; i++ {
			n := (i + 1) % 3
			msg.Write(top.P[i], top.P[n], colors.Red)
			msg.Write(bottom.P[i], bottom.P[n], colors.Blue)
		}
	}

	//gather all ancestor prisms
	if t.parent != nil {
		t.parent.PrismEdges(msg)
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
func (t *Tri) CalcVerticalExtents() { //called on the root triangle
	if len(t.children) == 0 {
		m := t.mesh
		//yMin := m.verts[t.vi[0]].p.Y
		//yMax := yMin
		for _, vi := range t.vi {
			//yMin = math.Min(yMin, m.verts[vi].p.Y)
			//yMax = math.Max(yMax, m.verts[vi].p.Y)
			t.updateExtents(m.verts[vi].p) //recursively bubble up and update all ancestors extents
		}
		if t.yMax-t.yMin < 0.01 {
			//log.Logit("flat triangle detected", t.yMax, t.yMin, t.Depth)
		}

		//t.updateExtents(yMin, yMax) //recursively bubble up and update all ancestors extents
	} else {
		for _, c := range t.children {
			c.CalcVerticalExtents() //recursively drill down to leaf triangles
		}
	}
}

func (t *Tri) SplitIfNeeded(camPos *vec.V3, camDir *vec.V3, fov float64) {

	//      0
	//		/\
	//     /  \
	//  5 /____\ 3
	//   / \  / \
	//  /___\/___\
	// 2     4    1
	if t.Depth >= len(t.mesh.kinks) {
		return
	}

	inFov := !t.allVertsLeftOrRightOfFov(camPos, camDir, fov)
	if t.Depth < 5 || inFov {
		//if t.Depth < 5 || t.Normal.Dot(camDir) < .2 { //is the traingle forward facing ? (relative to the camera)
		//if t.normal.dot((t.centre().sub(pos)).normalise()) < 0.3 { //lower number here cull more backfacing tris

		shouldBeSplitToLevel := 5.0

		if t.Depth >= 5 {
			distSQ := camPos.DistanceSQ(t.centre)
			//dist := camPos.DistanceFrom(t.centre)

			//apud := (2 * t.area()) / (0.0002 * (distSQ))
			//splitPressure := distSQ / float64((t.Depth+1)*(t.Depth+1)*(t.Depth+1)*(t.Depth+1)) //area per unit distance squared
			//splitPressure = float64(t.Depth/15) / ((dist+10)/(t.mesh.size *2)) // (t.mesh.size * 2)) //*(t.Depth)) //area per unit distance squared

			//we want triangles at 3 metres split to level 15 - and those at 10,000 metres split to level 5
			shouldBeSplitToLevel = 12 - math.Log10(distSQ/2) //)*2 // (dist*dist-9)/(2000*2000) // * (5-15) + 15

			dp := camDir.Dot(t.centre.Sub(camPos).Normalise())
			shouldBeSplitToLevel += dp * 4 //bring forward facing triangles forward up to 4 levels

			//splitPressure *= (.1 + dp) // / distSQ //.Normalise())
		}
		//at a value of 1 (area per unit distance), a notional 100 square metre square, would require splitting when it was 10 metres away
		//if apud > 8-(dp*4) || t.Depth < 5 { //.001 is a about 1cm triangles at the horizon
		if t.Depth < int(shouldBeSplitToLevel) {
			t.split()
			for _, c := range t.children {
				c.SplitIfNeeded(camPos, camDir, fov) //recurse
			}
		}

	}

}

func (t *Tri) splitDownTo(level int) {

	if t.Depth < level {
		t.split()
		for _, c := range t.children {
			c.splitDownTo(level) //recurse
		}
	}

}

func (t *Tri) splitIn2(a, b, c, m uint32) {

	t.removeFromTouches()     //remove this tri from the list tris touching this vertex
	t.addChild(true, a, m, c) //left (clockwise wound)
	t.addChild(true, a, b, m) //right

}

// returns the longest edge of the triangle divided by the shortest edge - so a big number is a 'slinny triangle (and no triangle can be 'fatter' than 0.5)
func (t *Tri) aspect() float64 {

	v := t.mesh.verts
	a := v[t.vi[0]].p
	b := v[t.vi[1]].p
	c := v[t.vi[2]].p

	ab := a.Sub(b).Length()
	ac := a.Sub(c).Length()
	bc := b.Sub(c).Length()

	return max(ab, ac, bc) / min(ab, ac, bc)

}

func (t *Tri) split() {

	//      V2
	//		/\
	//     /  \
	// V4 /____\ V5
	//   / \  / \
	//  /___\/___\
	// V1   V3    V0

	if len(t.children) == 0 {

		//	kink := (t.mesh.height/(float64(t.depth*t.depth)+1) - 1) * .5 //maximum kink in this edge
		//kink := (t.mesh.height/(float64(t.depth*5)+1) - 1) * .5 //maximum kink in this edge
		//kink := -t.mesh.height / math.Pow(2, float64(t.depth))
		kink := t.mesh.kinks[t.Depth] * t.mesh.height //maximum kink in this edge

		m := t.mesh
		v0, v1, v2 := t.vi[0], t.vi[1], t.vi[2]

		seed := uint64(m.verts[v1].p.Y)
		rnGen := rand.New(rand.NewPCG(seed, seed+1))
		//rnGe§n = &rand.New(rand.NewPCG(m.verts[v0].p.x, m.verts[v0].p.y))

		rn1, rn2, rn3 := rnGen.NormFloat64(), rnGen.NormFloat64(), rnGen.NormFloat64() //random number between -1 and 1

		v3 := m.splitEdge(v0, v1, rn1*kink, t.Depth)
		v4 := m.splitEdge(v1, v2, rn2*kink, t.Depth)
		v5 := m.splitEdge(v2, v0, rn3*kink, t.Depth)

		t.removeFromTouches() //the list of tirangles touching a vertex is used for normal calculation
		//Always put the horizontal edge in first
		//wind clockwise
		t.addChild(false, v5, v4, v2) //top
		t.addChild(false, v0, v3, v5) //right
		t.addChild(false, v3, v1, v4) //left
		t.addChild(false, v4, v5, v3) //centre

	} else {
		log.Logit("splitting a triangle that already has children ??")
	}
}

func (t *Tri) calcCentre() *vec.V3 {
	v := t.mesh.verts
	t.centre = vec.NewVec3(0, 0, 0)
	t.centre.AddInto(v[t.vi[0]].p, v[t.vi[1]].p, v[t.vi[2]].p)
	t.centre.MulIn(float64(1.0 / 3.0))
	//t.centre.Y += 0.1

	return t.centre
}

func (t *Tri) area() float64 {
	v := t.mesh.verts
	a := v[t.vi[0]].p
	b := v[t.vi[1]].p
	c := v[t.vi[2]].p
	return a.Sub(b).Cross(a.Sub(c)).Length() / 2
}

// returns the deepest (i.e. childless/leaf) triangle intersected by the ray from p0 to p1
// maintaining a count, and populating the slice of penetrations by refererence is easier to get your head around than appending slices (possibly faster too)
func (t *Tri) probe(ray *ray.Ray, stats *ProbeStats, depth int) {

	if depth > stats.maxDepth {
		stats.maxDepth = depth
	}

	if len(t.children) == 0 {

		if t.poly == nil {
			t.poly = newTrianglePoly(t) //cache the polygon
		}

		if t.Culled {
			stats.skippedCulled++
			return
		}

		// if ray.End == t.target {
		// 	*done = true      //check if this has any effect on occlusion counts
		// 	hitsomereturn false, nil //dont let triangles self occlude
		// }

		if t.poly.Probe(ray) {
			stats.leafHits++
			stats.hit = true //exit signal
			//pen := ray.Intersect.Clone()
			//*done = true
		} else {
			stats.leafMisses++
		}

	} else {

		if ray.Origin.Y > t.yMax && ray.End.Y > t.yMax { //|| ray.Origin.Y < t.yMin && ray.End.Y < t.yMin {
			stats.skips++
		}

		if t.prismContains(ray.Origin) || t.prismContains(ray.End) || t.probePrism(ray) {
			stats.prismHits++
			for _, ct := range t.children {

				if ray.End.Y > ct.yMax && ray.Origin.Y > ct.yMax ||
					ray.End.Y < ct.yMin && ray.Origin.Y < ct.yMin ||
					ray.End.X > ct.xMax && ray.Origin.X > ct.xMax ||
					ray.End.X < ct.xMin && ray.Origin.X < ct.xMin ||
					ray.End.Z > ct.zMax && ray.Origin.Z > ct.zMax ||
					ray.End.Z < ct.zMin && ray.Origin.Z < ct.zMin {
					stats.skips++
					continue
				}

				ct.probe(ray, stats, depth+1)
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
func (t *Tri) ProbeAll(ray *ray.Ray, stats *ProbeStats) {

	if len(t.children) == 0 {
		if t.poly == nil {
			t.poly = newTrianglePoly(t) //cache the polygon
		}

		if t.Culled {
			stats.skippedCulled++
		} else {
			if t.poly.Probe(ray) {
				stats.leafHits++
				pen := ray.Intersect.Clone()
				d := pen.DistanceFrom(ray.Origin)
				if d < stats.SDist {
					stats.SDist = d
					stats.nearestHit = pen
					stats.NearestTri = t
					ray.PointAt(pen) //move the ray end to the hit point
				}
			} else {
				stats.leafMisses++
			}
		}
	} else {

		if t.prismContains(ray.Origin) || t.prismContains(ray.End) || t.probePrism(ray) {
			stats.prismHits++
			for _, ct := range t.children {

				if ray.End.Y > ct.yMax && ray.Origin.Y > ct.yMax || ray.End.Y < ct.yMin && ray.Origin.Y < ct.yMin {
					stats.skips++
					continue
				}
				ct.ProbeAll(ray, stats)
			}
		} else {
			//none of the triangles in this prism need checking
			stats.prismMisses++
		}
	}

}

// EdgesAsMsg - return the edges of this triangle as a msg for clientside rendering/debugging
func (t *Tri) EdgesAsMsg() *msg.Msg {

	v := t.mesh.verts
	a := v[t.vi[0]].p
	b := v[t.vi[1]].p
	c := v[t.vi[2]].p

	return msg.NewMsg(
		msg.Vectors,
		a, b, colors.Magenta,
		b, c, colors.Magenta,
		c, a, colors.Magenta,
	)

}

func (t *Tri) prismContains(p *vec.V3) bool {

	if p.Y < t.yMin || p.Y > t.yMax {
		return false //outside the vertical extents of the prism
	}

	//we are within the vertical extents - we now do a 2d check agains the footprint
	return t.contains2D(p)

}

// test if the ray from p0 to p1 penetrates the volume of triangular based 'prism' extending between t.ymin and t.ymax
func (t *Tri) probePrism(ray *ray.Ray) bool {

	epsilon := 0.001
	//for all five sides of the prism - check for a penetration
	for i, s := range t.PrismFaces {
		if s.Probe(ray) {
			if ray.Intersect.Y > t.yMax+epsilon || ray.Intersect.Y < t.yMin-epsilon {
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

func (t *Tri) contains2D(p *vec.V3) bool {

	if t.poly == nil {
		t.poly = newTrianglePoly(t)

	}

	return t.poly.Contains2D(p)
}

func (t *Tri) vProbe(p *vec.V3) *Tri {

	if t.contains2D(p) {
		if len(t.children) == 0 {
			return t
		}

		for _, ct := range t.children {
			tt := ct.vProbe(p)
			if tt != nil {
				return tt
			}
		}

		log.Logit("No child contained point - but parent did")

	}

	return nil

	//panic("vProbe failed to find a tri")

}

func (t *Tri) calcNormal() *vec.V3 {

	v := t.mesh.verts

	numMeshVerts := uint32(len(v))
	if t.vi[0] >= numMeshVerts || t.vi[1] >= numMeshVerts || t.vi[2] >= numMeshVerts {
		panic("index out of range in tri.normal")
	}

	//n1 := v[t.Vi[1]].p.sub(v[t.Vi[0]].p).cross(v[t.Vi[2]].p.sub(v[t.Vi[0]].p)).normalise()
	//log.Logit(n1.X, n1.Y, n1.Z)
	ab := v[t.vi[1]].p.Sub(v[t.vi[0]].p) //.normalise()
	ac := v[t.vi[2]].p.Sub(v[t.vi[0]].p) //.normalise()
	n2 := (ab.Cross(ac)).Normalise()
	ln := n2.Length()
	if ln < .999 || ln > 1.00001 {
		panic("normal is not unit length")
	}

	t.Normal = n2
	return n2
}

func abs(a float64) float64 {
	if a < 0 {
		return -a
	}
	return a
}

func newTri(parent *Tri, m *TriMesh, depth int, vi ...uint32) *Tri {

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
	t := Tri{parent: parent, Depth: depth, vi: vi, children: []*Tri{},
		mesh: m,
		xMin: math.MaxFloat64, xMax: -math.MaxFloat64,
		yMin: math.MaxFloat64, yMax: -math.MaxFloat64,
		zMin: math.MaxFloat64, zMax: -math.MaxFloat64,
		PrismFaces: []*poly.ConvexPoly{nil, nil, nil, nil, nil},
	}

	t.calcCentre()
	//add this traingle to its verts list of triangles
	t.addToTouches()

	t.calcNormal()
	if t.Normal.Y < 0 {
		panic("triangle with downward normal")
	}

	return &t
}

func (t *Tri) ToSimpleMesh(id uint16, lm *TriMesh, fm *TriMesh, material string, asWater bool, faceTest func(face *Tri) bool) *mesh.SimpleMesh {

	//vc := uint32(len(lm.verts)) //vertex count
	vc := uint32(len(lm.verts))  //vertex count
	fis := make([]uint16, vc*10) //there will actually be many less faces than verts - but we need 3 uints per face

	if vc >= math.MaxUint16 {
		log.Logit("mesh too big - over 65535 verts")
	}

	lm.getNormals(asWater) //we must get normals (becuase it calculates them) before updating UVx's

	wp := uint32(0)

	t.getFacesInto(fis, &wp, faceTest) //populate Fis (recursivley from the root triangle)
	fis = fis[:wp]                     //truncate at the write pointer

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

	log.Logit(t.mesh.VertCount(), "verts reduced to", len(mapping), "for mesh", id)

	//normals := lm.getNormals(asWater) //we must get normals (becuase it calculates them) before updating UVx's
	//lm.updateUVxsFromNormals()

	//return mesh.NewFilledSimpleMesh(id, lm.getPositions(asWater), normals, lm.getUVs(), fis, material)
	return mesh.NewFilledSimpleMesh(id, np, nn, nuv, fis, material)

}

func (t *Tri) getYLowHigh() (low *vec.V3, high *vec.V3) {
	a := t.mesh.verts[t.vi[0]]
	b := t.mesh.verts[t.vi[1]]
	c := t.mesh.verts[t.vi[2]]

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
