package health

import (
	"fmt"
	"net"
	"time"
)

// WaitForPort polls localhost:<port> with TCP connections every 1 second
// for up to the given timeout. Returns nil once a connection succeeds.
func WaitForPort(port int, timeout time.Duration) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("port %d not responding after %s", port, timeout)
}
