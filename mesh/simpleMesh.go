package mesh

import (
	"fmt"
	"math"

	"github.com/nickax/gofu/colors"
	"github.com/nickax/gofu/game/msg"

	"github.com/nickax/gofu/terrainVert"

	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/tcs"
	"github.com/nickax/gofu/vec"
)

func (sm *SimpleMesh) AddFace(v1, v2, v3 uint16) {
	wp3 := sm.Tris * 3
	sm.fis[wp3] = v1
	sm.fis[wp3+1] = v2
	sm.fis[wp3+2] = v3

	sm.Tris += 1

}

type SimpleMesh struct {
	id           uint16
	materialName string
	pad          byte      //arays ,must be aligned
	p            []float32 //positions
	n            []float32 //normals (per vertex)
	uv           []float32 //TC's
	fis          []uint16
	//fn           []vec.V3 //face normals
	//materialName []string // material  name for each submesh
	Verts uint16
	Tris  uint16
}

func (sm *SimpleMesh) FillFrom(verts []*terrainVert.Vert, allNormals []vec.V3) {

	for i, v := range verts {
		wp3 := i * 3
		wp2 := i * 2
		sm.p[wp3] = float32(v.P.X)
		sm.p[wp3+1] = float32(v.P.Y)
		sm.p[wp3+2] = float32(v.P.Z)
		n := allNormals[i]
		sm.n[wp3] = float32(n.X)
		sm.n[wp3+1] = float32(n.Y)
		sm.n[wp3+2] = float32(n.Z)
		sm.uv[wp2] = float32(v.Uv.X)
		sm.uv[wp2+1] = float32(v.Uv.Y)
	}

	sm.Verts = uint16(len(verts))

}

//Many verts (with different normals) may be present at the same position
//This function finds the existing vert index with the normal best aligned to the given face normal
func (sm *SimpleMesh) BestNormalMatchAmongst(vis []uint16, faceNormal vec.V3) (uint16, float64) {

	bestVi := uint16(0)
	bestAlign := -2.0

	var align float64
	for _, k := range vis {
		align = sm.getNormal(k).Dot(faceNormal)
		if align > bestAlign {
			bestAlign = align
			bestVi = k
		}
	}

	return bestVi, bestAlign
}

func (sm *SimpleMesh) getNormal(vi uint16) vec.V3 {
	wp3 := vi * 3
	return vec.NewVec3(float64(sm.n[wp3]), float64(sm.n[wp3+1]), float64(sm.n[wp3+2]))
}

