package plant

import (
	"math"
)

type Species byte

const (
	Pine     Species = 1
	Grass    Species = 2
	Oak      Species = 3
	Willow   Species = 4
	Maple    Species = 5
	Knotweed Species = 6
	Bamboo   Species = 7
)

type Plant struct {
	Species     Species
	BcU         byte //barycentric U
	BcV         byte //barycentric V
	Rotation    byte
	Scale       byte //encoded/stored transportedorted as a single byte (0-255) representing 0.01 to 100.0 scale (logarithmically)
	Ycorrection int8 //as the terrain is split further - the plant may need to be moved up or down to sit on the surface
}

func New(u, v float64, species Species, scale float64, rotation byte) Plant {
	return Plant{
		Species:  species,
		BcU:      byte(u * 255),
		BcV:      byte(v * 255),
		Scale:    EncodeScale(scale), //store scale in a singe byte (logarithmically) 1.0 maps to 1.0 range 0.01 to 100.0
		Rotation: rotation,
	}
}

func EncodeScale(scale float64) byte {

	s := math.Min(100, math.Max(0.01, float64(scale)))
	v := math.Log10(s)               // [-2, 2]
	b := math.Round(128 + 127*(v/2)) // center 128 at v=0
	// Keep within 1..255 to avoid slight overshoot at extreme rounding
	return byte(math.Max(1, math.Min(255, b)))
}

func DecodeScale(b byte) float64 {
	v := (float64(b) - 128) * (2.0 / 127.0) // [-2, 2]
	s := math.Pow(10, v)
	return s
}
