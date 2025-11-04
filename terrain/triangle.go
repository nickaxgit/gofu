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
	cull       bool
	yMin       float64
	yMax       float64
	prismFaces []*poly.ConvexPoly //3 sides plus bottom and top
	centre     *vec.V3
	flat       *poly.ConvexPoly //cache of the flat polygon for 2d contains testing
	fireInfo   *fireInfo        //nil for land triangles
}

func newTrianglePoly(t *Tri) *poly.ConvexPoly {
	poly := poly.ConvexPoly{}
	for i := 0; i < 3; i++ {
		vp := t.mesh.verts[t.vi[i]].p
		poly.AddPoint(vp) //&vec3{vp.x, vp.y, vp.z})
	}
	return &poly
}

func (tri *Tri) NewEndCap(y float64) *poly.ConvexPoly {
	poly := poly.NewConvexPoly()
	for _, v := range tri.vi {
		poly.AddPoint(tri.mesh.verts[v].p.Add(vec.NewVec3(0, y, 0)))
	}
	return poly

}

func newFlatTrianglePoly(t *Tri) *poly.ConvexPoly {
	poly := poly.ConvexPoly{}
	for i := 0; i < 3; i++ {
		vp := t.mesh.verts[t.vi[i]].p
		poly.AddPoint(vp.Y0())
	}
	return &poly
}

func (tri *Tri) Plough(runwayStart *vec.V3, runwayEnd *vec.V3, runwayWidth float64) {

	m := tri.mesh

	for _, vi := range tri.vi {
		pp := m.verts[vi].p.Clone()
		pp.SetY(runwayStart.GetY()) //move the vertex to the runway height
		if pp.DistanceFromLineSegment(runwayStart, runwayEnd) < runwayWidth*2 {
			//cp := p.closestPointOnLineSegment(runwayStart, runwayEnd)
			m.verts[vi].p.SetY(runwayStart.GetY())
		}
	}
	tri.calcNormal() //SUPER important !

	for _, ct := range tri.children {
		ct.Plough(runwayStart, runwayEnd, runwayWidth)
	}

}

func (tri *Tri) MakePrisms() {

	for i, v := range tri.vi {
		vp := tri.mesh.verts[v].p
		vpn := tri.mesh.verts[tri.vi[(i+1)%3]].p
		poly := poly.ConvexPoly{}
		//poly.addPointAt(vp.x, t.yMax, vp.z)

		//TODO endcaps and sides could share vec3 verts
		poly.AddPoint(vpn.Clone().SetY(tri.yMax))
		poly.AddPoint(vpn.Clone().SetY(tri.yMin))
		poly.AddPoint(vp.Clone().SetY(tri.yMin))
		poly.AddPoint(vp.Clone().SetY(tri.yMax))
		tri.prismFaces[i] = &poly

	}

	//make endcaps

	tri.prismFaces[3] = tri.NewEndCap(tri.yMin) //bottom
	tri.prismFaces[4] = tri.NewEndCap(tri.yMax) //top
	tri.prismFaces[5] = newTrianglePoly(tri)

	for _, child := range tri.children {
		child.MakePrisms()
	}

}

func (tri *Tri) IsUnderwater(by float64) bool {

	m := tri.mesh
	a := m.verts[tri.vi[0]]
	b := m.verts[tri.vi[1]]
	c := m.verts[tri.vi[2]]

	if a.wl-a.p.GetY() > by && b.wl-b.p.GetY() > by && c.wl-c.p.GetY() > by { //if all verts are below water
		return true
	}
	return false
}

func (tri *Tri) getFacesInto(fis []uint16, p *uint32, test func(face *Tri) bool) {
	if len(tri.children) == 0 && !tri.cull {
		j := *p
		if test(tri) {
			fis[j] = uint16(tri.vi[0])
			fis[j+1] = uint16(tri.vi[1])
			fis[j+2] = uint16(tri.vi[2])
			*p += 3
		}
	}
	for _, c := range tri.children {
		c.getFacesInto(fis, p, test)
	}
}

