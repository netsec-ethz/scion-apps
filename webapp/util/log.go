package logs

import (
	log "github.com/inconshreveable/log15"
)

// CheckError handles Error logging
func CheckError(e error) bool {
	if e != nil {
		logError("Error:", "err", e)
	}
	return e != nil
}

// CheckFatal handles Fatal logging
func CheckFatal(e error) bool {
	if e != nil {
		logFatal("Fatal:", "err", e)
	}
	return e != nil
}

func logError(msg string, a ...interface{}) {
	log.Error(msg, a...)
}

func logFatal(msg string, a ...interface{}) {
	log.Crit(msg, a...)
}
