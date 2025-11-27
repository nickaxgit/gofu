package game

import (
	//"math/rand/v2"

	"github.com/nickax/gofu/game/msg"

	"github.com/nickax/gofu/mesh"
	"github.com/nickax/gofu/ray"
	"github.com/nickax/gofu/terrain"
	"github.com/nickax/gofu/vec"
)

func (game *Game) GetTreesFor(landMesh *terrain.TriMesh, camPos *vec.V3, camDir *vec.V3, message *msg.Msg) {

	//TODO only reposition/resend trees in new positions (most trees do not need resending)
	//need to do trees after waterlines so we don't get trees underwater

	treePositions := make([]float32, 100000) // a slice of zero length, and a CAPACITY of 100000 ///3000 xyz floats = 1000 trees

	hidden := 0

	treeBillboards := mesh.New(105, "tree", 4000, 1000)
	ray := ray.New(camPos, vec.NoWhereSpecial) //set up *one* ray for firing at the treetops (reuse it!)

	landMesh.FetchTrees(landMesh.Root, 10, treePositions, treeBillboards, camPos, camDir, ray, &hidden)

	//near trees (mesh intances)
	message.Write(msg.PositionInstances,
		uint16(len(treePositions)/3), //number on instances (near trees)
		treePositions,                //Slice of XYZ float32's
	)

	//far trees (billboards)
	treeBillboards.WriteTo(message, 1)

}