func (tri *Tri) updateExtents(yMin float64, yMax float64) {
	if yMin < tri.yMin {
		tri.yMin = yMin
	}
	if yMax > tri.yMax {
		tri.yMax = yMax
	}
	if tri.parent != nil {
		tri.parent.updateExtents(yMin, yMax)
	}
}

// keep track of the deepest (up to) 6 triangles touching this vert -- allows us to recalculate normals quickly
func (v *vert) touch(t ...*Tri) {

	for _, t := range t {
		v.touches[t] = true
		if len(v.touches) > 6 {
			panic("vertex touched by more than 6 triangles")
		}
	}
}

func newVert(p *vec.V3, u, v float64) *vert {
	return &vert{p: p, uv: vec.NewVec2(u, v), n: vec.NewVec3(0, 0, 0), touches: make(map[*Tri]bool, 6)}
}

func (tri *Tri) flatContains(p *vec.V3) bool {
	//cache the flat polygon - particularly useful for fire mesh (which is persistent)
	if tri.flat == nil {
		tri.flat = &poly.ConvexPoly{}
		for _, vi := range tri.vi {
			tri.flat.AddPoint(tri.mesh.verts[vi].p)
		}
	}
	return tri.flat.Contains(p)
}

func (tri *Tri) find(p *vec.V3) *Tri {

	if tri.flatContains(p) {
		if len(tri.children) == 0 {
			return tri
		}

		scorched := 0
		for _, c := range tri.children {

			f := c.find(p)
			if f != nil {
				return f
			}
		}
		if scorched == len(tri.children) {
			tri.fireInfo.flames = -1 //mark parent as scorched
			tri.children = nil
		}
	} else {
		return nil
	}
	log.Logit("warn: fTri find failed to find a tri")
	return nil
}

func (tri *Tri) addChild(vi ...uint32) *Tri {
	child := newTri(tri, tri.mesh, tri.Depth+1, vi...) //t.mesh.makeTri(t.depth+1, fi, a, b, c)
	tri.children = append(tri.children, child)
	return child
}

func (tri *Tri) scorchedAt(p *vec.V3) bool {
	if len(tri.children) == 0 {
		return tri.Scorched
	} else {
		for _, c := range tri.children {
			if c.prismFaces[5].Contains(p) {
				return c.scorchedAt(p)
			}
		}
		return false
	}
}

func (tri *Tri) FetchTrees(depth int, positions []float32, bbm *mesh.SimpleMesh, camPos *vec.V3, camDir *vec.V3, ray *ray.Ray, hidden *int) {

	mid := tri.centre
	tcs := mesh.NewTcs(0, 1, 1, 0)

	treeTop := vec.NewVec3(0, 0, 0) //
	if tri.Depth == depth {
		if !tri.scorchedAt(mid) && !tri.IsUnderwater(1) {

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
					if tri.prismFaces[5].Probe(ray) {
						*hidden++
						return
					}

					//place a (instanced mesh) tree here
					positions = append(positions, mid.AsFloat32s()...)
				}
			} else { //it's a faraway tree - only place it if the ground slopes towards the camera
				if dotProd > .25 { //trees generally in front of the camera}
					if toTree.Dot(tri.Normal) < 0 { //if the triangle slopes towards camera
						bbm.Billboard(mid, up, camPos, 20, 20, 20, 4, tcs) //billboard tree
					}

				}
			}
		}
	} else {
		for _, c := range tri.children {
			c.FetchTrees(depth, positions, bbm, camPos, camDir, ray, hidden)
		}
	}

}

// scorch - recurse through all land triangles flagging them as scorched by checking their centres in the fire mesh
func (tri *Tri) Scorch(fire *TriMesh) {
	if len(tri.children) == 0 && !tri.cull {
		if fire.scorchedAt(tri.centre) {
			tri.Scorched = true
		}
	}
	for _, c := range tri.children {
		c.Scorch(fire)
	}
}

