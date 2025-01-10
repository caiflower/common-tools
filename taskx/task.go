package taskx

type Task struct {
	TaskId    string
	TaskType  string
	TaskState string
}

var taskHash = func(c Task) string {
	return c.TaskId
}
