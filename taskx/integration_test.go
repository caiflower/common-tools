package taskx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caiflower/common-tools/taskx/executor"
	"github.com/stretchr/testify/assert"
)

// ===== 使用方式一：使用泛型 LocalExecutor 注册（推荐） =====

// MyTaskExecutor 实现 TaskExecutor 接口
type MyTaskExecutor struct {
	completedCount int32
}

func (e *MyTaskExecutor) Name() string                      { return "MyTaskExecutor" }
func (e *MyTaskExecutor) FinishedTask(data *TaskData) error { return nil }
func (e *MyTaskExecutor) FailedTask(data *TaskData) error   { return nil }
func (e *MyTaskExecutor) IncrementCompleted()               { atomic.AddInt32(&e.completedCount, 1) }
func (e *MyTaskExecutor) GetCompletedCount() int32          { return atomic.LoadInt32(&e.completedCount) }

// TestIT_LocalExecutor_RecommendedWay 展示推荐的注册方式：使用泛型 LocalExecutor
func TestIT_LocalExecutor_RecommendedWay(t *testing.T) {
	exec := &MyTaskExecutor{}
	task := NewTask("user-workflow")

	// ===== 定义步骤函数 =====
	step1Fn := func(ctx context.Context, input struct {
		RequestID string `json:"requestId"`
	}) (struct {
		UserID   string `json:"userId"`
		Username string `json:"username"`
	}, error) {
		// 查询用户
		return struct {
			UserID   string `json:"userId"`
			Username string `json:"username"`
		}{UserID: "u123", Username: "alice"}, nil
	}

	step2Fn := func(ctx context.Context, input struct {
		UserID string `json:"userId"`
	}) (struct {
		OrderIDs []string `json:"orderIds"`
		Total    float64  `json:"total"`
	}, error) {
		// 查询订单
		return struct {
			OrderIDs []string `json:"orderIds"`
			Total    float64  `json:"total"`
		}{OrderIDs: []string{"o1", "o2"}, Total: 299.9}, nil
	}

	step3Fn := func(ctx context.Context, input struct {
		Username string  `json:"username"`
		Total    float64 `json:"total"`
	}) (struct {
		Message string `json:"message"`
	}, error) {
		return struct {
			Message string `json:"message"`
		}{Message: fmt.Sprintf("%s 的订单总额: %.2f", input.Username, input.Total)}, nil
	}

	// ===== 创建子任务（同时绑定执行器） =====
	step1 := NewSubtask("queryUser").SetExecutor(executor.NewLocalExecutor(step1Fn))
	step2 := NewSubtask("queryOrders").SetExecutor(executor.NewLocalExecutor(step2Fn))
	step3 := NewSubtask("sendNotification").SetExecutor(executor.NewLocalExecutor(step3Fn))

	_ = task.AddSubtask(step1)
	_ = task.AddSubtask(step2)
	_ = task.AddSubtask(step3)

	// ===== 构建 DAG =====
	// step1 -> step2
	// step1 -> step3
	// step2 -> step3 (step3 等待 step1 和 step2 完成)
	_ = task.AddEdge(step1, step2)
	_ = task.AddEdge(step1, step3)
	_ = task.AddEdge(step2, step3)

	// ===== 编译 DAG =====
	_, err := task.Compile()
	assert.Nil(t, err)

	// ===== 注册任务回调（执行器已在 AddSubtask 时自动注册） =====
	task.RegisterTaskExecutor(exec)

	// ===== 模拟执行流程 =====
	// step1 先执行
	next := task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "queryUser", next[0].GetName())

	// 获取执行器并执行 step1
	provider := step1.GetExecutor()
	assert.NotNil(t, provider)
	assert.Equal(t, executor.ProtocolLocal, provider.Protocol())

	input1 := struct {
		RequestID string `json:"requestId"`
	}{RequestID: "req-001"}
	input1JSON, _ := json.Marshal(input1)
	result1, err := provider.Execute(context.Background(), &executor.TaskData{
		RequestId: "req-001",
		SubTaskId: step1.GetID(),
		Input:     string(input1JSON),
	})
	assert.Nil(t, err)

	// step1 完成后更新状态，通知后继
	_ = task.UpdateSubtaskState(step1.GetID(), NodeSucceeded)
	step1Output := result1.(struct {
		UserID   string `json:"userId"`
		Username string `json:"username"`
	})

	// step2 现在可执行（step1 完成）；step3 不可执行（等待 step2 完成）
	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "queryOrders", next[0].GetName())

	// 执行 step2
	provider2 := step2.GetExecutor()
	input2 := struct {
		UserID string `json:"userId"`
	}{UserID: step1Output.UserID}
	input2JSON, _ := json.Marshal(input2)
	result2, err := provider2.Execute(context.Background(), &executor.TaskData{
		RequestId: "req-001",
		SubTaskId: step2.GetID(),
		Input:     string(input2JSON),
	})
	assert.Nil(t, err)
	_ = task.UpdateSubtaskState(step2.GetID(), NodeSucceeded)

	// 执行 step3
	provider3 := step3.GetExecutor()
	input3 := struct {
		Username string  `json:"username"`
		Total    float64 `json:"total"`
	}{Username: step1Output.Username, Total: result2.(struct {
		OrderIDs []string `json:"orderIds"`
		Total    float64  `json:"total"`
	}).Total}
	input3JSON, _ := json.Marshal(input3)
	result3, err := provider3.Execute(context.Background(), &executor.TaskData{
		RequestId: "req-001",
		SubTaskId: step3.GetID(),
		Input:     string(input3JSON),
	})
	assert.Nil(t, err)
	_ = task.UpdateSubtaskState(step3.GetID(), NodeSucceeded)

	// 验证最终结果
	finalResult := result3.(struct {
		Message string `json:"message"`
	})
	assert.Contains(t, finalResult.Message, "alice")
	assert.Contains(t, finalResult.Message, "299.90")

	// 任务完成
	assert.True(t, task.IsFinished())
}

// ===== 使用方式二：HTTP 执行器 + 本地函数混合 =====

