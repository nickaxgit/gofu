package next

import (
	"fmt"
	"os"
	"sync"

	"github.com/nickax/gofu/errorplus"
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/log"
)

var counters = make(map[string]uint32) //global named counters
var mutex = sync.Mutex{}

func Id(name string) uint32 {

	mutex.Lock()
	defer mutex.Unlock()
	val, present := counters[name]
	if !present {
		val = 1 //start at 1 - 0 is the nothing/nobody/nowhere id (see none.go files)
	}
	counters[name] = val + 1
	return val
}

func LoadCounters(filename string) *errorplus.Event {
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return errorplus.New(nil, errorplus.Warn, "Counters file does not exist, starting fresh: "+filename)
		}
		return errorplus.New(err, errorplus.Error, "Failed to open counters file for reading: "+filename)
	}
	defer file.Close()

	buff := make([]byte, 1000)

	n, err := file.Read(buff)
	if err != nil || n == 0 || n == len(buff) {
		return errorplus.New(err, errorplus.Error, fmt.Sprintf("Failed to read counters from file: %v, %v bytes read", filename, n))
	}

	cmsg := msg.NewFromBytes(buff[:n])
	if cmsg.MsgType != msg.P_Counters {
		return errorplus.New(nil, errorplus.Error, "Counters file does not appear to contain counters: "+filename)
	}

	var numCounters = uint32(0)
	cmsg.Read(&numCounters)

	if numCounters == 0 || numCounters > 100 {
		return errorplus.New(nil, errorplus.Error, fmt.Sprintf("Counters file has invalid number of counters: %v: %v", filename, numCounters))
	}

	defer mutex.Unlock()
	mutex.Lock()

	summary := ""
	for i := uint32(0); i < numCounters; i++ {
		var k string
		var v uint32
		cmsg.Read(&k, &v)
		summary += fmt.Sprintf("%v=%d ", k, v) + ","
		counters[k] = v
	}

	log.Logit("Counters loaded from file:", summary)
	return errorplus.New(nil, errorplus.Info, "Counters loaded from file: "+summary)

}

func SaveCounters(filename string) *errorplus.Event {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return errorplus.New(err, errorplus.Error, "Failed to open counters file for overwrite: "+filename)
	}
	defer file.Close()

	msg := msg.NewMsg(msg.P_Counters, uint32(len(counters)))

	mutex.Lock()
	for k, v := range counters {
		msg.Write(k, v)
	}
	mutex.Unlock()

	file.Write(msg.AllBytes())

	return nil

}
