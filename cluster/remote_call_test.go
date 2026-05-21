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
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/stretchr/testify/assert"
)

const (
	printData    = "printDataFunc"
	caseOfBytes  = "caseOfBytesResult"
	caseOfObject = "caseOfObjectResult"
)

func printDataFn(data interface{}) (interface{}, error) {
	logger.Info("data=%v", data)
	return data, nil
}

func getOtherNode(cluster ICluster) string {
	names := cluster.GetAliveNodeNames()
	return names[rand.Intn(len(names))]
}

func getSelfNode(c ICluster) string {
	return c.GetMyName()
}

func caseOfBytesResult(data any) (any, error) {
	return data, nil
}

type testUser struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func caseOfObjectResult(data any) (any, error) {
	return data, nil
}

func callOnLeader(t *testing.T, clusters []ICluster, fn func(ICluster)) {
	for _, c := range clusters {
		if c.IsLeader() {
			fn(c)
			return
		}
	}
	t.Fatal("no leader found")
}

func TestRemoteCall(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	cluster1.RegisterFunc(printData, printDataFn)
	cluster2.RegisterFunc(printData, printDataFn)
	cluster3.RegisterFunc(printData, printDataFn)
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	defer cluster1.Close()
	defer cluster2.Close()
	defer cluster3.Close()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	assert.Equal(t, cluster1.GetLeaderName(), cluster2.GetLeaderName(), "leader must same")
	assert.Equal(t, cluster1.GetLeaderName(), cluster3.GetLeaderName(), "leader must same")

	fmt.Println("----- Test sync CallFuncAs ------")
	callOnLeader(t, []ICluster{cluster1, cluster2, cluster3}, func(c ICluster) {
		result, err := CallFuncAs[string](c, NewFuncSpec(getOtherNode(c), printData, "testParam", time.Second*3).SetTraceId("myTraceId"))
		assert.Nil(t, err)
		assert.Equal(t, "testParam", result)
	})

	fmt.Println("----- Test async FuncSpec ------")
	var f *FuncSpec
	callOnLeader(t, []ICluster{cluster1, cluster2, cluster3}, func(c ICluster) {
		f = NewAsyncFuncSpec(getOtherNode(c), printData, "testAsyncParam", time.Second*5).SetTraceId("myAsyncTraceId")
		_, err := CallFuncAs[any](c, f)
		assert.Nil(t, err)
	})

	for {
		select {
		case <-time.After(time.Second * 5):
			fmt.Println("timeout")
			return
		default:
			result, err := GetResultAs[string](f)
			if errors.Is(err, ErrResultNotReady) {
				fmt.Println("no result, wait async result sleep.")
				time.Sleep(time.Millisecond * 50)
				continue
			}
			assert.Nil(t, err)
			assert.Equal(t, "testAsyncParam", result)
			return
		}
	}
}

func TestRemoteCallWithBytesResult(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	cluster1.RegisterFunc(caseOfBytes, caseOfBytesResult)
	cluster2.RegisterFunc(caseOfBytes, caseOfBytesResult)
	cluster3.RegisterFunc(caseOfBytes, caseOfBytesResult)
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	defer cluster1.Close()
	defer cluster2.Close()
	defer cluster3.Close()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	assert.Equal(t, cluster1.GetLeaderName(), cluster2.GetLeaderName(), "leader must same")
	assert.Equal(t, cluster1.GetLeaderName(), cluster3.GetLeaderName(), "leader must same")

	input := []byte{'i', 'l', 'u'}

	fmt.Println("----- Test sync CallFuncAs with []byte ------")
	callOnLeader(t, []ICluster{cluster1, cluster2, cluster3}, func(c ICluster) {
		actual, err := CallFuncAs[[]byte](c, NewFuncSpec(getOtherNode(c), caseOfBytes, input, time.Second*3).SetTraceId("myTraceId"))
		assert.Nil(t, err)
		assert.Equal(t, input, actual)
	})

	fmt.Println("----- Test async FuncSpec with []byte ------")
	var f *FuncSpec
	callOnLeader(t, []ICluster{cluster1, cluster2, cluster3}, func(c ICluster) {
		f = NewAsyncFuncSpec(getOtherNode(c), caseOfBytes, input, time.Second*5).SetTraceId("myAsyncTraceId")
		_, err := CallFuncAs[any](c, f)
		assert.Nil(t, err)
	})

	for {
		select {
		case <-time.After(time.Second * 5):
			fmt.Println("timeout")
			return
		default:
			actual, err := GetResultAs[[]byte](f)
			if errors.Is(err, ErrResultNotReady) {
				fmt.Println("no result, wait async result sleep.")
				time.Sleep(time.Millisecond * 50)
				continue
			}
			assert.Nil(t, err)
			assert.Equal(t, input, actual)
			return
		}
	}
}