// TestIT_MixedProtocol_HTTPPAndLocal 展示混合使用 HTTP 和本地函数执行器
func TestIT_MixedProtocol_HTTPAndLocal(t *testing.T) {
	exec := &MyTaskExecutor{}
	task := NewTask("api-workflow")

	// 模拟外部 API
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/u456" {
			json.NewEncoder(w).Encode(map[string]any{
				"userId":   "u456",
				"username": "bob",
				"email":    "bob@example.com",
			})
		} else if r.URL.Path == "/validate" {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if email, ok := body["email"].(string); ok && strings.Contains(email, "@") {
				json.NewEncoder(w).Encode(map[string]any{"valid": true})
			} else {
				json.NewEncoder(w).Encode(map[string]any{"valid": false})
			}
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer apiServer.Close()

	// 步骤2：本地验证数据
	step2Fn := func(ctx context.Context, input struct {
		Email string `json:"email"`
	}) (struct {
		IsValid bool `json:"isValid"`
	}, error) {
		return struct {
			IsValid bool `json:"isValid"`
		}{IsValid: strings.Contains(input.Email, "@")}, nil
	}

	step1 := NewSubtask("fetchUser").SetExecutor(executor.NewHTTPExecutor[struct {
		UserID string `json:"userId"`
	}, map[string]any](apiServer.URL+"/user/u456", "GET",
		executor.WithHTTPTimeout(5*time.Second),
	))
	step2 := NewSubtask("validateUser").SetExecutor(executor.NewLocalExecutor(step2Fn))

	_ = task.AddSubtask(step1)
	_ = task.AddSubtask(step2)
	_ = task.AddEdge(step1, step2)

	_, err := task.Compile()
	assert.Nil(t, err)

	// 注册任务回调（执行器已在 AddSubtask 时自动注册）
	task.RegisterTaskExecutor(exec)

	// 执行 step1（通过 HTTP）
	next := task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "fetchUser", next[0].GetName())

	provider1 := step1.GetExecutor()
	assert.Equal(t, executor.ProtocolHTTP, provider1.Protocol())

	input1 := struct {
		UserID string `json:"userId"`
	}{UserID: "u456"}
	input1JSON, _ := json.Marshal(input1)
	result1, err := provider1.Execute(context.Background(), &executor.TaskData{
		Input: string(input1JSON),
	})
	assert.Nil(t, err)

	userData := result1.(map[string]any)
	assert.Equal(t, "u456", userData["userId"])
	assert.Equal(t, "bob@example.com", userData["email"])

	_ = task.UpdateSubtaskState(step1.GetID(), NodeSucceeded)

	// 执行 step2（本地）
	provider2 := step2.GetExecutor()
	assert.Equal(t, executor.ProtocolLocal, provider2.Protocol())

	_ = task.UpdateSubtaskState(step2.GetID(), NodeSucceeded)
	_ = result1 // 避免未使用警告
}

// ===== 使用方式三：泛型 LocalExecutor 替代旧风格 =====

// TestIT_GenericLocalExecutor 展示如何使用泛型 LocalExecutor 替代旧的 rawStringExecutor 包装方式
func TestIT_GenericLocalExecutor(t *testing.T) {
	exec := &MyTaskExecutor{}
	task := NewTask("legacy-workflow")

	// 使用泛型 LocalExecutor，输入输出类型明确，编译时类型检查
	step1 := NewSubtask("step1").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input string) (map[string]string, error) {
		return map[string]string{"result": "step1 done"}, nil
	}))
	step2 := NewSubtask("step2").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input string) (map[string]string, error) {
		return map[string]string{"result": "step2 done"}, nil
	}))

	_ = task.AddSubtask(step1)
	_ = task.AddSubtask(step2)
	_ = task.AddEdge(step1, step2)

	_, err := task.Compile()
	assert.Nil(t, err)

	// 注册任务回调（执行器已在 AddSubtask 时自动注册）
	task.RegisterTaskExecutor(exec)

	// 通过 GetExecutor 取得执行器并执行
	provider := step1.GetExecutor()
	assert.NotNil(t, provider)
	assert.Equal(t, executor.ProtocolLocal, provider.Protocol())

	// LocalExecutor 正常执行
	result, err := provider.Execute(context.Background(), &executor.TaskData{Input: `""`})
	assert.Nil(t, err)
	assert.NotNil(t, result)
}

// ===== 使用方式四：控制/数据依赖分离 + 执行器 =====

// TestIT_ControlDataSeparation_WithExecutors 展示控制边和数据边分离时的执行器使用
func TestIT_ControlDataSeparation_WithExecutors(t *testing.T) {
	exec := &MyTaskExecutor{}
	task := NewTask("separation-workflow")

	// step1: 验证（控制依赖 step3）
	// step2: 查询（控制依赖 step3，数据传给 step3）
	// step3: 汇总（等待 step1 和 step2 完成后执行）
	validate := NewSubtask("validate").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input struct{}) (struct {
		OK bool `json:"ok"`
	}, error) {
		return struct {
			OK bool `json:"ok"`
		}{OK: true}, nil
	}))
	query := NewSubtask("query").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input struct{}) (struct {
		Items []string `json:"items"`
	}, error) {
		return struct {
			Items []string `json:"items"`
		}{Items: []string{"a", "b", "c"}}, nil
	}))
	aggregate := NewSubtask("aggregate").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input struct {
		Items []string `json:"items"`
	}) (struct {
		Count int `json:"count"`
	}, error) {
		return struct {
			Count int `json:"count"`
		}{Count: len(input.Items)}, nil
	}))

	_ = task.AddSubtask(validate)
	_ = task.AddSubtask(query)
	_ = task.AddSubtask(aggregate)

	// validate -> aggregate: 仅控制依赖（aggregate 等待 validate 完成，但不用 validate 的数据）
	_ = task.AddControlEdge(validate, aggregate)
	// query -> aggregate: 控制依赖 + 数据依赖（aggregate 等待 query 完成，且接收 query 的数据）
	_ = task.AddDataEdge(query, aggregate)

	_, err := task.Compile()
	assert.Nil(t, err)

	// 注册任务回调（执行器已在 AddSubtask 时自动注册）
	task.RegisterTaskExecutor(exec)

	// 初始只有 validate 和 query 可执行（aggregate 等待数据前驱 query 未完成）
	next := task.NextSubTasks()
	assert.Equal(t, 2, len(next))

	// 执行 validate（不产生数据给 aggregate）
	p1 := validate.GetExecutor()
	r1, _ := p1.Execute(context.Background(), &executor.TaskData{Input: "{}"})
	assert.Equal(t, true, r1.(struct {
		OK bool `json:"ok"`
	}).OK)
	_ = task.UpdateSubtaskState(validate.GetID(), NodeSucceeded)

	// validate 完成后 aggregate 的控制依赖满足，但数据依赖 query 未完成，所以 aggregate 仍不可执行
	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next)) // 只有 query 可执行
	assert.Equal(t, "query", next[0].GetName())

	// 执行 query（产生数据给 aggregate）
	p2 := query.GetExecutor()
	_, _ = p2.Execute(context.Background(), &executor.TaskData{Input: "{}"})
	_ = task.UpdateSubtaskState(query.GetID(), NodeSucceeded)

	// query 完成后 aggregate 的数据依赖满足，现在可以执行
	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "aggregate", next[0].GetName())

	// 执行 aggregate（使用 query 传来的数据）
	p3 := aggregate.GetExecutor()
	// 注意：集群模式下数据会通过 TaskData.Input 传递；单元测试中 executor 收到什么取决于调用方
	// 这里传空数据，说明 aggregator 的 input.Items 为 nil，长度为 0
	result3, _ := p3.Execute(context.Background(), &executor.TaskData{Input: "{}"})
	_ = task.UpdateSubtaskState(aggregate.GetID(), NodeSucceeded)

	// aggregator 收到空 input，len(nil slice) = 0
	assert.Equal(t, 0, result3.(struct {
		Count int `json:"count"`
	}).Count)
}

