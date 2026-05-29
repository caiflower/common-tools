package dbv1

func SplitIndex(pageNumber, pageSize, total int) (canSplit bool, start int, end int) {
	if pageNumber <= 0 || pageSize <= 0 {
		return
	}
	start = (pageNumber - 1) * pageSize
	end = start + pageSize
	if start < total {
		if end > total {
			end = total
		}
		canSplit = true
	}
	return
}
