package persist

import (
	"github.com/nickax/gofu/errorplus"
	"github.com/nickax/gofu/game/msg"
	"github.com/nickax/gofu/next"
	"os"
)

func Append(filename string, item *msg.Msg) *errorplus.Event {

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errorplus.New(err, errorplus.Error, "Failed to open file for appending: "+filename)
	}
	defer file.Close()

	file.Write(item.AllBytes())

	next.SaveCounters(filename + ".counters")
	return nil
}