// ===== 使用方式五：分支条件 + 执行器 =====

// TestIT_BranchCondition_WithExecutors 展示条件分支时的执行器使用
func TestIT_BranchCondition_WithExecutors(t *testing.T) {
	exec := &MyTaskExecutor{}
	task := NewTask("branch-workflow")

	start := NewSubtask("start").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input struct{}) (struct {
		Decision string `json:"decision"`
	}, error) {
		return struct {
			Decision string `json:"decision"`
		}{Decision: "A"}, nil
	}))
	branchA := NewSubtask("branchA").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input struct{}) (struct {
		Path string `json:"path"`
	}, error) {
		return struct {
			Path string `json:"path"`
		}{Path: "A"}, nil
	}))
	branchB := NewSubtask("branchB").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input struct{}) (struct {
		Path string `json:"path"`
	}, error) {
		return struct {
			Path string `json:"path"`
		}{Path: "B"}, nil
	}))
	end := NewSubtask("end").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input struct{}) (struct {
		Summary string `json:"summary"`
	}, error) {
		return struct {
			Summary string `json:"summary"`
		}{Summary: "completed"}, nil
	}))

	_ = task.AddSubtask(start)
	_ = task.AddSubtask(branchA)
	_ = task.AddSubtask(branchB)
	_ = task.AddSubtask(end)

	_ = task.AddEdge(start, branchA)
	_ = task.AddEdge(start, branchB)
	_ = task.AddEdge(branchA, end)
	_ = task.AddEdge(branchB, end)

	// 注册分支条件
	task.RegisterBranchCondition(start.GetID(), "default", func(ctx interface{}, input any) (string, error) {
		// 选择 branchA
		return branchA.GetID(), nil
	})

	_, err := task.Compile()
	assert.Nil(t, err)

	// 注册任务回调（执行器已在 AddSubtask 时自动注册）
	task.RegisterTaskExecutor(exec)

	// start -> branchA/branchB 分支选择
	next := task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "start", next[0].GetName())

	_ = task.UpdateSubtaskState(start.GetID(), NodeSucceeded)

	// 模拟分支选择：branchB 跳过
	_ = task.SkipSubtask(branchB.GetID())

	// 只有 branchA 可执行
	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "branchA", next[0].GetName())

	pA := branchA.GetExecutor()
	rA, _ := pA.Execute(context.Background(), &executor.TaskData{Input: "{}"})
	assert.Equal(t, "A", rA.(struct {
		Path string `json:"path"`
	}).Path)

	_ = task.UpdateSubtaskState(branchA.GetID(), NodeSucceeded)

	// end 可执行（branchB 已跳过，不阻塞）
	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "end", next[0].GetName())
}

// ===== 使用方式六：Skip 传播 + 执行器 =====

// TestIT_SkipPropagation_WithExecutors 展示 Skip 传播时的执行器使用
func TestIT_SkipPropagation_WithExecutors(t *testing.T) {
	exec := &MyTaskExecutor{}
	task := NewTask("skip-workflow")

	step1 := NewSubtask("step1").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input struct{}) (struct{}, error) {
		return struct{}{}, nil
	}))
	step2 := NewSubtask("step2").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input struct{}) (struct{}, error) {
		return struct{}{}, nil
	}))
	step3 := NewSubtask("step3").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input struct{}) (struct{}, error) {
		return struct{}{}, nil
	}))

	_ = task.AddSubtask(step1)
	_ = task.AddSubtask(step2)
	_ = task.AddSubtask(step3)

	_ = task.AddEdge(step1, step2)
	_ = task.AddEdge(step2, step3)

	_, err := task.Compile()
	assert.Nil(t, err)

	// 注册任务回调（执行器已在 AddSubtask 时自动注册）
	task.RegisterTaskExecutor(exec)

	// 初始执行 step1
	next := task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "step1", next[0].GetName())

	_ = task.UpdateSubtaskState(step1.GetID(), NodeSucceeded)

	// step2 可执行
	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))

	// 跳过 step2（step3 应该自动跳过，因为它的唯一控制前驱 step2 跳过）
	_ = task.SkipSubtask(step2.GetID())

	// 检查 step3 的通道状态：应该被跳过
	ch := task.getCompiled().GetChannel(step3.GetID())
	assert.True(t, ch.isSkipped())
}

// ===== 使用方式七：实例隔离测试 =====

