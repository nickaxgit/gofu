package log

import (
	"fmt"
	"log"
)

func Logit(v ...interface{}) {
	fmt.Println(v...)
}

func Fatal(v ...interface{}) {
	log.Fatal(v...)
}
