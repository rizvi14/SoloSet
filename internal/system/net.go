package system

import (
	"net"
	"time"
)

// PortInUse reports whether something is already accepting connections on the
// given host:port. Used to catch the case where a non-SoloSet process is
// holding Superset's port before we try to publish it.
func PortInUse(hostPort string) bool {
	conn, err := net.DialTimeout("tcp", hostPort, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