// TestIT_ExecutorIsolation_BetweenTasks 展示两个 Task 实例的执行器互不影响
func TestIT_ExecutorIsolation_BetweenTasks(t *testing.T) {
	execA := &MyTaskExecutor{}
	execB := &MyTaskExecutor{}

	taskA := NewTask("taskA")
	taskB := NewTask("taskB")

	stepA := NewSubtask("stepA").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input struct{}) (struct {
		From string `json:"from"`
	}, error) {
		return struct {
			From string `json:"from"`
		}{From: "taskA"}, nil
	}))
	stepB := NewSubtask("stepB").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input struct{}) (struct {
		From string `json:"from"`
	}, error) {
		return struct {
			From string `json:"from"`
		}{From: "taskB"}, nil
	}))

	_ = taskA.AddSubtask(stepA)
	_ = taskB.AddSubtask(stepB)
	_, _ = taskA.Compile()
	_, _ = taskB.Compile()

	// 注册任务回调（执行器已在 AddSubtask 时自动注册）
	taskA.RegisterTaskExecutor(execA)
	taskB.RegisterTaskExecutor(execB)

	// taskA 的执行器拿不到 taskB 的 stepB
	providerA_forB := taskA.em.getProvider(execA.Name(), "stepB")
	assert.Nil(t, providerA_forB)

	// taskB 的执行器拿不到 taskA 的 stepA
	providerB_forA := taskB.em.getProvider(execB.Name(), "stepA")
	assert.Nil(t, providerB_forA)

	// 各自能通过 Subtask.GetExecutor() 拿到
	pA := stepA.GetExecutor()
	pB := stepB.GetExecutor()
	assert.NotNil(t, pA)
	assert.NotNil(t, pB)

	rA, _ := pA.Execute(context.Background(), &executor.TaskData{Input: "{}"})
	rB, _ := pB.Execute(context.Background(), &executor.TaskData{Input: "{}"})

	assert.Equal(t, "taskA", rA.(struct {
		From string `json:"from"`
	}).From)
	assert.Equal(t, "taskB", rB.(struct {
		From string `json:"from"`
	}).From)
}

// ===== 使用方式八：与 DAG 完整集成 =====

// TestIT_FullDAGWithExecutors 展示完整的 DAG 构建 + 执行器注册 + 逐步执行 + 结果验证
func TestIT_FullDAGWithExecutors(t *testing.T) {
	exec := &MyTaskExecutor{}
	task := NewTask("full-pipeline")

	// 构建一个 5 步的数据处理流水线
	// fetch -> transform -> filter -> enrich -> save
	fetch := NewSubtask("fetch").SetPriority(10).SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, _ struct{}) (struct {
		RawData []int `json:"rawData"`
	}, error) {
		return struct {
			RawData []int `json:"rawData"`
		}{RawData: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}}, nil
	}))
	transform := NewSubtask("transform").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input struct {
		RawData []int `json:"rawData"`
	}) (struct {
		Doubled []int `json:"doubled"`
	}, error) {
		doubled := make([]int, len(input.RawData))
		for i, v := range input.RawData {
			doubled[i] = v * 2
		}
		return struct {
			Doubled []int `json:"doubled"`
		}{Doubled: doubled}, nil
	}))
	filter := NewSubtask("filter").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input struct {
		Doubled []int `json:"doubled"`
	}) (struct {
		Filtered []int `json:"filtered"`
	}, error) {
		var filtered []int
		for _, v := range input.Doubled {
			if v > 10 {
				filtered = append(filtered, v)
			}
		}
		return struct {
			Filtered []int `json:"filtered"`
		}{Filtered: filtered}, nil
	}))
	enrich := NewSubtask("enrich").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input struct {
		Filtered []int `json:"filtered"`
	}) (struct {
		Enriched []string `json:"enriched"`
	}, error) {
		enriched := make([]string, len(input.Filtered))
		for i, v := range input.Filtered {
			enriched[i] = fmt.Sprintf("item_%d", v)
		}
		return struct {
			Enriched []string `json:"enriched"`
		}{Enriched: enriched}, nil
	}))
	save := NewSubtask("save").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input struct {
		Enriched []string `json:"enriched"`
	}) (struct {
		SavedCount int `json:"savedCount"`
	}, error) {
		return struct {
			SavedCount int `json:"savedCount"`
		}{SavedCount: len(input.Enriched)}, nil
	}))

	_ = task.AddSubtask(fetch)
	_ = task.AddSubtask(transform)
	_ = task.AddSubtask(filter)
	_ = task.AddSubtask(enrich)
	_ = task.AddSubtask(save)

	// DAG: fetch -> transform -> filter -> enrich -> save
	_ = task.AddEdge(fetch, transform)
	_ = task.AddEdge(transform, filter)
	_ = task.AddEdge(filter, enrich)
	_ = task.AddEdge(enrich, save)

	_, err := task.Compile()
	assert.Nil(t, err)

	// 注册任务回调（执行器已在 AddSubtask 时自动注册）
	task.RegisterTaskExecutor(exec)

	// ===== 模拟完整执行流程 =====
	var results []any

	for !task.IsFinished() {
		next := task.NextSubTasks()
		if len(next) == 0 {
			break
		}

		// 按优先级和顺序执行
		for _, st := range next {
			provider := st.GetExecutor()
			if provider == nil {
				continue
			}

			var inputJSON string
			if len(results) > 0 {
				lastResult := results[len(results)-1]
				bytes, _ := json.Marshal(lastResult)
				inputJSON = string(bytes)
			}

			result, err := provider.Execute(context.Background(), &executor.TaskData{
				Input: inputJSON,
			})
			if err != nil {
				_ = task.UpdateSubtaskState(st.GetID(), NodeFailed)
				t.Fatalf("step %s failed: %v", st.GetName(), err)
			}

			results = append(results, result)
			_ = task.UpdateSubtaskState(st.GetID(), NodeSucceeded)
		}
	}

	// 验证最终结果
	assert.True(t, task.IsFinished())
	assert.Equal(t, 5, len(results))

	// fetch: 10 个数字 -> rawData = [1..10]
	assert.Equal(t, 10, len(results[0].(struct {
		RawData []int `json:"rawData"`
	}).RawData))
	// transform: 每个 *2 -> doubled = [2, 4, 6, 8, 10, 12, 14, 16, 18, 20], doubled[0] = 2
	assert.Equal(t, 2, results[1].(struct {
		Doubled []int `json:"doubled"`
	}).Doubled[0])
	// filter: > 10 -> filtered = [12, 14, 16, 18, 20], 5 个
	assert.Equal(t, 5, len(results[2].(struct {
		Filtered []int `json:"filtered"`
	}).Filtered))
	// enrich: 添加前缀 -> enriched[0] = "item_12"
	assert.Equal(t, "item_12", results[3].(struct {
		Enriched []string `json:"enriched"`
	}).Enriched[0])
	// save: 返回保存数量 = 5
	assert.Equal(t, 5, results[4].(struct {
		SavedCount int `json:"savedCount"`
	}).SavedCount)
}

