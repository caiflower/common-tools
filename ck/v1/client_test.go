package v1

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
)

type Host struct {
	HostIp     string
	OsType     string
	OsVersion  string
	CreateTime time.Time
}

func TestNewClient(t *testing.T) {
	logger.InitLogger(&logger.Config{Level: "DEBUG"})
	client := NewClient(Config{
		User:     "root",
		Password: "",
		Url:      "ck.caiflower.cn:9004",
		DbName:   "host_meta",
		Debug:    true,
	})
	//// insert
	//host := Host{
	//	HostIp:     "127.0.0.1",
	//	OsType:     "centos7.2",
	//	OsVersion:  "CentOS Linux release 7.2.1512 (Core)",
	//	CreateTime: time.Now(),
	//}
	//if _, err := client.GetInsert(&host).Exec(context.Background()); err != nil {
	//	panic(err)
	//}

	// query
	var host1 []Host
	err := client.GetSelect(&host1).
		Where("os_type = ?", "centos7.2").
		Order("create_time desc").
		Scan(context.Background(), &host1)
	if err != nil {
		panic(err)
	}
	fmt.Println(host1)

	// truncate
	//err := client.TruncateTable(&Host{})
	//if err != nil {
	//	panic(err)
	//	return
	//}

	// drop
	//err := client.DropTable(&Host{})
	//if err != nil {
	//	panic(err)
	//	return
	//}

	defer client.Close()
}
