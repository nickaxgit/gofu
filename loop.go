package main

type loop struct {
	vi []uint16 //an ordered list of vertex indices - that form a closed loop
}

func NewLoop() *loop {
	return &loop{vi: []uint16{}}
}

func (l *loop) debugMesh(model *mesh, ctn *vec3, debugMesh *mesh) *mesh {

	for li := range l.vi { //index

		vi := l.vi[li]

		p := model.verts[vi].p
		np := model.verts[l.vi[(li+1)%len(l.vi)]].p
		t1 := debugMesh.addVert(p, false, 0, 0)

		dir := np.sub(p).normalise()
		t2 := debugMesh.addVert(p.sub(dir.cross(ctn).multiply(10)), false, 0, .5)
		t3 := debugMesh.addVert(np, false, 1, 0)

		debugMesh.addFi(t2, t1, t3)

	}

	return debugMesh
}

func (t *loop) merge(s *loop) {
	//merge the vertices of l into this loop (t becomes a degenerate loop)
	//last := t.vi[len(t.vi)-1]
	t.vi = append(t.vi, t.vi[0]) //close the first loop (add the 0th vert)
	t.vi = append(t.vi, s.vi...)
	t.vi = append(t.vi, s.vi[0])
	//t.vi = append(t.vi, t.vi[last)
}

func (loop *loop) triangulate(dbm *mesh, m *mesh, ctn *vec3) []uint16 {
	//triangulate this loop using the ear cutting algorithm
	//return a list of faces (triples of vertex indices)
	//the loop is assumed to be closed - vert indices may apear more than once as there may be bridges to inner holes

	facelist := []uint16{}

	ll := len(loop.vi)
	for {
		//find an ear
		facelist = append(facelist, loop.findAndRemoveEar(m, ctn)...) //remove the ear and add it to the face list (mutates l.vi)

		if len(loop.vi) < 3 {
			break
		}
	}
	logit("loop of ", ll, "verts triangulated into ", len(facelist)/3, " faces")

	return facelist
}

func (loop *loop) addVert(m *mesh, vi uint16) {
	loop.vi = append(loop.vi, vi)

	//check for verts in a line (can go eventually)
	if len(loop.vi) > 2 {
		l := len(loop.vi) - 1
		if m.inAline(loop.vi[l-2], loop.vi[l-1], loop.vi[l]) {
			logit("verts in a line")
		}
	}
}

// func (l *loop) reverse() {
// 	//reverse the order of the verts in the loop
// 	//this is used to ensure the loop is wound in the correct direction for the ear cutting algorithm
// 	//the loop is mutated
// 	ll := len(l.vi)
// 	n := make([]int, ll)
// 	for i := 0; i < len(l.vi); i++ {
// 		n[i] = l.vi[(ll-i)-1]
// 	}
// 	l.vi = n
// }

// check if the ear is empty (no other verts inside the triangle)
func (l *loop) isEarEmpty(bi int, m *mesh) bool {

	ll := len(l.vi) //length of the loop
	ai := (bi + ll - 1) % ll
	a := m.verts[l.vi[ai]].p
	b := m.verts[l.vi[bi]].p
	c := m.verts[l.vi[(bi+1)%ll]].p

	if m.inAline(l.vi[ai], l.vi[bi], l.vi[(bi+1)%ll]) {
		return false //triangle is degenerate and not one we want to keep
	}

	if a.equals(b) || a.equals(c) || b.equals(c) {
		panic("degenerate triangle")
	}

	for j := (bi + 2) % ll; j%ll != ai; j++ {
		v := m.verts[l.vi[j%ll]]

		if v.p.equals(a) || v.p.equals(b) || v.p.equals(c) {
			//panic("on ear vert") = can happen when there are hols
			return true //false
		}

		if v.p.isInsideTri(a, b, c, false, false) {
			return false
		}
	}
	return true

}

func (l *loop) findAndRemoveEar(m *mesh, ctn *vec3) (face []uint16) {
	//find an ear and remove it from the loop
	//return the indices of the three verts that make up the ear
	//the loop is mutated and a vertex is removed

	biggestAngle := float64(0)
	ear := []uint16{0, 0, 0}
	ei := uint16(65535)

	logit("loop contains", len(l.vi))
	for i := 0; i < len(l.vi); i++ {

		a := l.vi[i]
		bi := (i + 1) % len(l.vi)
		b := l.vi[bi]
		c := l.vi[(i+2)%len(l.vi)]

		ap := m.verts[a].p
		bp := m.verts[b].p
		cp := m.verts[c].p

		ab := bp.sub(ap)
		bc := cp.sub(bp)

		if ab.length() < 0.01 || bc.length() < 0.01 {
			panic("degenerate triangle")
		}

		turnAngle := -bc.SignedAngleFrom(ab, ctn) //math.Acos((cp.sub(bp)).normalise().dot((bp.sub(ap)).normalise()))

		if turnAngle == 0 {
			//	panic("zero turn angle") //can happen when ears are cut off
		}

		if turnAngle > 0 { // angle is postive for a left hand turn,
			logit("acute angle", turnAngle)
			if turnAngle > biggestAngle {
				if l.isEarEmpty(bi, m) {
					biggestAngle = turnAngle
					ear = []uint16{a, b, c}
					ei = uint16((i + 1) % len(l.vi)) //remove the middle 'B' vertex
				}
			}
		} else {
			logit("reflex angle (right turn)", turnAngle)
		}
	}

	if ei == 65535 {

		panic("no acute angle (or empty ear) found")
	}

	logit("Removing", ei, l.vi[ei])

	l.vi = append(l.vi[:ei], l.vi[ei+1:]...)

	if m.verts[ear[0]].p.equals(m.verts[ear[1]].p) || m.verts[ear[0]].p.equals(m.verts[ear[2]].p) || m.verts[ear[1]].p.equals(m.verts[ear[2]].p) {
		panic("degenerate ear by area")
	}
	if ear[0] == ear[1] || ear[0] == ear[2] || ear[1] == ear[2] {
		panic("degenerate ear")
	}

	return ear
}
