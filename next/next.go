package next

var counters = make(map[string]uint32) //global named counters

func Id(name string) uint32 {
	val, present := counters[name]
	if !present {
		val = 0
	}
	counters[name] = val + 1
	return val
}
