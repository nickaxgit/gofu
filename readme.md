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



* spawn into existing game (a table holds gameId, vehicle name, seat, position, orientation, velocity,water, fuel, engineRpms) - save/merge should be able to snapshot all these things
* vehicle views/seats - array of camera offsets and rotations (yaw and pitch), and folow types (hard/soft/withRoll) - for multiple internal and external (named) views


Welcome Nick(from cookie)

Join a game

2939 - Canadian forest - 1,244 acres alight 10,074 burned, 36 players
    Snowfall base - CL415
    Hardy Lake - Air tractor
    Box canyon - Parachute

2939 - Canadian forest - 12 acres alight, 700 burned, 12 players
    Chey mountain - on foot


Create a game

* Candian forest
Mediterrainian Islands
Californian suburbs


Only 'registered' players can create games
/createGame?world=foo&vehicle=bar (cookie determines player)


*Spawning into a running game*

/spawnin?gameId=foo&vehicle=bar&view=pilot (cookie determines player - absent, creates new player (and sets cookie)) - note, no cookie is *required* to join a game 

Vehicle is merged (at the position/velocity/rpm/fuel/water loading it was snapshot)

May need to wait for airspace to be clear at the spawn point - waits of more than 5 second should spawn at an offset


*Add view*
Adds a view/spectate/instrument panel/cockpit side window

//records the current camera position and direction 'vehicle space'
msgAddViewer
name=portWindow

save it into the vehicle (thing)

done by a creator.editor - gets saved into the thing

Thing.views.add Name,camera


msgView
playerToViewId
viewpoint

struct thing
    viewpoints map[string]*Camera //Additional *initial* (named) position, direction and up in vehicle space


    There are players, viewers and controllers
    A separation of concerns here allows a player to have many viewers and many controllers

    Game
        []players
            []viewers

struct Viewer
    ws Websocket
    *player  (to get vehicle)
    viewpoint string
    camera //initially a clone of viewPoint(within the vehicle) - Ongoing, additional position direction and up in vehicle space (our head swivel/slew)
    

If a player has no vehicle - the world is their vehicle and all normal rules apply 
(you are moving/rotating the camera in vehicle space)
Boarding a vehicle - sets the camera offset and rotation to that initially specified in the view
and the vehicle translation/roatation is (always) added to the camera when it is sent


You don't need an account, or permission to view
Multiple viewers of the same cameras are allowed (spectators might all want the pilot seat)
Each viewer has an additional camera offset (position and direction)

 - allows you hop in as co-pilot
/join?pid=4524&view=copilot&invite=fdfs (cookie determines player - absent, creates new player) 

sends a joinAsConstroller(token) message over a socket - then streams blob positions


each player has a collection of invites - they can be revoked


You are joining the existing players vehicle (in an unoccupied 'seat'), 

Both players have the same vehicle - your control inputs will be merged.
A player can only be in one game at a time so no game id is required

Invite/Mission is used so that links can be shared, but permission (which is automatically granted) can be revoked in future

Joins with an expired mission should be actively refused (and logged)

/spawnbehind?pid=4524&vehicle=cl415&seat=copilot

"Player 4524 is not presently in a game - spawning at default point for mission"





Missions encode a lot of things - spawning, joining, co-piloting, permissions and expiry, multiple views (multi-monitor setup)

ID:-
XKWPL

World:-
Canadian Forest

Game:
2542

Owner (Who can edit/revoke)
Nick (choose)

Name
Firefight 101

Description
Final approach on the fire, drop the water and return to base.

Expires:-
1 minute
1 hour
12 hours
24 hours
1 week
1 month
* Never

Scope:-
* Anyone
Named Group (choose/create)

Num Available:

1
10
100
1000
*Infinite

Numused:
683

Vehicle:-
None
CL415 (choose)

View:
* Pilot
CoPilot
Chase
outside Port
outside Starboard
Dash
Ovehead panel
Port window
Starboard window

Control Permissions:-
* StickX
* StickY
* Rudder
* Brakes left
* Water drop
* Gear Flaps


Join:-
* Spawn in new vehicle
Join inviter in their vehicle (if present)
Spawn behind inviter - if present (with their speed and heading)
Join as a new camera in your own vehicle (cockpit windows and dashboards)



*Taking control*
of a player (using a control token) *

/control?token=ddjsj

Means a device can provide additonal input via msg.ControlPositions - which sends a set of tracking blobs (which update control surfaces via standard Mixers)

The controller device should set a cookie of the control token so it remains permanently linked
(or can be re-linked via a single click)

