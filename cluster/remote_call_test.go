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

 package cluster

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
)

const (
	printData = "printDataFunc"
)

func printDataFn(data interface{}) (interface{}, error) {
	logger.Info("data=%v", data)
	return data, nil
}

func getOtherNode(cluster ICluster) string {

	names := cluster.GetAliveNodeNames()
	return names[rand.Intn(len(names))]

	//for {
	//	name := names[rand.Intn(len(names))]
	//	if name != cluster.GetMyName() {
	//		return name
	//	}
	//}
}

func TestRemoteCall(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	cluster1.RegisterFunc(printData, printDataFn)
	cluster2.RegisterFunc(printData, printDataFn)
	cluster3.RegisterFunc(printData, printDataFn)

	// 等待leader选出来
	time.Sleep(3 * time.Second)

	// 同步调用
	fmt.Println("----- Test sync FuncSpec ------")
	if cluster1.IsLeader() {
		response, err := cluster1.CallFunc(NewFuncSpec(getOtherNode(cluster1), printData, "testParam", time.Second*3).SetTraceId("myTraceId"))
		if err != nil {
			fmt.Printf("err = %v \n", err)
		} else {
			fmt.Printf("response = %v \n", response)
		}
	} else if cluster2.IsLeader() {
		response, err := cluster2.CallFunc(NewFuncSpec(getOtherNode(cluster2), printData, "testParam", time.Second*3).SetTraceId("myTraceId"))
		if err != nil {
			fmt.Printf("err = %v \n", err)
		} else {
			fmt.Printf("response = %v \n", response)
		}
	} else if cluster3.IsLeader() {
		response, err := cluster3.CallFunc(NewFuncSpec(getOtherNode(cluster3), printData, "testParam", time.Second*3).SetTraceId("myTraceId"))
		if err != nil {
			fmt.Printf("err = %v \n", err)
		} else {
			fmt.Printf("response = %v \n", response)
		}
	}

	// 异步调用
	fmt.Println("----- Test async FuncSpec ------")
	var f *FuncSpec
	if cluster1.IsLeader() {
		f = NewAsyncFuncSpec(getOtherNode(cluster1), printData, "testAsyncParam", time.Second*3).SetTraceId("myAsyncTraceId")
		response, err := cluster1.CallFunc(f)
		if err != nil {
			fmt.Printf("err = %v \n", err)
		} else {
			fmt.Printf("response = %v \n", response)
		}
	} else if cluster2.IsLeader() {
		f = NewAsyncFuncSpec(getOtherNode(cluster2), printData, "testAsyncParam", time.Second*3).SetTraceId("myAsyncTraceId")
		response, err := cluster2.CallFunc(f)
		if err != nil {
			fmt.Printf("err = %v \n", err)
		} else {
			fmt.Printf("response = %v \n", response)
		}
	} else if cluster3.IsLeader() {
		f = NewAsyncFuncSpec(getOtherNode(cluster3), printData, "testAsyncParam", time.Second*3).SetTraceId("myAsyncTraceId")
		response, err := cluster3.CallFunc(f)
		if err != nil {
			fmt.Printf("err = %v \n", err)
		} else {
			fmt.Printf("response = %v \n", response)
		}
	}

	for {
		select {
		case <-time.After(time.Second * 5):
			fmt.Println("timeout")
			return
		default:
			result, err := f.GetResult()
			if err != nil {
				fmt.Printf("err = %v \n", err)
				return
			}
			if result != nil {
				fmt.Printf("result = %v \n", result)
				return
			}
			fmt.Println("no result, sleep.")
			time.Sleep(time.Millisecond * 50)
		}
	}
}
