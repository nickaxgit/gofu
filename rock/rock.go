package rock

import (
	"github.com/nickax/gofu/mesh"
	"github.com/nickax/gofu/vec"
	"math/rand"
)

type rockFace struct {
	verts      [3]int //indices into rock verts
	childCount int
	children   [4]*rockFace
}

type rock struct {
	verts     []vec.V3
	seed      rockFace
	midpoints map[[2]int]int //edge (low vert index, high vert index) to midpoint vert index
}

func (rock *rock) splitEdge(v0 int, v1 int, offset float64) int {

	key := [2]int{v0, v1}
	vi, present := rock.midpoints[key]
	if !present {
		mid := rock.verts[v0].Add(rock.verts[v1]).Multiply(0.5)
		mid.NormaliseInPlace() //place it on the unit sphere
		vi = rock.addVert(mid)
		rock.midpoints[key] = vi
		return vi
	}
	return vi
}

func (rock *rock) addVert(v vec.V3) int {
	rock.verts = append(rock.verts, v)
	return len(rock.verts) - 1
}

func (rf *rockFace) addChild(r *rock, v0 int, v1 int, v2 int) {
	rf.children[rf.childCount] = &rockFace{
		verts: [3]int{v0, v1, v2},
	}
	rf.childCount++
}

func (f *rockFace) splitRecursive(rock *rock, maxdepth int, depth int) {

	for i := 0; i < f.childCount; i++ {
		child := f.children[i]
		child.split4(rock)
		if depth < maxdepth-1 {
			child.splitRecursive(rock, maxdepth, depth+1)
		}
	}
}

func (f *rockFace) split4(rock *rock) {

	//      V2
	//		/\
	//     /  \
	// V4 /____\ V5
	//   / \  / \
	//  /___\/___\
	// V1   V3    V0

	v0, v1, v2 := f.verts[0], f.verts[1], f.verts[2]

	kink := (rand.Float64() - 0.5) * 0.05
	v3 := rock.splitEdge(v0, v1, kink)
	v4 := rock.splitEdge(v1, v2, kink)
	v5 := rock.splitEdge(v2, v0, kink)

	//wind clockwise
	f.addChild(rock, v5, v4, v2) //top
	f.addChild(rock, v0, v3, v5) //right
	f.addChild(rock, v3, v1, v4) //left
	f.addChild(rock, v4, v5, v3) //centre

}

func MakeRock(meshid uint16, material string) *mesh.SimpleMesh {
	r := newRock()

	sm := mesh.NewSimpleMesh(meshid, material, 1000, 2000)

	r.seed.splitRecursive(r, 4, 0)
	sm.SetVertsFromVec3(r.verts)

	r.seed.walkChildFaces(r, sm)

	return sm

}

func (face *rockFace) walkChildFaces(rock *rock, sm *mesh.SimpleMesh) {
	if face.children[0] == nil {
		//this is a leaf face - add to mesh
		sm.AddFace(uint16(face.verts[0]), uint16(face.verts[1]), uint16(face.verts[2]))
	} else {
		for _, child := range face.children {
			child.walkChildFaces(rock, sm)
		}
	}
}

func newRock() *rock {

	m := 0.5773502
	return &rock{

		verts: []vec.V3{
			vec.NewVec3(m, m, m),
			vec.NewVec3(m, -m, -m),
			vec.NewVec3(-m, m, -m),
			vec.NewVec3(-m, -m, m),
		},

		seed: rockFace{children: [4]*rockFace{
			{verts: [3]int{0, 1, 2}},
			{verts: [3]int{0, 3, 1}},
			{verts: [3]int{0, 2, 3}},
			{verts: [3]int{1, 3, 2}},
		}},
		midpoints: make(map[[2]int]int),
	}
}