// TestIT_PreProcessor_ValidateInput 测试前置处理器：输入校验
func TestIT_PreProcessor_ValidateInput(t *testing.T) {
	exec := &MyTaskExecutor{}
	task := NewTask("pre-processor-validate")

	// 前置处理器：校验输入中必须包含 "value" 字段
	validatePreProcessor := func(ctx interface{}, data any) (any, error) {
		taskData, ok := data.(*executor.TaskData)
		if !ok {
			return nil, fmt.Errorf("invalid input type")
		}
		var input map[string]string
		if err := json.Unmarshal([]byte(taskData.Input), &input); err != nil {
			return nil, fmt.Errorf("invalid json input: %w", err)
		}
		if _, exists := input["value"]; !exists {
			return nil, fmt.Errorf("missing required field: value")
		}
		return data, nil // 校验通过，返回原始数据
	}

	step1 := NewSubtask("step1").
		SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input map[string]string) (map[string]string, error) {
			return map[string]string{"result": input["value"]}, nil
		})).
		SetPreProcessor(validatePreProcessor)

	_ = task.AddSubtask(step1)
	_, err := task.Compile()
	assert.NoError(t, err)
	task.RegisterTaskExecutor(exec)

	// 测试1：合法输入
	result, err := step1.Execute(context.Background(), &executor.TaskData{
		Input: `{"value": "hello"}`,
	})
	assert.NoError(t, err)
	assert.Equal(t, "hello", result.(map[string]string)["result"])

	// 测试2：非法输入（缺少 value 字段）
	_, err = step1.Execute(context.Background(), &executor.TaskData{
		Input: `{"name": "test"}`,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required field: value")
}

// TestIT_PreProcessor_ModifyInput 测试前置处理器：修改输入数据
func TestIT_PreProcessor_ModifyInput(t *testing.T) {
	exec := &MyTaskExecutor{}
	task := NewTask("pre-processor-modify")

	// 前置处理器：在输入中注入额外字段
	injectPreProcessor := func(ctx interface{}, data any) (any, error) {
		taskData, ok := data.(*executor.TaskData)
		if !ok {
			return data, nil
		}
		var input map[string]string
		if err := json.Unmarshal([]byte(taskData.Input), &input); err != nil {
			return data, nil
		}
		input["injected"] = "yes"
		modifiedInput, _ := json.Marshal(input)
		return &executor.TaskData{
			RequestId: taskData.RequestId,
			TaskId:    taskData.TaskId,
			SubTaskId: taskData.SubTaskId,
			Input:     string(modifiedInput),
			Subtasks:  taskData.Subtasks,
		}, nil
	}

	step1 := NewSubtask("step1").
		SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input map[string]string) (map[string]string, error) {
			return map[string]string{
				"original": input["value"],
				"injected": input["injected"],
			}, nil
		})).
		SetPreProcessor(injectPreProcessor)

	_ = task.AddSubtask(step1)
	_, err := task.Compile()
	assert.NoError(t, err)
	task.RegisterTaskExecutor(exec)

	result, err := step1.Execute(context.Background(), &executor.TaskData{
		Input: `{"value": "hello"}`,
	})
	assert.NoError(t, err)
	output := result.(map[string]string)
	assert.Equal(t, "hello", output["original"])
	assert.Equal(t, "yes", output["injected"])
}

// TestIT_PostProcessor_TransformOutput 测试后置处理器：转换输出数据
func TestIT_PostProcessor_TransformOutput(t *testing.T) {
	exec := &MyTaskExecutor{}
	task := NewTask("post-processor-transform")

	// 后置处理器：将输出包装成统一格式
	wrapPostProcessor := func(ctx interface{}, data any) (any, error) {
		return map[string]any{
			"success": true,
			"data":    data,
		}, nil
	}

	step1 := NewSubtask("step1").
		SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input map[string]string) (map[string]string, error) {
			return map[string]string{"result": input["value"]}, nil
		})).
		SetPostProcessor(wrapPostProcessor)

	_ = task.AddSubtask(step1)
	_, err := task.Compile()
	assert.NoError(t, err)
	task.RegisterTaskExecutor(exec)

	result, err := step1.Execute(context.Background(), &executor.TaskData{
		Input: `{"value": "hello"}`,
	})
	assert.NoError(t, err)
	output := result.(map[string]any)
	assert.Equal(t, true, output["success"])
	innerData := output["data"].(map[string]string)
	assert.Equal(t, "hello", innerData["result"])
}

