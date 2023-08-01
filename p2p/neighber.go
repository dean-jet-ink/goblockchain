package p2p

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"time"
)

func IsFoundHost(host string, port uint16) bool {
	target := fmt.Sprintf("%s:%d", host, port)

	if _, err := net.DialTimeout("tcp", target, time.Second*1); err != nil {
		return false
	}

	return true
}

var pattern = regexp.MustCompile(`^(((25[0-5]|2[0-4][0-9]|[10]?[0-9]?[0-9])\.){3})(25[0-5]|2[0-4][0-9]|[10]?[0-9]?[0-9])$`)

func FindNeighbors(myHost string, myPort uint16, ipStart, ipEnd uint8, portStart, portEnd uint16) []string {
	myAddress := fmt.Sprintf("%s:%d", myHost, myPort)

	m := pattern.FindStringSubmatch(myHost)
	if m == nil {
		fmt.Println("not match")
		return nil
	}

	prefixHost := m[1]
	lastIp, _ := strconv.Atoi(m[len(m)-1])
	var neighbors = make([]string, 0)

	for ip := ipStart; ip <= ipEnd; ip++ {
		for port := portStart; port <= portEnd; port++ {
			host := fmt.Sprintf("%s%d", prefixHost, lastIp+int(ip))
			target := fmt.Sprintf("%s:%d", host, port)

			if myAddress != target && IsFoundHost(host, port) {
				neighbors = append(neighbors, target)
			}
		}
	}

	return neighbors
}

func GetHost() string {
	hostName, err := os.Hostname()
	if err != nil {
		return "127.0.0.1"
	}

	address, err := net.LookupHost(hostName)
	if err != nil {
		return "127.0.0.1"
	}
	return address[len(address)-1]
}
