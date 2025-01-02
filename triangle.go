package main

//import "crypto/rand"

//import "math/rand/v2"

type vert struct {
	p   Vec3
	n   Vec3
	uv  Vector
	wl  float64 //water level
	acc float64 //accumulated water (during a pass)
}

var verts = []vert{}

type Tri struct {
	depth    int
	Vi       []int  `json:"vi"`
	Children []*Tri `json:"children"`
}

func NewTri(depth int, vi []int) *Tri {

	return &Tri{depth: depth, Vi: vi, Children: []*Tri{}}
}

func addVert(p Vec3) int {
	for i, v := range verts {
		if v.p.X > p.X-0.01 && v.p.X < p.X+0.01 {
			if v.p.Z > p.Z-0.01 && v.p.Z < p.Z+0.01 {
				return i
			}
		}
	}
	verts = append(verts, vert{p: p, uv: Vector{0, 0}, n: Vec3{0, 0, 0}})
	return len(verts) - 1
}

//creates a new vertex halfway between the indexed verts a and b and returns the index of the new vertex
func split(a int, b int, dy float64) int {

	p := verts[a].p.tween(&verts[b].p, 0.5)
	p.Y += dy
	return addVert(p)
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
func (t *Tri) split(depth int, maxDepth int) {

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

	v3 := split(v0, v1, (rnGen.Float64()-.5)*2500/float64(depth+1))
	v4 := split(v1, v2, (rnGen.Float64()-.5)*2500/float64(depth+1))
	v5 := split(v2, v0, (rnGen.Float64()-.5)*2500/float64(depth+1))

	t.addChild(v0, v3, v5) //top
	t.addChild(v3, v1, v4) //right
	t.addChild(v5, v4, v2) //left
	t.addChild(v3, v4, v5) //centre

	if depth < maxDepth {
		for _, c := range t.Children {
			c.split(depth+1, maxDepth)
		}
	}

}
