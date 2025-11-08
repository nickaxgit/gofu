package errorplus

import "time"
import "sync"
import "runtime/debug"
import "github.com/nickax/gofu/vec"
import "fmt"

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
	gid      uint32  //game id
	pid      uint32  //player id
	did      uint32  //device id
	vid      uint32  //vehicle id - index (in global assets)
	velocity *vec.V3 //velocity at time of error/event
	time     time.Time
	Extra    string
	stack    []byte
}

func New(err error, severity Severity, msg string) *Event {
	se := &Event{
		Err:      err,
		Severity: severity,
		Msg:      msg,
		gid:      0,
		pid:      0,
		did:      0,
		vid:      0,
		Extra:    "",
		time:     time.Now(),
		stack:    nil,
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
		printOut(e.Msg, e.Err, e.pid, e.did)
	}
}

func (e *Event) AddContext(gid uint32, pid uint32, did uint32, vid uint32, velocity *vec.V3) {
	e.gid = gid
	e.pid = pid
	e.did = did
	e.vid = vid
	e.velocity = velocity

}

func printOut(v ...any) {

	fmt.Println(v...)

}
