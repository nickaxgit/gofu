package label

import (
	"github.com/nickax/gofu/fiz/mass"
	"github.com/nickax/gofu/game/msg"
	//"fmt"
)

type Label struct {
	index           uint16
	text            string
	m1              *mass.Mass
	m2              *mass.Mass //the label will be displayed at the midpoint of these two massess - they can be the same (to label a mass)
	backgroundColor uint8
	voff            byte
	boundTo         *float64 //the address of a float64 to bind the label to (so it updates as the value changes)
}

func New(labels []*Label, text string, m1 *mass.Mass, m2 *mass.Mass, backgroundColor uint8, voff byte, boundTo *float64) *Label {
	labels = append(labels, &Label{index: uint16(len(labels)), text: text, m1: m1, m2: m2, backgroundColor: backgroundColor, voff: voff, boundTo: boundTo})
	return labels[len(labels)-1]
}

func (l *Label) WriteTo(msg *msg.Msg) {
	msg.Write(uint16(l.index), l.backgroundColor, l.m1.Index, l.m2.Index, l.voff, l.text)
}
func (l *Label) bind(f *float64) {
	l.boundTo = f
}
