package mesh

import (
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/vec"
	"math"
)

type SimpleMesh struct {
	id           uint16
	pad          byte      //arays ,must be aligned
	p            []float32 //positions
	n            []float32 //normals
	uv           []float32 //TC's
	fi           []uint16  //faces (3 indeices per triangle)
	materialName string
	vwp          uint16
	fwp          uint16
}

func (m *SimpleMesh) Reset() {
	m.vwp = 0
	m.fwp = 0

}

type Tcs struct {
	left, top, right, bottom float32
}

func NewTcs(left, top, right, bottom float32) *Tcs {
	return &Tcs{left: left, top: top, right: right, bottom: bottom}
}
func (i *Tcs) clone() *Tcs {
	return &Tcs{left: i.left, top: i.top, right: i.right, bottom: i.bottom}
}

func (i *Tcs) flipV() *Tcs {
	return &Tcs{left: i.left, top: i.bottom, right: i.right, bottom: i.top}
}

func (i *Tcs) flipH() *Tcs {
	return &Tcs{left: i.right, top: i.top, right: i.left, bottom: i.bottom}
}

func New(id uint16, materialName string, numVerts int, numFaces int) *SimpleMesh {
	// p []float32, n []float32, uv []float32, fi []uint16
	return &SimpleMesh{id: id, pad: 0, p: make([]float32, numVerts*3), n: make([]float32, numVerts*3), uv: make([]float32, numVerts*2), fi: make([]uint16, numFaces*3), materialName: materialName}
}
func (sm *SimpleMesh) AddTube(start *vec.V3, end *vec.V3, xAxis *vec.V3, startRadius float64, endRadius float64, tcs *Tcs, sides int) {
	//add a tube between start and end, with the given radii at each end
	//the tube is aligned with the vector start->end

	ftcs := tcs.clone() //flipable tcs

	yAxis := end.Sub(start).Normalise()
	zAxis := xAxis.Cross(yAxis).Normalise()

	v2 := uint16(0)
	v3 := uint16(1)

	ov0 := uint16(0)
	ov1 := uint16(1)

	for i := range sides { // goes from 0 to sides -1
		a := float64(i) * 2 * math.Pi / float64(sides)
		dx := xAxis.Multiply(math.Cos(a))
		dz := zAxis.Multiply(math.Sin(a))
		n := dx.Add(dz).Normalise()

		//toggle tcs as we wrap / and or flip them on alternate outbound segments
		ftcs = ftcs.flipH()
		v0 := sm.AddVert(start.Add(dx.Multiply(startRadius)).Add(dz.Multiply(startRadius)), n, ftcs.left, ftcs.top)
		v1 := sm.AddVert(end.Add(dx.Multiply(endRadius)).Add(dz.Multiply(endRadius)), n, ftcs.left, ftcs.bottom)

		if i > 0 {
			//sm.AddFace(v0, v1, v2)
			//sm.AddFace(v1, v3, v2)
			sm.AddFace(v0, v2, v1)
			sm.AddFace(v1, v2, v3)
		} else {
			ov0 = v0
			ov1 = v1
		}
		v2 = v0
		v3 = v1

	}

	//stitch the tube seam closed
	sm.AddFace(ov0, v2, v3)
	sm.AddFace(ov1, ov0, v3)

}

func (sm *SimpleMesh) Billboard(p *vec.V3, up *vec.V3, camPos *vec.V3, widthBottom float64, widthTop float64, height float64, shape int, tcs *Tcs) {

	//	up := NewVec3(0, 1, 0)
	toCam := camPos.Sub(p).Normalise()
	right := toCam.Cross(up).Normalise()
	rightTop := right.Multiply(widthTop * .5)
	rightBottom := right.Multiply(widthBottom * .5)

	top := p.Add(up.Multiply(height))

	v0 := sm.AddVert(p.Sub(rightBottom), toCam, tcs.left, tcs.bottom)
	v1 := sm.AddVert(p.Sub(rightBottom), toCam, tcs.right, tcs.bottom)

	//triangular billboard (pine trees/flames)
	if shape == 3 {
		tcMid := (tcs.left + tcs.right) * .5
		v2 := sm.AddVert(top, toCam, tcMid, tcs.top)
		sm.AddFace(v0, v1, v2)
		return
	}

	v2 := sm.AddVert(top.Add(rightTop), toCam, tcs.right, tcs.top)
	v3 := sm.AddVert(top.Sub(rightTop), toCam, tcs.left, tcs.top)

	//v3--v2
	//f1 /
	//  /f0
	// /
	//v0--v1
	sm.AddFace(v0, v2, v1) //f0
	sm.AddFace(v0, v3, v2) //f1

}

