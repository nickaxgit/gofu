package sound

import (
	"time"

	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/vec"
)

type Sound struct {
	name      string
	handle    uint16
	position  vec.V3
	volume    float32
	loop      bool
	playAfter uint16 //parent sound handle
	startTime time.Time
	length    time.Duration
}

// New creates a new sound and adds it to the sounds slice, returning the sound created
func New(sounds []*Sound, name string, position vec.V3, volume float32, loop bool, playAfter *Sound, durationSeconds int) *Sound {

	handle := nextFreeSlotIn(sounds)

	pah := uint16(0)
	if playAfter != nil {
		pah = playAfter.handle
	}
	sound := &Sound{name: name, handle: handle, position: position, volume: volume, loop: loop,
		playAfter: pah, length: time.Duration(durationSeconds) * time.Second}

	sounds[handle] = sound
	return sound

}

func nextFreeSlotIn(sounds []*Sound) uint16 {
	for i, s := range sounds {
		if !s.loop {
			if time.Now().After(s.startTime.Add(s.length)) {
				return uint16(i)
			}
		}
	}
	sounds = append(sounds, nil)
	return uint16(len(sounds))

}

func (s *Sound) WriteInto(response *msg.Msg) {
	response.Write(msg.Sound, s.name, s.position, s.volume, s.loop, s.playAfter)
	//return msg.NewMsg(msg.Sound, s.name, s.position, s.volume, s.loop, s.playAfter)
}
