// +build go1.18

package v1

import (
	"fmt"
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/uptrace/go-clickhouse/ch"
)

type Host struct {
	HostIp     string
	OsType     string
	OsVersion  string
	CreateTime time.Time
}

type TestDataFiler struct {
}

func (f *TestDataFiler) GetPage() (offset int, limit int, disable bool) {
	return 1, 1, false
}

func (f *TestDataFiler) Filter(db *ch.DB) *ch.SelectQuery {
	return db.NewSelect().Model(&Host{}).Where("os_version = ?", "CentOS Linux release 7.2.1512 (Core)")
}

func (f *TestDataFiler) GetOrders() []string {
	return nil
}

func TestNewClient(t *testing.T) {
	logger.InitLogger(&logger.Config{Level: "DEBUG"})
	client := NewClient(Config{
		User:     "root",
		Password: "",
		Url:      "ck.caiflower.cn:9005",
		DbName:   "host_meta",
		Debug:    true,
	})
	//// insert
	var hosts []Host
	hosts = append(hosts, Host{
		HostIp:     "127.0.0.3",
		OsType:     "centos7.2",
		OsVersion:  "CentOS Linux release 7.2.1512 (Core)",
		CreateTime: time.Now(),
	})
	hosts = append(hosts, Host{
		HostIp:     "127.0.0.4",
		OsType:     "centos7.2",
		OsVersion:  "CentOS Linux release 7.2.1512 (Core)",
		CreateTime: time.Now(),
	})
	if cnt, err := client.Insert(&hosts); err != nil {
		panic(err)
	} else {
		fmt.Printf("insert cnt = %d\n", cnt)
	}

	// queryPage
	var hostPage []Host
	cnt, err := client.QueryPage(&hostPage, &TestDataFiler{})
	if err != nil {
		return
	}
	fmt.Printf("count = %d, hosts=%v\n", cnt, hostPage)

	// queryAll
	//var hosts []Host
	//all, err := client.QueryAll(&hosts)
	//if err != nil {
	//	return
	//}
	//fmt.Printf("count = %d, hosts=%v\n", all, hosts)

	// query
	//var host1 []Host
	//err := client.GetSelect(&host1).
	//	Where("os_type = ?", "centos7.2").
	//	Order("create_time desc").
	//	Scan(context.Background(), &host1)
	//if err != nil {
	//	panic(err)
	//}
	//fmt.Println(host1)

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