func TestLocalCallWithObjectResult(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	cluster1.RegisterFunc(caseOfObject, caseOfObjectResult)
	cluster2.RegisterFunc(caseOfObject, caseOfObjectResult)
	cluster3.RegisterFunc(caseOfObject, caseOfObjectResult)
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	defer cluster1.Close()
	defer cluster2.Close()
	defer cluster3.Close()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	input := testUser{Name: "bob", Age: 25}

	fmt.Println("----- Test local sync CallFuncAs with object ------")
	for _, c := range []ICluster{cluster1, cluster2, cluster3} {
		actual, err := CallFuncAs[testUser](c, NewFuncSpec(getSelfNode(c), caseOfObject, input, time.Second*3))
		assert.Nil(t, err)
		assert.Equal(t, input, actual)
	}

	fmt.Println("----- Test local async FuncSpec with object ------")
	f := NewAsyncFuncSpec(getSelfNode(cluster1), caseOfObject, input, time.Second*5)
	_, err := CallFuncAs[any](cluster1, f)
	assert.Nil(t, err)

	for {
		select {
		case <-time.After(time.Second * 5):
			fmt.Println("timeout")
			return
		default:
			actual, err := GetResultAs[testUser](f)
			if errors.Is(err, ErrResultNotReady) {
				fmt.Println("no result, wait async result sleep.")
				time.Sleep(time.Millisecond * 50)
				continue
			}
			assert.Nil(t, err)
			assert.Equal(t, input, actual)
			return
		}
	}
}

func TestLocalCallWithBytesResult(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	cluster1.RegisterFunc(caseOfBytes, caseOfBytesResult)
	cluster2.RegisterFunc(caseOfBytes, caseOfBytesResult)
	cluster3.RegisterFunc(caseOfBytes, caseOfBytesResult)
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	defer cluster1.Close()
	defer cluster2.Close()
	defer cluster3.Close()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	input := []byte{'i', 'l', 'u'}

	fmt.Println("----- Test local sync CallFuncAs with []byte ------")
	for _, c := range []ICluster{cluster1, cluster2, cluster3} {
		actual, err := CallFuncAs[[]byte](c, NewFuncSpec(getSelfNode(c), caseOfBytes, input, time.Second*3))
		assert.Nil(t, err)
		assert.Equal(t, input, actual)
	}

	fmt.Println("----- Test local async FuncSpec with []byte ------")
	f := NewAsyncFuncSpec(getSelfNode(cluster1), caseOfBytes, input, time.Second*5)
	_, err := CallFuncAs[any](cluster1, f)
	assert.Nil(t, err)

	for {
		select {
		case <-time.After(time.Second * 5):
			fmt.Println("timeout")
			return
		default:
			actual, err := GetResultAs[[]byte](f)
			if errors.Is(err, ErrResultNotReady) {
				fmt.Println("no result, wait async result sleep.")
				time.Sleep(time.Millisecond * 50)
				continue
			}
			assert.Nil(t, err)
			assert.Equal(t, input, actual)
			return
		}
	}
}

func TestRemoteCallWithObjectResult(t *testing.T) {
	cluster1, cluster2, cluster3 := common()
	cluster1.RegisterFunc(caseOfObject, caseOfObjectResult)
	cluster2.RegisterFunc(caseOfObject, caseOfObjectResult)
	cluster3.RegisterFunc(caseOfObject, caseOfObjectResult)
	_ = cluster1.Start()
	_ = cluster2.Start()
	_ = cluster3.Start()
	defer cluster1.Close()
	defer cluster2.Close()
	defer cluster3.Close()
	waitAllForReady(t, cluster1, cluster2, cluster3)

	assert.Equal(t, cluster1.GetLeaderName(), cluster2.GetLeaderName(), "leader must same")
	assert.Equal(t, cluster1.GetLeaderName(), cluster3.GetLeaderName(), "leader must same")

	input := testUser{Name: "alice", Age: 30}

	fmt.Println("----- Test sync CallFuncAs with object ------")
	callOnLeader(t, []ICluster{cluster1, cluster2, cluster3}, func(c ICluster) {
		actual, err := CallFuncAs[testUser](c, NewFuncSpec(getOtherNode(c), caseOfObject, input, time.Second*3).SetTraceId("myTraceId"))
		assert.Nil(t, err)
		assert.Equal(t, input, actual)
	})

	fmt.Println("----- Test async FuncSpec with object ------")
	var f *FuncSpec
	callOnLeader(t, []ICluster{cluster1, cluster2, cluster3}, func(c ICluster) {
		f = NewAsyncFuncSpec(getOtherNode(c), caseOfObject, input, time.Second*5).SetTraceId("myAsyncTraceId")
		_, err := CallFuncAs[any](c, f)
		assert.Nil(t, err)
	})

	for {
		select {
		case <-time.After(time.Second * 5):
			fmt.Println("timeout")
			return
		default:
			actual, err := GetResultAs[testUser](f)
			if errors.Is(err, ErrResultNotReady) {
				fmt.Println("no result, wait async result sleep.")
				time.Sleep(time.Millisecond * 50)
				continue
			}
			assert.Nil(t, err)
			assert.Equal(t, input, actual)
			return
		}
	}
}
