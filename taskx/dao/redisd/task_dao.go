package redisd

import (
	"context"
	"fmt"
	"time"

	"github.com/caiflower/common-tools/pkg/basic"
	v2 "github.com/caiflower/common-tools/redis/v2"
	"github.com/caiflower/common-tools/taskx/dao"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/redis/go-redis/v9"
)

type taskDAO struct {
	client v2.RedisClient
	store  dao.Store
	keys   *keyBuilder
}

// NewTaskDAOWithClient creates a Redis-backed TaskDAO.
func NewTaskDAOWithClient(client v2.RedisClient) dao.TaskDAO {
	return NewTaskDAOWithConfig(client, nil)
}

// NewTaskDAOWithConfig creates a Redis-backed TaskDAO with custom key config.
func NewTaskDAOWithConfig(client v2.RedisClient, keyCfg *KeyConfig) dao.TaskDAO {
	return &taskDAO{
		client: client,
		store:  NewStore(client),
		keys:   newKeyBuilder(keyCfg),
	}
}

func (d *taskDAO) GetStore() dao.Store { return d.store }

func (d *taskDAO) Insert(ctx context.Context, data *model.Task) (int64, error) {
	fields, err := toHash(data)
	if err != nil {
		return 0, fmt.Errorf("task Insert toHash: %w", err)
	}
	key := d.keys.taskKey(data.ID)
	c := cmd(d.client)

	pipe := getPipe(ctx)
	if pipe != nil {
		pipe.HSet(ctx, key, fields)
		pipe.ZAdd(ctx, d.keys.todoSetKey(), redis.Z{Score: taskScore(data), Member: data.ID})
		return 1, nil
	}
	c.HSet(ctx, key, fields)
	c.ZAdd(ctx, d.keys.todoSetKey(), redis.Z{Score: taskScore(data), Member: data.ID})
	return 1, nil
}

func (d *taskDAO) GetByID(ctx context.Context, id string) (*model.Task, error) {
	key := d.keys.taskKey(id)
	c := cmd(d.client)

	m, err := c.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("task GetByID HGetAll: %w", err)
	}
	if len(m) == 0 {
		return nil, nil
	}
	task := new(model.Task)
	if err := fromHash(m, task); err != nil {
		return nil, fmt.Errorf("task GetByID fromHash: %w", err)
	}
	if task.Status == -1 {
		return nil, nil
	}
	return task, nil
}

func (d *taskDAO) GetByIDs(ctx context.Context, taskIDs []string) ([]model.Task, error) {
	if len(taskIDs) == 0 {
		return nil, nil
	}
	c := cmd(d.client)
	pipe := c.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(taskIDs))
	for i, id := range taskIDs {
		cmds[i] = pipe.HGetAll(ctx, d.keys.taskKey(id))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("task GetByIDs pipeline: %w", err)
	}

	var tasks []model.Task
	for _, cmd := range cmds {
		m, err := cmd.Result()
		if err != nil || len(m) == 0 {
			continue
		}
		task := new(model.Task)
		if err := fromHash(m, task); err != nil {
			continue
		}
		if task.Status > 0 {
			tasks = append(tasks, *task)
		}
	}
	return tasks, nil
}

func (d *taskDAO) GetTodoTask(ctx context.Context, taskState []string, t basic.Time) ([]model.Task, error) {
	c := cmd(d.client)
	score := float64(t.Time().Unix())

	// Get task IDs from sorted set with score <= time
	ids, err := c.ZRangeByScore(ctx, d.keys.todoSetKey(), &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%f", score),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("task GetTodoTask ZRangeByScore: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}

	// Build state set for filtering
	stateSet := make(map[string]bool, len(taskState))
	for _, s := range taskState {
		stateSet[s] = true
	}

	// Batch fetch task details
	pipe := c.Pipeline()
	hashCmds := make([]*redis.MapStringStringCmd, len(ids))
	for i, id := range ids {
		hashCmds[i] = pipe.HGetAll(ctx, d.keys.taskKey(id))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("task GetTodoTask pipeline: %w", err)
	}

	var tasks []model.Task
	for _, hcmd := range hashCmds {
		m, err := hcmd.Result()
		if err != nil || len(m) == 0 {
			continue
		}
		task := new(model.Task)
		if err := fromHash(m, task); err != nil {
			continue
		}
		if task.Status > 0 && stateSet[task.State] {
			tasks = append(tasks, *task)
		}
	}
	return tasks, nil
}

func (d *taskDAO) CASWorkerAndState(ctx context.Context, taskID string, worker, state string, oldWorker string) (int64, error) {
	c := cmd(d.client)
	result, err := casWorkerAndState.Run(ctx, c, []string{d.keys.taskKey(taskID)}, oldWorker, worker, state).Int64()
	if err != nil {
		return 0, fmt.Errorf("task CASWorkerAndState: %w", err)
	}
	return result, nil
}

func (d *taskDAO) SetState(ctx context.Context, id string, state string) (int64, error) {
	key := d.keys.taskKey(id)
	c := cmd(d.client)

	pipe := getPipe(ctx)
	if pipe != nil {
		pipe.HSet(ctx, key, "state", state)
	} else {
		c.HSet(ctx, key, "state", state)
	}

	// Remove from todo set if terminal state
	if state == "succeeded" || state == "failed" {
		if pipe != nil {
			pipe.ZRem(ctx, d.keys.todoSetKey(), id)
		} else {
			c.ZRem(ctx, d.keys.todoSetKey(), id)
		}
	}
	return 1, nil
}

// taskScore returns the sorted set score for a task (execute_time as unix timestamp).
func taskScore(t *model.Task) float64 {
	if t.ExecuteTime.IsZero() {
		return 0
	}
	return float64(t.ExecuteTime.Time().Unix())
}

// nowTime returns the current time formatted for last_run_time field.
func nowTime() string {
	t := basic.NewFromTime(time.Now())
	b, _ := t.MarshalJSON()
	return string(b)
}
