package html

import (
	"github.com/nickax/gofu/game/msg"
)

func InputBox(response *msg.Msg, id string, label string) {
	response.Write(`<input type=text id="` + id + `" placeholder="` + label + `">`)
}

func Button(response *msg.Msg, id string, label string, script string) {
	response.Write(`<button id="` + id + `" onclick="` + script + `">` + label + `</button>`)
}

func Literal(response *msg.Msg, content string) { //just to keep calling blocks cleaner
	response.Write(content)
}

func TableHead(response *msg.Msg, cols ...string) {

	response.Write("<tr>")
	for _, c := range cols {
		response.Write("<th>", c, "</th>")
	}
	response.Write("</tr>")

}

func TableRow(msg *msg.Msg, cols ...string) {
	msg.Write("<tr>")
	for _, c := range cols {
		msg.Write("<td>", c, "</td>")
	}
	msg.Write("</tr>")
}
