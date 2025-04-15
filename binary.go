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

func writeVec3(buff *bytes.Buffer, v *vec3) {
	binary.Write(buff, le, float32(v.x)) //write x
	binary.Write(buff, le, float32(v.y)) //write y
	binary.Write(buff, le, float32(v.z)) //write z
}

func writeFloat32(buff *bytes.Buffer, f float32) {
	binary.Write(buff, le, &f) //write float32
}

func writeBool(buff *bytes.Buffer, b bool) {
	var i byte
	if b {
		i = 1
	} else {
		i = 0
	}
	binary.Write(buff, le, &i) //write byte
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
