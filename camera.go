package main

import "encoding/binary"
import "bytes"

type camera struct {
	position  *Vec3
	direction *Vec3
	up        *Vec3
	farPos    *Vec3
}

func (c *camera) readBinary(buff *bytes.Buffer, e binary.ByteOrder) {

	c.position.fromByteBuffer(buff, e)
	c.direction.fromByteBuffer(buff, e)
	c.up.fromByteBuffer(buff, e)

}

func (c *camera) toBytes(e binary.ByteOrder) []byte {

	buff := new(bytes.Buffer)
	binary.Write(buff, e, byte(msgCamera))

	c.position.toByteBuffer(buff, e)
	c.direction.toByteBuffer(buff, e)
	c.up.toByteBuffer(buff, e)

	return buff.Bytes()
}
