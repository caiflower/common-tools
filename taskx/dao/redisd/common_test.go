package redisd

import (
	"testing"

	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/stretchr/testify/assert"
)

func TestKeyBuilder_DefaultPrefix(t *testing.T) {
	kb := newKeyBuilder(nil)
	assert.Equal(t, "taskx:task:{t1}", kb.taskKey("t1"))
	assert.Equal(t, "taskx:subtask:{s1}", kb.subtaskKey("s1"))
	assert.Equal(t, "taskx:edge:{e1}", kb.edgeKey("e1"))
	assert.Equal(t, "taskx:bak:task:{t1}", kb.bakTaskKey("t1"))
	assert.Equal(t, "taskx:bak:subtask:{s1}", kb.bakSubtaskKey("s1"))
	assert.Equal(t, "taskx:todo", kb.todoSetKey())
	assert.Equal(t, "taskx:task:{t1}:subtasks", kb.subtaskIndexKey("t1"))
	assert.Equal(t, "taskx:task:{t1}:edges", kb.edgeIndexKey("t1"))
	assert.Equal(t, "taskx:bak:task:{t1}:subtasks", kb.bakSubtaskIndexKey("t1"))
}

func TestKeyBuilder_CustomPrefix(t *testing.T) {
	kb := newKeyBuilder(&KeyConfig{Prefix: "myapp"})
	assert.Equal(t, "myapp:task:{t1}", kb.taskKey("t1"))
	assert.Equal(t, "myapp:todo", kb.todoSetKey())
}

func TestKeyConfig_Normalize(t *testing.T) {
	var nilCfg *KeyConfig
	assert.Equal(t, DefaultKeyPrefix, nilCfg.Normalize().Prefix)

	emptyCfg := &KeyConfig{}
	assert.Equal(t, DefaultKeyPrefix, emptyCfg.Normalize().Prefix)

	customCfg := &KeyConfig{Prefix: "custom"}
	assert.Equal(t, "custom", customCfg.Normalize().Prefix)
}

func TestToHashFromHash_Task(t *testing.T) {
	task := &model.Task{
		ID:        "task-1",
		TaskName:  "demo-task",
		State:     "pending",
		Worker:    "node-1",
		Retry:     3,
		Status:    1,
		Urgent:    true,
	}

	hash, err := toHash(task)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Equal(t, "task-1", hash["id"])
	assert.Equal(t, "demo-task", hash["taskName"])
	assert.Equal(t, "pending", hash["state"])
	assert.Equal(t, "node-1", hash["worker"])
	assert.Equal(t, "3", hash["retry"])
	assert.Equal(t, "1", hash["status"])
	assert.Equal(t, "true", hash["urgent"])

	// Round-trip
	restored := new(model.Task)
	err = fromHash(hash, restored)
	assert.NoError(t, err)
	assert.Equal(t, task.ID, restored.ID)
	assert.Equal(t, task.TaskName, restored.TaskName)
	assert.Equal(t, task.State, restored.State)
	assert.Equal(t, task.Worker, restored.Worker)
	assert.Equal(t, task.Retry, restored.Retry)
	assert.Equal(t, task.Status, restored.Status)
	assert.Equal(t, task.Urgent, restored.Urgent)
}

func TestToHashFromHash_Subtask(t *testing.T) {
	subtask := &model.Subtask{
		ID:       "sub-1",
		TaskID:   "task-1",
		TaskName: "step-1",
		State:    "running",
		Worker:   "node-2",
		Priority: 5,
		Timeout:  30,
		Status:   1,
	}

	hash, err := toHash(subtask)
	assert.NoError(t, err)
	assert.Equal(t, "sub-1", hash["id"])
	assert.Equal(t, "task-1", hash["taskID"])
	assert.Equal(t, "5", hash["priority"])
	assert.Equal(t, "30", hash["timeout"])

	restored := new(model.Subtask)
	err = fromHash(hash, restored)
	assert.NoError(t, err)
	assert.Equal(t, subtask.ID, restored.ID)
	assert.Equal(t, subtask.TaskID, restored.TaskID)
	assert.Equal(t, subtask.Priority, restored.Priority)
	assert.Equal(t, subtask.Timeout, restored.Timeout)
}

func TestToHashFromHash_TaskEdge(t *testing.T) {
	edge := &model.TaskEdge{
		ID:            "edge-1",
		TaskID:        "task-1",
		FromSubtaskID: "sub-1",
		ToSubtaskID:   "sub-2",
		EdgeType:      "control+data",
	}

	hash, err := toHash(edge)
	assert.NoError(t, err)
	assert.Equal(t, "edge-1", hash["id"])
	assert.Equal(t, "control+data", hash["edgeType"])

	restored := new(model.TaskEdge)
	err = fromHash(hash, restored)
	assert.NoError(t, err)
	assert.Equal(t, edge.ID, restored.ID)
	assert.Equal(t, edge.FromSubtaskID, restored.FromSubtaskID)
	assert.Equal(t, edge.ToSubtaskID, restored.ToSubtaskID)
}

func TestFromHash_EmptyMap(t *testing.T) {
	task := new(model.Task)
	err := fromHash(map[string]string{}, task)
	// Empty map should produce zero-value struct, not error
	assert.NoError(t, err)
	assert.Equal(t, "", task.ID)
}

func TestParseHelpers(t *testing.T) {
	assert.Equal(t, int8(3), parseInt8("3"))
	assert.Equal(t, int8(0), parseInt8("invalid"))
	assert.Equal(t, 42, parseInt("42"))
	assert.Equal(t, 0, parseInt("bad"))
	assert.Equal(t, int32(100), parseInt32("100"))
	assert.Equal(t, int32(0), parseInt32("nope"))
	assert.True(t, parseBool("true"))
	assert.False(t, parseBool("false"))
	assert.False(t, parseBool(""))
}