// TestIT_PostProcessor_ValidateOutput 测试后置处理器：输出校验失败
func TestIT_PostProcessor_ValidateOutput(t *testing.T) {
	exec := &MyTaskExecutor{}
	task := NewTask("post-processor-validate")

	// 后置处理器：校验输出必须包含 "status" 字段
	validatePostProcessor := func(ctx interface{}, data any) (any, error) {
		output, ok := data.(map[string]string)
		if !ok {
			return nil, fmt.Errorf("invalid output type")
		}
		if _, exists := output["status"]; !exists {
			return nil, fmt.Errorf("missing required field in output: status")
		}
		return data, nil
	}

	step1 := NewSubtask("step1").
		SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input map[string]string) (map[string]string, error) {
			return map[string]string{"result": "done"}, nil // 没有 status 字段
		})).
		SetPostProcessor(validatePostProcessor)

	_ = task.AddSubtask(step1)
	_, err := task.Compile()
	assert.NoError(t, err)
	task.RegisterTaskExecutor(exec)

	_, err = step1.Execute(context.Background(), &executor.TaskData{
		Input: `{"value": "hello"}`,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required field in output: status")
}

// TestIT_PreAndPostProcessor_Chain 测试前置+后置处理器链式组合
func TestIT_PreAndPostProcessor_Chain(t *testing.T) {
	exec := &MyTaskExecutor{}
	task := NewTask("processor-chain")

	var preCalled, postCalled bool

	step1 := NewSubtask("step1").
		SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input map[string]string) (map[string]string, error) {
			// 验证前置处理器已执行
			assert.True(t, preCalled, "preProcessor should have been called before executor")
			return map[string]string{"result": input["value"] + "-processed"}, nil
		})).
		SetPreProcessor(func(ctx interface{}, data any) (any, error) {
			preCalled = true
			return data, nil
		}).
		SetPostProcessor(func(ctx interface{}, data any) (any, error) {
			postCalled = true
			// 在输出中添加标记
			output := data.(map[string]string)
			output["postProcessed"] = "true"
			return output, nil
		})

	step2 := NewSubtask("step2").
		SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input map[string]string) (map[string]string, error) {
			return map[string]string{"final": input["value"]}, nil
		}))

	_ = task.AddSubtask(step1)
	_ = task.AddSubtask(step2)
	_ = task.AddEdge(step1, step2)
	_, err := task.Compile()
	assert.NoError(t, err)
	task.RegisterTaskExecutor(exec)

	// 执行 step1
	result, err := step1.Execute(context.Background(), &executor.TaskData{
		Input: `{"value": "hello"}`,
	})
	assert.NoError(t, err)
	output := result.(map[string]string)
	assert.Equal(t, "hello-processed", output["result"])
	assert.Equal(t, "true", output["postProcessed"])
	assert.True(t, preCalled)
	assert.True(t, postCalled)
}

// TestIT_PreProcessor_ErrorStopsExecution 测试前置处理器错误阻止执行器运行
func TestIT_PreProcessor_ErrorStopsExecution(t *testing.T) {
	exec := &MyTaskExecutor{}
	task := NewTask("pre-processor-block")

	var executorCalled bool

	step1 := NewSubtask("step1").
		SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input map[string]string) (map[string]string, error) {
			executorCalled = true
			return map[string]string{"result": "should not reach"}, nil
		})).
		SetPreProcessor(func(ctx interface{}, data any) (any, error) {
			return nil, fmt.Errorf("validation failed")
		})

	_ = task.AddSubtask(step1)
	_, err := task.Compile()
	assert.NoError(t, err)
	task.RegisterTaskExecutor(exec)

	_, err = step1.Execute(context.Background(), &executor.TaskData{
		Input: `{"value": "hello"}`,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "preProcessor failed")
	assert.Contains(t, err.Error(), "validation failed")
	assert.False(t, executorCalled, "executor should not be called when preProcessor fails")
}

// TestIT_SubtaskExecute_NoProvider 测试无执行器时 Execute 返回错误
func TestIT_SubtaskExecute_NoProvider(t *testing.T) {
	step1 := NewSubtask("step1")
	_, err := step1.Execute(context.Background(), &executor.TaskData{
		Input: `{"value": "hello"}`,
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrExecutorNotFound)
}

// ===== 条件节点（Branch）集成测试 =====

// TestIT_Branch_SelectPathB 选择 B 路径，A 路径自动跳过
func TestIT_Branch_SelectPathB(t *testing.T) {
	exec := &MyTaskExecutor{}
	task := NewTask("branch-select-B")

	start := NewSubtask("start").SetExecutor(executor.NewLocalExecutor(echoInput))
	pathA := NewSubtask("pathA").SetExecutor(executor.NewLocalExecutor(echoInput))
	pathB := NewSubtask("pathB").SetExecutor(executor.NewLocalExecutor(echoInput))
	end := NewSubtask("end").SetExecutor(executor.NewLocalExecutor(echoInput))

	_ = task.AddSubtask(start)
	_ = task.AddSubtask(pathA)
	_ = task.AddSubtask(pathB)
	_ = task.AddSubtask(end)

	_ = task.AddEdge(start, pathA)
	_ = task.AddEdge(start, pathB)
	_ = task.AddEdge(pathA, end)
	_ = task.AddEdge(pathB, end)

	// 添加分支：选择 pathB
	_ = task.AddBranch(start, &Branch{
		Condition: func(ctx interface{}, input any) (string, error) {
			return pathB.GetID(), nil
		},
		EndNodes: map[string]bool{pathA.GetID(): true, pathB.GetID(): true},
	})

	_, err := task.Compile()
	assert.Nil(t, err)
	task.RegisterTaskExecutor(exec)

	// start 先执行
	next := task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "start", next[0].GetName())

	// start 完成，分支选择 pathB，pathA 跳过
	_ = task.UpdateSubtaskState(start.GetID(), NodeSucceeded)
	_ = task.SkipSubtask(pathA.GetID())

	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "pathB", next[0].GetName())

	// pathB 完成后，end 可执行（pathA 已跳过，不阻塞）
	_ = task.UpdateSubtaskState(pathB.GetID(), NodeSucceeded)
	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "end", next[0].GetName())

	// 验证 pathA 的 subtask 状态为 Skipped
	assert.Equal(t, string(TaskSkipped), pathA.GetState())
}

// TestIT_Branch_WithDataFlow 分支节点输出通过数据流传递给选中路径
func TestIT_Branch_WithDataFlow(t *testing.T) {
	exec := &MyTaskExecutor{}
	task := NewTask("branch-data-flow")

	// start 输出包含 decision 字段
	start := NewSubtask("start").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input map[string]string) (map[string]string, error) {
		return map[string]string{"decision": "B", "value": "from_start"}, nil
	}))
	pathA := NewSubtask("pathA").SetExecutor(executor.NewLocalExecutor(echoInput))
	pathB := NewSubtask("pathB").SetExecutor(executor.NewLocalExecutor(echoInput))
	end := NewSubtask("end").SetExecutor(executor.NewLocalExecutor(echoInput))

	_ = task.AddSubtask(start)
	_ = task.AddSubtask(pathA)
	_ = task.AddSubtask(pathB)
	_ = task.AddSubtask(end)

	// 控制边 + 数据边
	_ = task.AddEdge(start, pathA)
	_ = task.AddEdge(start, pathB)
	_ = task.AddEdge(pathA, end)
	_ = task.AddEdge(pathB, end)

	// 添加分支：根据 start 输出选择路径
	_ = task.AddBranch(start, &Branch{
		Condition: func(ctx interface{}, input any) (string, error) {
			// 根据 start 输出选择 pathB
			if m, ok := input.(map[string]any); ok {
				if decision, ok := m["decision"].(string); ok && decision == "B" {
					return pathB.GetID(), nil
				}
			}
			return pathA.GetID(), nil
		},
		EndNodes: map[string]bool{pathA.GetID(): true, pathB.GetID(): true},
	})

	_, err := task.Compile()
	assert.Nil(t, err)
	task.RegisterTaskExecutor(exec)

	// start 先执行
	next := task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "start", next[0].GetName())

	_ = task.UpdateSubtaskState(start.GetID(), NodeSucceeded)

	// 获取 start 的输出数据
	chB := task.getCompiled().GetChannel(pathB.GetID())
	data, ready, _ := chB.get()
	assert.True(t, ready)
	assert.NotNil(t, data)

	// pathA 跳过
	_ = task.SkipSubtask(pathA.GetID())

	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "pathB", next[0].GetName())
}

