package redisd

import (
	"context"
	"fmt"

	v2 "github.com/caiflower/common-tools/redis/v2"
	"github.com/caiflower/common-tools/taskx/dao"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/redis/go-redis/v9"
)

type taskEdgeDAO struct {
	client v2.RedisClient
	store  dao.Store
	keys   *keyBuilder
}

// NewTaskEdgeDAOWithClient creates a Redis-backed TaskEdgeDAO.
func NewTaskEdgeDAOWithClient(client v2.RedisClient) dao.TaskEdgeDAO {
	return NewTaskEdgeDAOWithConfig(client, nil)
}

// NewTaskEdgeDAOWithConfig creates a Redis-backed TaskEdgeDAO with custom key config.
func NewTaskEdgeDAOWithConfig(client v2.RedisClient, keyCfg *KeyConfig) dao.TaskEdgeDAO {
	return &taskEdgeDAO{
		client: client,
		store:  NewStore(client),
		keys:   newKeyBuilder(keyCfg),
	}
}

func (d *taskEdgeDAO) BatchInsert(ctx context.Context, data []model.TaskEdge) (int64, error) {
	if len(data) == 0 {
		return 0, nil
	}
	c := cmd(d.client)
	pipe := getPipe(ctx)
	if pipe == nil {
		pipe = c.Pipeline()
	}

	for i := range data {
		e := &data[i]
		fields, err := toHash(e)
		if err != nil {
			return 0, fmt.Errorf("edge BatchInsert toHash: %w", err)
		}
		pipe.HSet(ctx, d.keys.edgeKey(e.ID), fields)
		pipe.SAdd(ctx, d.keys.edgeIndexKey(e.TaskID), e.ID)
	}

	// Only exec if we created the pipeline ourselves
	if getPipe(ctx) == nil {
		if _, err := pipe.Exec(ctx); err != nil {
			return 0, fmt.Errorf("edge BatchInsert pipeline: %w", err)
		}
	}
	return int64(len(data)), nil
}

func (d *taskEdgeDAO) GetByTaskID(ctx context.Context, taskID string) ([]model.TaskEdge, error) {
	c := cmd(d.client)

	// Get edge IDs from index set
	ids, err := c.SMembers(ctx, d.keys.edgeIndexKey(taskID)).Result()
	if err != nil {
		return nil, fmt.Errorf("edge GetByTaskID SMembers: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}

	// Batch fetch edge details
	pipe := c.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(ids))
	for i, id := range ids {
		cmds[i] = pipe.HGetAll(ctx, d.keys.edgeKey(id))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("edge GetByTaskID pipeline: %w", err)
	}

	var edges []model.TaskEdge
	for _, hcmd := range cmds {
		m, err := hcmd.Result()
		if err != nil || len(m) == 0 {
			continue
		}
		e := new(model.TaskEdge)
		if err := fromHash(m, e); err != nil {
			continue
		}
		edges = append(edges, *e)
	}
	return edges, nil
}
