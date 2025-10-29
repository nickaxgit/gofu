package log

import (
	"log"
)

import (
	"fmt"
)

func Logit(v ...interface{}) {
	fmt.Println(v...)
}

func Fatal(v ...interface{}) {
	log.Fatal(v...)
}
