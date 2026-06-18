package redisd

import (
	"context"
	"testing"
	"time"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/taskx/dao"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_RedisTaskLifecycle tests the full task lifecycle through Redis DAO:
// Submit (Insert task + subtasks + edges) → Query → CAS allocate → SetState → complete.
func TestIntegration_RedisTaskLifecycle(t *testing.T) {
	rc, _ := createTestRedis(t)
	ctx := context.Background()

	taskDao := NewTaskDAOWithClient(rc)
	subDao := NewSubtaskDAOWithClient(rc)
	edgeDao := NewTaskEdgeDAOWithClient(rc)
	store := taskDao.GetStore()

	// --- Phase 1: SubmitTask ---
	err := store.RunInTx(ctx, func(ctx context.Context) error {
		if _, err := taskDao.Insert(ctx, &model.Task{
			ID: "life-1", TaskName: "lifecycle-test", State: "pending",
			Worker: "", Status: 1, Retry: 3,
		}); err != nil {
			return err
		}
		if _, err := subDao.BatchInsert(ctx, []model.Subtask{
			{ID: "ls-1", TaskID: "life-1", TaskName: "step-1", State: "pending", Status: 1, Worker: "", Retry: 2},
			{ID: "ls-2", TaskID: "life-1", TaskName: "step-2", State: "pending", Status: 1, Worker: "", Retry: 2},
		}); err != nil {
			return err
		}
		if _, err := edgeDao.BatchInsert(ctx, []model.TaskEdge{
			{ID: "le-1", TaskID: "life-1", FromSubtaskID: "ls-1", ToSubtaskID: "ls-2", EdgeType: "control"},
		}); err != nil {
			return err
		}
		return nil
	})
	require.NoError(t, err)

	// --- Phase 2: GetTodoTask (master scheduling) ---
	futureTime := basic.NewFromTime(time.Now().Add(time.Hour))
	todos, err := taskDao.GetTodoTask(ctx, []string{"pending"}, futureTime)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(todos), 1)

	// --- Phase 3: Allocate worker (CAS) ---
	n, err := taskDao.CASWorkerAndState(ctx, "life-1", "node-A", "running", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// Verify another node can't steal the task
	n2, err := taskDao.CASWorkerAndState(ctx, "life-1", "node-B", "running", "")
	require.NoError(t, err)
	assert.Equal(t, int64(0), n2, "second CAS should fail")

	// --- Phase 4: Allocate subtask workers ---
	n, err = subDao.CASWorkerAndState(ctx, "ls-1", "node-A", "running", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// --- Phase 5: Execute subtask, set output ---
	err = subDao.SetOutputAndState(ctx, "ls-1", `{"step1":"done"}`, "succeeded")
	require.NoError(t, err)

	// --- Phase 6: Execute second subtask ---
	n, err = subDao.CASWorkerAndState(ctx, "ls-2", "node-A", "running", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	err = subDao.SetOutputAndState(ctx, "ls-2", `{"step2":"done"}`, "succeeded")
	require.NoError(t, err)

	// --- Phase 7: Mark task as succeeded ---
	n, err = taskDao.SetState(ctx, "life-1", "succeeded")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// --- Phase 8: Verify final state ---
	task, err := taskDao.GetByID(ctx, "life-1")
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Equal(t, "succeeded", task.State)
	assert.Equal(t, "node-A", task.Worker)

	subs, err := subDao.GetByTaskID(ctx, "life-1")
	require.NoError(t, err)
	assert.Len(t, subs, 2)
	for _, s := range subs {
		assert.Equal(t, "succeeded", s.State)
	}

	edges, err := edgeDao.GetByTaskID(ctx, "life-1")
	require.NoError(t, err)
	assert.Len(t, edges, 1)
	assert.Equal(t, "control", edges[0].EdgeType)
}

// TestIntegration_RedisRollbackLifecycle tests the rollback flow:
// Subtask fails → SetRetry → retry exhausted → CASWorkerAndRollback → SetRollbackAndState.
func TestIntegration_RedisRollbackLifecycle(t *testing.T) {
	rc, _ := createTestRedis(t)
	ctx := context.Background()

	subDao := NewSubtaskDAOWithClient(rc)

	// Insert subtask
	_, _ = subDao.BatchInsert(ctx, []model.Subtask{
		{ID: "rb-1", TaskID: "t-rb", TaskName: "risky-step", State: "pending", Worker: "", Status: 1, Retry: 2},
	})

	// Allocate worker
	n, _ := subDao.CASWorkerAndState(ctx, "rb-1", "node-A", "running", "")
	assert.Equal(t, int64(1), n)

	// Subtask fails, set retry
	err := subDao.SetRetry(ctx, "rb-1", 1)
	require.NoError(t, err)
	got, _ := subDao.GetByID(ctx, "rb-1")
	assert.Equal(t, "pending", got.State)
	assert.Equal(t, int8(1), got.Retry)

	// Re-allocate and fail again (worker is still "node-A" after SetRetry)
	n, _ = subDao.CASWorkerAndState(ctx, "rb-1", "node-A", "running", "node-A")
	assert.Equal(t, int64(1), n)

	err = subDao.SetRetry(ctx, "rb-1", 0)
	require.NoError(t, err)

	// Trigger rollback (worker is still "node-A" after SetRetry)
	n, err = subDao.CASWorkerAndRollback(ctx, "rb-1", "node-A", "rollback_all", "node-A")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	err = subDao.SetRollbackAndState(ctx, "rb-1", "rollback_succeeded", `{"rollback":"done"}`)
	require.NoError(t, err)

	// Verify final state
	got, _ = subDao.GetByID(ctx, "rb-1")
	require.NotNil(t, got)
	assert.Equal(t, "rollback_succeeded", got.State)
}

// TestIntegration_StorageSwitchInjection verifies that both DAO implementations
// satisfy the same interface and can be swapped transparently.
func TestIntegration_StorageSwitchInjection(t *testing.T) {
	rc, _ := createTestRedis(t)
	ctx := context.Background()

	// Create Redis DAOs
	var taskDao dao.TaskDAO = NewTaskDAOWithClient(rc)
	var subDao dao.SubtaskDAO = NewSubtaskDAOWithClient(rc)
	var edgeDao dao.TaskEdgeDAO = NewTaskEdgeDAOWithClient(rc)

	// Verify they satisfy the DAO interface contracts
	_, err := taskDao.Insert(ctx, &model.Task{ID: "sw-1", TaskName: "switch-test", State: "pending", Status: 1})
	require.NoError(t, err)

	_, err = subDao.BatchInsert(ctx, []model.Subtask{
		{ID: "sw-s1", TaskID: "sw-1", TaskName: "s1", State: "pending", Status: 1},
	})
	require.NoError(t, err)

	_, err = edgeDao.BatchInsert(ctx, []model.TaskEdge{
		{ID: "sw-e1", TaskID: "sw-1", FromSubtaskID: "sw-s1", ToSubtaskID: "sw-s1", EdgeType: "control"},
	})
	require.NoError(t, err)

	// Verify Store is accessible through interface
	store := taskDao.GetStore()
	require.NotNil(t, store)

	// Verify GetByIDs works through interface
	tasks, err := taskDao.GetByIDs(ctx, []string{"sw-1"})
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
}

// TestIntegration_GetTaskOutput tests the pattern used by dispatch.GetTaskOutput:
// Check bak first, then fall back to active data.
func TestIntegration_GetTaskOutput(t *testing.T) {
	rc, mr := createTestRedis(t)
	ctx := context.Background()

	taskDao := NewTaskDAOWithClient(rc)
	taskBakDao := NewTaskBakDAOWithClient(rc)
	subDao := NewSubtaskDAOWithClient(rc)
	subBakDao := NewSubtaskBakDAOWithClient(rc)

	// Insert active task with output
	_, _ = taskDao.Insert(ctx, &model.Task{
		ID: "out-1", TaskName: "output-test", State: "succeeded",
		Output: `{"result":"active"}`, Status: 1,
	})
	_, _ = subDao.BatchInsert(ctx, []model.Subtask{
		{ID: "out-s1", TaskID: "out-1", TaskName: "step-1", State: "succeeded",
			Output: `{"step":"active"}`, Status: 1},
	})

	// No backup exists → should get active data
	bak, _ := taskBakDao.GetByID(ctx, "out-1")
	assert.Nil(t, bak, "no backup should exist yet")

	task, _ := taskDao.GetByID(ctx, "out-1")
	require.NotNil(t, task)
	assert.Equal(t, `{"result":"active"}`, task.Output)

	// Simulate backup: manually insert bak data
	mr.HSet("taskx:bak:task:{out-bak}", "id", "out-bak")
	mr.HSet("taskx:bak:task:{out-bak}", "taskName", "backup-test")
	mr.HSet("taskx:bak:task:{out-bak}", "output", `{"result":"backed-up"}`)
	mr.HSet("taskx:bak:task:{out-bak}", "status", "1")

	bakTask, _ := taskBakDao.GetByID(ctx, "out-bak")
	require.NotNil(t, bakTask)
	assert.Equal(t, `{"result":"backed-up"}`, bakTask.Output)

	// Verify subtask bak is also empty for active task
	bakSubs, _ := subBakDao.GetByTaskID(ctx, "out-1")
	assert.Nil(t, bakSubs)
}
