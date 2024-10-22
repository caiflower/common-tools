package env

import (
	"fmt"
	"net"
	"os"
)

var (
	LocalhostIP string
)

func init() {
	findLocalHostIP()
}

func findLocalHostIP() {
	addrs, err := net.InterfaceAddrs()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				LocalhostIP = ipnet.IP.String()
			}
		}
	}
}

func GetLocalHostIP() string {
	return LocalhostIP
}