If the player being controlled disconnects - the controller should periodically (every 1 second) attempt reconnection.


Q: How do clients reliably communicate who they are
A: Cookie (auth token) until socket is connected







Playing on your TV

If you have an Android Phone and Google chrome - you can 'cast' the game (you are in) to the TV

If you have an Apple iPhone it's easier to start or join a game using the TV's web browser, and then take control of it by scanning the QR code on your TV





Timeline:-

by 5th November - Major refactor complete - high speed dynamic terrain
by 12th November - aircraft loading, flying, multiple viewers and controllers
by 19th November - alighting, boarding, spawning, persistence - server restarts/upgrades
by 25th November - collisions, bouyancy, scooping
by 1st Decemeber - water & fire FX ()
by 8th Decemeber - fire dynamics/modeling - slopes wind, basic smoke

* UI/Gameplay
    * Action replay 
    * Visual Rewind/DoOver (note you can't doOver in a multiplayer game - you would have to rewind time for everyone)
    * Ground 'tracking' camera (Firebeaters PoV of your drop)
    * Ground & fire mini/moving maps
    * Missions, setup/definition, Scoring/completion

    * Music
* Commms
    * Audio comms (radio/PTT)
    * Beeping/deswearing (FFT, stretching, matching (delta F's))

* Marketing
    * Name/Branding/Domain
    * Frictionless easy 'drop in' missions
    * Invites/Colab play

* Controls & display
    * Flight controls (inputs - blob tracking, controllers)
    * Touchscreen control
    * Reverse blob tracking (yoke control - sensor fusion (cameras and accelerometer)

* Realism/simulation
    * Gear, flaps and control surface animations
    * (working) Cockpit instruments
    * Structural failures (overspeed/crashes)

* Landscape
    * Trees variations and habitats
    * Pruning and planting
    * Snow-pack
    * Flowing/better water
    * Infrasturcture - roads and buildings

* Editor    
    * Editor fixes, cleaning and re-work
    * Skin editing

* Characters
    
    * Angle and twist constraints (on springs)
    * Humans / ragdolls
    * balancing/walking

* Aircraft
    * Fire Tractor
    * Drone

* Server
    * Security
    * Performance
    * Deployment
    * Persistence/updating

* Membership
    * Signup 
    * Payment processing    
    * Mailing engine
    * In-game mailing/(off line) messaging
    * Monetisation - fuel costs money, aircraft cost money - beating/putting out fires earns money
   
   
    possible to play for free - but without a vehicle
    AT-802F's = 4.8M$
    CL415 30M$
    Pivotal "BlackFly" drone 190k$ (base)

    Avgas - $7000 full load (10,000 lbs) for the CL415

    at 1:1000 - it would cost 7 real-world dollars to fill
    
    Each (real) USD buys you 1 million game dollars


   players $balance is on display 

   $/€/£/AUG - Money - the game operates in grams of gold (approx 100$) - balances can be displayed in any currency at the day market rate - the will fluctuate when displayed in antyhing other than AUG
   10kg of gold - approx $1M dollars - will cost £1 - which makes a super scooper cost £30
   A $7,000 fuel load would cost 0.7cents (70 AUG)


   XP - Experience points
   RP - Reputation points (increase with succesful missions, teamwork, co-piloting/spotting, donations, ROK's, decrease with crashes, incomplete missions, 'cowardice', dangerous flying (high G) , quitting, fighting, swearing (you can knock someone reputation by twice what you are prepared to give up - or maybe it's some inverse proportion) ) 
     


A viewer has a player (except prior to) watch 
It is a lightweight object holding, a camera and a playerId

A game can exist without players - it doesn't (need to) "belong" to anyone

viewer.msgCreateGame  returnd gid

msgCreatePlayer("name") - returns Pid

AddPlayerToGame(gid,pid)

msgWatch(PlayerId)

msgReconnectViewer(viewerId)


A viewer is created upon connection - added to the blobal viewrs and its viewer.socket is set to the ws
It's watchingplayerID is initially -1
viewers have a watchingPlayerId
players Have a game id
The player.viewers and game.players collection have been removed (so player.gameId and viewer.watchingPlayerId are single sources of truth)

all commands come from viewers (not players)
CreateGame - only creates a game
A viewer and player must be added



viewer.msgWatch(pid)

A player should be able to leave a game and join another - it's viewers should follow automatically
A viewer should be able to change it's watchingPlayer
A viewer should be able to switch to a different camera (clone a differnt camera in the vehicle)


Drop cams .. should appear on the ground - some distance ahead of the vehicle, then track the vehicle
(parachuting drop cams ?)