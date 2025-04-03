package main

import (
	"bytes"
	"encoding/binary"
)

type boundValue struct {
	id           string
	valuePointer *float64 //the value we will be changing
	min          float32
	max          float32
	step         float32
	//	mass         *mass //which mass needs to be updated when this value changes
	labelSet byte //0 or an index to a if this is an enumeration, this is the number of values
	//conversion   float32 //values will be
}

func (bv *boundValue) toByteBuffer(buff *bytes.Buffer, e binary.ByteOrder) {

	binary.Write(buff, e, byte(len(bv.id)))
	binary.Write(buff, e, []byte(bv.id))
	binary.Write(buff, e, bv.min)
	binary.Write(buff, e, bv.max)
	binary.Write(buff, e, bv.step)
	binary.Write(buff, e, bv.labelSet)

	binary.Write(buff, e, float32(*bv.valuePointer)) //send the actual value
}
