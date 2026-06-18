package redisd

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caiflower/common-tools/pkg/basic"
	v2 "github.com/caiflower/common-tools/redis/v2"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestRedis creates a miniredis instance and returns a RedisClient wrapping it.
func createTestRedis(t *testing.T) (v2.RedisClient, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	// Wrap in our v2.RedisClient interface
	rc := &testRedisClient{client: client}
	t.Cleanup(func() {
		client.Close()
		mr.Close()
	})
	return rc, mr
}

// testRedisClient is a minimal v2.RedisClient adapter for testing.
type testRedisClient struct {
	client *redis.Client
}

func (c *testRedisClient) Cmd() v2.Cmdable {
	return &testCmd{Cmdable: c.client}
}
func (c *testRedisClient) GetRedis() redis.Cmdable { return c.client }
func (c *testRedisClient) AddHook(hook redis.Hook) {}
func (c *testRedisClient) Close()                  { c.client.Close() }

type testCmd struct {
	redis.Cmdable
}

func (c *testCmd) Key(key string) string { return key } // no prefix in tests

// --- TaskDAO Tests ---

func TestTaskDAO_InsertAndGetByID(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewTaskDAOWithClient(rc)
	ctx := context.Background()

	task := &model.Task{
		ID:       "task-1",
		TaskName: "demo",
		State:    "pending",
		Worker:   "",
		Status:   1,
	}

	n, err := dao.Insert(ctx, task)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	got, err := dao.GetByID(ctx, "task-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "demo", got.TaskName)
	assert.Equal(t, "pending", got.State)
	assert.Equal(t, int8(1), got.Status)
}

func TestTaskDAO_GetByID_NotFound(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewTaskDAOWithClient(rc)
	ctx := context.Background()

	got, err := dao.GetByID(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestTaskDAO_GetByID_SoftDeleted(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewTaskDAOWithClient(rc)
	ctx := context.Background()

	task := &model.Task{ID: "task-del", TaskName: "deleted", State: "failed", Status: -1}
	_, err := dao.Insert(ctx, task)
	require.NoError(t, err)

	got, err := dao.GetByID(ctx, "task-del")
	require.NoError(t, err)
	assert.Nil(t, got, "soft-deleted task should return nil")
}

func TestTaskDAO_GetByIDs(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewTaskDAOWithClient(rc)
	ctx := context.Background()

	for i, id := range []string{"t1", "t2", "t3"} {
		_, err := dao.Insert(ctx, &model.Task{ID: id, TaskName: id, State: "pending", Status: 1})
		require.NoError(t, err, "insert %d", i)
	}

	tasks, err := dao.GetByIDs(ctx, []string{"t1", "t3", "t-missing"})
	require.NoError(t, err)
	assert.Len(t, tasks, 2)

	ids := map[string]bool{}
	for _, t := range tasks {
		ids[t.ID] = true
	}
	assert.True(t, ids["t1"])
	assert.True(t, ids["t3"])
}

func TestTaskDAO_CASWorkerAndState_Success(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewTaskDAOWithClient(rc)
	ctx := context.Background()

	_, err := dao.Insert(ctx, &model.Task{ID: "cas-1", TaskName: "cas", State: "pending", Worker: "node-a", Status: 1})
	require.NoError(t, err)

	n, err := dao.CASWorkerAndState(ctx, "cas-1", "node-b", "running", "node-a")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	got, _ := dao.GetByID(ctx, "cas-1")
	assert.Equal(t, "node-b", got.Worker)
	assert.Equal(t, "running", got.State)
}

func TestTaskDAO_CASWorkerAndState_Failure(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewTaskDAOWithClient(rc)
	ctx := context.Background()

	_, err := dao.Insert(ctx, &model.Task{ID: "cas-2", TaskName: "cas", State: "pending", Worker: "node-a", Status: 1})
	require.NoError(t, err)

	n, err := dao.CASWorkerAndState(ctx, "cas-2", "node-b", "running", "node-WRONG")
	require.NoError(t, err)
	assert.Equal(t, int64(0), n, "CAS should fail when oldWorker doesn't match")

	got, _ := dao.GetByID(ctx, "cas-2")
	assert.Equal(t, "node-a", got.Worker)
	assert.Equal(t, "pending", got.State)
}

func TestTaskDAO_SetState(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewTaskDAOWithClient(rc)
	ctx := context.Background()

	_, err := dao.Insert(ctx, &model.Task{ID: "ss-1", TaskName: "ss", State: "running", Status: 1})
	require.NoError(t, err)

	n, err := dao.SetState(ctx, "ss-1", "succeeded")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	got, _ := dao.GetByID(ctx, "ss-1")
	assert.Equal(t, "succeeded", got.State)
}

func TestTaskDAO_GetTodoTask(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewTaskDAOWithClient(rc)
	ctx := context.Background()

	// Insert tasks with different states
	_, _ = dao.Insert(ctx, &model.Task{ID: "todo-1", TaskName: "t1", State: "pending", Status: 1})
	_, _ = dao.Insert(ctx, &model.Task{ID: "todo-2", TaskName: "t2", State: "running", Status: 1})
	_, _ = dao.Insert(ctx, &model.Task{ID: "todo-3", TaskName: "t3", State: "succeeded", Status: 1})

	// Use a future time to ensure all tasks (score=0) are within range
	futureTime := basic.NewFromTime(time.Now().Add(time.Hour))
	tasks, err := dao.GetTodoTask(ctx, []string{"pending", "running"}, futureTime)
	require.NoError(t, err)
	// Only pending and running should pass state filter
	assert.GreaterOrEqual(t, len(tasks), 2)
}

// --- SubtaskDAO Tests ---

func TestSubtaskDAO_BatchInsertAndGetByTaskID(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewSubtaskDAOWithClient(rc)
	ctx := context.Background()

	subtasks := []model.Subtask{
		{ID: "s1", TaskID: "t1", TaskName: "step-1", State: "pending", Status: 1},
		{ID: "s2", TaskID: "t1", TaskName: "step-2", State: "pending", Status: 1},
		{ID: "s3", TaskID: "t2", TaskName: "other", State: "pending", Status: 1},
	}

	n, err := dao.BatchInsert(ctx, subtasks)
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)

	got, err := dao.GetByTaskID(ctx, "t1")
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestSubtaskDAO_GetByID(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewSubtaskDAOWithClient(rc)
	ctx := context.Background()

	_, _ = dao.BatchInsert(ctx, []model.Subtask{
		{ID: "s-get", TaskID: "t1", TaskName: "findme", State: "running", Status: 1},
	})

	got, err := dao.GetByID(ctx, "s-get")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "findme", got.TaskName)
}

func TestSubtaskDAO_CASWorkerAndState(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewSubtaskDAOWithClient(rc)
	ctx := context.Background()

	_, _ = dao.BatchInsert(ctx, []model.Subtask{
		{ID: "scas", TaskID: "t1", TaskName: "cas", State: "pending", Worker: "w1", Status: 1},
	})

	n, err := dao.CASWorkerAndState(ctx, "scas", "w2", "running", "w1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	got, _ := dao.GetByID(ctx, "scas")
	assert.Equal(t, "w2", got.Worker)
	assert.Equal(t, "running", got.State)
}

func TestSubtaskDAO_CASWorkerAndRollback(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewSubtaskDAOWithClient(rc)
	ctx := context.Background()

	_, _ = dao.BatchInsert(ctx, []model.Subtask{
		{ID: "srb", TaskID: "t1", TaskName: "rb", State: "running", Worker: "w1", Status: 1},
	})

	n, err := dao.CASWorkerAndRollback(ctx, "srb", "w2", "rollback_all", "w1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	got, _ := dao.GetByID(ctx, "srb")
	assert.Equal(t, "w2", got.Worker)
	assert.Equal(t, "rollback_all", got.Rollback)
}

func TestSubtaskDAO_SetOutputAndState(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewSubtaskDAOWithClient(rc)
	ctx := context.Background()

	_, _ = dao.BatchInsert(ctx, []model.Subtask{
		{ID: "sout", TaskID: "t1", TaskName: "out", State: "running", Status: 1},
	})

	err := dao.SetOutputAndState(ctx, "sout", `result-ok`, "succeeded")
	require.NoError(t, err)

	got, err := dao.GetByID(ctx, "sout")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "succeeded", got.State)
}

func TestSubtaskDAO_SetRetry(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewSubtaskDAOWithClient(rc)
	ctx := context.Background()

	_, _ = dao.BatchInsert(ctx, []model.Subtask{
		{ID: "sretry", TaskID: "t1", TaskName: "retry", State: "failed", Status: 1, Retry: 3},
	})

	err := dao.SetRetry(ctx, "sretry", 2)
	require.NoError(t, err)

	got, _ := dao.GetByID(ctx, "sretry")
	assert.Equal(t, "pending", got.State)
}

func TestSubtaskDAO_SetInput(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewSubtaskDAOWithClient(rc)
	ctx := context.Background()

	_, _ = dao.BatchInsert(ctx, []model.Subtask{
		{ID: "sin", TaskID: "t1", TaskName: "inp", State: "pending", Status: 1},
	})

	err := dao.SetInput(ctx, "sin", `{"key":"value"}`)
	require.NoError(t, err)
}

func TestSubtaskDAO_GetByIDs(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewSubtaskDAOWithClient(rc)
	ctx := context.Background()

	_, _ = dao.BatchInsert(ctx, []model.Subtask{
		{ID: "s1", TaskID: "t1", TaskName: "a", State: "pending", Status: 1},
		{ID: "s2", TaskID: "t1", TaskName: "b", State: "pending", Status: 1},
	})

	got, err := dao.GetByIDs(ctx, []string{"s1", "s2", "missing"})
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

// --- TaskEdgeDAO Tests ---

func TestTaskEdgeDAO_BatchInsertAndGetByTaskID(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewTaskEdgeDAOWithClient(rc)
	ctx := context.Background()

	edges := []model.TaskEdge{
		{ID: "e1", TaskID: "t1", FromSubtaskID: "s1", ToSubtaskID: "s2", EdgeType: "control"},
		{ID: "e2", TaskID: "t1", FromSubtaskID: "s2", ToSubtaskID: "s3", EdgeType: "data"},
	}

	n, err := dao.BatchInsert(ctx, edges)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)

	got, err := dao.GetByTaskID(ctx, "t1")
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestTaskEdgeDAO_GetByTaskID_Empty(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewTaskEdgeDAOWithClient(rc)
	ctx := context.Background()

	got, err := dao.GetByTaskID(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

// --- TaskBakDAO Tests ---

func TestTaskBakDAO_GetByID_NotFound(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewTaskBakDAOWithClient(rc)
	ctx := context.Background()

	got, err := dao.GetByID(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

// --- SubtaskBakDAO Tests ---

func TestSubtaskBakDAO_GetByTaskID_Empty(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewSubtaskBakDAOWithClient(rc)
	ctx := context.Background()

	got, err := dao.GetByTaskID(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

// --- Store Tests ---

func TestStore_RunInTx(t *testing.T) {
	rc, _ := createTestRedis(t)
	store := NewStore(rc)
	ctx := context.Background()

	taskDao := NewTaskDAOWithClient(rc)

	err := store.RunInTx(ctx, func(ctx context.Context) error {
		_, err := taskDao.Insert(ctx, &model.Task{ID: "tx-1", TaskName: "tx-task", State: "pending", Status: 1})
		if err != nil {
			return err
		}
		_, err = taskDao.Insert(ctx, &model.Task{ID: "tx-2", TaskName: "tx-task-2", State: "pending", Status: 1})
		return err
	})
	require.NoError(t, err)

	// Verify both tasks were inserted
	got1, _ := taskDao.GetByID(ctx, "tx-1")
	got2, _ := taskDao.GetByID(ctx, "tx-2")
	assert.NotNil(t, got1)
	assert.NotNil(t, got2)
}

// --- BatchInsert Empty ---

func TestSubtaskDAO_BatchInsert_Empty(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewSubtaskDAOWithClient(rc)
	ctx := context.Background()

	n, err := dao.BatchInsert(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
}

func TestTaskEdgeDAO_BatchInsert_Empty(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewTaskEdgeDAOWithClient(rc)
	ctx := context.Background()

	n, err := dao.BatchInsert(ctx, []model.TaskEdge{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
}

// --- Hash Tag Tests (2.5.3) ---

func TestKeyBuilder_HashTagConsistency(t *testing.T) {
	kb := newKeyBuilder(nil)
	taskID := "abc-123"

	// All keys for the same task must contain {taskID} hash tag
	taskKey := kb.taskKey(taskID)
	subtaskIdx := kb.subtaskIndexKey(taskID)
	edgeIdx := kb.edgeIndexKey(taskID)
	bakTaskKey := kb.bakTaskKey(taskID)
	bakSubtaskIdx := kb.bakSubtaskIndexKey(taskID)

	tag := "{" + taskID + "}"
	assert.Contains(t, taskKey, tag, "task key must contain hash tag")
	assert.Contains(t, subtaskIdx, tag, "subtask index key must contain hash tag")
	assert.Contains(t, edgeIdx, tag, "edge index key must contain hash tag")
	assert.Contains(t, bakTaskKey, tag, "bak task key must contain hash tag")
	assert.Contains(t, bakSubtaskIdx, tag, "bak subtask index key must contain hash tag")
}

func TestKeyBuilder_HashTagExtractedCorrectly(t *testing.T) {
	kb := newKeyBuilder(nil)

	// Subtask and edge keys should use their own ID but still be in the
	// same slot when accessed via the task index (which uses {taskID})
	// The subtask key uses {subtaskID}, not {taskID}, because subtasks
	// are accessed individually. The index set uses {taskID} for co-location.
	subKey := kb.subtaskKey("sub-1")
	assert.Contains(t, subKey, "{sub-1}")

	edgeKey := kb.edgeKey("edge-1")
	assert.Contains(t, edgeKey, "{edge-1}")
}

// --- Lua Script Tests (2.5.2) ---

func TestLuaScript_CASWorkerAndState_ConcurrentSimulation(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewTaskDAOWithClient(rc)
	ctx := context.Background()

	// Insert a task
	_, _ = dao.Insert(ctx, &model.Task{ID: "lua-1", TaskName: "lua", State: "pending", Worker: "w1", Status: 1})

	// First CAS succeeds
	n1, _ := dao.CASWorkerAndState(ctx, "lua-1", "w2", "running", "w1")
	assert.Equal(t, int64(1), n1)

	// Second CAS with old worker fails (already changed to w2)
	n2, _ := dao.CASWorkerAndState(ctx, "lua-1", "w3", "failed", "w1")
	assert.Equal(t, int64(0), n2)

	// Third CAS with correct current worker succeeds
	n3, _ := dao.CASWorkerAndState(ctx, "lua-1", "w3", "succeeded", "w2")
	assert.Equal(t, int64(1), n3)

	// Verify final state
	got, _ := dao.GetByID(ctx, "lua-1")
	assert.Equal(t, "w3", got.Worker)
	assert.Equal(t, "succeeded", got.State)
}

func TestLuaScript_CASWorkerAndRollback_ConcurrentSimulation(t *testing.T) {
	rc, _ := createTestRedis(t)
	dao := NewSubtaskDAOWithClient(rc)
	ctx := context.Background()

	_, _ = dao.BatchInsert(ctx, []model.Subtask{
		{ID: "lua-rb", TaskID: "t1", TaskName: "rb", State: "running", Worker: "w1", Status: 1},
	})

	// First CAS succeeds
	n1, _ := dao.CASWorkerAndRollback(ctx, "lua-rb", "w2", "rollback_all", "w1")
	assert.Equal(t, int64(1), n1)

	// Second CAS fails (worker already changed)
	n2, _ := dao.CASWorkerAndRollback(ctx, "lua-rb", "w3", "none", "w1")
	assert.Equal(t, int64(0), n2)

	got, _ := dao.GetByID(ctx, "lua-rb")
	assert.Equal(t, "w2", got.Worker)
	assert.Equal(t, "rollback_all", got.Rollback)
}

func TestStore_RunInTx_AtomicBatch(t *testing.T) {
	rc, _ := createTestRedis(t)
	store := NewStore(rc)
	taskDao := NewTaskDAOWithClient(rc)
	subDao := NewSubtaskDAOWithClient(rc)
	edgeDao := NewTaskEdgeDAOWithClient(rc)
	ctx := context.Background()

	// Simulate SubmitTask: insert task + subtasks + edges atomically
	err := store.RunInTx(ctx, func(ctx context.Context) error {
		if _, err := taskDao.Insert(ctx, &model.Task{ID: "atomic-1", TaskName: "atomic", State: "pending", Status: 1}); err != nil {
			return err
		}
		if _, err := subDao.BatchInsert(ctx, []model.Subtask{
			{ID: "as-1", TaskID: "atomic-1", TaskName: "s1", State: "pending", Status: 1},
			{ID: "as-2", TaskID: "atomic-1", TaskName: "s2", State: "pending", Status: 1},
		}); err != nil {
			return err
		}
		if _, err := edgeDao.BatchInsert(ctx, []model.TaskEdge{
			{ID: "ae-1", TaskID: "atomic-1", FromSubtaskID: "as-1", ToSubtaskID: "as-2", EdgeType: "control"},
		}); err != nil {
			return err
		}
		return nil
	})
	require.NoError(t, err)

	// Verify all data was written
	task, _ := taskDao.GetByID(ctx, "atomic-1")
	require.NotNil(t, task)
	assert.Equal(t, "atomic", task.TaskName)

	subs, _ := subDao.GetByTaskID(ctx, "atomic-1")
	assert.Len(t, subs, 2)

	edges, _ := edgeDao.GetByTaskID(ctx, "atomic-1")
	assert.Len(t, edges, 1)
}
