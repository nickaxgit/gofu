package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	//"golang.org/x/tools/playground/socket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type user struct {
	mtx *sync.Mutex // a mutex is required to 'lock' access to each users connection (for writing)
	// many calls (to wsEndpoint) can be running in paralell - and more than one of them may attempt to write to a single users socket at the same time (not allowed!)
	conn *websocket.Conn // a pointer to the socket
}

var users []user

func reader(conn *websocket.Conn) {
	for {
		// read in a message
		messageType, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}

		sender := msg[0] //the sender id is in the first byte of the message
		//	fmt.Printf("sender was %d", sender)

		for i := range users { //send the message to everyone except the originator
			if byte(i) != sender { //the index (which we are interting) IS the byte userID (which will need to be a long at some point)
				send(&users[i], messageType, &msg)
			}
		}

		// if err := conn.WriteMessage(messageType, p); err != nil {
		// 	log.Println(err)
		// 	return
		// }

	}
}

func send(user *user, messageType int, msg *[]byte) error {
	user.mtx.Lock()
	defer user.mtx.Unlock()                          //the defer statement runs this code when the function exits .. it's "idiomatic" in go .. it's excatly the same as if we made this the last line of the function
	return user.conn.WriteMessage(messageType, *msg) //write the message (and return any error)
}

func homePage(w http.ResponseWriter, _ *http.Request) {
	logit("homePage", "called")
	fmt.Fprintf(w, "<h1>Dozer game server</h1>")
	fmt.Fprintf(w, "<p>%d users are connected", len(users))
}

func reset(w http.ResponseWriter, _r *http.Request) {

	log.Println("Resetting")

	bye := []byte("BYE")

	for i := range users {
		//fmt.Println(i)
		send(&users[i], websocket.TextMessage, &bye)
		users[i].conn.Close()
	}

	users = nil

	fmt.Fprintf(w, "<h1>Dozer game server</h1>")
	fmt.Fprintf(w, "<p>%d users are connected", len(users))

	log.Println("Reset")
}

func wsEndpoint(w http.ResponseWriter, r *http.Request) {
	// upgrade this connection to a WebSocket
	// connection
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}

	var u user
	u.conn = ws //hold a reference to the websocket
	users = append(users, u)

	m := []byte(fmt.Sprint(len(users) - 1))
	send(&u, websocket.TextMessage, &m) //write (as a string) your index in the slice of users

	log.Printf(fmt.Sprintf("Client %s Connected", strconv.Itoa(len(users))))

	// err = ws.WriteMessage(1, []byte("Hi Client!"))
	// if err != nil {
	// 	log.Println(err)
	// }

	// listen indefinitely for new messages coming
	// through on our WebSocket connection
	reader(ws)
}

func main() {
	port := ":8081" //":443" //":8081"
	logit("Gofu server - listening on " + port)
	//fs := http.FileServer(http.Dir("../dozer"))

	//important!
	games = make(map[int]*State)

	accountsByGuid = make(map[string]*account)

	//obq = make(map[string]*qHolder)

	//see customHeaders
	http.HandleFunc("/gi", gameTraffic)
	//http.HandleFunc("/home", homePage)
	//http.Handle("/", fs)

	//http.HandleFunc("/reset", reset)

	//	http.HandleFunc("/ws", wsEndpoint) //web socket upgrader

	//this blocks the main thread
	go http.ListenAndServe(port, nil) //, customHeaders(fs))

	logit("Starting ticker")
	stepWorlds()

	//log.Fatal(http.ListenAndServeTLS(port, "dozer_world.crt", "./dozer.key", customHeaders(fs)))

}

func gameTraffic(w http.ResponseWriter, r *http.Request) {

	//logit("gameTraffic", "called")

	var block block
	// n, err := r.Body.Read(jsonBytes)

	w.Header().Set("Access-Control-Allow-Origin", "*") //TODO - tighten this up
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "*") //POST, OPTIONS")

	//logit(r.Method)
	if r.Method == "POST" {
		err := json.NewDecoder(r.Body).Decode(&block)

		if err != nil {
			logit(err.Error())
			w.Write([]byte("Error reading JSON (not a block) ?" + err.Error()))
			//w.WriteHeader(http.StatusBadRequest)
			return
		}

		//its possible there are 0 bytes in the request
		// if len(block) == 0 {
		// 	w.WriteHeader(http.StatusNoContent)
		// 	//nothing in the outbound queue for you
		// 	//w.response([]byte("{}"))
		// 	return
		// }
		//w.WriteHeader(http.StatusOK)

		response := processBlock(block)

		w.Write(response) //process the request que (from this player) and return the response (typically moved masses)
	}
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
