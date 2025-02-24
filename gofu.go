package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	//"golang.org/x/tools/playground/socket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// type user struct {
// 	mtx *sync.Mutex // a mutex is required to 'lock' access to each users connection (for writing)
// 	// many calls (to wsEndpoint) can be running in paralell - and more than one of them may attempt to write to a single users socket at the same time (not allowed!)
// 	conn *websocket.Conn // a pointer to the socket
// }

//var users []user

// func (player *Player) readForever() {

// 	for {
// 		// read in a message
// 		//websocket.BinaryMessage or websocket.TextMessage
// 		messageType, msg, err := player.socket.ReadMessage()

// 		if err != nil {
// 			logit ("Error reading message from websocket: " + err.Error())
// 		}

// 		if messageType == websocket.TextMessage{

// 			var block block
// 			//err = json.NewDecoder(msg).Decode(&block)
// 			err = json.Unmarshal(msg, &block)

// 			processBlock(block) //block{MessageType: messageType, Message: msg})

// 		}
// 	}
// }

// func send(user *user, messageType int, msg *[]byte) error {
// 	user.mtx.Lock()
// 	defer user.mtx.Unlock()                          //the defer statement runs this code when the function exits .. it's "idiomatic" in go .. it's excatly the same as if we made this the last line of the function
// 	return user.conn.WriteMessage(messageType, *msg) //write the message (and return any error)
// }

func homePage(w http.ResponseWriter, _ *http.Request) {
	logit("homePage", "called")
	fmt.Fprintf(w, "<h1>Dozer game server</h1>")
	for i, g := range games {
		fmt.Fprintf(w, "<p>Game %d has %d players", i, len(g.players))
		for _, p := range g.players {
			fmt.Fprintf(w, "<p>Player %s", p.name)
		}
	}
}

// func reset(w http.ResponseWriter, _r *http.Request) {

// 	log.Println("Resetting")

// 	bye := []byte("BYE")

// 	for i := range users {
// 		//fmt.Println(i)
// 		send(&users[i], websocket.TextMessage, &bye)
// 		users[i].conn.Close()
// 	}

// 	users = nil

// 	fmt.Fprintf(w, "<h1>Dozer game server</h1>")
// 	fmt.Fprintf(w, "<p>%d users are connected", len(users))

// 	log.Println("Reset")
// }

// func wsEndpoint(w http.ResponseWriter, r *http.Request) {
// 	// upgrade this connection to a WebSocket
// 	// connection
// 	ws, err := upgrader.Upgrade(w, r, nil)
// 	if err != nil {
// 		log.Println(err)
// 	}

// 	var u user
// 	u.conn = ws //hold a reference to the websocket
// 	users = append(users, u)

// 	m := []byte(fmt.Sprint(len(users) - 1))
// 	send(&u, websocket.TextMessage, &m) //write (as a string) your index in the slice of users

// 	log.Printf(fmt.Sprintf("Client %s Connected", strconv.Itoa(len(users))))

// 	// err = ws.WriteMessage(1, []byte("Hi Client!"))
// 	// if err != nil {
// 	// 	log.Println(err)
// 	// }

// 	// listen indefinitely for new messages coming
// 	// through on our WebSocket connection
// 	reader(ws)
// }

func main() {
	port := ":8081" //":443" //":8081"
	logit("Gofu server - listening on " + port)
	fs := http.FileServer(http.Dir("../dozer"))

	//important!
	games = make(map[uint32]*state)

	accountsByGuid = make(map[string]*account)

	//obq = make(map[string]*qHolder)

	//see customHeaders
	http.HandleFunc("/gi", gameTraffic)
	//http.HandleFunc("/home", homePage)
	//http.Handle("/", fs)

	//http.HandleFunc("/reset", reset)

	//http.HandleFunc("/ws", wsEndpoint) //web socket upgrader

	//this blocks the main thread
	go http.ListenAndServe(port, customHeaders(fs)) //, nil) // customHeaders(fs))
	//go http.ListenAndServeTLS(port, "dozer_world.crt", "./dozer.key", customHeaders(fs))

	logit("Starting ticker")
	stepWorlds()

}

// shit name - should be 'join' (or somesuch)
func gameTraffic(w http.ResponseWriter, r *http.Request) {

	// upgrade this connection to a WebSocket
	ws, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		logit(err)
	}

	//var state *state //initially nil set inside processMsg
	//var player *player

	//when a player creates or joins a game - their websocket is hooked up to the player
	var player *player
	for {
		// read in a messages forever on this socket (from this player)

		messageType, msgBytes, err := ws.ReadMessage()

		if err != nil {
			logit("Error reading message from websocket: " + err.Error())
		}

		if messageType == websocket.TextMessage {
			var structuredMessage msg
			//err = json.NewDecoder(msg).Decode(&block)
			err := json.Unmarshal(msgBytes, &structuredMessage)
			if err != nil {
				logit(err.Error())
			}

			processMsg(structuredMessage, player, ws) //player is set by creategame/joingame

		} else if messageType == websocket.BinaryMessage {
			//msgbytes is a slice of bytes
			if player == nil {
				player = processCreateOrJoin(msgBytes, ws)
			} else {
				player.processBinaryMsg(msgBytes)
			}

		}
	}
}

func customHeaders(fs http.Handler) http.HandlerFunc {
	// found at https://stackoverflow.com/a/65905091
	return func(w http.ResponseWriter, r *http.Request) {
		// add headers etc here
		// return if you do not want the FileServer handle a specific request

		if strings.HasSuffix(r.RequestURI, "/gi") {
			gameTraffic(w, r)
			return
		}

		if strings.HasSuffix(r.RequestURI, "/home") {
			homePage(w, r)
			return
		}
		if strings.HasSuffix(r.RequestURI, ".js") {
			w.Header().Set("Cache-Control", "no-cache")
		}
		//w.Header().Set("x-server", "hello, world!")
		fs.ServeHTTP(w, r)
	}
}
