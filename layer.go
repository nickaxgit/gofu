package main

type Prop struct {
	position Vector
	angle    float64
	radius   float64
	pic      string
}

type Layer struct {
	props     []Prop
	name      string
	pics      []string
	extension string
}

func NewLayer(name string, pics []string, extension string) *Layer {
	return &Layer{name: name, pics: pics, extension: extension}
}