// for every bottom level triangle, look to see if there is a vertex at the midpoint of each edge (caused by a more divided neighbouring tri)
// if so, split in two to the opposite vertex
func (tri *Tri) Patch() {

	m := tri.mesh
	if len(tri.children) == 0 && !tri.cull {

		for i := 0; i < 3; i++ {

			ai := tri.vi[i]
			bi := tri.vi[(i+1)%3]
			ci := tri.vi[(i+2)%3]
			//mi := m.midpoint(ai, bi) //looks both ways for a midpoint   -- m.vertAtMidPointXZ(ai, bi) //this could be way faster by recording mid verts
			mi := m.midpoint(bi, ci) //looks both ways for a midpoint   -- m.vertAtMidPointXZ(ai, bi) //this could be way faster by recording mid verts

			if mi != math.MaxUint32 { //is there a midpoint ?

				//if t.aspect() < 2 { //is it 'fat'
				tri.splitIn2(ai, bi, ci, mi)
				break //only one edge of this tri (becuase it is now multiple child tris)
				//} else {
				//	t.split()
				//}

			}

		}

	}

	for _, c := range tri.children {
		c.Patch()
	}

}

func (tri *Tri) addToTouches() {

	m := tri.mesh
	m.verts[tri.vi[0]].touch(tri)
	m.verts[tri.vi[1]].touch(tri)
	m.verts[tri.vi[2]].touch(tri)

}

func (tri *Tri) removeFromTouches() {
	if len(tri.children) > 0 {
		panic("Tri has children")
	}
	for _, vi := range tri.vi {
		v := tri.mesh.verts[vi]
		delete(v.touches, tri) //remove this tri from the list of tris touching this vertex
	}
}

func (tri *Tri) facesTowards(direction *vec.V3) bool {
	//the extra -.1 is to account for traingles facing away at less than half the camera vertical FOV
	return tri.Normal.Dot(direction) < -.1 //is the traingle forward facing ? (relative to the camera)

}

func (tri *Tri) allVertsLeftOrRightOfFov(camPos *vec.V3, camDir *vec.V3, fov float64) bool {

	onLeft := 0
	for _, vi := range tri.vi {
		cam2vert := tri.mesh.verts[vi].p.Sub(camPos).Normalise()
		if camDir.Dot(cam2vert) > fov {
			return false //a vertex is within the FOV
		}
		//we're outside the FOV
		cp := cam2vert.Cross(camDir)
		if cp.GetY() < 0 {
			onLeft++
		}
	}

	if onLeft == 3 {
		return true //all vertices are outside and on the same side of the FOV

	}
	if onLeft == 0 {
		return true
	}

	return false //vertices straddle the viewing frustum

}

func (tri *Tri) hasVertexWithinFov(pos *vec.V3, focus *vec.V3, fov float64) bool {

	camDir := focus.Sub(pos).Normalise()
	for _, vi := range tri.vi {
		cam2vert := tri.mesh.verts[vi].p.Sub(pos).Normalise()
		if camDir.Dot(cam2vert) > fov {
			return true //a vertex is within the FOV
		}
	}

	return false //no vertices are within the FOV

}

func (tri *Tri) countChildren(count *int) {

	*count += len(tri.children)

	for _, c := range tri.children {
		c.countChildren(count)
	}

}

// for each triangle T - check if none of its verts can been seen from pos
func (tri *Tri) OccludeVerts(pos *vec.V3) {

	oc := 0
	//defining a slice once, and using/resetting a penetration count is faster
	pens := make([]vec.V3, 10)
	penCount := 0

	ray := ray.New(pos, nowhereSpecial)
	for _, v := range tri.mesh.verts {
		penCount = 0
		ray.PointAt(v.p)
		if tri.probe(ray, pens, &penCount, true) {
			v.occluded = true
			oc++
		}
	}

	log.Logit("occluded", oc, " of ", tri.mesh.VertCount(), " verts")
}

