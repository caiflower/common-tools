package v2

import (
	"fmt"
	"testing"

	"github.com/caiflower/common-tools/pkg/basic"
)

type Host struct {
	HostIp     string
	OsType     string
	OsVersion  string
	CreateTime basic.Time
}

func getClient() IClickHouseDB {
	client := NewClient(Config{
		User:     "root",
		Password: "",
		Urls:     []string{"ck.caiflower.cn:9005"},
		DbName:   "host_meta",
		Debug:    true,
	})

	return client
}

func TestNewClient(t *testing.T) {
	client := getClient()

	if query, err := client.GetDB().Query("SELECT * FROM `host`"); err != nil {
		panic(err)
	} else {
		var host Host
		for query.Next() {
			var hostIp, OsType, OsVersion string
			var CreateTime basic.Time
			err = query.Scan(&hostIp, &OsType, &OsVersion, &CreateTime)
			if err != nil {
				panic(err)
				return
			} else {
				host.HostIp = hostIp
				host.OsType = OsType
				host.OsVersion = OsVersion
				host.CreateTime = CreateTime
				fmt.Printf("hostIp= %v osType=%v osVersion=%v createTime=%s\n", hostIp, OsType, OsVersion, CreateTime.String())
			}
		}
	}
}

func TestModel(t *testing.T) {
	client := getClient()

	model := client.NewInsert().Model(&Host{})
	fmt.Println(model)

	model = client.NewInsert().Model(Host{})
	fmt.Println(model)

	model = client.NewInsert().Model([]*Host{})
	fmt.Println(model)

	model = client.NewInsert().Model([]Host{})
	fmt.Println(model)
}
