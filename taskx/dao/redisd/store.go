package redisd

import (
	"context"

	"github.com/caiflower/common-tools/taskx/dao"
	v2 "github.com/caiflower/common-tools/redis/v2"
	"github.com/redis/go-redis/v9"
)

// redisStore implements dao.Store for Redis.
// RunInTx uses a pipeline to batch commands and executes them atomically.
type redisStore struct {
	client v2.RedisClient
}

// pipelineKey is the context key for storing the Redis pipeline during RunInTx.
type pipelineKey struct{}

// NewStore creates a Redis-backed Store.
func NewStore(client v2.RedisClient) dao.Store {
	return &redisStore{client: client}
}

// RunInTx executes fn within an atomic pipeline context.
// If a pipeline already exists in the context (nested call), it reuses it.
func (s *redisStore) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	// If already in a pipeline context, reuse it (nested transaction)
	if existing := ctx.Value(pipelineKey{}); existing != nil {
		return fn(ctx)
	}

	// Create a new pipeline
	pipe := cmd(s.client).Pipeline()
	txCtx := context.WithValue(ctx, pipelineKey{}, pipe)

	// Execute the function (collects commands into the pipeline)
	if err := fn(txCtx); err != nil {
		return err
	}

	// Execute all collected commands atomically
	_, err := pipe.Exec(ctx)
	return err
}

// getPipe returns the redis.Pipeliner from context if present, nil otherwise.
func getPipe(ctx context.Context) redis.Pipeliner {
	if p := ctx.Value(pipelineKey{}); p != nil {
		if pipe, ok := p.(redis.Pipeliner); ok {
			return pipe
		}
	}
	return nil
}
