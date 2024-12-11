package v1

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type Host struct {
	HostIp     string
	OsType     string
	OsVersion  string
	CreateTime time.Time
}

func TestNewClient(t *testing.T) {
	client := NewClient(Config{
		User:     "root",
		Password: "",
		Url:      "ck.caiflower.cn:9004",
		DbName:   "host_meta",
	})
	//
	//host := Host{
	//	HostIp:     "127.0.0.2",
	//	OsType:     "centos7.2",
	//	OsVersion:  "CentOS Linux release 7.2.1512 (Core)",
	//	CreateTime: time.Now(),
	//}

	//if _, err := client.db.NewInsert().Model(&host).Exec(context.Background()); err != nil {
	//	panic(err)
	//}

	host1 := []Host{}
	err := client.db.NewSelect().Model(&host1).
		Where("os_type = ?", "centos7.2").
		Order("create_time desc").
		Scan(context.Background(), &host1)
	if err != nil {
		panic(err)
	}
	fmt.Println(host1)

	defer client.db.Close()
}
