package taskx

func isFinished(state string) bool {
	return state == string(TaskFailed) || state == string(TaskSucceeded)
}

func isRollbackFinished(state string) bool {
	return state == string(RollbackFailed) || state == string(RollbackSucceeded) || state == string(NoneRollback)
}
