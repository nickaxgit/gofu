package game

//lighteright game state - the objects do not have methods (as they are deserialised from server data)
import (
	"bufio"
	"bytes"
	"encoding/binary"
	"github.com/nickax/gofu/errorplus"
	"github.com/nickax/gofu/fiz/mass"
	"github.com/nickax/gofu/fiz/thing"
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/game/sound"
	"github.com/nickax/gofu/log"
	//"github.com/nickax/gofu/persist"
	"github.com/nickax/gofu/terrain"
	"github.com/nickax/gofu/vec"
	"io"
	"math"
	"os"
	"sync"
)

//var rnGen *rand.Rand //nd.NewPCG(42, uint64(time.Microsecond)))

type Game struct { //the DATA of a game in progress - it can be entirely replaced at any point by rejoining a game
	Id   uint32
	Name string

	Masses []*mass.Mass
	Things []*thing.Thing
	Sounds []*sound.Sound

	Running     bool
	RunwayStart vec.V3
	RunwayEnd   vec.V3
	runwayWidth float64
	stretchDir  bool
	ZeroG       bool

	Fire       *terrain.TriMesh
	LandSize   float64   //size of land square
	LandHeight float64   //max height of land
	Kinks      []float64 //land bends
	Land       *terrain.TriMesh
}

var mutex = sync.RWMutex{}

func New(id uint32, name string) *Game {
	//create a new game

	landSize := 10000.0
	landHeight := 400.0
	kinks := []float64{1.0, 0.8, 1.0, 0.5, 0.25, 0.125, 1.0 / 16, 1.0 / 32, 1.0 / 64, 1.0 / 128, 1.0 / 256, 1.0 / 512, 1.0 / 1024, 1.0 / 2048, 1.0 / 4096, 1.0 / 8192} //, 1.0 / 16384} //, 1.0 / 32768, 1.0 / 65536} //how much to pull down the midpoint at each level of recursion

	game := &Game{
		Id:   id,
		Name: name,
		//Players:    []*player.Player{},
		// currentMutex: &sync.Mutex{},
		// CurrentPlayers: make(map[*player]bool),
		// CurrentViewers:  make(map[*viewer]bool),
		Sounds:     []*sound.Sound{},
		Running:    false,
		Masses:     []*mass.Mass{},
		Things:     []*thing.Thing{},
		LandSize:   landSize,
		LandHeight: landHeight,
		Kinks:      kinks,
	}

	game.BuildFireMesh()

	return game
}

var games map[uint32]*Game = map[uint32]*Game{} // DONT put the sentinels in the map 0: None} //all games by id

func Get(id uint32) *Game {
	mutex.RLock()
	g, ok := games[id]
	mutex.RUnlock()
	if !ok {
		return None //game.None
	}
	return g
}

func Set(g *Game) {
	mutex.Lock()
	games[g.Id] = g
	mutex.Unlock()
}

func AllRunning() []*Game {
	mutex.RLock()
	defer mutex.RUnlock()
	result := make([]*Game, 0)
	for _, g := range games {
		//if g.Running {
		result = append(result, g)
		//}
	}
	return result
}

func (game *Game) GetFire() *terrain.TriMesh {
	return game.Fire
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
	game.Fire.Burn(game.Fire.Root)
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
	writer.Write(thing.ThingsAsMsg(game.Things, true).AllBytes())                //write all things (springs, meshnames, offsets, scales, rotations)

	writer.Flush()

	//write the bytes slice to a file

}
func (game *Game) landSizeFromByteBuffer(buff *bytes.Buffer) {
	le := binary.LittleEndian

	binary.Read(buff, le, &game.LandSize)
	binary.Read(buff, le, &game.LandHeight)
	kinkCount := byte(0)
	binary.Read(buff, le, &kinkCount)
	game.Kinks = make([]float64, kinkCount)
	binary.Read(buff, le, &game.Kinks)

}

func landToBytes(s *Game) []byte {

	buff := new(bytes.Buffer)
	le := binary.LittleEndian
	binary.Write(buff, le, s.LandSize)
	binary.Write(buff, le, s.LandHeight)
	kinkCount := byte(len(s.Kinks))
	binary.Write(buff, le, kinkCount)
	binary.Write(buff, le, s.Kinks)

	return buff.Bytes()
}

func Load(filename string, owner uint32) *Game {

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

	state := New(1, filename)
	m := msg.NewFromBytes(allBytes) //beware sets message type from first byte

	state.Masses = mass.MassesFromMsg(m, owner)
	state.Things = thing.ThingsFromMsg(m, state.Masses)

	//fix up wing areas on loading
	for _, m := range state.Masses {
		m.CalcWingArea()
	}

	log.Logit("Loaded state from " + filename)

	return state

}

func (game *Game) SetRunway(start vec.V3, vector vec.V3, width float64) {
	game.RunwayStart = start
	game.RunwayEnd = start.Add(vector)
	game.runwayWidth = width
}

