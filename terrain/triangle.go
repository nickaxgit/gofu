package terrain

import (
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/mesh"
	"github.com/nickax/gofu/poly"
	"github.com/nickax/gofu/ray"
	"github.com/nickax/gofu/vec"
	//"github.com/nickax/gofu/cam"

	"math"
	"math/rand/v2"
)

var up = vec.NewVec3(0, 1, 0)
var nowhereSpecial = vec.NewVec3(0, -99999, 0)

type tcs struct {
	left, top, right, bottom float64
}

type vert struct {
	p        *vec.V3
	n        *vec.V3
	uv       *vec.V2
	wl       float64       //water level
	acc      float64       //accumulated water (during a pass)
	incount  int           //number of ferts flowing into this vert
	touches  map[*Tri]bool //the triangles that touch this vertex (whos face normals contribute to the vertex normal)
	occluded bool
}

type Tri struct {
	parent     *Tri
	Depth      int
	vi         []uint32
	children   []*Tri
	mesh       *TriMesh //a reference to the mesh this tri is part of (that the vi's point into v's of)
	Normal     *vec.V3
	Scorched   bool //note this is not part of the fireInfo - it's a cache of which land triangles are burned out
	Culled     bool
	yMin       float64
	yMax       float64
	prismFaces []*poly.ConvexPoly //3 sides plus bottom and top
	poly       *poly.ConvexPoly   // made/cached JIT
	//shadow  *poly.ConvexPoly //the trinagle pojected onto y=0 JIT/cached for vprobe
	centre *vec.V3
	//flat       *poly.ConvexPoly //cache of the flat polygon for 2d contains testing
	FireInfo *fireInfo //nil for land triangles
}

func newTrianglePoly(t *Tri) *poly.ConvexPoly {
	poly := poly.NewConvexPoly()
	for i := 0; i < 3; i++ {
		poly.AddPoint(t.mesh.verts[t.vi[i]].p)
	}
	return poly
}

func (t *Tri) NewEndCap(y float64) *poly.ConvexPoly {
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

	if len(t.children) == 0 {
		return //leaf triangles don't need prisms
	}

	for i, v := range t.vi {
		vp := t.mesh.verts[v].p
		vpn := t.mesh.verts[t.vi[(i+1)%3]].p
		poly := poly.NewConvexPoly()
		//poly.addPointAt(vp.x, t.yMax, vp.z)

		//TODO endcaps and sides could share vec3 verts

		tl := vp.Clone()
		tl.Y = t.yMax

		tr := vpn.Clone()
		tr.Y = t.yMax

		br := vpn.Clone()
		br.Y = t.yMin

		bl := vp.Clone()
		bl.Y = t.yMin

		poly.AddPoint(tl)
		poly.AddPoint(tr)
		poly.AddPoint(br)
		poly.AddPoint(bl)
		t.prismFaces[i] = poly

	}

	//make endcaps

	t.prismFaces[3] = t.NewEndCap(t.yMin) //bottom
	t.prismFaces[4] = t.NewEndCap(t.yMax) //top
	//tri.prismFaces[5] = newTrianglePoly(tri)

	for _, child := range t.children {
		child.MakePrisms()
	}

}

func (t *Tri) OnOrUnderWater() bool {
	m := t.mesh
	a := m.verts[t.vi[0]]
	b := m.verts[t.vi[1]]
	c := m.verts[t.vi[2]]

	epsilon := 0.0001
	if a.p.Y <= a.wl+epsilon && b.p.Y <= b.wl+epsilon && c.p.Y <= c.wl+epsilon { //if all verts are at water level
		return true
	}
	return false
}

// func (t *Tri) IsSubmerged() bool {

// 	m := t.mesh
// 	a := m.verts[t.vi[0]]
// 	b := m.verts[t.vi[1]]
// 	c := m.verts[t.vi[2]]

// 	if a.p.Y < a.wl && b.p.Y < b.wl && c.p.Y < c.wl { //if all verts are below water
// 		return true
// 	}
// 	return false
// }

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

