package errorplus

import "time"
import "sync"
import "runtime/debug"
import "github.com/nickax/gofu/vec"

var log = make([]*Event, 0)
var mutex = sync.Mutex{}

type Severity int

const (
	Info Severity = iota
	Warn
	Error
	Critical
)

type Event struct {
	Err      error
	Severity Severity
	Msg      string
	gid      int32   //game id
	pid      int32   //player id
	did      int32   //device id
	vid      int32   //vehicle id - index (in global assets)
	velocity *vec.V3 //velocity at time of error/event
	time     time.Time
	stack    []byte
}

func New(err error, severity Severity, msg string) *Event {
	se := &Event{
		Err:      err,
		Severity: severity,
		Msg:      msg,
		gid:      -1,
		pid:      -1,
		did:      -1,
		vid:      -1,

		time:  time.Now(),
		stack: nil,
	}

	if err != nil {
		se.stack = debug.Stack()
	}

	return se
}

func Log(e *Event) {
	if e != nil {
		mutex.Lock()
		defer mutex.Unlock()
		log = append(log, e)
	}
}

func (e *Event) AddContext(gid int32, pid int32, did int32, vid int32, velocity *vec.V3) {
	e.gid = gid
	e.pid = pid
	e.did = did
	e.vid = vid
	e.velocity = velocity

}
