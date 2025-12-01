/*
 * Copyright 2024 caiflower Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package env

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	shell "github.com/caiflower/common-tools/pkg/shell"
)

var (
	LocalhostIP string
	LocalDNS    string
	Kubernetes  bool
	ConfigPath  string
	AESKey      string
	Replicas    int // 服务副本数，用于控制cluster
)

func init() {
	initConfigPath()
	initEnv()
	findLocalHostIP()
	findLocalDNS()
}

func initConfigPath() {
	ConfigPath = os.Getenv("CONFIG_PATH")
	if ConfigPath == "" {
		ConfigPath = "./etc"
	}
}

func findLocalHostIP() {
	if LocalhostIP == "" {
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
}

func findLocalDNS() {
	if LocalDNS == "" && Kubernetes {
		result, err := shell.Exec("cat", "/etc/hosts")
		if err == nil {
			hostLines := strings.Split(result.Stdout.String(), "\n")
			for _, hostLine := range hostLines {
				if strings.Contains(hostLine, "cluster.local") {
					strs := strings.Fields(hostLine)
					for _, str := range strs {
						if strings.Contains(str, "cluster.local") {
							LocalDNS = str
							return
						}
					}
				}
			}
		}
	}
}

func initEnv() {
	LocalDNS = os.Getenv("LOCAL_DNS")
	LocalhostIP = os.Getenv("LOCAL_HOST_IP")
	Replicas, _ = strconv.Atoi(os.Getenv("REPLICAS"))

	AESKey = os.Getenv("AES_KEY")
	if AESKey != "" && len(AESKey) != 32 && len(AESKey) != 24 && len(AESKey) != 16 {
		panic("invalid AES_KEY length, AES_KEY must 16 or 24 or 32")
	}

	result, err := shell.Exec("ls", "/var/run/secrets/kubernetes.io/serviceaccount")
	if err == nil && !strings.Contains(result.Stdout.String(), "No such file or directory") {
		Kubernetes = true
	}
}

func GetLocalHostIP() string {
	return LocalhostIP
}

func GetLocalDNS() string {
	return LocalDNS
}

func GetReplicas() int {
	return Replicas
}

func SetDefaultConfigPath(path string) {
	ConfigPath = path
}
