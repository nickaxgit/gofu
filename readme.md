A Golang world simulator

Designed to implement a firefighting, flight simulation over a very large procedurally gerated terrain.

All the computational heavy lifting is performed server side - final meshes are streamed to a very thin Three.JS/TypseScript client.

Inputs from the clients affect the flight controls - of mass-spring models with aerodynamic flight surfaces.

Drag, lift and thrust are modelled, and these forces accelerate masses which are attached together with a network of springs - the emergent flight behaviour, structural loading and failures are very realistic.

The game can be played (viewed) on one device and controlled with another - so you can (for example) play on your Internet enabled wide-screen TV (in it's web browser) whilst flying the plane with your mobile phone (using it's touchscreen, camera,accelerometers and Lidar - to gain spatial and orientation input - to use it as a realistic flight yoke)

It is also possible to use your dektop webcam to optically track the position of physical flight controllers (joysticks, pedals, yokes, throttles etc) - allowing you to use 'homebrewed' cockpits - you can fly the plane with a wooden spoon if you like (although you will need to attach a brighly coloured sticky dot). The more adeventurous could 3D print realistic controller parts and rig up hydruallic actuators and LED indicators (made from disposable medical syringes, coloured fluids, and cheap clear plastic tubing) - your webcam can then track a single panel where (switched) LEDs and Hydraulic indicators are clustered - the webcam then picks up these as control inputs.

The choice of a thin, browser based client - means game loading is almost instant and it can run at high frame rates on very low end devices - not requiring expensive 3D cards.

The game will run on phones (Apple and Android), tablets, laptops, desktops, TVs (LG/Samsung/Sony), Windows, Linux, Raspberry Pi, possibly even Firestick and Roku

All logic, simulation, and physics, and most updates/upgrades/licenesing takes place serverside.
The server is written in GO, a modern, a high performance, platform agnostic, concurrent language - it can be hosted on linux servers for a very low operating cost.


Fires are modeled at dynamic resolution, very high (1m) on the firefront, and low (100m) on scorched or unlit terrain

Terrains are 100's of square kilometers (thousands are very possible), with procedural terrain details to sub-metre resolution, and vegetation down to individual leaves and petals.

Terrian and vegetation is generated in realtime, frustrum, backface and occlusion culled and sent per client.

Caching of the mesh views can trade storage for CPU - and views can be re-used and shared accross players

Many (tens, or hundreds of) players can interact in the same world - and many worlds can be run on one low cost, hosted server.

The subject matter, realistic flight, an expansive natural world, coopertive and heroic play and emotionally charged side missions should appeal to a wide and mature audience - not necessarily one of 'gamers'

Frictionless, one click entry to dramatic scenes in the game (water drops, landings, scooping operations) should entice older men - whilst scenes of natural (or manicured) beauty could bring ladies in to craft gardens and parks.

Downstream, the addition of fauna (animals) could heighten drama - roads, schools, factories and houses could all need protection

Career modes, would allow you to explore the terrain (and fight the fire) in a number of vehicles, from 'fireJumping' in on a parachute, to bulldozing a fire-break, to running medical supplies on a scrambler, flying an 'air tractor', drone or CL-415 'Super Scooper'

The game can be monetised through the sale of "aviation fuel" .. with the more glamorous vehicles being more thirsty

A system of experience points, mission rewards and pay-to-play needs to be devised and fine tuned



MiniGames (long term)

Walking/exploring
Wildlife photography "nice" hunting/shooting
gardening (planting, pruning)
Tobagon run
Potholing/caving
Drone piloting
Wingsuit flying
Whitewater rafting
Hot air balooning
Driving
(causing) Avalanches/rockfalls - tree destruction
Archery (camera based bow draw/arrow release)



TODO

Disentangle mass.selected from the game .. I need to be able to send selections to the client - but there is no need to perist them

Players should reside in global list - they can only be in one game (player.gameId) at once

games do not need to hold a set of players - ony a set of player ID's (or have server.sendToPlayersIn(gameId))

``
server
    players []Players
        mixers []Mixer
        selectedMasses map[*mass.Mass]bool
        highlit
        socket
        controllerSocket
    games[] game
        players []*players
        masses
        things
            springs

``


Hot updates

Be able to migrate a game and all its players to another server

and/or drop all state to disk, update and continue (reconnect web-sockets)

Action replay

Dashcam type functionality - but with a fully '3D' movie 
Playback mission higlights (or lowlights) from multiple angles on RTB





* Players can be signed in on many devices (authToken(cookie)>player)
* Players only be in one game at once - all devices are dragged into the most recent game
* Many viewing devices (3 monitor) - configure camera rotations (and offsets)
* same device running multiple browsers - gives flexible options for simultaenous internal and external views in different (browser) windows

* Many controlling devices
* Many devices/players can drive the same vehicle (e.g. pilot/navigator) - "You have control"
* Players don't have to be in a vehicle





Playing on your TV

If you have an Android Phone and Google chrome - you can 'cast' the game (you are in) to the TV

If you have an Apple iPhone it's easier to start or join a game using the TV's web browser, and then take control of it by scanning the QR code on your TV

