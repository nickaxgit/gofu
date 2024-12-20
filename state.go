package main

//lighteright game state - the objects do not have methods (as they are deserialised from server data)
import (
	"bufio"
	"go.mongodb.org/mongo-driver/bson" //once stuctures are stabilised - can probaly just use bufio direclty
	"io"
	"math"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
)

type skinPayload struct {
	Ti       int     `json:"ti"`
	Scale    Vector  `json:"scale"`
	Rotation float64 `json:"rotation"`
}

type soundPayload struct {
	Sound    string  `json:"sound"`
	Position Vector  `json:"position"`
	Volume   float32 `json:"volume"`
	Label    string  `json:"label"`
	Loop     bool    `json:"loop"`
}

type revsPayload struct {
	Player string  `json:"player"`
	Revs   float32 `json:"revs"`
}

type massPayload struct {
	//index: I, mass: state.masses[I]}}) //we receive a new thing sfrom someone -
	I    int  `json:"i"`
	Mass Mass `json:"mass"`
}

type springPayload struct {
	Ti     int    `json:"ti"`
	Si     int    `json:"si"`
	Spring Spring `json:"spring"`
}

type thingPayload struct {
	Ti    int   `json:"ti"`
	Thing Thing `json:"thing"`
}

//	type pos struct{
//		fl Vector
//		rl Vector
//		fr Vector
//		rr Vector
//	}
type track struct {
	Pointer int       `json:"pointer"`
	Points  []float64 `json:"points"` //x,y pairs for 4 verts per frame
}

type State struct { //the data of a game in progress - it is serialised and should have no methods - it can be entirely replaced at any point by rejoining a game
	GameId int `json:"gameId" bson:"gameId"`
	Sqn    int `json:"sqn"`
	//host      string
	Players   map[string]*Player `json:"players"`
	Masses    []*Mass            `json:"masses"`
	Things    []*Thing           `json:"things"`
	deathList []*Player
	Tracks    map[string]*track `json:"tracks"` //a stream of point quads by player name
	Layers    map[string]*Layer `json:"layers"`
}

func (s *State) AddMass(m *Mass) int {
	s.Masses = append(s.Masses, m)
	return len(s.Masses) - 1 // return the index of the new mass
}

func (state *State) step() {

	state.moveAll(3) //<- this is a physics step - move, count coins and deaths, falls etc

	mm := make([]int, 4*len(state.Masses))

	moved := int(0)
	for i, m := range state.Masses {
		if m.P.X != m.op.X || m.P.Y != m.op.Y || m.Z != m.oz {
			mm[moved*4] = i
			mm[moved*4+1] = int(m.P.X * 100)
			mm[moved*4+2] = int(m.P.Y * 100)
			mm[moved*4+3] = int(m.Z * 100)
			moved++
		}
	}

	mm = mm[:moved*4] //truncate

	if moved > 0 {
		state.Sqn++
		state.q4all(&reply{Cmd: "mps", Payload: mm})
	}

	//count money and send to player
	for _, p := range state.Players {

		if p.stepCoinsValue > 0 { //did we win any coins this step
			p.Coins += p.stepCoinsValue * p.stepCoinCount
			if p.stepCoinCount > 1 {
				state.q4one(p, &reply{Cmd: "banner", Payload: "X" + strconv.Itoa(p.stepCoinCount)})
			}
			state.q4one(p, &reply{Cmd: "coins", Payload: p.Coins})
		}
		p.stepCoinsValue = 0 //reset for next step
		p.stepCoinCount = 0
	}

}