// TestIT_Branch_SkipAutoPropagation 未选中的分支自动 Skip，收敛节点仍可执行
func TestIT_Branch_SkipAutoPropagation(t *testing.T) {
	task := NewTask("branch-skip-propagation")

	start := NewSubtask("start")
	pathA := NewSubtask("pathA")
	pathB := NewSubtask("pathB")
	pathC := NewSubtask("pathC")
	end := NewSubtask("end")

	_ = task.AddSubtask(start)
	_ = task.AddSubtask(pathA)
	_ = task.AddSubtask(pathB)
	_ = task.AddSubtask(pathC)
	_ = task.AddSubtask(end)

	_ = task.AddEdge(start, pathA)
	_ = task.AddEdge(start, pathB)
	_ = task.AddEdge(start, pathC)
	_ = task.AddEdge(pathA, end)
	_ = task.AddEdge(pathB, end)
	_ = task.AddEdge(pathC, end)

	// 三路分支，只选 pathB
	_ = task.AddBranch(start, &Branch{
		Condition: func(ctx interface{}, input any) (string, error) {
			return pathB.GetID(), nil
		},
		EndNodes: map[string]bool{pathA.GetID(): true, pathB.GetID(): true, pathC.GetID(): true},
	})

	_, err := task.Compile()
	assert.Nil(t, err)

	// start 完成
	_ = task.UpdateSubtaskState(start.GetID(), NodeSucceeded)

	// 跳过未选中的路径
	_ = task.SkipSubtask(pathA.GetID())
	_ = task.SkipSubtask(pathC.GetID())

	// pathB 可执行
	next := task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "pathB", next[0].GetName())

	// pathB 完成后，end 可执行（pathA、pathC 已跳过，不阻塞）
	_ = task.UpdateSubtaskState(pathB.GetID(), NodeSucceeded)
	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "end", next[0].GetName())

	// 验证 pathA 和 pathC 的 subtask 状态为 Skipped
	assert.Equal(t, string(TaskSkipped), pathA.GetState())
	assert.Equal(t, string(TaskSkipped), pathC.GetState())
}

// TestIT_Branch_AnyPredecessorConverge AnyPredecessor 触发模式 + 分支收敛
func TestIT_Branch_AnyPredecessorConverge(t *testing.T) {
	task := NewTask("branch-any-converge")

	start := NewSubtask("start")
	pathA := NewSubtask("pathA")
	pathB := NewSubtask("pathB")
	// end 使用 AnyPredecessor：任一前驱完成即触发
	end := NewSubtask("end").SetTriggerMode(AnyPredecessor)

	_ = task.AddSubtask(start)
	_ = task.AddSubtask(pathA)
	_ = task.AddSubtask(pathB)
	_ = task.AddSubtask(end)

	_ = task.AddEdge(start, pathA)
	_ = task.AddEdge(start, pathB)
	_ = task.AddEdge(pathA, end)
	_ = task.AddEdge(pathB, end)

	_, err := task.Compile()
	assert.Nil(t, err)

	// start 完成
	_ = task.UpdateSubtaskState(start.GetID(), NodeSucceeded)

	// pathA 完成，pathB 跳过
	_ = task.UpdateSubtaskState(pathA.GetID(), NodeSucceeded)
	_ = task.SkipSubtask(pathB.GetID())

	// end 使用 AnyPredecessor，pathA 完成即可触发
	chEnd := task.getCompiled().GetChannel(end.GetID())
	assert.True(t, chEnd.isReady())

	next := task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "end", next[0].GetName())
}

// TestIT_Branch_Nested 嵌套分支：外层分支选择后，内层再分支
func TestIT_Branch_Nested(t *testing.T) {
	task := NewTask("nested-branch")

	start := NewSubtask("start")
	outerA := NewSubtask("outerA")
	outerB := NewSubtask("outerB")
	innerA1 := NewSubtask("innerA1")
	innerA2 := NewSubtask("innerA2")
	end := NewSubtask("end")

	_ = task.AddSubtask(start)
	_ = task.AddSubtask(outerA)
	_ = task.AddSubtask(outerB)
	_ = task.AddSubtask(innerA1)
	_ = task.AddSubtask(innerA2)
	_ = task.AddSubtask(end)

	// 外层分支
	_ = task.AddEdge(start, outerA)
	_ = task.AddEdge(start, outerB)
	// 内层分支（outerA 下再分）
	_ = task.AddEdge(outerA, innerA1)
	_ = task.AddEdge(outerA, innerA2)
	// 收敛
	_ = task.AddEdge(innerA1, end)
	_ = task.AddEdge(innerA2, end)
	_ = task.AddEdge(outerB, end)

	// 外层分支：选 outerA
	_ = task.AddBranch(start, &Branch{
		Condition: func(ctx interface{}, input any) (string, error) {
			return outerA.GetID(), nil
		},
		EndNodes: map[string]bool{outerA.GetID(): true, outerB.GetID(): true},
	})

	// 内层分支：选 innerA1
	_ = task.AddBranch(outerA, &Branch{
		Condition: func(ctx interface{}, input any) (string, error) {
			return innerA1.GetID(), nil
		},
		EndNodes: map[string]bool{innerA1.GetID(): true, innerA2.GetID(): true},
	})

	_, err := task.Compile()
	assert.Nil(t, err)

	// start 完成
	_ = task.UpdateSubtaskState(start.GetID(), NodeSucceeded)
	// outerB 跳过
	_ = task.SkipSubtask(outerB.GetID())

	next := task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "outerA", next[0].GetName())

	// outerA 完成，innerA2 跳过
	_ = task.UpdateSubtaskState(outerA.GetID(), NodeSucceeded)
	_ = task.SkipSubtask(innerA2.GetID())

	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "innerA1", next[0].GetName())

	// innerA1 完成后，end 可执行
	_ = task.UpdateSubtaskState(innerA1.GetID(), NodeSucceeded)
	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "end", next[0].GetName())
}

