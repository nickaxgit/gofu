package main

import "encoding/binary"
import "bytes"

type camera struct {
	position  *vec3
	direction *vec3
	up        *vec3
	farPos    *vec3
}

func (c *camera) clone() *camera {
	return &camera{position: c.position.clone(), direction: c.direction.clone(), up: c.up.clone(), farPos: c.farPos.clone()}

}

func (cam *camera) follow(t *thing) {

	o := t.springs[0].m2.p

	fl := t.springs[1].m2.p
	//xa:=  o.sub(m[t.springs[0].m1].P)
	za := fl.sub(o).normalise()

	cam.position = o.sub(za.multiply(50))
	// leaf := p.landTri.vProbe(newVec3(p.camera.position.x, 0, p.camera.position.z))

	// if leaf != nil {

	//cp = leaf.probePlane(newVec3(cp.x, -10000, cp.z), newVec3(cp.x, 10000, cp.z))
	cam.position.y = o.y + 10
	//cp.y = 2000

	if !o.equals(cam.position) {
		cam.direction = o.sub(cam.position).normalise()
	}

}

func (c *camera) fromByteBuffer(buff *bytes.Buffer, e binary.ByteOrder) {

	msg := byte(0)
	binary.Read(buff, e, &msg)
	if msgEnum(msg) != msgCamera {
		panic("camera not next")
	}
	c.position.fromByteBuffer(buff)
	c.direction.fromByteBuffer(buff)
	c.up.fromByteBuffer(buff)

}

func (c *camera) toBytes() []byte {

	buff := new(bytes.Buffer)
	binary.Write(buff, le, byte(msgCamera))

	c.position.toByteBuffer(buff)
	c.direction.toByteBuffer(buff)
	c.up.toByteBuffer(buff)

	return buff.Bytes()

}
