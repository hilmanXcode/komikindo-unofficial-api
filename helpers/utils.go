package helpers

import (
	"net"
	"time"
)

func CheckKomikindoConnection() bool {
	timeout := 1 * time.Second
	_, err := net.DialTimeout("tcp", "komikindo.ch:8080", timeout)

	if err != nil {
		// log.Fatal("Site unreachable, error: ", err)
		return false
	}

	return true
}
