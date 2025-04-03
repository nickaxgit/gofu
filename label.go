package main

import (
	"bytes"
	"encoding/binary"
	//"fmt"
)

type label struct {
	index           uint16
	text            string
	m1              *mass
	m2              *mass //the label will be displayed at the midpoint of these two massess - they can be the same (to label a mass)
	backgroundColor uint8
	voff            byte
	boundTo         *float64
}

func NewLabel(text string, m1 *mass, m2 *mass, backgroundColor uint8, voff byte, boundTo *float64) *label {
	return &label{index: 0, text: text, m1: m1, m2: m2, backgroundColor: backgroundColor, voff: voff, boundTo: boundTo}
}

func (l *label) toByteBuffer(buff *bytes.Buffer) {
	binary.Write(buff, le, l.index)

	binary.Write(buff, le, l.backgroundColor) //use bit7 to indicate the text has changed (is present)

	binary.Write(buff, le, l.m1.index)
	binary.Write(buff, le, l.m2.index)
	binary.Write(buff, le, l.voff) //vertical offset in 256/ths of a meter
	writeString(buff, l.text)
	binary.Write(buff, le, float32(*l.boundTo)) //write the value of the bound variable

}

func (l *label) bind(f *float64) {
	l.boundTo = f
}

// func updateLabels(state *state) {
// 	for _, l := range state.labels {
// 		if l.boundTo != nil {
// 			l.text = fmt.Sprintf("%f", *l.boundTo)
// 		}
// 	}

// }
