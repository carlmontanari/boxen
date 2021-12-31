package util

import (
	"log"
	"net"
)

// GetPreferredIP returns the IP address the host machine prefers for outbound requests.
func GetPreferredIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String()
}