func (t *Tri) updateExtents(yMin float64, yMax float64) {
	if yMin < t.yMin {
		t.yMin = yMin
	}
	if yMax > t.yMax {
		t.yMax = yMax
	}
	if t.parent != nil {
		t.parent.updateExtents(yMin, yMax)
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

func (t *Tri) addChild(vi ...uint32) *Tri {
	child := newTri(t, t.mesh, t.Depth+1, vi...) //t.mesh.makeTri(t.depth+1, fi, a, b, c)
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

	m := t.mesh
	if len(t.children) == 0 && !t.Culled {

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

// for each triangle T - check if none of its verts can been seen from pos
func (root *Tri) OccludeVerts(pos *vec.V3) {

	oc := 0
	//defining a slice once, and using/resetting a penetration count is faster
	pens := make([]vec.V3, 10)
	penCount := 0
	probeCount := 0

	ray := ray.New(pos, nowhereSpecial)
	for _, v := range root.mesh.verts {
		penCount = 0 //important
		ray.PointAt(v.p)
		if root.probe(ray, pens, &penCount, &probeCount, true) { //reursively probe the prisms/faces
			v.occluded = true
			oc++
		} else {
			v.occluded = false
		}

	}

	log.Logit("occluded", oc, " of ", root.mesh.VertCount(), " verts", " probes", probeCount)
}

func (t *Tri) OcclusionCull(culled *int, kept *int) {

	if t.Culled {
		panic("already culled")
	}
	if !t.Culled && len(t.children) == 0 {

		hidden := 0
		for _, vi := range t.vi {
			if t.mesh.verts[vi].occluded {
				hidden++
			}
		}
		if hidden == 3 {
			t.Culled = true
			*culled++

		} else {
			*kept++
		}
	}
	for _, c := range t.children {
		c.OcclusionCull(culled, kept)
	}
}

// drill down from the land root triangle - bubbling up and calculating y extents for all ancestors of all leaf triangles
func (t *Tri) CalcVerticalExtents() { //called on the root triangle
	if len(t.children) == 0 {
		m := t.mesh
		yMin := m.verts[t.vi[0]].p.Y
		yMax := yMin
		for _, vi := range t.vi[1:] {
			yMin = math.Min(yMin, m.verts[vi].p.Y)
			yMax = math.Max(yMax, m.verts[vi].p.Y)
		}
		t.updateExtents(yMin, yMax) //recursively update all ancestors extents
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

	//triCentre := t.centre()

	//if t.hasVertexInFrontOf(pos,focus){
	//if t.facesTowards(focus.sub(pos)) { //is the traingle forward facing ? (relative to the camera)

	inFov := !t.allVertsLeftOrRightOfFov(camPos, camDir, fov)
	if t.Depth < 5 || inFov {
		//if t.Depth < 5 || t.Normal.Dot(camDir) < .2 { //is the traingle forward facing ? (relative to the camera)
		//if t.normal.dot((t.centre().sub(pos)).normalise()) < 0.3 { //lower number here cull more backfacing tris

		dist := camPos.DistanceFrom(t.centre)
		//apud := (2 * t.area()) / (dist * dist)
		//apud := (2 * t.area()) / (0.0005 * (dist * dist))
		apud := (2 * t.area()) / (0.0002 * (dist * dist))

		dp := camDir.Dot(t.centre.Sub(camPos).Normalise())
		//at a value of 1 (area per unit distance), a notional 100 square metre square, would require splitting when it was 10 metres away
		if apud > 8-(dp*4) || t.Depth < 5 { //.001 is a about 1cm triangles at the horizon

			//log.Logit("splitting", t.depth, apud, dist, t.area())
			t.split()
			for _, c := range t.children {
				c.SplitIfNeeded(camPos, camDir, fov) //recurse
			}
		}
		//}

		//} //else {
		//		t.cull = true
		//		}
		//}
	}

	//}

	if len(t.children) == 0 && t.Depth > 5 && t.Normal.Dot((t.centre.Sub(camPos)).Normalise()) > 0.2 {
		//t.cull = true
		//final triangle is backfacing - cull it
	}

}

//}

func (t *Tri) splitDownTo(level int) {

	if t.Depth < level {
		t.split()
		for _, c := range t.children {
			c.splitDownTo(level) //recurse
		}
	}

}

func (t *Tri) splitIn2(a, b, c, m uint32) {

	t.removeFromTouches() //remove this tri from the list tris touching this vertex
	t.addChild(a, m, c)   //left (clockwise wound)
	t.addChild(a, b, m)   //right

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
	if len(t.children) == 0 {

		//	kink := (t.mesh.height/(float64(t.depth*t.depth)+1) - 1) * .5 //maximum kink in this edge
		//kink := (t.mesh.height/(float64(t.depth*5)+1) - 1) * .5 //maximum kink in this edge
		//kink := -t.mesh.height / math.Pow(2, float64(t.depth))
		kink := t.mesh.kinks[t.Depth] * t.mesh.height //maximum kink in this edge

		m := t.mesh
		v0 := t.vi[0]
		v1 := t.vi[1]
		v2 := t.vi[2]

		seed := uint64(m.verts[v1].p.Y)
		rnGen := rand.New(rand.NewPCG(seed, seed+1))
		//rnGe§n = &rand.New(rand.NewPCG(m.verts[v0].p.x, m.verts[v0].p.y))

		rn1 := rnGen.NormFloat64() //random number between -1 and 1
		rn2 := rnGen.NormFloat64()
		rn3 := rnGen.NormFloat64()

		v3 := m.splitEdge(v0, v1, rn1*kink, t.Depth)
		v4 := m.splitEdge(v1, v2, rn2*kink, t.Depth)
		v5 := m.splitEdge(v2, v0, rn3*kink, t.Depth)

		t.removeFromTouches() //the list of tirangles touching a vertex is used for normal calculation

		t.addChild(v0, v3, v5) //top
		t.addChild(v3, v1, v4) //right
		t.addChild(v5, v4, v2) //left
		t.addChild(v3, v4, v5) //centre

	} else {
		log.Logit("splitting a triangle that already has children ??")
	}
}

func (t *Tri) calcCentre() *vec.V3 {
	v := t.mesh.verts
	t.centre = vec.NewVec3(0, 0, 0)
	t.centre.AddInto(v[t.vi[0]].p, v[t.vi[1]].p, v[t.vi[2]].p)
	t.centre.MulInto(t.centre, 1.0/3.0)
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
func (t *Tri) probe(ray *ray.Ray, pens []vec.V3, penCount *int, probeCount *int, earlyExit bool) bool {

	if len(t.children) == 0 {

		if t.poly == nil {
			t.poly = newTrianglePoly(t) //cache the polygon
		}
		if !t.poly.Has(ray.End) {
			//log.Logit("ray end outside tri bounds")
			//log.Logit("probing leaf tri")
			if t.poly.Probe(ray) {
				//DONT use ray.intersect directly as it will be overwritten on the next penetration
				if !earlyExit {
					pens[*penCount] = *ray.Intersect.Clone()
				} //the clone may be redundant as we are dereferencing
				(*penCount)++
				(*probeCount)++
			} else {
				if *penCount != 0 {
					panic("wtf")
				}
				//log.Logit("missed leaf tri")
			}
		}

	} else {
		//vExtend := t.mesh.kinks[t.depth] * t.mesh.height
		if t.prismContains(ray.Origin) || t.prismContains(ray.End) || t.probePrism(ray) {
			for _, ct := range t.children {
				ct.probe(ray, pens, penCount, probeCount, earlyExit) //recurse
				if *penCount > 0 && earlyExit {
					return true
				}
			}
		} else {
			//none of the trianlges in this prism need checking
			//log.Logit("missed prism")
		}
	}

	return *penCount > 0

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

	//for all five sides of the prism - check for a penetration
	for _, s := range t.prismFaces {
		if s.Probe(ray) {
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
		mesh: m, yMin: math.MaxFloat64, yMax: -math.MaxFloat64,
		prismFaces: []*poly.ConvexPoly{nil, nil, nil, nil, nil},
	}

	t.calcCentre()
	//add this traingle to its verts list of triangles
	t.addToTouches()

	t.calcNormal()

	return &t
}

func (t *Tri) ToSimpleMesh(id uint16, lm *TriMesh, fm *TriMesh, material string, asWater bool, faceTest func(face *Tri) bool) *mesh.SimpleMesh {

	//vc := uint32(len(lm.verts)) //vertex count
	vc := uint32(len(lm.verts))  //vertex count
	fis := make([]uint16, vc*10) //there will actually be many less faces than verts - but we need 3 uints per face

	if vc >= math.MaxUint16 {
		log.Logit("mesh too big - over 65535 verts")
	}

	wp := uint32(0)

	t.getFacesInto(fis, &wp, faceTest) //populate Fis (recursivley from the root triangle)

	fis = fis[:wp] //truncate at the write pointer

	normals := lm.getNormals(asWater) //we must get normals (becuase it calculates them) before updating UVx's
	lm.updateUVxsFromNormals()
	return mesh.NewFilledSimpleMesh(id, lm.getPositions(asWater), normals, lm.getUVs(), fis, material)

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