// func (m *simpleMesh) offset(offset *vec.V3) *simpleMesh {
// 	for i := 0; i < len(m.p); i += 3 {
// 		m.p[i] += float32(offset.x)
// 		m.p[i+1] += float32(offset.y)
// 		m.p[i+2] += float32(offset.z)
// 	}
// 	return m
// }

func NewFilledSimpleMesh(id uint16, p []float32, n []float32, uv []float32, fi []uint16, materialName string) *SimpleMesh {
	return &SimpleMesh{id: id, pad: 0, p: p, n: n, uv: uv, fi: fi, materialName: materialName, vwp: uint16(len(p) / 3), fwp: uint16(len(fi) / 3)}
}

func (sm *SimpleMesh) AddVert(p *vec.V3, n *vec.V3, u float32, v float32) uint16 {

	//sm.p = append(sm.p, p.AsFloat32s()...)

	wp3 := int(sm.vwp) * 3
	wp2 := int(sm.vwp) * 2
	sm.p[wp3] = float32(p.X) //:vwp+3] = pappend(sm.p, p.AsFloat32s()...)
	sm.p[wp3+1] = float32(p.Y)
	sm.p[wp3+2] = float32(p.Z)
	//sm.vwp += 3

	//sm.n = append(sm.n, n.AsFloat32s()...)
	sm.n[wp3] = float32(n.X) //:vwp+3] = pappend(sm.n, n.AsFloat32s()...)
	sm.n[wp3+1] = float32(n.Y)
	sm.n[wp3+2] = float32(n.Z)

	//sm.uv = append(sm.uv, u, v)
	sm.uv[wp2] = u //:vwp+2] = pappend(sm.uv, u, v)
	sm.uv[wp2+1] = v

	sm.vwp += 1

	return sm.vwp - 1

}

func (sm *SimpleMesh) VertCount() uint16 {
	return sm.vwp
}

func (sm *SimpleMesh) FaceCount() uint16 {
	return sm.fwp
}

func (sm *SimpleMesh) AddFace(v1, v2, v3 uint16) {

	wp3 := uint16(sm.fwp * 3)
	sm.fi[wp3] = v1
	sm.fi[wp3+1] = v2
	sm.fi[wp3+2] = v3
	sm.fwp += 1
	//sm.fi = append(sm.fi, v1, v2, v3)

}

func (sm *SimpleMesh) WriteTo(message *msg.Msg, maxInstances uint16) {

	wp := message.WritePointer()
	numPadBytes := 4 - ((wp + 1) % 4) + 1
	padBytes := make([]byte, numPadBytes)
	message.Write(msg.Mesh, sm.id, //0,1,2
		uint32(len(sm.p)/3),  //number of vertices 3,4,5,6
		uint32(len(sm.fi)/3), //number of faces 7,8,9,10
		byte(numPadBytes),    //11, because buffers are now in a single message - we need variable padding to align the float arrays
		padBytes,             //0 pad bytes are awkward - so we will pad with 1,2,3 or 4 bytes as needed		               //because buffers are now in a single message - we need variable padding to align the float arrays
		sm.p,                 //vertex positions (slice of Float32, 3 per vert)
		sm.n,                 //vertex normals (slice of Float32, 3 per vert)
		sm.uv,                //uv coordinates (slice of Float32, 2 per vert)
		sm.fi,                //faces (slice of Uint16, 3 per face)
		sm.materialName,
		maxInstances,
	)
}

// func (sm *SimpleMesh) ToMsg(maxInstances uint16) *msg.Msg {

// 	msg := msg.NewMsg(msg.Mesh, sm.id,

// 	return msg

// }
