package game

import (
	//"math/rand/v2"
	"strconv"
	"time"

	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/mesh"
	"github.com/nickax/gofu/ray"
	"github.com/nickax/gofu/terrain"
	"github.com/nickax/gofu/vec"
)

func (game *Game) GetTreesFor(root *terrain.Tri, camPos *vec.V3, camDir *vec.V3, message *msg.Msg) {

	//TODO only reposition/resend trees in new positions (most trees do not need resending)
	//need to do trees after waterlines so we don't get trees underwater

	treePositions := make([]float32, 100000) // a slice of zero length, and a CAPACITY of 100000 ///3000 xyz floats = 1000 trees

	hidden := 0

	treeBillboards := mesh.New(105, "tree", 4000, 1000)
	ray := ray.New(camPos, vec.NoWhereSpecial) //set up *one* ray for firing at the treetops (reuse it!)

	root.FetchTrees(10, treePositions, treeBillboards, camPos, camDir, ray, &hidden)

	//near trees (mesh intances)
	message.Write(msg.PositionInstances,
		uint16(len(treePositions)/3), //number on instances (near trees)
		treePositions,                //Slice of XYZ float32's
	)

	//far trees (billboards)
	treeBillboards.WriteTo(message, 1)

}

// Makeland - generates lands,scorched land, water and trees for the given camera position and direction (into Message)
func (game *Game) MakeLand(camPos *vec.V3, camDir *vec.V3, response *msg.Msg) *terrain.TriMesh {

	land := terrain.NewTriMesh("land", 65000, game.landSize, game.kinks, game.landHeight)
	ts := time.Now()
	//up := vec.Up

	//note - runway is projected onto y=0 up to here

	rs := game.runwayStart
	re := game.runwayEnd
	rs.Y = 0
	re.Y = 0

	log.Logit(land.VertCount(), " verts")

	ts = time.Now()
	seed := uint64(0) //uint64(time.Now().Nanosecond())
	log.Logit("seed:" + strconv.FormatUint(seed, 10))

	//rnGen := rand.New(rand.NewPCG(seed+1, seed))
	//size := player.state.landSize
	//(rnGen.Float64()-.5)*maxHeight

	land.Root.SplitIfNeeded(camPos, camDir, 0.4) //split the triangle into 4 recursively
	log.Logit("splitting to focus took", time.Since(ts).Milliseconds(), "ms")

	ts = time.Now()
	land.Root.Patch()
	log.Logit("patch took", time.Since(ts).Milliseconds(), "ms")

	ts = time.Now()
	waterlines := []float64{game.landHeight * 0.71, game.landHeight * 0.41, 0.1, -game.landHeight * 0.52}

	// //makes the water surface mesh messages - one for each waterline, into the message
	land.FloodAndDrain(waterlines, response)
	log.Logit("flood and drain took", time.Since(ts).Milliseconds(), "ms")
	//land.Flood(-10000)

	ts = time.Now()
	land.Root.CalcVerticalExtents() //we need the y extents for occlusion culling
	log.Logit("calced y extents took", time.Since(ts).Milliseconds(), "ms")

	ts = time.Now()
	land.Root.MakePrisms() //we want to construct volumes once for each triangle - not repeatedly during occlusion culling
	log.Logit("made prisms took", time.Since(ts).Milliseconds(), "ms")

	ts = time.Now()
	land.Root.Occlude(camPos)
	log.Logit("occlude took", time.Since(ts).Milliseconds(), "ms")

	// culled, kept := 0, 0
	// ts = time.Now()
	// land.Root.OccludeVerts(camPos) //TODO- only occlude verts in view frustum
	// log.Logit("occlude verts took", time.Since(ts).Milliseconds(), "ms")

	// ts = time.Now()
	// land.Root.OcclusionCull(&culled, &kept)
	// log.Logit("occlusion cull took", time.Since(ts).Milliseconds(), "ms")

	//	log.Logit("culled:", culled, " kept:", kept, " tris")

	//patch convert and send

	//groundPosition, groundTriangle := land.Root.VprobeLand(camPos)

	// game.runwayStart, _ = land.Root.VprobeLand(game.runwayStart)
	// game.runwayEnd.Y = (game.runwayStart.Y) //keep the runway level with the start point

	// land.Root.Plough(game.runwayStart, game.runwayEnd, game.runwayWidth) //recurse down through and plough a runway

	// //probe the land at the four corners of the runway and add two triangles
	// cross := game.runwayStart.Sub(game.runwayEnd).Normalise().Cross(vec.Up).Multiply(game.runwayWidth / 2)
	// bl := game.runwayStart.Sub(cross)
	// br := game.runwayStart.Add(cross)
	// tl := game.runwayEnd.Sub(cross)
	// tr := game.runwayEnd.Add(cross)
	// //var n *vec3
	// bl, _ = land.Root.VprobeLand(bl) //find the ground surface
	// tl, _ = land.Root.VprobeLand(tl) //find the ground surface
	// br, _ = land.Root.VprobeLand(br) //find the ground surface
	// tr, _ = land.Root.VprobeLand(tr) //find the ground surface

	// bl.Y += 0.05
	// tl.Y += 0.05
	// br.Y += 0.05
	// tr.Y += 0.05

	// runwayMesh := mesh.New(3, "runway", 4, 2) //2 faces

	// bli := runwayMesh.AddVert(bl, vec.Up, 0, 1)
	// bri := runwayMesh.AddVert(br, vec.Up, 1, 1)
	// tli := runwayMesh.AddVert(tl, vec.Up, 0, 0)
	// trix := runwayMesh.AddVert(tr, vec.Up, 1, 0)

	// runwayMesh.AddFace(tli, bri, bli)
	// runwayMesh.AddFace(tli, trix, bri)

	log.Logit("splitting took", time.Since(ts).Milliseconds())
	//}

	//runwayMesh.WriteTo(message, 1)

	//if t.Culled || t.Scorched || t.OnOrUnderWater() {
	isLand := func(t *terrain.Tri) bool {

		if t.Culled || t.Scorched || t.IsSubmerged() {
			return false
		}

		return true
	}

	ts = time.Now()
	//TODO REINSTATE (But is causes poly contains checks to screw up)
	//land.Root.Scorch(game.fire) //update the scorched state of non culled leaf triangles
	log.Logit("scorching took", time.Since(ts).Milliseconds(), "ms")

	smallLandMesh := land.Root.ToSimpleMesh(2, land, game.fire, "land", false, isLand)
	log.Logit("converted land mesh in", time.Since(ts).Milliseconds(), "ms")
	log.Logit("small land mesh has", smallLandMesh.FaceCount(), "faces ", smallLandMesh.VertCount(), " verts")

	///scorchedLand := land.Root.ToSimpleMesh(56, land, game.fire, "scorched", false, func(t *terrain.Tri) bool { return t.Scorched })
	wireframe := land.Root.ToSimpleMesh(32, land, game.fire, "whiteWires", false, isLand)

	smallLandMesh.WriteTo(response, 1)
	///scorchedLand.WriteTo(message, 1)
	wireframe.WriteTo(response, 1)

	return land
}
