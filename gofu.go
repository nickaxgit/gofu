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
		fmt.Fprintf(w, "<p>Game %d has %d players", i, len(g.Players))
		for _, p := range g.Players {
			fmt.Fprintf(w, "<p>Player %s", p.Name)
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
	port := ":443" //":8081"
	logit("Gofu server - listening on " + port)
	fs := http.FileServer(http.Dir("../dozer"))

	//important!
	games = make(map[int]*State)

	accountsByGuid = make(map[string]*account)

	//obq = make(map[string]*qHolder)

	//see customHeaders
	http.HandleFunc("/gi", gameTraffic)
	//http.HandleFunc("/home", homePage)
	//http.Handle("/", fs)

	//http.HandleFunc("/reset", reset)

	//http.HandleFunc("/ws", wsEndpoint) //web socket upgrader

	//this blocks the main thread
	//go http.ListenAndServe(port, nil) //, customHeaders(fs))
	go http.ListenAndServeTLS(port, "dozer_world.crt", "./dozer.key", customHeaders(fs))

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

	var state *State //initially nil set inside processMsg
	var player *Player

	//when a player creates of joins a game - their websocket is hooked up to the player
	for {
		// read in a messages forever on this socket

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

			player, state = processMsg(structuredMessage, state, player, ws) //block.GameId, from, m,player)

		}
	} //this is an infinite loop

	// player := state.AddPlayer(playerName, state.RandomStartPos(),ws)

	// 	var block block
	// 	// n, err := r.Body.Read(jsonBytes)

	// 	w.Header().Set("Access-Control-Allow-Origin", "*") //TODO - tighten this up
	// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	// 	w.Header().Set("Access-Control-Allow-Methods", "*") //POST, OPTIONS")

	// 	//logit(r.Method)
	// 	if r.Method == "POST" {  //it's either a JOIN or CREATE game command
	// 		err := json.NewDecoder(r.Body).Decode(&block)

	// 		if err != nil {
	// 			logit(err.Error())
	// 			w.Write([]byte("Error reading JSON (not a block) ?" + err.Error()))
	// 			//w.WriteHeader(http.StatusBadRequest)
	// 			return
	// 		}

	// 		//its possible there are 0 bytes in the request
	// 		// if len(block) == 0 {
	// 		// 	w.WriteHeader(http.StatusNoContent)
	// 		// 	//nothing in the outbound queue for you
	// 		// 	//w.response([]byte("{}"))
	// 		// 	return
	// 		// }
	// 		//w.WriteHeader(http.StatusOK)

	// 		response := processBlock(block)

	// 		w.Write(response) //process the request que (from this player) and return the response (typically moved masses)
	//	}
}

func customHeaders(fs http.Handler) http.HandlerFunc {
	// found at https://stackoverflow.com/a/65905091
	return func(w http.ResponseWriter, r *http.Request) {
		// add headers etc here
		// return if you do not want the FileServer handle a specific request
		// if strings.HasSuffix(r.RequestURI, "/gi") {
		// 	gameTraffic(w, r)
		// 	return
		// }
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
