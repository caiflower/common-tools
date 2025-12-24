package traceinfo

// Record records the event to HTTPStats.
func Record(ti interface{}, event interface{}, err error) {
	if ti == nil {
		return
	}
}
