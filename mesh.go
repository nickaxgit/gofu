package main

type mesh struct {
	verts []vert  //{}
	fi    []int   //{} //face indices
	yMax  float64 //= 0
}

func (m *mesh) addOrReuseVertAtXZ(p *Vec3) int {

	for i, v := range m.verts {
		if v.p.X > p.X-0.01 && v.p.X < p.X+0.01 {
			if v.p.Z > p.Z-0.01 && v.p.Z < p.Z+0.01 {
				return i
			}
		}
	}

	m.verts = append(m.verts, vert{p: p, uv: Vector{0, 0}, n: &Vec3{0, 0, 0}})
	return len(m.verts) - 1
}

//creates a triangle (which is NOT a face) from the indices of the verts
//holds a pointer to this mesh for access to its verts
//note - it does not add vertices or face indices to the mesh
func (m *mesh) makeTri(depth int, vi ...int) *Tri {
	return &Tri{depth: depth, Vi: vi, Children: []*Tri{}, mesh: m}
}

func (m *mesh) triangleFrom(fi int) *Tri {
	return &Tri{depth: 0, Vi: []int{fi, fi + 1, fi + 2}, Children: []*Tri{}, mesh: m}
}

func (tool *mesh) cut(clay *mesh) {
	//the tool is unharmed (for now)
	//the clay has its faces subdivided

	//for each face of the clay
	for i := 0; i < len(clay.fi); i += 3 {
		ct := clay.triangleFrom(i)
		//for each face of the tool
		for j := 0; j < len(tool.fi); j += 3 {
			ct.penetrationsBy(tool.triangleFrom(j))
		}
	}

}
