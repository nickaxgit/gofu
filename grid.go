package main

import "bytes"
import "encoding/binary"

type grid struct {
	origin *vec3 //defines the plane of this players construction grid
	Xaxis  *vec3 //defines the plane of this players construction grid
	Yaxis  *vec3 //defines the plane of this players construction grid
}

func (g *grid) send(p *player) {
	p.sendBytes(g.toBytes())
	// buff := new(bytes.Buffer)
	// e := binary.LittleEndian
	// binary.Write(buff, e, byte(msgGrid))
	// g.origin.toByteBuffer(buff)
	// g.Xaxis.toByteBuffer(buff)
	// g.Yaxis.toByteBuffer(buff)
	// p.sendBytes(buff.Bytes())
}

func (g *grid) normal() *vec3 {
	return g.Yaxis.cross(g.Xaxis).normalise()
}

func (g *grid) updateGridPosAndSpacePos(p *player) {
	gn := g.normal()

	offsetPlane := p.grid.origin.add(gn.multiply(p.zOff))
	c2g := p.camera.position.closestPointOnPlane(offsetPlane, gn).sub(p.camera.position) //vector from the camera pos to the grid
	ttg := p.camera.farPos.closestPointOnPlane(offsetPlane, gn).sub(p.camera.farPos)

	if c2g.dot(ttg) < 0 {
		d0 := c2g.length()
		d1 := ttg.length()
		f := d0 / (d0 + d1)
		p.spacePos = p.camera.position.tween(p.camera.farPos, f)
		p.gridPos = p.spacePos.sub(gn.multiply(p.zOff))
	}

}

func (g *grid) fromByteBuffer(buff *bytes.Buffer, e binary.ByteOrder) {

	msg := byte(0)
	binary.Read(buff, e, &msg)
	if msgEnum(msg) != msgGrid {
		panic("grid not next")
	}
	g.origin.fromByteBuffer(buff)
	g.Xaxis.fromByteBuffer(buff)
	g.Yaxis.fromByteBuffer(buff)

}

func (g *grid) toBytes() []byte {

	buff := new(bytes.Buffer)
	binary.Write(buff, le, byte(msgGrid))

	g.origin.toByteBuffer(buff)
	g.Xaxis.toByteBuffer(buff)
	g.Yaxis.toByteBuffer(buff)

	return buff.Bytes()

}
