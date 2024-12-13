package main

type Prop struct {
	Position Vector  `json:"position"`
	Angle    float64 `json:"angle"`
	Radius   float64 `json:"radius"`
	Pic      string  `json:"pic"`
}

type Layer struct {
	Props     []Prop   `json:"props"`
	Name      string   `json:"name"`
	Pics      []string `json:"pics"`
	Extensiom string   `json:"extension"`
}

func NewLayer(name string, pics []string, extension string) *Layer {
	return &Layer{Name: name, Pics: pics, Extensiom: extension}
}
