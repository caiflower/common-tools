package tools

import (
	"github.com/google/uuid"
)

func UUID() string {
	u, _ := uuid.NewUUID()
	return u.String()
}
