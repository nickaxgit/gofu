package main

import (
	"encoding/json"
	"fmt"
	dev "github.com/nickax/gofu/device"
	"github.com/nickax/gofu/errorplus"
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/global"
	"github.com/nickax/gofu/jsonmsg"
	"github.com/nickax/gofu/loader"
	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/server"

	"github.com/nickax/gofu/vec"
	//"log"

	"net/http"
	_ "net/http/pprof"
	"strings"

	"github.com/gorilla/websocket"
	//"golang.org/x/tools/playground/socket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func homePage(w http.ResponseWriter, _ *http.Request) {
	log.Logit("homePage called")
	fmt.Fprintf(w, "<h1>Game server</h1>")
	for _, v := range global.Devices {
		if v.WebSocket != nil {
			if v.ViewingPlayer == nil {
				fmt.Fprintf(w, "<p>Device %v (%v) has no player", v.Name, v.Id)
				continue
			}
			fmt.Fprintf(w, "<p>Device %v (%v) watches player %v (%v)", v.Name, v.Id, v.ViewingPlayer.Name, v.ViewingPlayer.Id)
		}

	}
}

func ValueOr[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
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
	fs := http.FileServer(http.Dir("../dozer")) //the typescript app/front end is in here

	//important!

	//obq = make(map[string]*qHolder)

	//see customHeaders
	http.HandleFunc("/gi", upgradeToWebSocketAndListenForever)
	//http.HandleFunc("/home", homePage)
	//http.Handle("/", fs)

	//http.HandleFunc("/reset", reset)

	//http.HandleFunc("/ws", wsEndpoint) //web socket upgrader

	errorplus.Log(loader.LoadAll("repo.bin"))
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

// return an html table of the headers
func tabulate(header http.Header) string {
	var sb strings.Builder
	sb.WriteString("<table>")
	for key, values := range header {
		sb.WriteString("<tr>")
		sb.WriteString(fmt.Sprintf("<td>%s</td>", key))
		sb.WriteString("<td>")
		for i, value := range values {
			sb.WriteString(value)
			if i < len(values)-1 {
				sb.WriteString(", ")
			}
		}
		sb.WriteString("</td>")
		sb.WriteString("</tr>")
	}
	sb.WriteString("</table>")
	return sb.String()
}

func upgradeToWebSocketAndListenForever(w http.ResponseWriter, r *http.Request) {

	// upgrade this connection to a WebSocket
	ws, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		erp := errorplus.New(err, errorplus.Error, "Websocket upgrade error")
		erp.Extra = tabulate(r.Header)
		errorplus.Log(erp)
		return
	}

	//initial anonymous/unknown device
	var device *dev.Device = nil

	for { // read in a messages forever on this socket (from this player)

		var evt *errorplus.Event = nil
		messageType, msgBytes, err := ws.ReadMessage()

		if err != nil {
			evt = errorplus.New(err, errorplus.Info, "Websocket read error")
			errorplus.Log(evt)
			break
		}

		response := msg.Empty()

		switch messageType {
		case websocket.BinaryMessage:

			//msgbytes is a slice of bytes

			m := msg.NewFromBytes(msgBytes)

			if device == nil {

				//this is where the socket is bound to the device and the device reference is set for all future requests on this socket
				//it also send the player their home screen
				evt, device = dev.ReConnect(m, global.Devices, global.Players, ws)

			} else {
				device.InMtx.Lock()
				//beware this (potentially) reassigns the device (from an anonymous -1) device to a kno
				evt, device = device.ProcessBinaryMsg(m, global.Games, global.Players, global.Devices)
				device.InMtx.Unlock()
			}

		case websocket.TextMessage:

			var jsonMessage jsonmsg.Msg
			err := json.Unmarshal(msgBytes, &jsonMessage)
			if err != nil {
				log.Logit(err.Error())
			}

			evt = device.ProcessStructuredMsg(&jsonMessage, response)
		default:
			log.Logit("Unknown message type:", messageType)
		}

		//there has been an error or something to log
		if evt != nil {
			var did, pid, gid, vid uint32

			velocity := (*vec.V3)(nil)
			if device != nil {
				did = device.Id

				if device.ViewingPlayer != nil {
					pid = device.ViewingPlayer.Id
					if device.ViewingPlayer.Game != nil {
						gid = device.ViewingPlayer.Game.Id
					}
					if device.ViewingPlayer.GetVehicle() != nil {
						vid = device.ViewingPlayer.GetVehicle().AssetId
						velocity = device.ViewingPlayer.GetVehicle().Om.GetVelocity()
					}
				}
			}
			evt.AddContext(gid, pid, did, vid, velocity)
			errorplus.Log(evt)
		}

	}

	if device != nil {
		device.ReleaseWebSocket()
	}

	//log.Logit("socket error/ended for", player.Name)
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