func (game *Game) BuildFireMesh() *terrain.TriMesh {
	game.Fire = terrain.NewTriMesh("fire", 20000, game.LandSize, game.Kinks, game.LandHeight)
	return game.Fire
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

func (game *Game) closestMass(wp vec.V3) *mass.Mass {

	//let closestDistance=within
	for _, m := range game.Masses {
		if m.Contains(wp) {
			return m
		}
	}
	return nil
}

func (game *Game) resolvePenetrations() {

	for _, m := range game.Masses {
		if m.Collideable {
			for _, t := range game.Things {
				if t.Index != m.ThingIndex { //don't collide masses against the things they belong to
					t.PushAway(m)
				}
			}

			hit, where, tri := game.Land.Root.VprobeLand(m.P, game.Land, m.Owner)

			if hit {
				penDepth := where.Y - (m.P.Y - m.R)

				if penDepth > 0 {
					m.ResolvePenetration(penDepth, where, tri)
				}
			}
		}
	}

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

func (g *Game) Ignite(position vec.V3) {
	g.Fire.Ignite(position)
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
func (game *Game) MoveAll(substeps int) *msg.Msg {
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

		//it's very important we only write the terminator once!
		if substep == 0 {
			activity.Write(uint16(65535))
		} //terminator (index)}

		game.RunEngines(activity) //places thrust on some springs
		game.FlyMasses()

		game.stretchSprings()
		game.stretchSprings()
		game.stretchSprings()

		game.resolvePenetrations()

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

func (g *Game) Persist() *errorplus.Event {

	//gameMsg := msg.NewMsg(msg.P_Game)
	//g.WriteTo(gameMsg)
	return nil //persist.Append("repo.bin", gameMsg)
}

func (game *Game) PlaceTrees() int {

	// 	occludedTrees := 0
	// 	wp := 0
	// 	dev.Land.Root.Flatten(6, dev.treeTris, &wp) //get all triangles at depth 10  -*potential* tree sites

	// 	ray := ray.New(dev.Camera.Position, vec.NoWhereSpecial) //set up *one* ray for firing at the treetops (reuse it!)
	// 	treeTop := vec.NewVec3(0, 0, 0)                         //scratch
	// 	toTree := vec.NewVec3(0, 0, 0)

	// 	stats := terrain.NewProbeStats()

	// 	dev.nearTreeCount = 0 //track the number of positions we will need to send
	// 	dev.farTreeCount = 0
	// 	//dev.farTreeBillBoardMesh.Reset()

	// 	for i := 0; i < wp; i++ { //_, tri := range tris {
	// 		tri := dev.treeTris[i]
	// 		if tri.IsSubmerged(dev.Land) {
	// 			continue
	// 		} //no trees underwater
	// 		//mid := tri.Centre
	// 		points := tri.MeshPoints(dev.Land, 6) //divide the level 6 triangles 6 further times (yeilding 28 points each)
	// 		//tcs := mesh.NewTcs(0, 1, 1, 0)

	// 		for _, plot := range points {

	// 			if !dev.Land.ScorchedAt(dev.Land.Root, plot) {

	// 				d := plot.DistanceFrom(dev.Camera.Position)
	// 				if d > 5000 {
	// 					continue
	// 				}
	// 				surfacePoint, _ := dev.Land.Root.VprobeLand(plot)
	// 				if surfacePoint == nil {
	// 					continue
	// 				} //should not happen (but does)

	// 				toTree := surfacePoint.Sub(dev.Camera.Position)
	// 				toTree.NormaliseInPlace()
	// 				dotProd := toTree.Dot(dev.Camera.Direction)

	// 				if d < 500 {
	// 					if dotProd > -0.2 { //trees in front of, or somewhat behind the camera

	// 						treeTop.SetFrom(surfacePoint)
	// 						treeTop.Y += 3

	// 						ray.PointAt(treeTop)

	// 						//check for occlusion (by the triangle it stands on)
	// 						//TODO - check against whole landscape (although these are nearby trees)

	// 						stats.Hit = false
	// 						dev.Land.Root.Probe(dev.Land, ray, stats, 0)
	// 						if stats.Hit {
	// 							occludedTrees++
	// 							continue
	// 						}

	// 						if dev.nearTreeCount < maxNearTrees {
	// 							//place a (instanced mesh) tree here
	// 							wp := dev.nearTreeCount * 3
	// 							dev.nearTreePositions[wp] = float32(surfacePoint.X)
	// 							dev.nearTreePositions[wp+1] = float32(surfacePoint.Y)
	// 							dev.nearTreePositions[wp+2] = float32(surfacePoint.Z)
	// 							dev.nearTreeCount++
	// 						} else {
	// 							log.Logit("Max near trees reached")
	// 						}

	// 					}
	// 				} else { //it's a faraway tree - only place it if the ground slopes towards the camera
	// 					//todo - occlusion cull far trees too
	// 					if dotProd > .35 { //trees generally in front of the camera}
	// 						if toTree.Dot(&tri.Normal) < 0 { //if the triangle slopes towards camera

	// 							stats.Hit = false
	// 							treeTop.SetFrom(surfacePoint)
	// 							treeTop.Y += 6
	// 							ray.PointAt(treeTop)

	// 							dev.Game.Land.Root.Probe(dev.Land, ray, stats, 0)
	// 							if stats.Hit {
	// 								occludedTrees++
	// 								continue
	// 							}

	// 							if dev.farTreeCount < maxFarTrees {
	// 								wp := dev.farTreeCount * 3
	// 								dev.farTreePositions[wp] = float32(surfacePoint.X)
	// 								dev.farTreePositions[wp+1] = float32(surfacePoint.Y)
	// 								dev.farTreePositions[wp+2] = float32(surfacePoint.Z)

	// 								dev.farTreeCount++

	// 							} else {
	// 								log.Logit("Max far trees reached")
	// 							}
	// 						}
	// 					}
	// 				}
	// 			}
	// 		}
	// 	}

	// 	return occludedTrees

	return 0
}
