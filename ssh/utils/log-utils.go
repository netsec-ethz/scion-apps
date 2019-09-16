package utils

import (
	"log"
)

// Panicf is just a stub for log.Panicf
func Panicf(msg string, ctx ...interface{}) {
	log.Panicf(msg, ctx...)
}
