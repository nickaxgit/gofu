package main

//import "crypto/rand"
import "strconv"

//import "math/rand/v2"

type vert struct {
	p   *Vec3
	n   *Vec3
	uv  Vector
	wl  float64 //water level
	acc float64 //accumulated water (during a pass)
}

type Tri struct {
	depth    int
	Vi       []int  `json:"vi"`
	Children []*Tri `json:"children"`
	mesh     *mesh  //a reference to the mesh this tri is part of (that the vi's point into v's of)
}

// func NewTri(depth int, vi []int) *Tri {
// 	return &Tri{depth: depth, Vi: vi, Children: []*Tri{}}
// }

//creates a new vertex halfway between the indexed verts a and b and returns the index of the new vertex
func (m *mesh) splitEdge(a int, b int, dy float64) int {

	p := m.verts[a].p.tween(m.verts[b].p, 0.5)
	p.Y += dy
	return m.addOrReuseVertAtXZ(p)
}

func (t *Tri) addChild(a, b, c int) {
	t.Children = append(t.Children, t.mesh.makeTri(t.depth+1, a, b, c))
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
func (t *Tri) split(m *mesh, maxRdepth int, maxHeight float64) {

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

	v3 := m.splitEdge(v0, v1, (rnGen.Float64()-.5)*Yrange)
	v4 := m.splitEdge(v1, v2, (rnGen.Float64()-.5)*Yrange)
	v5 := m.splitEdge(v2, v0, (rnGen.Float64()-.5)*Yrange)

	t.addChild(v0, v3, v5) //top
	t.addChild(v3, v1, v4) //right
	t.addChild(v5, v4, v2) //left
	t.addChild(v3, v4, v5) //centre

	for _, c := range t.Children {
		if c.depth < maxRdepth {
			c.split(m, maxRdepth, maxHeight)
		}
	}

}

func (t *Tri) distanceFrom(p *Vec3) float64 {

	v := t.mesh.verts
	a := v[t.Vi[0]].p
	b := v[t.Vi[1]].p
	c := v[t.Vi[2]].p

	//get the normal of the triangle
	n := b.sub(a).cross(c.sub(a)).normalise() //todo - cache/gen the normals once

	pop := p.sub(a)

	//get the distance from the point to the plane
	d := pop.dot(n)

	return d

}

func (t *Tri) contains(pop *Vec3) bool {

	v := t.mesh.verts //this is a refeerence not a copy

	a := v[t.Vi[0]].p
	b := v[t.Vi[1]].p
	c := v[t.Vi[2]].p

	//get the normal of the triangle
	n := b.sub(a).cross(c.sub(a)).normalise()

	//project the point onto the plane of the triangle
	//and get the vector from the point to the plane
	j := pop.sub(a)

	//get the distance from the point to the plane
	d := j.dot(n)

	if d > 0.01 || d < -0.01 {
		panic("point not on plane " + strconv.Itoa(int(d*1000)))
	}

	//get the vectors from the projected point to the vertices of the triangle
	va := pop.sub(a)
	vb := pop.sub(b)
	vc := pop.sub(c)

	//cross each edge with the point-to-vertex vector
	na := b.sub(a).cross(va).normalise()
	nb := c.sub(b).cross(vb).normalise()
	nc := a.sub(c).cross(vc).normalise()

	//get the dot products of the normals with the normal of the triangle
	da := na.dot(n)
	db := nb.dot(n)
	dc := nc.dot(n)

	//if the dot products are all positive, then the point is inside the triangle
	if da > 0 && db > 0 && dc > 0 {
		return true
	}

	return false

}

func (t *Tri) normal() *Vec3 {
	v := t.mesh.verts
	return v[t.Vi[1]].p.sub(v[t.Vi[0]].p).cross(v[t.Vi[2]].p.sub(v[t.Vi[0]].p)).normalise()
}