func (tri *Tri) OcclusionCull(culled *int, kept *int) {

	if !tri.cull && len(tri.children) == 0 {

		hidden := 0
		for _, vi := range tri.vi {
			if tri.mesh.verts[vi].occluded {
				hidden++
			}
		}
		if hidden == 3 {
			tri.cull = true
			*culled++

		} else {
			*kept++
		}
	}
	for _, c := range tri.children {
		c.OcclusionCull(culled, kept)
	}
}

// drill down from the land root triangle - bubbling up and calculating y extents for all ancestors of all leaf triangles
func (tri *Tri) CalcVerticalExtents() { //called on the root triangle
	if len(tri.children) == 0 {
		m := tri.mesh
		yMin := m.verts[tri.vi[0]].p.Y
		yMax := yMin
		for _, vi := range tri.vi[1:] {
			yMin = math.Min(yMin, m.verts[vi].p.Y)
			yMax = math.Max(yMax, m.verts[vi].p.Y)
		}
		tri.updateExtents(yMin, yMax) //recursively update all ancestors extents
	} else {
		for _, c := range tri.children {
			c.CalcVerticalExtents() //recursively drill down to leaf triangles
		}
	}
}

func (tri *Tri) SplitIfNeeded(camPos *vec.V3, camDir *vec.V3, fov float64) {

	//      0
	//		/\
	//     /  \
	//  5 /____\ 3
	//   / \  / \
	//  /___\/___\
	// 2     4    1
	if tri.Depth >= len(tri.mesh.kinks) {
		return
	}

	//triCentre := t.centre()

	//if t.hasVertexInFrontOf(pos,focus){
	//if t.facesTowards(focus.sub(pos)) { //is the traingle forward facing ? (relative to the camera)

	inFov := !tri.allVertsLeftOrRightOfFov(camPos, camDir, fov)
	if tri.Depth < 5 || inFov { //high numbers here gives a narrow field of view getting split
		//if t.normal.dot(camDir) < -0.1 { //is the traingle forward facing ? (relative to the camera)
		//if t.normal.dot((t.centre().sub(pos)).normalise()) < 0.3 { //lower number here cull more backfacing tris

		dist := camPos.DistanceFrom(tri.centre)
		//apud := (2 * t.area()) / (dist * dist)
		apud := (2 * tri.area()) / (0.0005 * (dist * dist))

		dp := camDir.Dot(tri.centre.Sub(camPos).Normalise())
		//at a value of 1 (area per unit distance), a notional 100 square metre square, would require splitting when it was 10 metres away
		if apud > 8-(dp*4) || tri.Depth < 5 { //.001 is a about 1cm triangles at the horizon

			//log.Logit("splitting", t.depth, apud, dist, t.area())
			tri.split()
			for _, c := range tri.children {
				c.SplitIfNeeded(camPos, camDir, fov) //recurse
			}
		}

		//} //else {
		//		t.cull = true
		//		}
		//}
	}

	//}

	if len(tri.children) == 0 && tri.Depth > 5 && tri.Normal.Dot((tri.centre.Sub(camPos)).Normalise()) > 0.2 {
		//t.cull = true
		//final triangle is backfacing - cull it
	}

}

//}

func (tri *Tri) splitDownTo(level int) {

	if tri.Depth < level {
		tri.split()
		for _, c := range tri.children {
			c.splitDownTo(level) //recurse
		}
	}

}

func (tri *Tri) splitIn2(a, b, c, m uint32) {

	tri.removeFromTouches() //remove this tri from the list tris touching this vertex
	tri.addChild(a, m, c)   //left (clockwise wound)
	tri.addChild(a, b, m)   //right

}

// returns the longest edge of the triangle divided by the shortest edge - so a big number is a 'slinny triangle (and no triangle can be 'fatter' than 0.5)
func (tri *Tri) aspect() float64 {

	v := tri.mesh.verts
	a := v[tri.vi[0]].p
	b := v[tri.vi[1]].p
	c := v[tri.vi[2]].p

	ab := a.Sub(b).Length()
	ac := a.Sub(c).Length()
	bc := b.Sub(c).Length()

	return max(ab, ac, bc) / min(ab, ac, bc)

}

