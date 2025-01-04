package main

//import "crypto/rand"

//import "math/rand/v2"
import "slices"

type vert struct {
	p   *Vec3
	n   *Vec3
	uv  Vector
	wl  float64 //water level
	acc float64 //accumulated water (during a pass)
}

type angleIndex struct {
	angle float64
	index int
}

type Ring struct {
	vi       []int
	children []*Ring
}

type Tri struct {
	depth    int
	Vi       []int  `json:"vi"`
	Children []*Tri `json:"children"`
}

func NewTri(depth int, vi []int) *Tri {

	return &Tri{depth: depth, Vi: vi, Children: []*Tri{}}
}

func (state *State) addVert(p *Vec3) int {

	for i, v := range state.land.verts {
		if v.p.X > p.X-0.01 && v.p.X < p.X+0.01 {
			if v.p.Z > p.Z-0.01 && v.p.Z < p.Z+0.01 {
				return i
			}
		}
	}

	state.land.verts = append(state.land.verts, vert{p: p, uv: Vector{0, 0}, n: &Vec3{0, 0, 0}})
	return len(state.land.verts) - 1
}

//creates a new vertex halfway between the indexed verts a and b and returns the index of the new vertex
func (state *State) splitEdge(a int, b int, dy float64) int {

	verts := state.land.verts
	p := verts[a].p.tween(verts[b].p, 0.5)
	p.Y += dy
	return state.addVert(p)
}

func (t *Tri) addChild(a, b, c int) {
	t.Children = append(t.Children, NewTri(t.depth+1, []int{a, b, c}))
}

func (t *Tri) gather(depth int, i []int, p *int) {

	if t.depth == depth {
		i[*p] = t.Vi[0]
		*p++
		i[*p] = t.Vi[1]
		*p++
		i[*p] = t.Vi[2]
		*p++

	}

	for _, c := range t.Children {
		c.gather(depth, i, p)
	}

}
func (t *Tri) split(state *State, maxRdepth int, maxHeight float64) {

	//      0
	//		/\
	//     /  \
	//  5 /____\ 3
	//   / \  / \
	//  /___\/___\
	// 2     4    1

	v0 := t.Vi[0]
	v1 := t.Vi[1]
	v2 := t.Vi[2]

	Yrange := maxHeight / float64(t.depth+1) //maximum kink in this edge

	v3 := state.splitEdge(v0, v1, (rnGen.Float64()-.5)*Yrange)
	v4 := state.splitEdge(v1, v2, (rnGen.Float64()-.5)*Yrange)
	v5 := state.splitEdge(v2, v0, (rnGen.Float64()-.5)*Yrange)

	t.addChild(v0, v3, v5) //top
	t.addChild(v3, v1, v4) //right
	t.addChild(v5, v4, v2) //left
	t.addChild(v3, v4, v5) //centre

	for _, c := range t.Children {
		if c.depth < maxRdepth {
			c.split(state, maxRdepth, maxHeight)
		}
	}

}

//you have a tool and some clay
//the tool is not effected by the clay  (but the process is symetrical so you can later use the clay as the tool and vice versa)
//find all the double penetrations of the tool into the clay (where 2)
//both penetrations should be in the same direction and are a common edge of two faces
//comparing this direction to the normal of the face will tell you if the tool is entering or exiting the clay
//each triangle of the tool yields a pair of penetrations (although not neccessariy in the same clay triangle)

//aditionally the (bounded) plane of the tool must be intersected with the edges of the clay to produce additiaonl points of penetration

func (tri *Tri) intersectsOrContainsChildHoleOf(mesh *mesh, ring *Ring) bool {

	//if any of this rings child (hole) rings interect, or are completely contained by tri - return true
	for _, child := range ring.children {
		for v := range child.vi {
			if tri.contains(mesh.verts, mesh.verts[v].p) {
				return true //early exit - if any vertex of the hole is inside the triangle
			}
		}
		if tri.intersects(mesh, child) {
			return true
		}
	}

	return false

}

