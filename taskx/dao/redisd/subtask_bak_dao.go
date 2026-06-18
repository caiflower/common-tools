package redisd

import (
	"context"
	"fmt"

	v2 "github.com/caiflower/common-tools/redis/v2"
	"github.com/caiflower/common-tools/taskx/dao"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/redis/go-redis/v9"
)

type subtaskBakDAO struct {
	client v2.RedisClient
	store  dao.Store
	keys   *keyBuilder
}

// NewSubtaskBakDAOWithClient creates a Redis-backed SubtaskBakDAO.
func NewSubtaskBakDAOWithClient(client v2.RedisClient) dao.SubtaskBakDAO {
	return NewSubtaskBakDAOWithConfig(client, nil)
}

// NewSubtaskBakDAOWithConfig creates a Redis-backed SubtaskBakDAO with custom key config.
func NewSubtaskBakDAOWithConfig(client v2.RedisClient, keyCfg *KeyConfig) dao.SubtaskBakDAO {
	return &subtaskBakDAO{
		client: client,
		store:  NewStore(client),
		keys:   newKeyBuilder(keyCfg),
	}
}

func (d *subtaskBakDAO) GetByTaskID(ctx context.Context, taskID string) ([]model.SubtaskBak, error) {
	c := cmd(d.client)

	// Get backup subtask IDs from bak index set
	ids, err := c.SMembers(ctx, d.keys.bakSubtaskIndexKey(taskID)).Result()
	if err != nil {
		return nil, fmt.Errorf("subtaskBak GetByTaskID SMembers: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}

	// Batch fetch backup subtask details
	pipe := c.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(ids))
	for i, id := range ids {
		cmds[i] = pipe.HGetAll(ctx, d.keys.bakSubtaskKey(id))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("subtaskBak GetByTaskID pipeline: %w", err)
	}

	var subtasks []model.SubtaskBak
	for _, hcmd := range cmds {
		m, err := hcmd.Result()
		if err != nil || len(m) == 0 {
			continue
		}
		s := new(model.SubtaskBak)
		if err := fromHash(m, s); err != nil {
			continue
		}
		if s.Status > 0 {
			subtasks = append(subtasks, *s)
		}
	}
	return subtasks, nil
}
