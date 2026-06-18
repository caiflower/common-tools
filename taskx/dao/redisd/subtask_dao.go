package redisd

import (
	"context"
	"fmt"

	v2 "github.com/caiflower/common-tools/redis/v2"
	"github.com/caiflower/common-tools/taskx/dao"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/redis/go-redis/v9"
)

type subtaskDAO struct {
	client v2.RedisClient
	store  dao.Store
	keys   *keyBuilder
}

// NewSubtaskDAOWithClient creates a Redis-backed SubtaskDAO.
func NewSubtaskDAOWithClient(client v2.RedisClient) dao.SubtaskDAO {
	return NewSubtaskDAOWithConfig(client, nil)
}

// NewSubtaskDAOWithConfig creates a Redis-backed SubtaskDAO with custom key config.
func NewSubtaskDAOWithConfig(client v2.RedisClient, keyCfg *KeyConfig) dao.SubtaskDAO {
	return &subtaskDAO{
		client: client,
		store:  NewStore(client),
		keys:   newKeyBuilder(keyCfg),
	}
}

func (d *subtaskDAO) BatchInsert(ctx context.Context, data []model.Subtask) (int64, error) {
	if len(data) == 0 {
		return 0, nil
	}
	c := cmd(d.client)
	pipe := getPipe(ctx)
	if pipe == nil {
		pipe = c.Pipeline()
	}

	for i := range data {
		s := &data[i]
		fields, err := toHash(s)
		if err != nil {
			return 0, fmt.Errorf("subtask BatchInsert toHash: %w", err)
		}
		pipe.HSet(ctx, d.keys.subtaskKey(s.ID), fields)
		pipe.SAdd(ctx, d.keys.subtaskIndexKey(s.TaskID), s.ID)
	}

	// Only exec if we created the pipeline ourselves
	if getPipe(ctx) == nil {
		if _, err := pipe.Exec(ctx); err != nil {
			return 0, fmt.Errorf("subtask BatchInsert pipeline: %w", err)
		}
	}
	return int64(len(data)), nil
}

func (d *subtaskDAO) GetByID(ctx context.Context, id string) (*model.Subtask, error) {
	key := d.keys.subtaskKey(id)
	c := cmd(d.client)

	m, err := c.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("subtask GetByID HGetAll: %w", err)
	}
	if len(m) == 0 {
		return nil, nil
	}
	s := new(model.Subtask)
	if err := fromHash(m, s); err != nil {
		return nil, fmt.Errorf("subtask GetByID fromHash: %w", err)
	}
	if s.Status == -1 {
		return nil, nil
	}
	return s, nil
}

func (d *subtaskDAO) GetByTaskID(ctx context.Context, taskID string) ([]model.Subtask, error) {
	c := cmd(d.client)

	// Get subtask IDs from index set
	ids, err := c.SMembers(ctx, d.keys.subtaskIndexKey(taskID)).Result()
	if err != nil {
		return nil, fmt.Errorf("subtask GetByTaskID SMembers: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}

	// Batch fetch subtask details
	pipe := c.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(ids))
	for i, id := range ids {
		cmds[i] = pipe.HGetAll(ctx, d.keys.subtaskKey(id))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("subtask GetByTaskID pipeline: %w", err)
	}

	var subtasks []model.Subtask
	for _, hcmd := range cmds {
		m, err := hcmd.Result()
		if err != nil || len(m) == 0 {
			continue
		}
		s := new(model.Subtask)
		if err := fromHash(m, s); err != nil {
			continue
		}
		if s.Status > 0 {
			subtasks = append(subtasks, *s)
		}
	}
	return subtasks, nil
}

func (d *subtaskDAO) CASWorkerAndState(ctx context.Context, id string, worker, state string, oldWorker string) (int64, error) {
	c := cmd(d.client)
	result, err := casWorkerAndState.Run(ctx, c, []string{d.keys.subtaskKey(id)}, oldWorker, worker, state).Int64()
	if err != nil {
		return 0, fmt.Errorf("subtask CASWorkerAndState: %w", err)
	}
	return result, nil
}

func (d *subtaskDAO) CASWorkerAndRollback(ctx context.Context, id string, worker, rollback string, oldWorker string) (int64, error) {
	c := cmd(d.client)
	result, err := casWorkerAndRollback.Run(ctx, c, []string{d.keys.subtaskKey(id)}, oldWorker, worker, rollback).Int64()
	if err != nil {
		return 0, fmt.Errorf("subtask CASWorkerAndRollback: %w", err)
	}
	return result, nil
}

func (d *subtaskDAO) GetByIDs(ctx context.Context, ids []string) ([]model.Subtask, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	c := cmd(d.client)
	pipe := c.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(ids))
	for i, id := range ids {
		cmds[i] = pipe.HGetAll(ctx, d.keys.subtaskKey(id))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("subtask GetByIDs pipeline: %w", err)
	}

	var subtasks []model.Subtask
	for _, hcmd := range cmds {
		m, err := hcmd.Result()
		if err != nil || len(m) == 0 {
			continue
		}
		s := new(model.Subtask)
		if err := fromHash(m, s); err != nil {
			continue
		}
		if s.Status > 0 {
			subtasks = append(subtasks, *s)
		}
	}
	return subtasks, nil
}

func (d *subtaskDAO) SetOutputAndState(ctx context.Context, id string, output, state string) error {
	key := d.keys.subtaskKey(id)
	c := cmd(d.client)
	pipe := getPipe(ctx)
	fields := map[string]interface{}{
		"output":        output,
		"state":         state,
		"last_run_time": nowTime(),
	}
	if pipe != nil {
		pipe.HSet(ctx, key, fields)
	} else {
		c.HSet(ctx, key, fields)
	}
	return nil
}

func (d *subtaskDAO) SetRollbackAndState(ctx context.Context, id, rollback, output string) error {
	key := d.keys.subtaskKey(id)
	c := cmd(d.client)
	pipe := getPipe(ctx)
	fields := map[string]interface{}{
		"rollback":      rollback,
		"output":        output,
		"state":         rollback,
		"last_run_time": nowTime(),
	}
	if pipe != nil {
		pipe.HSet(ctx, key, fields)
	} else {
		c.HSet(ctx, key, fields)
	}
	return nil
}

func (d *subtaskDAO) SetInput(ctx context.Context, id, input string) error {
	key := d.keys.subtaskKey(id)
	c := cmd(d.client)
	pipe := getPipe(ctx)
	if pipe != nil {
		pipe.HSet(ctx, key, "input", input)
	} else {
		c.HSet(ctx, key, "input", input)
	}
	return nil
}

func (d *subtaskDAO) SetRetry(ctx context.Context, id string, retry int8) error {
	key := d.keys.subtaskKey(id)
	c := cmd(d.client)
	pipe := getPipe(ctx)
	fields := map[string]interface{}{
		"retry":         retry,
		"state":         "pending",
		"last_run_time": nowTime(),
	}
	if pipe != nil {
		pipe.HSet(ctx, key, fields)
	} else {
		c.HSet(ctx, key, fields)
	}
	return nil
}
