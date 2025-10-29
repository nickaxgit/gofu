package jsonmsg

// these must be upper cased or theu don't get unmarshalled
type Msg struct {
	Cmd     string    `json:"cmd"`
	Key     string    `json:"key"`
	Payload []float64 `json:"payload"`
}

// these must be uppercased for marshalling
type Reply struct {
	Cmd     string      `json:"cmd"`
	Payload interface{} `json:"payload"`
}
