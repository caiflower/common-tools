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
	"github.com/stretchr/testify/assert"
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
}

func TestRemoteCall(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	cluster1.RegisterFunc(printData, printDataFn)
	cluster2.RegisterFunc(printData, printDataFn)
	cluster3.RegisterFunc(printData, printDataFn)
	defer cluster1.Close()
	defer cluster2.Close()
	defer cluster3.Close()

	// 等待leader选出来
	for {
		if cluster1.IsReady() && cluster2.IsReady() && cluster3.IsReady() {
			break
		}
	}
	assert.Equal(t, cluster1.GetLeaderName(), cluster2.GetLeaderName(), "leader must same")
	assert.Equal(t, cluster1.GetLeaderName(), cluster3.GetLeaderName(), "leader must same")

	// 同步调用
	fmt.Println("----- Test sync FuncSpec ------")
	if cluster1.IsLeader() {
		result, err := cluster1.CallFunc(NewFuncSpec(getOtherNode(cluster1), printData, "testParam", time.Second*3).SetTraceId("myTraceId"))
		assert.Nil(t, err)
		assert.Equal(t, result, "testParam")
	} else if cluster2.IsLeader() {
		result, err := cluster2.CallFunc(NewFuncSpec(getOtherNode(cluster2), printData, "testParam", time.Second*3).SetTraceId("myTraceId"))
		assert.Nil(t, err)
		assert.Equal(t, result, "testParam")
	} else if cluster3.IsLeader() {
		result, err := cluster3.CallFunc(NewFuncSpec(getOtherNode(cluster3), printData, "testParam", time.Second*3).SetTraceId("myTraceId"))
		assert.Nil(t, err)
		assert.Equal(t, result, "testParam")
	}

	// 异步调用
	fmt.Println("----- Test async FuncSpec ------")
	var f *FuncSpec
	if cluster1.IsLeader() {
		f = NewAsyncFuncSpec(getOtherNode(cluster1), printData, "testAsyncParam", time.Second*5).SetTraceId("myAsyncTraceId")
		_, err := cluster1.CallFunc(f)
		assert.Nil(t, err)
	} else if cluster2.IsLeader() {
		f = NewAsyncFuncSpec(getOtherNode(cluster2), printData, "testAsyncParam", time.Second*5).SetTraceId("myAsyncTraceId")
		_, err := cluster2.CallFunc(f)
		assert.Nil(t, err)
	} else if cluster3.IsLeader() {
		f = NewAsyncFuncSpec(getOtherNode(cluster3), printData, "testAsyncParam", time.Second*5).SetTraceId("myAsyncTraceId")
		_, err := cluster3.CallFunc(f)
		assert.Nil(t, err)
	}

	for {
		select {
		case <-time.After(time.Second * 5):
			fmt.Println("timeout")
			return
		default:
			result, err := f.GetResult()
			assert.Nil(t, err)
			if result != nil {
				assert.Equal(t, "testAsyncParam", result)
				return
			}
			fmt.Println("no result, wait async result sleep.")
			time.Sleep(time.Millisecond * 50)
		}
	}
}
