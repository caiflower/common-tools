package env

import (
	"fmt"
	"testing"
)

func TestName(t *testing.T) {
	ip := GetLocalHostIP()
	fmt.Println(ip)
}

func TestGetKubernetesDNS(t *testing.T) {
	fmt.Println(GetLocalDNS())
}
