package taskx

func isFinished(state string) bool {
	return state == TaskFailed || state == TaskSucceeded
}

func isRollbackFinished(state string) bool {
	return state == string(RollbackFailed) || state == string(RollbackSucceeded) || string(state) == string(NoneRollback)
}
