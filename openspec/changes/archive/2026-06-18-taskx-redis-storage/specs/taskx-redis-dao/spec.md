## ADDED Requirements

### Requirement: Redis Hash storage for task entities
The system SHALL store Task, Subtask, and TaskEdge entities as Redis Hash structures. Each entity's fields SHALL be stored as Hash field-value pairs using JSON serialization, with keys following the pattern `taskx:{entity}:{id}`.

#### Scenario: Insert Task to Redis
- **WHEN** `TaskDAO.Insert(ctx, task)` is called with Redis backend
- **THEN** the system SHALL create a Redis Hash at key `taskx:task:{task.ID}` with all task fields serialized as JSON, add the task ID to the todo Sorted Set with execute_time as score, and return 1 as rows affected

#### Scenario: GetByID from Redis
- **WHEN** `TaskDAO.GetByID(ctx, id)` is called
- **THEN** the system SHALL execute `HGETALL` on key `taskx:task:{id}`, deserialize the JSON fields into a `model.Task`, and return nil if the key does not exist or status is -1

#### Scenario: Insert Subtask to Redis
- **WHEN** `SubtaskDAO.Insert(ctx, subtask)` is called
- **THEN** the system SHALL create a Hash at `taskx:subtask:{subtask.ID}` and add the subtask ID to the task's subtask index Set at `taskx:task:{subtask.TaskID}:subtasks`

#### Scenario: BatchInsert Subtasks to Redis
- **WHEN** `SubtaskDAO.BatchInsert(ctx, subtasks)` is called with multiple subtasks
- **THEN** the system SHALL atomically create all subtask Hashes and update the task's subtask index Set using a pipeline or Lua script

### Requirement: Redis Sorted Set for task scheduling index
The system SHALL maintain a Sorted Set `taskx:todo` indexed by execute_time for efficient task scheduling queries. Tasks with no execute_time SHALL use 0 as score (immediate execution).

#### Scenario: GetTodoTask query
- **WHEN** `TaskDAO.GetTodoTask(ctx, states, time)` is called
- **THEN** the system SHALL query the Sorted Set with `ZRANGEBYSCORE` for tasks matching the given states where score <= time, and return the corresponding Task entities

#### Scenario: Task state update reflects in index
- **WHEN** a task's state changes to a terminal state (succeeded/failed)
- **THEN** the system SHALL remove the task ID from the todo Sorted Set

#### Scenario: Task insert updates index
- **WHEN** a new task is inserted with state=pending
- **THEN** the system SHALL add the task ID to the todo Sorted Set with execute_time as score (0 if not set)

### Requirement: Redis Lua scripts for CAS operations
All CAS (Compare-And-Swap) operations SHALL be implemented as Redis Lua scripts to ensure atomicity. The scripts MUST compare the old value and conditionally update, returning 1 for success and 0 for failure.

#### Scenario: CASWorkerAndState CAS success for task
- **WHEN** `TaskDAO.CASWorkerAndState(ctx, taskID, worker, state, oldWorker)` is called and the task's current worker matches oldWorker
- **THEN** the Lua script SHALL atomically update worker and state fields, and return 1

#### Scenario: CASWorkerAndState CAS failure for task
- **WHEN** `TaskDAO.CASWorkerAndState(ctx, taskID, worker, state, oldWorker)` is called and the task's current worker does NOT match oldWorker
- **THEN** the Lua script SHALL NOT modify any fields, and return 0

#### Scenario: CASWorkerAndState CAS success for subtask
- **WHEN** `SubtaskDAO.CASWorkerAndState(ctx, id, worker, state, oldWorker)` is called and the subtask's current worker matches oldWorker
- **THEN** the Lua script SHALL atomically update the subtask's worker and state, and return 1

#### Scenario: CASWorkerAndRollback CAS success for subtask
- **WHEN** `SubtaskDAO.CASWorkerAndRollback(ctx, id, worker, rollback, oldWorker)` is called and the subtask's current worker matches oldWorker
- **THEN** the Lua script SHALL atomically update the subtask's worker and rollback fields, and return 1

### Requirement: Redis field-level partial updates
Methods that update specific fields (SetState, SetOutputAndState, SetRetry, SetInput, SetRollbackAndState) SHALL use Redis `HSET` to update only the affected fields, not the entire entity.

#### Scenario: SetState updates only state field
- **WHEN** `TaskDAO.SetState(ctx, id, state)` is called
- **THEN** the system SHALL execute `HSET taskx:task:{id} state {state}` and update the todo Sorted Set accordingly

#### Scenario: SetOutputAndState updates multiple fields
- **WHEN** `SubtaskDAO.SetOutputAndState(ctx, id, output, state)` is called
- **THEN** the system SHALL execute `HSET` to update output, state, and last_run_time fields atomically

#### Scenario: SetRetry updates retry and resets state
- **WHEN** `SubtaskDAO.SetRetry(ctx, id, retry)` is called
- **THEN** the system SHALL update retry count, reset state to "pending", and update last_run_time

### Requirement: Redis Set-based index for task relationships
The system SHALL use Redis Sets to maintain parent-child relationships: `taskx:task:{taskID}:subtasks` for subtask IDs and `taskx:task:{taskID}:edges` for edge IDs.

#### Scenario: GetByTaskID for subtasks
- **WHEN** `SubtaskDAO.GetByTaskID(ctx, taskID)` is called
- **THEN** the system SHALL retrieve all subtask IDs from `taskx:task:{taskID}:subtasks` Set, then batch `HGETALL` each subtask Hash, filtering out status=-1 entries

#### Scenario: GetByTaskID for edges
- **WHEN** `TaskEdgeDAO.GetByTaskID(ctx, taskID)` is called
- **THEN** the system SHALL retrieve all edge IDs from `taskx:task:{taskID}:edges` Set, then batch `HGETALL` each edge Hash

#### Scenario: DeleteByTaskID cascades
- **WHEN** `TaskEdgeDAO.DeleteByTaskID(ctx, taskID)` is called
- **THEN** the system SHALL delete all edge Hashes referenced in the task's edge Set, then delete the Set itself

### Requirement: Redis backup DAO implementation
TaskBakDAO and SubtaskBakDAO SHALL store backup entities in Redis using the same Hash structure with a `taskx:bak:` key prefix.

#### Scenario: Insert TaskBak
- **WHEN** `TaskBakDAO.Insert(ctx, taskBak)` is called
- **THEN** the system SHALL create a Hash at `taskx:bak:task:{taskBak.ID}` with all fields

#### Scenario: GetByTaskID for SubtaskBak
- **WHEN** `SubtaskBakDAO.GetByTaskID(ctx, taskID)` is called
- **THEN** the system SHALL retrieve all backup subtask Hashes that reference the given taskID using a maintained index Set `taskx:bak:task:{taskID}:subtasks`

### Requirement: Redis Cluster hash tag compatibility
All Redis keys that are accessed together in a single Lua script or pipeline MUST share the same hash slot. The system SHALL use Redis hash tags `{taskID}` in key patterns to ensure co-location.

#### Scenario: SubmitTask atomic write
- **WHEN** a task with subtasks and edges is submitted
- **THEN** all keys (`taskx:task:{taskID}`, `taskx:subtask:{subtaskID}`, `taskx:edge:{edgeID}`) where IDs contain the same `{taskID}` hash tag SHALL be in the same Redis Cluster slot

#### Scenario: Lua script execution in cluster
- **WHEN** a Lua script accesses multiple keys
- **THEN** all keys MUST contain the same hash tag to ensure they route to the same slot
