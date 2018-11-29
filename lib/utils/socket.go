package utils

// GetSCIOND returns the path to the default SCION socket
func GetSCIOND() string {
	return "/run/shm/sciond/default.sock"
}

// GetDispatcher returns the path to the default SCION dispatcher
func GetDispatcher() string {
	return "/run/shm/dispatcher/default.sock"
}
