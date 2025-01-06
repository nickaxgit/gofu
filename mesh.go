package main

type mesh struct {
	verts []vert  //{}
	fi    []int   //{} //face indices
	yMax  float64 //= 0
}

func (m *mesh) addVert(p *Vec3) int {
	m.verts = append(m.verts, vert{p: p, uv: Vector{0, 0}, n: &Vec3{0, 0, 0}})
	return len(m.verts) - 1
}

func (m *mesh) addOrReuseVertAtXZ(p *Vec3) int {

	for i, v := range m.verts {
		if v.p.X > p.X-0.01 && v.p.X < p.X+0.01 {
			if v.p.Z > p.Z-0.01 && v.p.Z < p.Z+0.01 {
				return i
			}
		}
	}

	return m.addVert(p)
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
		penSet := NewPenSet()
		ct := clay.triangleFrom(i)
		//for each face of the tool mesh
		for j := 0; j < len(tool.fi); j += 3 {
			tt := tool.triangleFrom(j)
			penSet.append(ct.penetrationsBy(tt)) //edges of the tool through the interior face of the clay
			penSet.append(tt.penetrationsBy(ct)) //edges of the clay through the interior face of the tool
		}

		//we have gathered penetrations of many tool triangles of one clay triangle
		//assemble rings from those penetrations
		//some are of edges (of the clay) - in this case - the edge penetrating is the clay, and the face penetrated is the tool

		ring := newRing(ct.Vi...)

		pen := penSet.unused() //select an arbitrary first unused penetration

		for {
			childRing := newRing([]int{}...)
			for {
				childRing.vi = append(ring.vi, clay.addVert(pen.p))
				pen.used = true
				penSet.usedCount++ //this smells
				//find an 'opposite' penetration, at the same position (from an adjoining face)
				op := penSet.oppositePenByEdge(pen)
				if op != nil {
					op.used = true
					penSet.usedCount++
					pen = penSet.otherPenByToolFace(op)
				} else { //there is no opposite penetration by an edge so this is a clay edge penetration of a tool triangle
					//find the one other penetration where the penetrator is this penetrators target
					pen = penSet.penetratorIs(pen.clayTri) //pen.clayTri here is infact the tool
				}

				if pen.used {
					break //loop complete
				}
			}
			ring.addChild(childRing.vi...)

			if penSet.usedCount == len(penSet.pens) { //we have used all penatrations
				break
			}
			pen = penSet.unused() //find an unused pen
			if pen == nil {
				panic("no unused penetration")

			}

			ring.triangulate(ct) //trangulates around the child holes held in the ring (at ct.depth+1)
		}
	}

}
