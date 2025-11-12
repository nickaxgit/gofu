A Golang world simulator

Designed to implement a firefighting, flight simulation over a very large procedurally gerated terrain.

All the computational heavy lifting is performed server side - final meshes are streamed to a very thin Three.JS/TypseScript client.

Inputs from the clients affect the flight controls - of mass-spring models with aerodynamic flight surfaces.

Drag, lift and thrust are modelled, and these forces accelerate masses which are attached together with a network of springs - the emergent flight behaviour, structural loading and failures are very realistic.

The game can be played (viewed) on one device and controlled with another - so you can (for example) play on your Internet enabled wide-screen TV (in it's web browser) whilst flying the plane with your mobile phone (using it's touchscreen, camera,accelerometers and Lidar - to gain spatial and orientation input - to use it as a realistic flight yoke)

Multiple devices can watch the same player/vehice, from multiple angles - allowing spectate functionality - but also multi screen (multi-device) cockpits - with side windows instrument panels. 

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
dashcam records a continuous loop (1 minute) of all mass positions
playback will require the game metadata (landSize,things, asset meshes),
but you can move to any camera/position during blackback and land will be re-rendered for your camera
Firemesh contents will also need to be encoded per fire growth interval
Recording absolutes rather than deltas simplifies rewind and file sizes should be tiny anyway (the client actuall only needs the three key masses per thing)





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


    There are players, and devices - some of which are controllers, some of which view, some do both
    A separation of concerns here allows a player to have many viewers and many controllers

    Game
        []players
            []viewers

struct Device
    ws Websocket
    *player  (to get vehicle)
    viewpoint string
    camera //initially a clone of viewPoint(within the vehicle) - Ongoing, additional position direction and up in vehicle space (our head swivel/slew)
    

If a player has no vehicle - the world is their vehicle and all normal rules apply 
(you are moving/rotating the camera in vehicle space)
Boarding a vehicle - sets the camera offset and rotation to that initially specified in the device
and the vehicle translation/roatation is (always) added to the camera when it is sent


You don't need an account, or permission to view
Multiple devices can view from the same initial camera position are allowed (spectators might all want the pilot seat)
Each device has an additional camera offset (position and direction)

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
    * Dugan mixing (open mic) ?

* Marketing
    * Name/Branding/Domain (fireflight, wfhero, aboveandbeyond,)
    * Frictionless easy 'drop in' missions
    * Invites/Colab play

* Controls & display
    * Flight controls (inputs - blob tracking, controllers)
    * Touchscreen control
    * Reverse blob tracking (yoke control - sensor fusion (cameras and accelerometer)

* Client
    * Stripdown (things, springs, coins, footprints, tracks, props, players)
    * Left with masses, localMeshes and vectors
    * Reinstate/add control sticks (touch)

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
    * Balancing / walking
    * Torque drivers (for diggers)

* Vehilces/tools
    * CL415
    * Fire Tractor (AT8F)
    * Drone (blackfly/quadcopter)
    * Rescue heli
    * Dozer
    * JCB/Minidigger/giant digger) (think of a cool mission)
    * Komatsu XT465L-5 (400k$)  feller buncher (https://www.youtube.com/watch?v=z3pOMI1nSoo) https://www.komatsuforest.com/-/media/komatsu-forest/images/komatsu-forest-na/brochure-files/fpsb1040-00_xt430-5-xt445l-5-xt465l-5_en.pdf?la=en
    * Chainsaw
    * Fire beater
    * Rake Hoe (McLeod ?)
    * Parachute (Ram air 7 cell ?)

* Server
    * Security (player and device tokens)
    * Abuse - ip logging, ASN lookup, rate limiting curl https://ipinfo.io/8.8.8.8/json
    * Performance
    * Deployment
    * Concurrency
    * Persistence/updating (new releases)
    * Error and event logging (and viewer)
    * Entropy encoding (mesh vert deltas)
    * Compression - gorilla - websocket.Upgrader{enableCompression}

* Membership
    * Signup 
    * Payment processing    
    * Mailing engine
    * In-game mailing/(off line) messaging
    * Monetisation - fuel costs money, aircraft cost money - beating/putting out fires earns money

* Ranks and leauges
    * Total fires extinguished
    * Fuel burned
    * Hangar worth (net worth)
    * XP
    * RP
    * Flying hours
    * Aircraft destroyed
    * Rescues/lives saved

   
* Virtual currency and 'shop'
    * Bank accounts and transaction, balances, reversals
    * Deposits (paypal etc)
    * Assets - things you can buy
    * Possessions - instances of assets you own (via a transaction,or have owned)
    * Destruction (damage/wear)- possesion.destroyedInGames[]
    * 2nd hand vehicle sales (?) pros and cons
   
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
   RP - Reputation points (increase with succesful missions, area extinguished, teamwork, co-piloting/spotting, donations, RAK's, decrease with crashes, incomplete missions, 'cowardice', dangerous flying (high G) , quitting, fighting, swearing (you can knock someone reputation by twice what you are prepared to give up - or maybe it's some inverse proportion) ) 
     


