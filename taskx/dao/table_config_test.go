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

package dao

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/stretchr/testify/assert"
)

// createTestDB returns a SQLite-backed dbv1.Client initialised with the
// taskx schema. The caller is responsible for closing the client.
func createTestDB(t *testing.T) *dbv1.Client {
	t.Helper()
	dir := t.TempDir()
	cfg := dbv1.Config{
		Dialect: "sqlite",
		Url:     filepath.Join(dir, "test.db"),
	}
	client, err := dbv1.NewDBClient(cfg)
	if err != nil {
		t.Fatalf("create db client: %v", err)
	}

	// Load schema. The path is relative to the package directory.
	wd, _ := os.Getwd()
	schemaPath := filepath.Join(wd, "sql", "table-sqlite.sql")
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if _, err := client.DB.ExecContext(context.Background(), string(data)); err != nil {
		t.Fatalf("exec schema: %v", err)
	}
	return client
}

// cloneTableSchema creates a new table with the same column list as src by
// issuing `CREATE TABLE dst AS SELECT * FROM src WHERE 0`. CREATE TABLE AS
// does not preserve PRIMARY KEY / NOT NULL, so we add a UNIQUE index on id
// to keep bun's `id,pk,notnull` declaration happy.
func cloneTableSchema(t *testing.T, client *dbv1.Client, src, dst string) {
	t.Helper()
	ctx := context.Background()
	stmt := "CREATE TABLE " + dst + " AS SELECT * FROM " + src + " WHERE 0"
	if _, err := client.DB.ExecContext(ctx, stmt); err != nil {
		t.Fatalf("clone table %s -> %s: %v", src, dst, err)
	}
	// SQLite's CREATE TABLE AS does not preserve PRIMARY KEY / NOT NULL,
	// which the DAO relies on (the bun model declares id as pk,notnull).
	// We work around by adding a primary-key index and trust the test data
	// to never set id=NULL.
	if _, err := client.DB.ExecContext(ctx, "CREATE UNIQUE INDEX IF NOT EXISTS idx_"+dst+"_id ON "+dst+"(id)"); err != nil {
		t.Fatalf("create pk index for %s: %v", dst, err)
	}
}

func TestTableConfig_DefaultValues(t *testing.T) {
	cfg := DefaultTableConfig()
	assert.Equal(t, "task", cfg.Task)
	assert.Equal(t, "subtask", cfg.Subtask)
	assert.Equal(t, "task_bak", cfg.TaskBak)
	assert.Equal(t, "subtask_bak", cfg.SubtaskBak)
	assert.Equal(t, "task_edge", cfg.TaskEdge)
}

func TestTableConfig_NormalizeFillsEmpty(t *testing.T) {
	cfg := &TableConfig{Task: "custom_task"}
	out := cfg.Normalize()
	assert.Equal(t, "custom_task", out.Task)
	assert.Equal(t, "subtask", out.Subtask)
	assert.Equal(t, "task_bak", out.TaskBak)
	assert.Equal(t, "subtask_bak", out.SubtaskBak)
	assert.Equal(t, "task_edge", out.TaskEdge)
}

func TestTableConfig_NormalizeNilReceiver(t *testing.T) {
	var cfg *TableConfig
	out := cfg.Normalize()
	assert.Equal(t, "task", out.Task)
	assert.Equal(t, "task_edge", out.TaskEdge)
}

func TestTaskDAO_DefaultTableName(t *testing.T) {
	client := createTestDB(t)
	defer client.Close()
	dao := NewTaskDAOWithClient(client)

	bean := &model.Task{ID: "t-default", TaskName: "demo", State: "pending", Status: 1}
	_, err := dao.Insert(context.Background(), bean)
	assert.NoError(t, err)

	got, err := dao.GetByID(context.Background(), "t-default")
	assert.NoError(t, err)
	if assert.NotNil(t, got) {
		assert.Equal(t, "demo", got.TaskName)
	}
}