func (s *State) save(filename string) {

	bytes, err := bson.Marshal(s)

	if err != nil {
		logit(err.Error())
		return
	}

	file, err := os.Create(filename)
	if err != nil {
		logit(err.Error() + " save failed")
		return
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	writer.Write(bytes)
	writer.Flush()

	//write the bytes slice to a file

}

func load(filename string) *State {

	file, err := os.Open(filename)
	if err != nil {
		logit(err.Error() + " load failed")
		return &State{}
	}
	defer file.Close()

	reader := io.Reader(file)
	bytes, err := io.ReadAll(reader)
	if err != nil {
		logit(err.Error())
	}

	state := NewState()
	err = bson.Unmarshal(bytes, &state)
	if err != nil {
		logit(err.Error())
	}

	logit("Loaded state from " + filename)

	return state

}

func (s *State) addThing(layer, picName string, isHole bool) int {
	t := NewThing(layer, picName, isHole)
	s.Things = append(s.Things, t)
	return len(s.Things) - 1 // return the index of the new thing
}

func (s *State) AddSpring(thing, m1, m2 int, collideable bool) int {
	s.Things[thing].Springs = append(s.Things[thing].Springs, NewSpring(s.Masses, m1, m2, collideable))
	return len(s.Things[thing].Springs) - 1 // NB: returns the index of the spring in the thing
}

func (state *State) AddPlayer(name string, position Vector) *Player {
	dozer := state.addThing("dozers", "dozer", false) // position,0,radius,"",0,false,"dozers",1)
	rl := state.AddMass(NewMass(position, 1, false, false, true, dozer))
	w := float64(100)
	h := float64(140)
	rr := state.AddMass(NewMass(position.add(Vector{w, 0}), 1, false, false, true, dozer))
	fl := state.AddMass(NewMass(position.add(Vector{0, -h}), 10, false, false, true, dozer))
	fr := state.AddMass(NewMass(position.add(Vector{w, -h}), 10, false, false, true, dozer))
	state.Things[dozer].Scale.Y = h / w
	state.Things[dozer].Scale.X = 1

	state.AddSpring(dozer, rr, rl, true) // bottom
	state.AddSpring(dozer, rl, fl, true) // left
	state.AddSpring(dozer, fl, fr, true) // top
	state.AddSpring(dozer, fr, rr, true) // right

	state.AddSpring(dozer, rr, fl, false) // cross members (not collideable)
	state.AddSpring(dozer, rl, fr, false)

	p := NewPlayer(name, dozer, state)
	state.Players[name] = p

	return p

}

func (p *Player) Move(state *State) {

	if !p.dying && !p.dead { //you loose all traction when dying
		dozer := state.Things[p.Dozer]
		rr := &state.Masses[dozer.Springs[0].M1].P
		rl := &state.Masses[dozer.Springs[0].M2].P
		fl := &state.Masses[dozer.Springs[1].M2].P
		fr := &state.Masses[dozer.Springs[2].M2].P

		leftside := fl.subtract(rl)
		lt := leftside.normalise() //left track

		rightside := fr.subtract(rr)
		rt := rightside.normalise() //right track

		const speed = 2
		fl.addIn(lt.multiply(p.LeftDrive * speed))
		rl.addIn(lt.multiply(p.LeftDrive * speed))

		fr.addIn(rt.multiply(p.RightDrive * speed))
		rr.addIn(rt.multiply(p.RightDrive * speed))

		revs := float32(math.Abs(float64(p.LeftDrive)) + math.Abs(float64(p.RightDrive)))
		if revs != p.oRevs {
			p.oRevs = revs
			state.q4all(&reply{Cmd: "revs", Payload: revsPayload{Player: p.Name, Revs: revs}})
		}
	}
}

func (state *State) closestSpring(wp Vector) (spring int, thing int) {
	closestSpring := -1
	closestThing := -1
	closestDistance := float64(1000)

	for t := 0; t < len(state.Things); t++ {
		for s := 0; s < len(state.Things[t].Springs); s++ {

			spring := state.Things[t].Springs[s]
			m1p := state.Masses[spring.M1].P
			m2p := state.Masses[spring.M2].P

			if wp.liesBetween(&m1p, &m2p) {
				d := wp.distanceFromLine(&m1p, &m2p)
				if d < closestDistance {
					closestDistance = d
					closestSpring = s
					closestThing = t
				}
			}
		}
	}
	return closestSpring, closestThing

}

func (state *State) closestMass(wp *Vector) int {

	//let closestDistance=within
	for i := 0; i < len(state.Masses); i++ {
		m := state.Masses[i]
		d := wp.distanceFrom(&m.P)
		if d < m.R {
			return i
		}
	}
	return -1
}

func (state *State) checkHoles() {

	//check for escapes
	for _, m := range state.Masses {
		if !m.IsCoin { //coins can never escape holes
			if m.fallingInto > -1 {
				if !state.Things[m.fallingInto].contains(&m.P, state.Masses) {
					m.fallingInto = -1 //phew, escaped
				}
			}
		}
	}

	for ti, t := range state.Things {

		if t.IsHole {
			for _, m := range state.Masses {
				if m.ThingNum != ti { //masses cannot fall into things they belong to
					if !m.Fixed && m.fallingInto == -1 { //you can only be falling into one hole at once - and fixed masses can't fall into anything
						if t.contains(&m.P, state.Masses) { //todo - optimise - non moving masses cant fall in holes
							m.fallingInto = ti
							if m.IsCoin {

								state.qSound("coin-flip", m.P, 0.2, "", false)
							} else {
								state.qSound("clank", m.P, 0.2, "", false)
							}
						}
					}
				}
			}
		}
	}
}

func (state *State) resolvePenetrations() bool {

	penetrated := false
	for _, m := range state.Masses {
		if m.Collideable && m.enabled {
			for ti, t := range state.Things {

				if !t.IsHole {
					if state.pushApart(m, ti) {
						penetrated = true
					}
				}
			}
		}
	}

	return penetrated
}

func (state *State) pushApart(mass *Mass, thingNum int) bool {

	penetrated := false
	thing := state.Things[thingNum]
	for _, spring := range thing.Springs {
		if spring.Collideable {
			if spring.contains(state.Masses, &mass.P) { //are we within the endpoints of the spring
				dist := mass.sideof(state.Masses, spring) //distance from the line .. negative means its gone to the right hand (wrong side)
				pen := dist - mass.R

				if pen < 0 && pen > -20 { //this.r*2 ) { //we're on the wrong side
					mass.lastThingTouched = thingNum
					mass.resolveMasSpringOverlap(state.Masses, spring, pen) //pen is some negative number
					penetrated = true
				}
			}
		}
	}

	return penetrated

}

// Used client side as the objects are dehyrdrated
func (state *State) centreOf(thingNum int) Vector {

	thing := state.Things[thingNum]
	r := Vector{0, 0}
	for _, spring := range thing.Springs {
		r.addIn(state.Masses[spring.M1].P)
	}

	f := 1 / float64(len(thing.Springs))
	return r.multiply(f)
}

func (state *State) countCoin(lastThingCoinTouched int, value int) {
	//for each player

	for _, p := range state.Players {
		if p.Dozer == lastThingCoinTouched {
			p.stepCoinsValue += value //value of coins won this step
			p.stepCoinCount += 1      //number of coins won this step
		}
	}
}

func (state *State) tumbleCoins() {
	for _, m := range state.Masses {
		if m.fallingInto > -1 && m.enabled {
			hole := state.centreOf(m.fallingInto) //state.things[m.fallingInto].centre(state.masses)
			m.moveTowards(&hole, 1)               //also dragging dozer tracks into holes 2 was too strong

			if m.IsCoin {
				m.Z -= 0.1 //fall down

				if m.Z < -5 {
					state.qSound("coin-drop", m.P, 0.2, "", false) //hit the bottom
					state.countCoin(m.lastThingTouched, int(m.R))  //this is a server side function
					m.enabled = false
				}
			}

		}
	}
}

func (state *State) stretchSprings() {
	for _, t := range state.Things {
		for _, s := range t.Springs {
			s.stretch(state.Masses)
		}
	}
}

func (state *State) anyPlayers() bool {
	return len(state.Players) > 0
}

func (state *State) scatterCoins(w float64, h float64) {

	countValues := []int{100, 10, 20, 20, 10, 30, 5, 50}
	for i := 0; i < len(countValues); i += 2 {
		v := countValues[i+1]
		for j := 0; j < countValues[i]; j++ {
			p := Vector{X: math.Floor(rand.Float64() * w), Y: math.Floor(rand.Float64() * h)}
			state.AddMass(NewMass(p, float64(v), false, true, true, -1)) //coins don't have a thingNum
		}
	}
}

func (state *State) checkDeaths() {
	for _, p := range state.Players {

		if p.Dozer > -1 && !p.dead {
			dozer := state.Things[p.Dozer]
			if p.dying {

				dozer.Scale.Y -= 0.01
				dozer.Scale.X -= 0.01
				dozer.Rotation += 0.1

				if dozer.Scale.X <= 0 {
					p.lives--

					if p.lives > 0 {
						dozer.Scale.Y = 140 / float64(100)
						dozer.Scale.X = 1
						dozer.Rotation = 0
						p.dying = false
						state.resetDozer(p)
						state.q4all(&reply{Cmd: "banner", Payload: p.Name + " has " + strconv.Itoa(p.lives) + " lives left"})
						sendWholeThing(state, p.Dozer) //does a q4all
					} else {
						p.dead = true
						p.dying = false
						state.q4all(&reply{Cmd: "banner", Payload: p.Name + " is dead"})
						state.q4one(p, &reply{Cmd: "dead", Payload: ""})
						state.deathList = append(state.deathList, p) //TODO - respawn/ spectate etc
					}
				}

				r := reply{Cmd: "skin", Payload: skinPayload{Ti: p.Dozer, Scale: dozer.Scale, Rotation: dozer.Rotation}}
				state.q4all(&r)

			} else {
				for _, s := range dozer.Springs {
					m := state.Masses[s.M1]
					if m.fallingInto > -1 {
						hole := state.Things[m.fallingInto]
						cs := hole.closestPointOnEdge(state.Masses, &m.P)
						d := cs.distanceFrom(&m.P)
						if d > 100 { //more than 50 units over the edge (we already know we are inside the hole)
							if !p.dying {
								p.dying = true
								state.qSound("dozer-fall", m.P, 0.4, "", false)
							}
						}
					}
				}
			}
		}
	}
}

func (state *State) resetDozer(player *Player) {

	var pos Vector = state.RandomStartPos()

	w := float64(100)
	h := float64(140)

	m := player.getMasses(state)

	//fl, rl, fr, rr
	m[0].P = pos
	m[1].P = pos.add(Vector{0, h})
	m[2].P = pos.add(Vector{w, 0})
	m[3].P = pos.add(Vector{w, h})

	//kill their velocity
	for _, m := range m {
		m.op.X = m.P.X
		m.op.Y = m.P.Y
	}

}

func (state *State) makeHoles(numHoles int, w float64, h float64) {
	for i := 0; i < numHoles; i++ {
		x := math.Floor(rand.Float64() * w)
		y := 400 + math.Floor(rand.Float64()*h)
		r := 100 + rand.Float64()*400
		state.MakeHole(x, y, r)
	}
}

func (state *State) MakeHole(x float64, y float64, r float64) {
	t := state.addThing("holes", "hole", true)
	state.Things[t].Scale.Y = 2.1
	state.Things[t].Scale.X = 2
	state.Things[t].Offset.X = -r / 2

	const sides = 9
	for i := range sides {
		a := float64(i) * math.Pi * 2 / sides
		b := float64(i+1) * math.Pi * 2 / sides
		p1 := Vector{x + math.Cos(a)*r, y + math.Sin(a)*r}
		p2 := Vector{x + math.Cos(b)*r, y + math.Sin(b)*r}
		m1 := state.AddMass(NewMass(p1, 1, true, false, false, t))
		m2 := state.AddMass(NewMass(p2, 1, true, false, false, t))
		state.AddSpring(t, m1, m2, false) //holes don't have thingnums (which are just a way to track who coins belong to)
	}
}

// executes a physics step and returns the index and new position for all the masses that move
func (state *State) moveAll(substeps int) {

	//movedMasses := []int{} //return the index, x and y of all masses that move

	if state.anyPlayers() && len(state.Masses) > 3 {

		for substep := 0; substep < substeps; substep++ {

			for _, m := range state.Masses {
				m.op = Vector{m.P.X, m.P.Y}
				m.oz = m.Z //record all position (AND depths)
			}

			//move by inertia and friction
			for _, m := range state.Masses {
				m.P.addIn(m.v.multiply(.8)) //was .9
			}

			for _, p := range state.Players {
				if p.Dozer > -1 { //controller players don't have dozers
					p.Move(state) //state.movePlayer(p) //based on a players keyboard/touch inputs move their track masses
				}
			}

			state.stretchSprings()
			state.stretchSprings()
			state.stretchSprings()

			// do{
			// }while (this.resolvePenetrations()) //loop unit all mass-thing pepetrations are resolved

			state.checkHoles()
			state.checkDeaths()

			state.resolvePenetrations()
			//masses are pushed out of things (and things away from masses)
			state.resolveMassOverlaps()
			state.tumbleCoins() //may change angle

			// //calculate velocity based on moevent
			for _, m := range state.Masses {
				m.v = m.P.subtract(&m.op)

				if m.v.lengthSq() < 0.01 {
					m.v = Vector{0, 0}
				}
			}

		}
	}

	for _, p := range state.Players {
		if p.Dozer > -1 {
			if p.moved(state) {
				state.RecordTrack(p)
			}
		}
	}

}

func (player *Player) getMasses(state *State) []*Mass {

	dozer := state.Things[player.Dozer]
	port := dozer.Springs[1]
	starboard := dozer.Springs[3]

	m := state.Masses
	fl := m[port.M2]
	rl := m[port.M1]
	fr := m[starboard.M1]
	rr := m[starboard.M2]

	return []*Mass{fl, rl, fr, rr}
}

func (state *State) RecordTrack(player *Player) {

	if state.Tracks[player.Name] == nil {
		state.Tracks[player.Name] = &track{Pointer: 0, Points: make([]float64, 800)}
	}
	track := state.Tracks[player.Name]

	track.record(player.getMasses(state))

}

// encode float 64's into the tracks points
// A track is a stream of float 64's 8 per frame per player, 2 (x/y)  values per vert, 4 verts
// this is to (massively) reduce the JSON overhead
func (track *track) record(masses []*Mass) {
	if len(masses) > 4 {
		panic("more than 4 track masses!" + strconv.Itoa(len(masses)))
	}
	if len(track.Points) < 8 {
		panic("track points too small!")
	}
	for _, m := range masses {
		track.Points[track.Pointer] = m.P.X
		track.Points[track.Pointer+1] = m.P.Y
		track.Pointer += 2
	}
	if track.Pointer >= len(track.Points) {
		track.Pointer = 0
	}
}

func (player *Player) moved(state *State) bool {

	for _, m := range player.getMasses(state) {
		if m.v.lengthSq() > 0.01 {
			return true
		}
	}

	return false
}

func (state *State) resolveMassOverlaps() {

	for o, a := range state.Masses {

		for i := o + 1; i < len(state.Masses); i++ {
			b := state.Masses[i]

			if a.Fixed || b.Fixed || !a.enabled || !b.enabled {
				continue
			} //no need to check fixed masses
			//optimise here - we dont need to do the full distance calculation
			d := a.P.distanceFrom(&b.P) //Vector.distanceBetween(a.position,b.position)
			overlap := (a.R + b.R) - d

			if overlap > 0 {
				//let v = ap.subtract(bp).normalise().multiply(0.5)
				delta := b.P.subtract(&a.P)
				delta = delta.normalise()
				delta = delta.multiply(overlap)

				afix := .5 //b.mass/(a.mass+b.mass)
				if b.Fixed {
					afix = 1
				} //if b is fixed then a is pushed out of b
				if !a.Fixed {
					a.P.subIn(delta.multiply(afix))
				}
				if !b.Fixed {
					b.P.addIn(delta.multiply((1 - afix)))
				}

				//transfer the last touch from the coin moving fastest
				if a.IsCoin && b.IsCoin {
					if a.v.lengthSq() > b.v.lengthSq() {
						b.lastThingTouched = a.lastThingTouched
					} else {
						a.lastThingTouched = b.lastThingTouched
					}
				} else if a.IsCoin { //or off the object touched if one of the masses is not a coin
					a.lastThingTouched = b.ThingNum
				} else if b.IsCoin {
					b.lastThingTouched = a.ThingNum

				}
			}
		}
	}
}

func (state *State) setupTiledLayer(layerName string, pic string, tileSize float64, extension string, w float64, h float64) {

	layer := NewLayer(layerName, []string{pic}, extension)
	state.Layers[layerName] = layer
	x := -tileSize * 2
	y := -tileSize * 2
	down := w / tileSize
	for i := float64(0); i < down+4; i++ {
		for j := float64(0); j < h/tileSize+4; j++ {
			p := Prop{Position: Vector{x, y}, Angle: 0, Radius: tileSize / 2, Pic: pic}
			layer.Props = append(layer.Props, p)
			x += tileSize
		}
		x = -tileSize * 2
		y += tileSize
	}
}

func (state *State) setupRandomLayer(layerName string, picList []string, extension string, numProps int, minRadius float64, maxRadius float64, w float64, h float64) {

	layer := NewLayer(layerName, picList, extension)
	state.Layers[layerName] = layer
	layer.Props = make([]Prop, numProps)

	for i := range numProps {

		var p Vector
		var pic string
		var radius float64

		for {
			x := math.Floor(rand.Float64() * w)
			y := math.Floor(rand.Float64() * h)
			p = Vector{x, y}
			pic = picList[int(math.Floor(rand.Float64()*float64(len(picList))))]
			radius = minRadius + rand.Float64()*(maxRadius-minRadius)
			if !spotOccupied(layer.Props, &p, radius) && !state.inHole(p) { //don't pile things ontop of each other
				break
			}
		}

		prop := Prop{Position: p, Angle: 0, Radius: radius, Pic: pic}
		layer.Props[i] = prop
	}
}

func (state *State) setupLayers(w float64, h float64) {
	state.setupTiledLayer("snow", "snow", 512, ".jpg", w, h)
	//NB: Trees are drawn with a drawScale of 1.4 (ie.. substantially bigger than their 'collidable' circles)
	//this.setupRandomLayer("trees","trees,trees1,trees2,trees3,trees4,trees5,trees6,trees7,trees8,trees9,trees10,trees11,trees12,trees13,trees14,trees15,trees16,trees17,trees18",".png", 50, true,150,25,1.4)
	state.setupRandomLayer("trees", []string{"trees1", "trees5", "trees9", "trees11", "trees14"}, ".png", 100, 50, 200, w, h) //nick removed some of the more 'exotic' trees
	//this.setupRandomLayer("puddles", "puddle2",".png", 30, false,50,150,1,0)
	//this.setupRandomLayer("leaves", "leaf",".png", 150, false,10,10,1,0)
	state.setupRandomLayer("coins", []string{"coin"}, ".png", 1, 1, 1, 100, 100)
	state.setupRandomLayer("holes", []string{"hole"}, ".png", 1, 10, 10, 10, 10)
	state.setupRandomLayer("dozers", []string{"dozer"}, ".png", 1, 100, 100, 100, 100)

}

func NewState() *State {
	gameId := int(rand.Float32() * 1000000)
	return &State{GameId: gameId, Players: map[string]*Player{}, Masses: []*Mass{}, Things: []*Thing{}, deathList: []*Player{}, Layers: map[string]*Layer{}, Tracks: map[string]*track{}}
}

func (state *State) qSound(sound string, position Vector, volume float32, label string, loop bool) {
	//this is a queue for sounds to be played

	// = (sound:sound,position:position,volume:volume,label:label,loop:loop)

	payload := soundPayload{Sound: sound, Position: position, Volume: volume, Label: label, Loop: loop}
	state.q4all(&reply{Cmd: "sound", Payload: payload})

}

func (state *State) q4all(msg *reply) {

	for _, p := range state.Players { //for every outbound que (player)
		state.q4one(p, msg)
	}

}

func (state *State) q4one(p *Player, msg *reply) {

	if !strings.HasPrefix(p.Name, "bot") {
		p.qh.mutex.Lock()
		p.qh.q[state.Sqn] = append(p.qh.q[state.Sqn], msg)
		p.qh.mutex.Unlock()
	}

}
