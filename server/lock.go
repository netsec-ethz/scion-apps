package server

import "time"

type lock chan struct{}

func makeLock() lock {
	return make(lock, 1)
}

func (lock lock) unlock() {
	<-lock
}

func (lock lock) tryLockTimeout(timeout time.Duration) bool {
	select {
	case lock <- struct{}{}:
		return true
	case <-time.After(timeout):
		return false
	}
}
