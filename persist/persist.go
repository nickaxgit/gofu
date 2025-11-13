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

	n, werr := file.Write(item.AllBytes())

	if werr != nil || n == 0 {
		return errorplus.New(werr, errorplus.Error, "Failed to append to file: "+filename)
	}

	return next.SaveCounters(filename + ".counters")

}
