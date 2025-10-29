package terrain

import (
	"math/rand/v2"

	"github.com/nickax/gofu/cam"
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/mesh"
	"github.com/nickax/gofu/vec"
)

func (fm *TriMesh) Ignite(firePos *vec.V3) {
	//find the triangle containing this position, and ignite it

	result := fm.Root.splitUntil(fm, firePos, 10)
	if result != nil { // are we on the map ?
		if len(result.children) > 0 {
			log.Logit("Igniting a non-leaf triangle at depth", result.depth)
		}
		if result.fireInfo == nil {
			result.fireInfo = &fireInfo{}
		}
		if result.fireInfo.flames == 0 {
			result.fireInfo.flames = 1
		}

	}
}

func (fm *TriMesh) Burn(ft *Tri) int {

	fi := ft.fireInfo
	if fi.flames > 0 {
		fi.flames++
		if fi.flames > int(fi.sparkAt) && fi.flames < int(fi.sparkAt)+10 {
			fm.Ignite(ft.centre.Add(vec.NewVec3((rand.Float64()-0.5)*50, 0, (rand.Float64()-0.5)*50)))
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
func (tri *Tri) GetFlames(lt *Tri, fm *TriMesh, intoMesh *mesh.SimpleMesh, cam *cam.Camera, tcs *mesh.Tcs) int {

	if tri.fireInfo.flames > 0 { //fTri.allBLTsAlight() { //fTri.flames > 0 {

		if tri.fireInfo.landDepth < 10 { //sampling at level 12 is 'good enough' for flame base
			p, t := lt.VprobeLand(tri.centre) //TODO - do once and cache - also normal (for slope)
			if t.depth < 8 {
				return 0
			} //it's either very far away, or behind the camera

			if t.depth > tri.fireInfo.landDepth {
				tri.fireInfo.y = p.Y //we have a better observation (of the land height) - update the flame base height
				tri.normal = t.normal
			}
			if t.IsUnderwater(0) {
				tri.fireInfo.flames = -1 //extinguish the flame
				return 0
			} //flames under water do not burn

		}

		//careful not to mutate the ftri centre
		base := tri.centre.Clone()
		base.Y = tri.fireInfo.y

		intoMesh.Billboard(base, vec.Up, cam.Position, 4, 0, 8, 3, tcs) //triangular flame
		return 1

	} else {
		flames := 0
		for _, c := range tri.children {
			flames += c.GetFlames(lt, fm, intoMesh, cam, tcs)
		}
		return flames
	}

}

func (tri *Tri) contains2D(firePos *vec.V3) bool {
	//checks whether the x/z of firePos is inside this triangle
	if tri.flat == nil {
		tri.flat = newFlatTrianglePoly(tri)
	}
	if firePos.GetY() != 0 {
		panic("contains2D called with non zero y")
	}
	return tri.flat.Contains(firePos)
}

func (fm *TriMesh) scorchedAt(p *vec.V3) bool {

	leaf := fm.Root.find(p) //once leafs have burned out - they can be retracted into a single scorched parent

	if leaf == nil {
		return false //no leaf node here
	}
	fi := leaf.fireInfo
	if fi.flames == -1 || fi.flames > 10 {
		return true
	}

	return false
}

func (tri *Tri) splitUntil(fm *TriMesh, firePos *vec.V3, maxDepth int) *Tri {
	//recursively split triangles until the firePos is contained in a triangle at maxDepth

	if tri.contains2D(firePos) {
		if tri.depth == maxDepth {
			return tri
		}

		//split only if necessary
		if len(tri.children) == 0 {
			tri.split()
		}

		miss := 0
		for _, c := range tri.children {
			res := c.splitUntil(fm, firePos, maxDepth)
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
