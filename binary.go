package main

import (
	"bytes"
	"encoding/binary"
)

func writeByte(buff *bytes.Buffer, b byte) {
	binary.Write(buff, le, b) //write byte
}

func writeString(buff *bytes.Buffer, s string) {
	binary.Write(buff, le, uint16(len(s))) //write the length of the string
	binary.Write(buff, le, []byte(s))      //write the string
}

func readString(buff *bytes.Buffer) string {
	sl := uint16(0)
	binary.Read(buff, le, &sl) //read the length of the string
	s := make([]byte, sl)
	binary.Read(buff, le, &s)
	return string(s)
}

func readUInt32(buff *bytes.Buffer) uint32 {
	var i uint32
	binary.Read(buff, le, &i)
	return i
}
