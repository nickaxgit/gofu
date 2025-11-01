package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/game/player"
	"github.com/nickax/gofu/jsonmsg"
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/server"
	"github.com/nickax/gofu/viewer"

	"github.com/nickax/gofu/vec"
	//"log"

	"net/http"
	_ "net/http/pprof"
	"strings"

	"github.com/gorilla/websocket"
	//"golang.org/x/tools/playground/socket"
)

var globalOrigin = vec.NewVec3(0, 0, 0)

var le = binary.LittleEndian
var ntm = 0.000000001 //newtons to metres of movement per substep

var accountsByGuid map[string]*account

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func homePage(w http.ResponseWriter, _ *http.Request) {
	log.Logit("homePage called")
	fmt.Fprintf(w, "<h1>Dozer game server</h1>")
	for i, g := range server.Games {
		fmt.Fprintf(w, "<p>Game %s has %d players", i, len(g.Players))
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
	port := "127.0.0.1:8081" //":443" //":8081"
	log.Logit("Gofu server - listening on " + port)
	fs := http.FileServer(http.Dir("../dozer"))

	//important!

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

	go func() {
		log.Logit("pprof listening :6060")
		log.Fatal(http.ListenAndServe("localhost:6060", nil))
	}()

	log.Logit("Starting ticker")
	server.StepWorldsForever() //step the worlds every 33ms

}

func upgradeToWebSocketAndListenForever(w http.ResponseWriter, r *http.Request) {

	// upgrade this connection to a WebSocket
	ws, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Logit(err)
	}

	var viewer *viewer.Viewer //this is set processing a create/join/control message
	for {                     // read in a messages forever on this socket (from this player)

		messageType, msgBytes, err := ws.ReadMessage()

		if err != nil {
			log.Logit("Error reading message from websocket: " + err.Error())
			break
		}

		if viewer == nil {
			viewer := viewer.New(nil, 0, "", ws) //temporary viewer to hold the socket

			if messageType == websocket.TextMessage {
				if viewer != nil {
					var jsonMessage jsonmsg.Msg
					err := json.Unmarshal(msgBytes, &jsonMessage)
					if err != nil {
						log.Logit(err.Error())
					}

					player := server.GlobalPlayers[viewer.ViewingPlayerId]
					server.ProcessStructuredMsg(viewer, player, &jsonMessage, ws)
				} else {
					panic(errors.New("viewer is nil in text message processing"))

				}

			} else if messageType == websocket.BinaryMessage {
				//msgbytes is a slice of bytes

				m := msg.NewFromBytes(msgBytes)

				if viewer != nil {
					viewer.InMtx.Lock()
				}
				server.Games[player.GameId].ProcessBinaryMsg(m, player, server.Games)
				//player.ProcessBinaryMsg(m)
				player.InMtx.Unlock()

			}

		}

	}
	ws.Close()
	if ws == player.Socket {
		player.Socket = nil
	}
	if ws == player.ControllerSocket {
		player.ControllerSocket = nil
	}

	log.Logit("socket error/ended for", player.Name)
}

func customHeaders(fs http.Handler) http.HandlerFunc {
	// found at https://stackoverflow.com/a/65905091
	return func(w http.ResponseWriter, r *http.Request) {
		// add headers etc here
		// return if you do not want the FileServer handle a specific request

		if strings.HasSuffix(r.RequestURI, "/gi") {
			upgradeToWebSocketAndListenForever(w, r)
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
