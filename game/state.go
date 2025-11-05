package game

//lighteright game state - the objects do not have methods (as they are deserialised from server data)
import (
	"bufio"
	//	"sync"

	//"github.com/gorilla/websocket"

	//	"go.mongodb.org/mongo-driver/bson" //once stuctures are stabilised - can probaly just use bufio direclty
	"bytes"
	"encoding/binary"

	"github.com/nickax/gofu/fiz/mass"
	"github.com/nickax/gofu/fiz/thing"
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/mutex"

	//game does not know about devices or players (devices and players know about the game)
	//"github.com/nickax/gofu/game/player"
	//"github.com/nickax/gofu/viewer"

	"github.com/nickax/gofu/game/sound"
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/terrain"
	"github.com/nickax/gofu/vec"

	"io"
	"math"
	"math/rand/v2"
	"os"
)

//var rnGen *rand.Rand //nd.NewPCG(42, uint64(time.Microsecond)))

type Game struct { //the DATA of a game in progress - it can be entirely replaced at any point by rejoining a game
	Id uint32

	Masses []*mass.Mass
	Things []*thing.Thing
	Sounds []*sound.Sound

	Running     bool
	runwayStart *vec.V3
	runwayEnd   *vec.V3
	runwayWidth float64
	stretchDir  bool
	ZeroG       bool

	fire       *terrain.TriMesh
	landSize   float64   //size of land square
	landHeight float64   //max height of land
	kinks      []float64 //land bends
}

func New(games map[uint32]*Game) *Game {
	//create a new game
	var gameId uint32

	//make a random 4 digit game id
	for gameId < 1000 {
		gameId = uint32(rand.Float32() * 9999)
	}

	landSize := 10000.0
	landHeight := 600.0
	kinks := []float64{1.0, 0.8, 1.0, 0.5, 0.25, 0.125, 1.0 / 16, 1.0 / 32, 1.0 / 64, 1.0 / 128, 1.0 / 256, 1.0 / 512, 1.0 / 1024, 1.0 / 2048, 1.0 / 4096, 1.0 / 8192} //, 1.0 / 16384} //, 1.0 / 32768, 1.0 / 65536} //how much to pull down the midpoint at each level of recursion

	game := &Game{
		Id: gameId,
		//Players:    []*player.Player{},
		// currentMutex: &sync.Mutex{},
		// CurrentPlayers: make(map[*player]bool),
		// CurrentViewers:  make(map[*viewer]bool),
		Sounds:     []*sound.Sound{},
		Running:    false,
		Masses:     []*mass.Mass{},
		Things:     []*thing.Thing{},
		landSize:   landSize,
		landHeight: landHeight,
		kinks:      kinks,
	}

	game.BuildFireMesh()

	if games != nil { //when we merge assets, we don't want to create a new game
		mutex.Games.Lock()
		games[gameId] = game
		mutex.Games.Unlock()
	}
	return game
}

func (game *Game) GetFire() *terrain.TriMesh {
	return game.fire
}

func (game *Game) MergeThing(t *thing.Thing, tm []*mass.Mass) *thing.Thing {

	game.Things = append(game.Things, t)
	t.Index = uint32(len(game.Things) - 1)

	ml := len(game.Masses)
	for i, s := range t.Springs {
		game.Masses[ml+i*2] = tm[s.M1.Index]
		game.Masses[ml+i*2+1] = tm[s.M2.Index]
	}

	return t

}

func (g *Game) DeleteLastMass() {
	if len(g.Masses) == 0 {
		return
	}
	g.Masses = g.Masses[:len(g.Masses)-1] //delete the last mass
}

func (game *Game) Burn() {
	game.fire.Burn(game.fire.Root)
}

