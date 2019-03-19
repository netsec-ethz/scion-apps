package logs

import (
	log "github.com/inconshreveable/log15"
)

func CheckError(e error) bool {
	if e != nil {
		LogError("Error:", "err", e)
	}
	return e != nil
}

func CheckFatal(e error) bool {
	if e != nil {
		LogFatal("Fatal:", "err", e)
	}
	return e != nil
}

func LogError(msg string, a ...interface{}) {
	log.Error(msg, a...)
}

func LogFatal(msg string, a ...interface{}) {
	log.Crit(msg, a...)
}