//does the triangle intect any segment of the ring
func (tri *Tri) intersects(mesh *mesh, ring *Ring) bool {

	for i := 0; i < len(ring.vi); i++ {
		for j := 0; j < 3; j++ {
			if mesh.linesIntersect(tri.Vi[j], tri.Vi[(j+1)%3], ring.vi[i], ring.vi[(i+1)%len(ring.vi)]) {
				return true
			}
		}
	}
	return false

}

//do two coplanar lines intersect ?
func (mesh *mesh) linesIntersect(ai, bi, ci, di int) bool {

	a := mesh.verts[ai].p
	b := mesh.verts[bi].p
	c := mesh.verts[ci].p
	d := mesh.verts[di].p
	ab := b.subtract(a)
	cd := d.subtract(c)

	n := ab.cross(cd)
	if n.lengthSq() < 0.00000001 { //lines are parallel
		return false
	}

	b = b.projectOntoPlane(a, n)
	c = c.projectOntoPlane(a, n)
	d = d.projectOntoPlane(a, n)

	d0 := a.distanceFromLine(c, d)
	d1 := b.distanceFromLine(c, d)

	ip := a.add(ab.multiply(d0 / (d0 + d1)))
	if ip.liesBetween(a, b) && ip.liesBetween(c, d) {
		return true
	}

	return false

}

func (p *Vec3) insideAngle(a, b, c *Vec3) float64 {
	//is p inside the angle abc
	ab := b.subtract(a)
	ac := c.subtract(a)
	ap := p.subtract(a)

	return ab.cross(ac).dot(ab.cross(ap)) //>0

}

func (ring *Ring) gatherPointsInside(mesh *mesh, a, b, c *Vec3, points []angleIndex) {

	for vi := range ring.vi {
		p := mesh.verts[ring.vi[vi]].p
		if b.distanceFrom(p) > 0.0001 { //do not include the 'hinge' of the sector
			ia := p.insideAngle(a, b, c) //needs testing
			if ia > 0 {
				points = append(points, angleIndex{ia, vi})
			}
		}
	}
}

func compAngle(i, j angleIndex) int {
	return int((i.angle - j.angle) * 10000)
}

func sortedSectorVerts(mesh *mesh, ring *Ring, i int) []int {
	//return the vertices of the ring sorted by their angle with the vertex i
	//clockwise

	points := make([]angleIndex, 100) //a list of vert indexes, and their angle within the sector

	a := mesh.verts[ring.vi[(i+1)%len(ring.vi)]].p
	b := mesh.verts[ring.vi[i]].p
	c := mesh.verts[ring.vi[(i-1+len(ring.vi))%len(ring.vi)]].p

	ring.gatherPointsInside(mesh, a, b, c, points)

	for _, childRing := range ring.children {
		childRing.gatherPointsInside(mesh, a, b, c, points)
	}

	slices.SortFunc(points, compAngle)

	return []int{}
}

//triangulate an arbitrary polygon, which may have holes
func (ringWithChildHoles *Ring) triangulate(mesh *mesh) []Tri {
	//return a list of triangles that cover the area of the polygon

	faces := []Tri{}
	//edgelist := make([][]int,100) //vert index point pairs
	for vi, i := range ringWithChildHoles.vi {

		a := vi //state.land.verts[vi] //for each vertex of the outer ring (typically a triangle)

		//sort ALL the vertices in the sector between i+1,i and i-1 by their angle i,i+1 (clockeise)
		//include the next and previous veritces on the outer ring
		clock := sortedSectorVerts(mesh, ringWithChildHoles, i)

		for i := 0; i < len(clock)-1; i++ {
			b := clock[i]
			c := clock[i+1]
			pFace := NewTri(0, []int{a, b, c}) //potential face
			if !pFace.intersectsOrContainsChildHoleOf(mesh, ringWithChildHoles) {
				// if !crosses(b,c,edgelist){
				// 	edgelist = append(edgelist,[]int{b,c})
				faces = append(faces, *pFace) //add the triangle to the list
			}
		}

	}

	return faces
}

// //return 0, 1 or 2 points of intersection of the edges of triangle b, with the plane of triangle a
// func(a *Tri) intersectedBy(b *Tri) []Vec3 {

// }