// TestTaskDAO_CustomTableName writes to a shadow table with the same schema
// as `task`, but a different physical name. The DAO is constructed with the
// shadow name; the row should appear there, not in the default `task` table.
func TestTaskDAO_CustomTableName(t *testing.T) {
	client := createTestDB(t)
	defer client.Close()
	ctx := context.Background()
	cloneTableSchema(t, client, "task", "task_custom")

	defaultDAO := NewTaskDAOWithClient(client)
	customDAO := NewTaskDAOWithConfig(client, "task_custom")

	_, err := customDAO.Insert(ctx, &model.Task{ID: "t-custom", TaskName: "custom", State: "pending", Status: 1})
	assert.NoError(t, err)

	// Default DAO must NOT see the row in the custom table.
	got, err := defaultDAO.GetByID(ctx, "t-custom")
	assert.NoError(t, err)
	assert.Nil(t, got, "default DAO should not see row in task_custom")

	// Custom DAO can see its own row.
	got, err = customDAO.GetByID(ctx, "t-custom")
	assert.NoError(t, err)
	if assert.NotNil(t, got) {
		assert.Equal(t, "custom", got.TaskName)
	}

	// GetByIDs is also routed to the custom table.
	tasks, err := customDAO.GetByIDs(ctx, []string{"t-custom"})
	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "t-custom", tasks[0].ID)
}

func TestTaskDAO_CustomTableName_EmptyFallsBackToDefault(t *testing.T) {
	client := createTestDB(t)
	defer client.Close()

	dao := NewTaskDAOWithConfig(client, "")
	bean := &model.Task{ID: "t-fallback", TaskName: "fallback", State: "pending", Status: 1}
	_, err := dao.Insert(context.Background(), bean)
	assert.NoError(t, err)

	got, err := dao.GetByID(context.Background(), "t-fallback")
	assert.NoError(t, err)
	if assert.NotNil(t, got) {
		assert.Equal(t, "fallback", got.TaskName)
	}
}

func TestSubtaskDAO_CustomTableName(t *testing.T) {
	client := createTestDB(t)
	defer client.Close()
	ctx := context.Background()
	cloneTableSchema(t, client, "subtask", "subtask_custom")

	dao := NewSubtaskDAOWithConfig(client, "subtask_custom")
	bean := &model.Subtask{ID: "s-custom", TaskID: "t1", TaskName: "demo", State: "pending", Status: 1}
	_, err := dao.Insert(ctx, bean)
	assert.NoError(t, err)

	got, err := dao.GetByID(ctx, "s-custom")
	assert.NoError(t, err)
	if assert.NotNil(t, got) {
		assert.Equal(t, "demo", got.TaskName)
	}

	// GetByTaskID should also be routed to the custom table.
	list, err := dao.GetByTaskID(ctx, "t1")
	assert.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "s-custom", list[0].ID)

	// SetInput should write to the custom table.
	err = dao.SetInput(ctx, "s-custom", `{"x":1}`)
	assert.NoError(t, err)
	got, err = dao.GetByID(ctx, "s-custom")
	assert.NoError(t, err)
	if assert.NotNil(t, got) {
		assert.Equal(t, `{"x":1}`, got.Input)
	}
}

func TestTaskEdgeDAO_CustomTableName(t *testing.T) {
	client := createTestDB(t)
	defer client.Close()
	ctx := context.Background()
	cloneTableSchema(t, client, "task_edge", "task_edge_custom")

	dao := NewTaskEdgeDAOWithConfig(client, "task_edge_custom")
	bean := &model.TaskEdge{
		ID:            "e1",
		TaskID:        "t1",
		FromSubtaskID: "a",
		ToSubtaskID:   "b",
		EdgeType:      "control+data",
	}
	_, err := dao.Insert(ctx, bean)
	assert.NoError(t, err)

	edges, err := dao.GetByTaskID(ctx, "t1")
	assert.NoError(t, err)
	assert.Len(t, edges, 1)
	assert.Equal(t, "e1", edges[0].ID)

	// Default DAO must NOT see this row.
	defaultDAO := NewTaskEdgeDAOWithClient(client)
	edges, err = defaultDAO.GetByTaskID(ctx, "t1")
	assert.NoError(t, err)
	assert.Empty(t, edges)
}