A viewer has a player (except prior to) watch 
It is a lightweight object holding, a camera and a playerId

A game can exist without players - it doesn't (need to) "belong" to anyone

viewer.msgCreateGame  returnd gid

msgSignUp("name") - returns Pid

AddPlayerToGame(gid,pid)

msgWatch(PlayerId)

msgReconnect(viewerId, token) -1 for unknown


A viewer is created upon connection - added to the global viewers and its viewer.socket is set to the ws
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





Sign up
    email
    password
    player name



Sign In

    email (or playername)
    password    (forgot password)

Welcome

    Hi Nick - PID:19277

    Your devices:-

    * This device

 Account sign inss
 |Type     |Name                 |status |Action    |
 |---------|---------------------|-------|----------|
 |Unknown\/|This device [edit]   |Active |[sign out]|
 |DeskTop  |                     |Off    |[sign out]|
 |laptop   |Work laptop          |Active |[sign out]|

 note a device token (for a viewer/controller) is a different thing from an account/device management token (stores the players login)
 
 

 Veiwing Devices 
 id   |Name     |Controls  |Point of View        |Look         |Status | Action   | label| 
 -----|---------           |---------------------|-------------|-------|----------|------|
 A9G8 |TV           None   |Pilot                |forward      |Active |[forget]  |
 4F74 |Desktop  |   all    |Front External       |forward      |Off    |[forget]  |
 4FG8 |iPad     |   all    |Pilot                |instruments  |Active |[forget]  [configure] |
 2832 |iPhone   |   Some   |Pilot           |                  |Off    |[forget]  |  
 10W8 |laptop   |   some   |pilot                |right        |        [forget]  |Lenovo
 0T72 |laptop   |   some   |pilot                |Pilot left   |       |[forget]  |
 FSUS |Android  |   some   |none                 |none         |Active |[forget]  [configure] |

 8JQW |Choose\/ |Choose \/            |choose     \/|waiting|[forget]  |[...]  
[Add new device] (encodes PID)

Controllers      Role
 4FG8 |Ipad     |Flight yoke          |Active |[forget]  [configure] |
 

XXXXX
XXXXX
XXXXX

Scan the QR code with the device to connect it
Or go to https://fire.com/ad/8JQW

Fill in the type, role and Choose a vehicel camera for the new device


Notes:-
Any device (viewer/controller) that has a valid token will auto-connect
Most devices (viewers) don't need to "sign in" per se - they just (re)connect
You can sign in to your player account on any device/browser to manage devices.
You can remove a device regardless of it being connected (it's token will be invalidated server side)
"Add a device" will prompt for a name, role and camera - and generate a serverside auth token and display it as a QR code/Url/4 digit code - entering that on the device will save it as a cookie - and submit it - the token keys a PID, upon 'connection'  (the viewer has its .Player set)
You can add a device, and as you choose the camera - the view on that device should change (live)




Sign Out  (Of device management/player account) deletes local cookie
    Are you sure (you probably don't need to - you can have/leave multiple devices signed into your) account
    We recommend you have an email set if you are goinf to sign out - for easy password recovery.


		//player id must exist player name must match
		//a player can be signed in on (have valid auth tokens) on many devices
		//devices (viewers/controllers) attach to a player and are issued an auth token
		//When signed in - player sees a list of devices that are authed
		//A player can be signed in, and not in any game
		//A can player join any (one) game
		//Additional devices can join as viewers/controllers
		//If he joins a *different* game on a second device - the players game id is changed - all viewers now view the player in the new game 
		//Any vehicle is left unpiloted (unless under dual control) - warn if airbourne
		//A player owns an aircraft, and can use it in any game until it is destroyed
		//If the player survives (say) a crash landing, he can board another aircraft (from his stable) - he might be in for a long walk home
		//A player can request control of an aircraft they don't own (from its owner)
		//A player can have many viewers
		//A player can only ride one vehicle  at a time
		//Many viewers can control a players vehicle



Each vehicle can be used until it is destroyed in a game
If you crash/land out, you can radio for another of your vehiciles to be autopiloted to your location
Owning a rescue drone or helicopter becomes pretty imporant.
Or you can hike to the nearest lake for a floatplane recovery.
You *can* be rescued by another player and returned to base

You can die in a game - primarily by smoke inhalation/heat stroke - nothing too graphic - or by fatal air crash.
Ejecting/parachuting out is an option.

Staying alive for the duration of a game, until the fire is out yeilds a big (XP) bonus (proportional to your contribution/time spent)
The 'fallen' in a game should be given a gravestone high on the scorched ground - they can continue to observe the game - but they fall silent - possibly you view as an "angel" - maybe you can still have some influence .. direct the wind or something 'ethereal' (the wind always blows away from angels)






GDC

$100/day expenses

10% of the 'company' - =10k$

I will use not less than 20% of revenue to buy back until paid OR you can keep the shares (or any porion thereof) at that value
You will be offered the same terms as me on any sale of shares
Your shares will by diluted equally if/when we take furthe investment

Say - we had $1m investment - in the company valued at 100k your 10k is not wo