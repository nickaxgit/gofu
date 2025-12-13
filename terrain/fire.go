package terrain

import (
	"math/rand/v2"

	"github.com/nickax/gofu/cam"
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/mesh"
	"github.com/nickax/gofu/vec"
)

func (fm *TriMesh) Ignite(firePos vec.V3) {
	//find the triangle containing this position, and ignite it

	result := fm.Root.splitUntil(fm, firePos, 10)
	if result != nil { // are we on the map ?
		if result.childCount > 0 {
			log.Logit("Igniting a non-leaf triangle at depth", result.Depth)
		}
		if result.FireInfo == nil {
			result.FireInfo = &fireInfo{}
		}
		if result.FireInfo.flames == 0 {
			result.FireInfo.flames = 1
		}

	}
}

func (fm *TriMesh) Burn(ft *Tri) int {

	fi := ft.FireInfo

	if fi == nil {
		//log.Logit("no fire")
		return 0
	}

	if fi.flames > 0 {
		fi.flames++
		if fi.flames > int(fi.sparkAt) && fi.flames < int(fi.sparkAt)+10 {
			fm.Ignite(ft.Centre.Add(vec.NewVec3((rand.Float64()-0.5)*50, 0, (rand.Float64()-0.5)*50)))
		}
		if fi.flames == 300 {
			fi.flames = -1 //burnt out
		}
		return fi.flames
	}
	tflames := 0
	for _, c := range ft.children {
		tflames += fm.Burn(c)
	}
	return tflames
}

// recurse from fTri to find all triangles with flames, insert them (as billboards) into the simpleMesh
func (tri *Tri) GetFlames(deviceId uint32, lm *TriMesh, fm *TriMesh, intoMesh *mesh.SimpleMesh, cam *cam.Camera, tcs mesh.Tcs) int {

	if lm != nil && tri.FireInfo != nil && tri.FireInfo.flames > 0 { //fTri.allBLTsAlight() { //fTri.flames > 0 {

		if tri.FireInfo.landDepth < 10 { //sampling at level 10 is 'good enough' for flame base
			hit, p, t := lm.Root.VprobeLand(tri.Centre, lm, deviceId) //TODO - do once and cache - also normal (for slope)
			if hit {
				if t.Depth < 8 {
					return 0
				} //it's either very far away, or behind the camera

				if t.Depth > tri.FireInfo.landDepth {
					tri.FireInfo.y = p.Y //we have a better observation (of the land height) - update the flame base height
					tri.Normal = t.Normal

				}

				//TODO REINSSTAE
				// if t.OnOrUnderWater(lm) {
				// 	tri.FireInfo.flames = -1 //extinguish the flame
				// 	return 0
				// } //flames under water do not burn
			}

		}

		//careful not to mutate the ftri centre
		base := tri.Centre.Clone()
		base.Y = tri.FireInfo.y

		intoMesh.Billboard(base, vec.Up, cam.Position, 4, 0, 8, 3, tcs) //triangular flame
		return 1

	} else {
		flames := 0
		for _, c := range tri.children {
			flames += c.GetFlames(deviceId, lm, fm, intoMesh, cam, tcs)
		}
		return flames
	}

}

// func (tri *Tri) contains2D(firePos *vec.V3) bool {
// 	//checks whether the x/z of firePos is inside this triangle
// 	if tri.flat == nil {
// 		tri.flat = newFlatTrianglePoly(tri)
// 	}
// 	if firePos.GetY() != 0 {
// 		panic("contains2D called with non zero y")
// 	}
// 	return tri.flat.Contains(firePos)
// }

// func (fm *TriMesh) scorchedAt(p *vec.V3) bool {

// 	leaf := fm.Root.find2D(fm,p) //once leafs have burned out - they can be retracted into a single scorched parent

// 	if leaf == nil {
// 		return false //no leaf node here
// 	}

// 	//TODO - why do we need this ?
// 	if leaf.FireInfo == nil {
// 		return false
// 	}

// 	fi := leaf.FireInfo
// 	if fi.flames == -1 || fi.flames > 10 {
// 		return true
// 	}

// 	return false
// }

// recursively split triangles until the firePos is contained in a triangle at maxDepth
func (tri *Tri) splitUntil(fm *TriMesh, firePos vec.V3, maxDepth int) *Tri {

	if tri.contains2D(firePos, fm) {
		if tri.Depth == maxDepth {
			return tri
		}

		//split only if necessary
		if tri.childCount == 0 {
			tri.split(fm)
		}

		miss := 0
		//for _, c := range t.children {
		for i := 0; i < tri.childCount; i++ {
			res := tri.children[i].splitUntil(fm, firePos, maxDepth)
			if res != nil {
				return res
			}
			miss++
		}
		if miss == 4 {
			log.Logit("missed all children")
		}

	}
	return nil

}