func (tri *Tri) split() {
	if len(tri.children) == 0 {

		//	kink := (t.mesh.height/(float64(t.depth*t.depth)+1) - 1) * .5 //maximum kink in this edge
		//kink := (t.mesh.height/(float64(t.depth*5)+1) - 1) * .5 //maximum kink in this edge
		//kink := -t.mesh.height / math.Pow(2, float64(t.depth))
		kink := tri.mesh.kinks[tri.Depth] * tri.mesh.height //maximum kink in this edge

		m := tri.mesh
		v0 := tri.vi[0]
		v1 := tri.vi[1]
		v2 := tri.vi[2]

		seed := uint64(m.verts[v1].p.GetY())
		rnGen := rand.New(rand.NewPCG(seed, seed+1))
		//rnGe§n = &rand.New(rand.NewPCG(m.verts[v0].p.x, m.verts[v0].p.y))

		rn1 := rnGen.NormFloat64() //random number between -1 and 1
		rn2 := rnGen.NormFloat64()
		rn3 := rnGen.NormFloat64()

		v3 := m.splitEdge(v0, v1, rn1*kink, tri.Depth)
		v4 := m.splitEdge(v1, v2, rn2*kink, tri.Depth)
		v5 := m.splitEdge(v2, v0, rn3*kink, tri.Depth)

		tri.removeFromTouches() //the list of tirangles touching a vertex is used for normal calculation

		tri.addChild(v0, v3, v5) //top
		tri.addChild(v3, v1, v4) //right
		tri.addChild(v5, v4, v2) //left
		tri.addChild(v3, v4, v5) //centre

	} else {
		log.Logit("splitting a triangle that already has children ??")
	}
}

func (tri *Tri) calcCentre() *vec.V3 {
	v := tri.mesh.verts
	tri.centre = vec.NewVec3(0, 0, 0)
	tri.centre.AddInto(v[tri.vi[0]].p, v[tri.vi[1]].p, v[tri.vi[2]].p)
	tri.centre.MulInto(tri.centre, 1.0/3.0)
	return tri.centre
}

func (tri *Tri) area() float64 {
	v := tri.mesh.verts
	a := v[tri.vi[0]].p
	b := v[tri.vi[1]].p
	c := v[tri.vi[2]].p
	return a.Sub(b).Cross(a.Sub(c)).Length() / 2
}

// returns the deepest (i.e. childless/leaf) triangle intersected by the ray from p0 to p1
// maintaining a count, and populating the slice of penetrations by refererence is easier to get your head around than appending slices (possibly faster too)
func (tri *Tri) probe(ray *ray.Ray, pens []vec.V3, penCount *int, earlyExit bool) bool {

	if len(tri.children) == 0 {

		if tri.prismFaces[5].Probe(ray) {
			//DONT use ray.intersect directly as it will be overwritten on the next penetration
			pens[*penCount] = *ray.Intersect.Clone() //the clone may be redundant as we are dereferencing
			(*penCount)++
		}

	} else {
		//vExtend := t.mesh.kinks[t.depth] * t.mesh.height
		if tri.prismContains(ray.Origin) || tri.prismContains(ray.End) || tri.probePrism(ray) {
			for _, ct := range tri.children {
				ct.probe(ray, pens, penCount, earlyExit)
				if *penCount > 0 && earlyExit {
					return true
				}
			}
		}
	}

	return *penCount > 0

}

