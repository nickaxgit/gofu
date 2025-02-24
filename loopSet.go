package main

import (
	"slices"
)

type loopSet struct {
	loops []*loop
}

func newLoopSet() *loopSet {
	return &loopSet{loops: []*loop{}}
}

func (ls *loopSet) append(l *loop) {
	ls.loops = append(ls.loops, l)
}

func (ls *loopSet) merge() *loop {
	compound := ls.loops[0].clone()
	for l := 1; l < len(ls.loops); l++ {
		//loops[l].reverse()
		compound.merge(ls.loops[l])
	}

	return compound
}

func (ls *loopSet) contains(vi uint16) bool {
	for _, l := range ls.loops {
		if slices.Contains(l.vi, vi) {
			return false
		}
	}
	return true
}