func (game *Game) Save(filename string, selectedMasses map[*mass.Mass]bool) {

	file, err := os.Create(filename + ".bin")
	if err != nil {
		log.Logit(err.Error() + " save failed")
		return
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	//writer.Write(landToBytes(s))                                 //write land size and kinks
	writer.Write(mass.MassesAsMsg(game.Masses, true, selectedMasses).AllBytes()) //write all masses, with detail
	writer.Write(thing.ThingsAsMsg(game.Things).AllBytes())                      //write all things (springs, meshnames, offsets, scales, rotations)

	writer.Flush()

	//write the bytes slice to a file

}
func (game *Game) landSizeFromByteBuffer(buff *bytes.Buffer) {
	le := binary.LittleEndian

	binary.Read(buff, le, &game.landSize)
	binary.Read(buff, le, &game.landHeight)
	kinkCount := byte(0)
	binary.Read(buff, le, &kinkCount)
	game.kinks = make([]float64, kinkCount)
	binary.Read(buff, le, &game.kinks)

}

func landToBytes(s *Game) []byte {

	buff := new(bytes.Buffer)
	le := binary.LittleEndian
	binary.Write(buff, le, s.landSize)
	binary.Write(buff, le, s.landHeight)
	kinkCount := byte(len(s.kinks))
	binary.Write(buff, le, kinkCount)
	binary.Write(buff, le, s.kinks)

	return buff.Bytes()
}

// func (game *State) SeceneStart(player *player.Player, viewer *viewer.Viewer) []*msg.Msg {
// 	viewer.Send(plant.GrowTree().ToMsg(200)) //prep for 200 instance meshed trees (there will be many more billboarded)
// 	viewer.Send(player.Grid.AsMsg())
// 	viewer.SendCamera() //sends *their* camera to them
// 	viewer.Send(player.Grid.AsMsg())

// 	viewer.SendMasses(game.Masses, true, player.selectedMasses)
// 	viewer.Send(thing.ThingsAsMsg(game.Things)) //[]*thing.Thing{p.vehicle}) //sends mesh name and springs

// 	viewer.SendLabelSets()

// 	viewer.SendVectors(game)
// 	viewer.SendCentreOfMass(viewer.currentThing)

// 	viewer.Notify("Loaded", "info")
// 	viewer.SendGameId(gameId) //game id starts it running
// }

func Load(games map[uint32]*Game, filename string) *Game {

	file, err := os.Open(filename + ".bin")
	if err != nil {
		log.Logit(err.Error() + " load failed")
		return &Game{}
	}
	defer file.Close()

	reader := io.Reader(file)
	allBytes, err := io.ReadAll(reader)
	if err != nil {
		log.Logit(err.Error())
	}

	m := msg.NewFromBytes(allBytes)

	masses := mass.MassesFromMsg(m)
	things := thing.ThingsFromMsg(m, masses)
	//players := player.PlayersFromBuff(buff, filename, things)

	state := New(games)
	state.Masses = masses
	state.Things = things

	//state.landSizeFromByteBuffer(buff)

	//fix up wing areas on loading
	for _, m := range state.Masses {
		m.CalcWingArea()
	}

	log.Logit("Loaded state from " + filename)

	return state

}

func (game *Game) SetRunway(start *vec.V3, vector *vec.V3, width float64) {
	game.runwayStart = start
	game.runwayEnd = start.Add(vector)
	game.runwayWidth = width
}

func (game *Game) BuildFireMesh() *terrain.TriMesh {
	game.fire = terrain.NewTriMesh("fire", 20000, game.landSize, game.kinks, game.landHeight)
	return game.fire
}

// func (game *State) MoveCameras() {
// 	for _, p := range game.Players {
// 		p.MoveCamera(game.Masses)
// 	}

// }

// func (game *State) closestSpring(wp *vec.V3) (*spring.Spring, *thing.Thing) {

// 	var closestSpring *spring.Spring
// 	var closestThing *thing.Thing

// 	closestDistance := math.MaxFloat64

// 	for _, thing := range game.Things {
// 		thing.CloserSpringToPointThan(wp, &closestDistance, &closestThing, &closestSpring)

// 	}
// 	return closestSpring, closestThing

// }

func (game *Game) closestMass(wp *vec.V3) *mass.Mass {

	//let closestDistance=within
	for _, m := range game.Masses {
		if m.Contains(wp) {
			return m
		}
	}
	return nil
}

func (game *Game) resolvePenetrations(lands []*terrain.Tri) {

	for _, m := range game.Masses {
		if m.Collideable {
			for _, t := range game.Things {
				if t.Index != m.ThingIndex { //don't collide masses against the things they belong to
					t.PushAway(m)
				}
			}

			impact, tri := game.probeMostDetailedLandAt(m.P, lands)

			if impact != nil {
				pen := impact.Y - (m.P.Y - m.R)

				if pen > 0 {
					m.ResolvePenetration(pen, impact, tri)
				}
			}
		}
	}

}

func (game *Game) probeMostDetailedLandAt(p *vec.V3, lands []*terrain.Tri) (impact *vec.V3, tri *terrain.Tri) {

	deepest := 0
	var bestTri *terrain.Tri = nil

	for _, rt := range lands {
		poi, t := rt.VprobeLand(p)
		if t.Depth > deepest {
			deepest = t.Depth
			bestTri = t
			impact = poi
		}
	}
	return impact, bestTri
}

// runEngines use the fuel burn (and KW) to accelerate the prop disc/engineRPM AND move the engine spring/masses
func (game *Game) RunEngines(activity *msg.Msg) {

	//there's a lot to unpack here, runengines is called on each player, accelerating engines
	// and returning a set of zero or more sound messages (pitch changes) for all the engines or all the players
	// any sounds are send to all players in the game

	for _, t := range game.Things {
		for _, engine := range t.Engines {
			if engine.IsStarted() {
				engine.Run(activity) //Moves the masses (of the engines spring)
			}
		}
	}

}

func (g *Game) Ignite(position *vec.V3) {
	g.fire.Ignite(position)
}

// pass vms as 0 to use actual mass velocities
func (game *Game) FlyMasses() {

	//	gravity := 9.81 * (1 / 30.0 * 1 / 30.0) //DONT half this

	for _, m := range game.Masses {
		m.Fly(game.Running)
	}

}
func (game *Game) stretchSprings() {

	for _, t := range game.Things {
		t.StretchSprings()

	}

}

// executes a physics step and returns the index and new position for all the masses that move
func (game *Game) MoveAll(substeps int, lands []*terrain.Tri) *msg.Msg {

	//movedMasses := []int{} //return the index, x and y of all masses that move

	//distance an object falls in 1/30th of a second
	//0.5 * G * T^2
	//0.5 * 9.81 * 1/30^2 = 0.0054

	activity := msg.NewMsg(msg.Movement)

	gravity := 1 * 9.81 * math.Pow(1/(30*float64(substeps)), 2)
	if game.ZeroG {
		gravity = 0
	}

	for substep := 0; substep < substeps; substep++ {

		//move by inertia and friction
		for _, m := range game.Masses {

			//TODO optimise - reduce allocs
			v := m.P.Sub(m.Op)
			m.Op = m.P.Clone()
			m.P.AddIn(v.Multiply(.999)) //inertia and friction (and damping)
			m.P.Y -= gravity

			if substep == 0 {
				if m.HasMoved() {
					m.WriteInto(activity, false, 0)
				}
			}

		}

		game.RunEngines(activity) //places thrust on some springs
		game.FlyMasses()

		game.stretchSprings()
		game.stretchSprings()
		game.stretchSprings()

		game.resolvePenetrations(lands)

		//masses are pushed out of things (and things away from masses)
		game.resolveMassOverlaps()

	}

	return activity

}

func (game *Game) resolveMassOverlaps() {

	for o, a := range game.Masses {

		for i := o + 1; i < len(game.Masses); i++ {
			b := game.Masses[i]

			if a.Fixed || b.Fixed {
				continue
			} //no need to check fixed masses
			//optimise here - we dont need to do the full distance calculation
			d := a.P.DistanceFrom(b.P) //Vector.distanceBetween(a.position,b.position)
			overlap := (a.R + b.R) - d

			if overlap > 0 {
				//let v = ap.subtract(bp).normalise().multiply(0.5)
				delta := b.P.Sub(a.P)
				if delta.LengthSq() == 0 {
					log.Logit("zero length delta")
				} else {
					delta = delta.Normalise()
					delta = delta.Multiply(overlap)

					afix := .5 //b.mass/(a.mass+b.mass)
					if b.Fixed {
						afix = 1
					} //if b is fixed then a is pushed out of b
					if !a.Fixed {
						a.P.SubIn(delta.Multiply(afix))
					}
					if !b.Fixed {
						b.P.AddIn(delta.Multiply((1 - afix)))
					}

				}
			}
		}
	}
}

func lerp(x float64, data []float64) float64 {

	if x <= data[0] {
		return data[1]
	}

	if x >= data[len(data)-2] {
		return data[len(data)-1]
	}

	for i := 0; i < len(data); i += 2 {
		if data[i] > x {
			t := (x - data[i-2]) / (data[i] - data[i-2])
			return data[i-1] + t*(data[i+1]-data[i-1])
		}
	}

	panic("lerp failed")

}