func clamp(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// func (sm *SimpleMesh) ReduceToUsedVerts(verts []*terrainVert.Vert, allNormals []vec.V3, steepnesses []float64, asWater bool) {

// 	//kill two birds with one stone - generate a subset of verts just for the face sets - and set all their Y's

// 	vertCount := len(verts)
// 	np := make([]float32, vertCount*3)  // new position
// 	nn := make([]float32, vertCount*3)  // new normals
// 	nuv := make([]float32, vertCount*2) // new Uvs

// 	mapping := make(map[uint16]uint16) //map from old vert index to new vert index

// 	n := vec.NewVec3(0, 1, 0)

// 	for i := range sm.Tris * 3 {

// 		fvi := sm.fis[i]
// 		tfi, present := mapping[fvi]
// 		if present {
// 			fi := i / 3
// 			ntn := vec.NewVec3(float64(nn[tfi*3]), float64(nn[tfi*3+1]), float64(nn[tfi*3+2]))
// 			if sm.fn[fi].Dot(ntn) < .85 { //our normal differs significantly from that at the vertex we are mapping to
// 			} else {

// 				sm.fis[i] = uint16(tfi) //update the face index to point to the new vert index
// 			}
// 		} else {
// 			v := verts[fvi]

// 			ni := uint16(len(mapping))
// 			ni2 := ni * 2 //new index (for uv)
// 			ni3 := ni * 3 //new index (for normal/pos)

// 			nn[ni3], nn[ni3+1], nn[ni3+2] = float32(n.X), float32(n.Y), float32(n.Z) //float32(v.n.X), float32(v.n.Y), float32(v.n.Z)

// 			//lower TC's of flat ground (equivalent to raising the TC's of steep ground)
// 			// tcv := .3 + float32(v.uv.Y-steepness[fvi]*.3)
// 			// if tcv < .01 {
// 			// 	tcv = 0
// 			// }
// 			// if tcv > 0.99 {
// 			// 	tcv = 0.99
// 			// }

// 			//contours is a curve, converting Y (height) to Texture Number

// 			uvy := 0.0
// 			if !asWater {
// 				uvy = terrainVert.Contours.GetX(v.P.Y)
// 				//steepOff := terrainVert.Steeps.GetY(math.Floor(uvy))

// 				//uvy += steepOff * (1 - steepnesses[fvi]) //* 10 //lower tc's of steep ground, snow becomes rock, grass becomes sand, mud becomes water
// 			}

// 			nuv[ni2], nuv[ni2+1] = float32(v.Uv.X), float32(uvy)

// 			np[ni3] = float32(v.P.X)
// 			if asWater {
// 				np[ni3+1] = float32(v.P.Y + v.Wl)
// 			} else {
// 				np[ni3+1] = float32(v.P.Y)
// 			}
// 			np[ni3+2] = float32(v.P.Z)

// 			mapping[fvi] = ni

// 			sm.fis[i] = ni //update the face index to point to the new vert index
// 		}

// 	}
// 	if len(mapping) > 65530 {
// 		panic("large mesh:" + fmt.Sprint(len(mapping)) + "verts for mesh")
// 	}

// 	sm.Verts = uint16(len(mapping))

// 	sm.p = np[0 : sm.Verts*3] //truncate to actual size
// 	sm.n = nn[0 : sm.Verts*3]
// 	sm.uv = nuv[0 : sm.Verts*2]

// 	log.Logit(len(verts), "verts reduced to", sm.Verts)

// }

// func (sm *SimpleMesh) FromUsedTerrainVerts(verts []*terrainVert.Vert, allNormals []vec.V3, steepnesses []float64, asWater bool) {

// 	//kill two birds with one stone - generate a subset of verts just for the face sets - and set all their Y's

// 	vertCount := len(verts)
// 	np := make([]float32, vertCount*3)  // new position
// 	nn := make([]float32, vertCount*3)  // new normals
// 	nuv := make([]float32, vertCount*2) // new Uvs

// 	mapping := make(map[uint16]uint16) //map from old vert index to new vert index

// 	n := vec.NewVec3(0, 1, 0)

// 	for i := range sm.Tris * 3 {

// 		fvi := sm.fis[i]
// 		tfi, present := mapping[fvi]
// 		if present {
// 			fi := i / 3
// 			ntn := vec.NewVec3(float64(nn[tfi*3]), float64(nn[tfi*3+1]), float64(nn[tfi*3+2]))
// 			if sm.fn[fi].Dot(ntn) < .85 { //our normal differs significantly from that at the vertex we are mapping to
// 			} else {

// 				sm.fis[i] = uint16(tfi) //update the face index to point to the new vert index
// 			}
// 		} else {
// 			v := verts[fvi]

// 			ni := uint16(len(mapping))
// 			ni2 := ni * 2 //new index (for uv)
// 			ni3 := ni * 3 //new index (for normal/pos)

// 			nn[ni3], nn[ni3+1], nn[ni3+2] = float32(n.X), float32(n.Y), float32(n.Z) //float32(v.n.X), float32(v.n.Y), float32(v.n.Z)

// 			//lower TC's of flat ground (equivalent to raising the TC's of steep ground)
// 			// tcv := .3 + float32(v.uv.Y-steepness[fvi]*.3)
// 			// if tcv < .01 {
// 			// 	tcv = 0
// 			// }
// 			// if tcv > 0.99 {
// 			// 	tcv = 0.99
// 			// }

// 			//contours is a curve, converting Y (height) to Texture Number

// 			uvy := 0.0
// 			if !asWater {
// 				uvy = terrainVert.Contours.GetX(v.P.Y)
// 				//steepOff := terrainVert.Steeps.GetY(math.Floor(uvy))

// 				//uvy += steepOff * (1 - steepnesses[fvi]) //* 10 //lower tc's of steep ground, snow becomes rock, grass becomes sand, mud becomes water
// 			}

// 			nuv[ni2], nuv[ni2+1] = float32(v.Uv.X), float32(uvy)

// 			np[ni3] = float32(v.P.X)
// 			if asWater {
// 				np[ni3+1] = float32(v.P.Y + v.Wl)
// 			} else {
// 				np[ni3+1] = float32(v.P.Y)
// 			}
// 			np[ni3+2] = float32(v.P.Z)

// 			mapping[fvi] = ni

// 			sm.fis[i] = ni //update the face index to point to the new vert index
// 		}

// 	}
// 	if len(mapping) > 65530 {
// 		panic("large mesh:" + fmt.Sprint(len(mapping)) + "verts for mesh")
// 	}

// 	sm.Verts = uint16(len(mapping))

// 	sm.p = np[0 : sm.Verts*3] //truncate to actual size
// 	sm.n = nn[0 : sm.Verts*3]
// 	sm.uv = nuv[0 : sm.Verts*2]

// 	log.Logit(len(verts), "verts reduced to", sm.Verts)

// }

func NewSimpleMesh(id uint16, materialName string, numVerts int, numTris int) *SimpleMesh {
	// p []float32, n []float32, uv []float32, fi []uint16
	sm := SimpleMesh{id: id, materialName: materialName, pad: 0,
		p:   make([]float32, numVerts*3),
		n:   make([]float32, numVerts*3),
		uv:  make([]float32, numVerts*2),
		fis: make([]uint16, numVerts*3),
		//fn:  make([]vec.V3, numTris)
	}
	return &sm
}

func (sm *SimpleMesh) WriteNormalsTo(msg *msg.Msg) {

	for i := 0; i < int(sm.Verts); i++ {
		rp := i * 3
		n := vec.NewVec3(float64(sm.n[rp]), float64(sm.n[rp+1]), float64(sm.n[rp+2]))
		p := vec.NewVec3(float64(sm.p[rp]), float64(sm.p[rp+1]), float64(sm.p[rp+2]))
		msg.Write(p, p.Add(n.Multiply(0.5)), colors.LightGreen)

	}

}

func (sm *SimpleMesh) AddTube(start vec.V3, end vec.V3, xAxis vec.V3, startRadius float64, endRadius float64, tcs tcs.Tcs, sides int) {
	//add a tube between start and end, with the given radii at each end
	//the tube is aligned with the vector start->end

	ftcs := tcs.Clone() //flipable tcs

	yAxis := end.Sub(start).Normalised()
	zAxis := xAxis.Cross(yAxis).Normalised()

	v2 := uint16(0)
	v3 := uint16(1)

	ov0 := uint16(0)
	ov1 := uint16(1)

	n := vec.NewVec3(0, 0, 0)
	for i := range sides { // goes from 0 to sides -1
		a := float64(i) * 2 * math.Pi / float64(sides)
		dx := xAxis.Multiply(math.Cos(a))
		dz := zAxis.Multiply(math.Sin(a))
		n = dx.Add(dz).Normalised()

		//toggle tcs as we wrap / and or flip them on alternate outbound segments
		ftcs = ftcs.FlipH()
		v0 := sm.AddVert(start.Add(dx.Multiply(startRadius)).Add(dz.Multiply(startRadius)), n, ftcs.Left, ftcs.Top)
		v1 := sm.AddVert(end.Add(dx.Multiply(endRadius)).Add(dz.Multiply(endRadius)), n, ftcs.Left, ftcs.Bottom)

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
	sm.AddFace(ov0, v2, v3) //slopy - this normal is not qute right (mind you none of them are)
	sm.AddFace(ov1, ov0, v3)

}

func (sm *SimpleMesh) Billboard(p vec.V3, up vec.V3, camPos vec.V3, widthBottom float64, widthTop float64, height float64, shape int, tcs tcs.Tcs) {

	//TODO - obsolete really use IM's (of a 2 triangle mesh)

	//	up := NewVec3(0, 1, 0)
	toCam := camPos.Sub(p).Normalised()
	right := toCam.Cross(up).Normalised()
	rightTop := right.Multiply(widthTop * .5)
	rightBottom := right.Multiply(widthBottom * .5)

	top := p.Add(up.Multiply(height))

	v0 := sm.AddVert(p.Sub(rightBottom), toCam, tcs.Left, tcs.Bottom)
	v1 := sm.AddVert(p.Sub(rightBottom), toCam, tcs.Right, tcs.Bottom)

	//triangular billboard (pine trees/flames)
	if shape == 3 {
		tcMid := (tcs.Left + tcs.Right) * .5
		v2 := sm.AddVert(top, toCam, tcMid, tcs.Top)
		sm.AddFace(v0, v1, v2)
		return
	}

	v2 := sm.AddVert(top.Add(rightTop), toCam, tcs.Right, tcs.Top)
	v3 := sm.AddVert(top.Sub(rightTop), toCam, tcs.Left, tcs.Top)

	//v3--v2
	//f1 /
	//  /f0
	// /
	//v0--v1
	sm.AddFace(v0, v2, v1) //f0
	sm.AddFace(v0, v3, v2) //f1

}

func (m *SimpleMesh) Mutate(id uint16, materialName string) *SimpleMesh {
	m.id = id
	m.materialName = materialName
	return m
}

func (sm *SimpleMesh) AddVert(p vec.V3, normal vec.V3, u, v float64) uint16 {

	ls := normal.LengthSq()
	if ls < 0.99 || ls > 1.01 || math.IsNaN(ls) || math.IsInf(ls, 0) {
		panic(fmt.Sprintf("bad normal passed to AddVert: n=%v ls=%v p=%v", normal, ls, p))
	}

	wp3 := sm.Verts * 3
	wp2 := int(sm.Verts) * 2

	sm.p[wp3] = float32(p.X)
	sm.p[wp3+1] = float32(p.Y)
	sm.p[wp3+2] = float32(p.Z)

	sm.n[wp3] = float32(normal.X)
	sm.n[wp3+1] = float32(normal.Y)
	sm.n[wp3+2] = float32(normal.Z)

	sm.uv[wp2] = float32(u)
	sm.uv[wp2+1] = float32(v)

	sm.Verts++

	return sm.Verts - 1 //return the index of the added vert

}

func (sm *SimpleMesh) WriteGeometryTo(message *msg.Msg, maxInstances uint32, maxBillBoards uint32) {

	if sm.Verts == 0 || sm.Tris == 0 {
		panic("Attempt to write empty mesh")
	}
	if sm.Verts > 65530 {
		panic("Mesh has too many vertices " + fmt.Sprint(sm.Verts))
	}
	//wp := message.WritePointer()
	//numPadBytes := 4 - ((wp + 1) % 4) + 1
	//padBytes := make([]byte, numPadBytes)

	message.Write(msg.Mesh, //it's a mesh (byte)
		sm.id, //mesh id (uint16)
		sm.materialName,
		maxInstances,  //max instances (uint32)
		maxBillBoards, //if >0 billboard and adssign this many instances

		uint32(sm.Verts), //number of vertices (vertex write pointer)
		uint32(sm.Tris),  //number of faces (derived from write pointer
	)
	message.Align()
	//BYTE pointers
	vbp := uint32(sm.Verts) * 3 //it is VITAL to cast VWP before multiplying (or it silently wraps within the uint16)!!!!
	vbp2 := uint32(sm.Verts) * 2
	message.Write(
		sm.p[0:vbp],   //positions
		sm.n[0:vbp],   //vertex normals (slice of Float32, 3 per vert)
		sm.uv[0:vbp2], //uv coordinates (slice of Float32, 2 per vert)
	)

	if len(sm.fis) > int(sm.Tris)*3 {
		log.Logit("Warning: mesh", sm.id, " has excess face space - truncating ", len(sm.fis), " to ", int(sm.Tris)*3)
		sm.fis = sm.fis[0 : (int(sm.Tris))*3]
	}
	message.Write(sm.fis) //faces (slice of Uint16, 3 per face)

}

func (sm *SimpleMesh) SetVertsFromVec3(verts []vec.V3) {
	for i, v := range verts {
		wp3 := i * 3
		sm.p[wp3] = float32(v.X)
		sm.p[wp3+1] = float32(v.Y)
		sm.p[wp3+2] = float32(v.Z)

		n := v.Normalised()
		sm.n[wp3] = float32(n.X)
		sm.n[wp3+1] = float32(n.Y)
		sm.n[wp3+2] = float32(n.Z)
	}
	sm.Verts = uint16(len(verts))
}

//Finish truncates the arrays to their used size
func (sm *SimpleMesh) Finish() {

	sm.fis = sm.fis[0 : int(sm.Tris)*3]
	sm.p = sm.p[0 : int(sm.Verts)*3]
	sm.n = sm.n[0 : int(sm.Verts)*3]
	sm.uv = sm.uv[0 : int(sm.Verts)*2]

	if sm.Tris > 65000 || sm.Verts > 65000 {
		panic("Huge simple mesh")
	}

	log.Logit("Finished simple mesh", sm.id, sm.materialName, " with ", sm.Verts, " verts and ", sm.Tris, " faces")

}

// func (sm *SimpleMesh) ToMsg(maxInstances uint16) *msg.Msg {

// 	msg := msg.NewMsg(msg.Mesh, sm.id,

// 	return msg

// }