func (tri *Tri) prismContains(p *vec.V3) bool {

	if p.GetY() < tri.yMin || p.GetY() > tri.yMax {
		return false //outside the vertical extents of the prism
	}

	bottom := tri.prismFaces[3] //newEndCap(t, t.yMin)
	//if bottom.probe(p, p.add(NewVec3(0, -1000000, 0))) == nil {
	return bottom.Contains(vec.NewVec3(p.GetX(), tri.yMin, p.GetZ())) //
	// 	return false //we are below the bottom cap
	// }

	// top := t.prismSides[4] //newEndCap(t, t.yMax)
	// if top.probe(p, p.add(NewVec3(0, 1000000, 0))) == nil {
	// 	return false //we are above the top cap
	// }

	// return true //both probes hit, we are betwen the endcaps

}

// test if the ray from p0 to p1 penetrates the volume of triangular based 'prism' extending between t.ymin and t.ymax
func (tri *Tri) probePrism(ray *ray.Ray) bool {

	//for all five sides of the prism - check for a penetration
	for _, s := range tri.prismFaces {
		if s.Probe(ray) {
			return true
		}
	}

	return false
}

func (tri *Tri) VprobeLand(p *vec.V3) (surfacePoint *vec.V3, surfaceTri *Tri) {

	pc := p.Clone()
	pc.SetY(0)
	t := tri.vProbe(pc) //recursively find the leaf tri that contains the point

	if t == nil {
		return nil, nil
	}

	//fire a ray through that plane
	ray := ray.New(vec.NewVec3(p.GetX(), 100000, p.GetZ()), vec.NewVec3(p.GetX(), -100000, p.GetZ()))
	if t.prismFaces[5].Probe(ray) {
		return ray.Intersect, t
	}

	return nil, nil
}

func (tri *Tri) vProbe(p *vec.V3) *Tri {

	bottom := tri.prismFaces[3] //newEndCap(t, t.yMin)
	p.SetY(tri.yMin)
	if bottom.Contains(p) {
		if len(tri.children) == 0 {
			return tri
		}

		for _, ct := range tri.children {
			tt := ct.vProbe(p)
			if tt != nil {
				return tt
			}
		}
	}

	//panic("vProbe failed to find a tri")
	return nil

}

func (tri *Tri) calcNormal() *vec.V3 {

	v := tri.mesh.verts

	numMeshVerts := uint32(len(v))
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
		prismFaces: []*poly.ConvexPoly{nil, nil, nil, nil, nil, nil},
	}

	t.calcCentre()
	//add this traingle to its verts list of triangles
	t.addToTouches()

	t.calcNormal()

	return &t
}

func (tri *Tri) ToSimpleMesh(id uint16, lm *TriMesh, fm *TriMesh, material string, asWater bool, faceTest func(face *Tri) bool) *mesh.SimpleMesh {

	//vc := uint32(len(lm.verts)) //vertex count
	vc := uint32(len(lm.verts))  //vertex count
	fis := make([]uint16, vc*10) //there will actually be many less faces than verts - but we need 3 uints per face

	if vc >= math.MaxUint16 {
		log.Logit("mesh too big - over 65535 verts")
	}

	wp := uint32(0)

	tri.getFacesInto(fis, &wp, faceTest) //populate Fis (recursivley from the root triangle)

	fis = fis[:wp] //truncate at the write pointer

	normals := lm.getNormals(asWater) //we must get normals (becuase it calculates them) before updating UVx's
	lm.updateUVxsFromNormals()
	return mesh.NewFilledSimpleMesh(id, lm.getPositions(asWater), normals, lm.getUVs(), fis, material)

}

func (tri *Tri) getYLowHigh() (low *vec.V3, high *vec.V3) {
	a := tri.mesh.verts[tri.vi[0]]
	b := tri.mesh.verts[tri.vi[1]]
	c := tri.mesh.verts[tri.vi[2]]

	yh := a
	if b.p.GetY() > yh.p.GetY() {
		yh = b
	}
	if c.p.GetY() > yh.p.GetY() {
		yh = c
	}

	yl := a
	if b.p.GetY() < yl.p.GetY() {
		yl = b
	}
	if c.p.GetY() < yl.p.GetY() {
		yl = c
	}

	return yl.p, yh.p

}