// TestIT_Branch_ConditionError 分支条件返回错误
func TestIT_Branch_ConditionError(t *testing.T) {
	task := NewTask("branch-condition-error")

	start := NewSubtask("start")
	pathA := NewSubtask("pathA")
	pathB := NewSubtask("pathB")
	end := NewSubtask("end")

	_ = task.AddSubtask(start)
	_ = task.AddSubtask(pathA)
	_ = task.AddSubtask(pathB)
	_ = task.AddSubtask(end)

	_ = task.AddEdge(start, pathA)
	_ = task.AddEdge(start, pathB)
	_ = task.AddEdge(pathA, end)
	_ = task.AddEdge(pathB, end)

	// 分支条件返回错误
	_ = task.AddBranch(start, &Branch{
		Condition: func(ctx interface{}, input any) (string, error) {
			return "", errors.New("condition evaluation failed")
		},
		EndNodes: map[string]bool{pathA.GetID(): true, pathB.GetID(): true},
	})

	_, err := task.Compile()
	assert.Nil(t, err)

	// start 完成
	_ = task.UpdateSubtaskState(start.GetID(), NodeSucceeded)

	// 分支条件错误时，两个路径都不会自动跳过，需要手动处理
	// 验证两个路径都处于等待状态
	chA := task.getCompiled().GetChannel(pathA.GetID())
	chB := task.getCompiled().GetChannel(pathB.GetID())
	assert.False(t, chA.isSkipped())
	assert.False(t, chB.isSkipped())
	assert.True(t, chA.isReady()) // 控制依赖已满足，但分支选择未执行
	assert.True(t, chB.isReady())
}

// TestIT_Branch_InvalidEndNode 编译校验：分支 endNode 不存在
func TestIT_Branch_InvalidEndNode(t *testing.T) {
	task := NewTask("branch-invalid-end")

	start := NewSubtask("start")
	pathA := NewSubtask("pathA")

	_ = task.AddSubtask(start)
	_ = task.AddSubtask(pathA)
	_ = task.AddEdge(start, pathA)

	// endNode "nonExist" 不在图中，AddBranch 时不校验，Compile 时校验
	err := task.AddBranch(start, &Branch{
		Condition: func(ctx interface{}, input any) (string, error) {
			return pathA.GetID(), nil
		},
		EndNodes: map[string]bool{pathA.GetID(): true, "nonExist": true},
	})
	assert.Nil(t, err) // AddBranch 不校验 endNode

	// Compile 时校验 endNode
	_, err = task.Compile()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "nonExist")
}

// TestIT_Branch_WithExecutorsAndDataFlow 分支 + 执行器 + 数据流完整流程
func TestIT_Branch_WithExecutorsAndDataFlow(t *testing.T) {
	exec := &MyTaskExecutor{}
	task := NewTask("branch-full-flow")

	// start 输出 decision
	start := NewSubtask("start").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input map[string]string) (map[string]string, error) {
		return map[string]string{"decision": "fast", "payload": "important_data"}, nil
	}))
	// fastPath 处理
	fastPath := NewSubtask("fastPath").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input map[string]string) (map[string]string, error) {
		result := make(map[string]string)
		for k, v := range input {
			result[k] = "fast_" + v
		}
		return result, nil
	}))
	// slowPath 处理
	slowPath := NewSubtask("slowPath").SetExecutor(executor.NewLocalExecutor(func(ctx context.Context, input map[string]string) (map[string]string, error) {
		result := make(map[string]string)
		for k, v := range input {
			result[k] = "slow_" + v
		}
		return result, nil
	}))
	// end 汇总
	end := NewSubtask("end").SetExecutor(executor.NewLocalExecutor(echoInput))

	_ = task.AddSubtask(start)
	_ = task.AddSubtask(fastPath)
	_ = task.AddSubtask(slowPath)
	_ = task.AddSubtask(end)

	_ = task.AddEdge(start, fastPath)
	_ = task.AddEdge(start, slowPath)
	_ = task.AddEdge(fastPath, end)
	_ = task.AddEdge(slowPath, end)

	// 分支：根据 decision 选择路径
	_ = task.AddBranch(start, &Branch{
		Condition: func(ctx interface{}, input any) (string, error) {
			if m, ok := input.(map[string]any); ok {
				if decision, ok := m["decision"].(string); ok {
					if decision == "fast" {
						return fastPath.GetID(), nil
					}
				}
			}
			return slowPath.GetID(), nil
		},
		EndNodes: map[string]bool{fastPath.GetID(): true, slowPath.GetID(): true},
	})

	_, err := task.Compile()
	assert.Nil(t, err)
	task.RegisterTaskExecutor(exec)

	// start 执行
	next := task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "start", next[0].GetName())

	// 执行 start
	result, err := start.Execute(context.Background(), &executor.TaskData{Input: `{}`})
	assert.Nil(t, err)
	assert.NotNil(t, result)

	_ = task.UpdateSubtaskState(start.GetID(), NodeSucceeded)

	// slowPath 跳过
	_ = task.SkipSubtask(slowPath.GetID())

	// fastPath 可执行
	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "fastPath", next[0].GetName())

	// 执行 fastPath
	result, err = fastPath.Execute(context.Background(), &executor.TaskData{Input: `{"decision":"fast","payload":"important_data"}`})
	assert.Nil(t, err)
	outputMap, ok := result.(map[string]string)
	assert.True(t, ok)
	assert.Equal(t, "fast_important_data", outputMap["payload"])

	_ = task.UpdateSubtaskState(fastPath.GetID(), NodeSucceeded)

	// end 可执行
	next = task.NextSubTasks()
	assert.Equal(t, 1, len(next))
	assert.Equal(t, "end", next[0].GetName())
}
