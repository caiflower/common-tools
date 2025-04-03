package env

import (
	"fmt"
	"net"
	"os"
)

var (
	LocalhostIP string
	ConfigPath  string
	AESKey      string
)

func init() {
	findLocalHostIP()
	initConfigPath()
	initEnv()
}

func initConfigPath() {
	ConfigPath = os.Getenv("CONFIG_PATH")
	if ConfigPath == "" {
		ConfigPath = "./etc"
	}
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

func initEnv() {
	AESKey = os.Getenv("AES_KEY")
	if AESKey != "" && len(AESKey) != 32 && len(AESKey) != 24 && len(AESKey) != 16 {
		panic("invalid AES_KEY length, AES_KEY must 16 or 24 or 32")
	}
}

func GetLocalHostIP() string {
	return LocalhostIP
}

func SetDefaultConfigPath(path string) {
	ConfigPath = path
}
