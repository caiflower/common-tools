package tools

import (
	"bytes"
	"math/rand"

	"github.com/google/uuid"
)

func UUID() string {
	u, _ := uuid.NewUUID()
	return u.String()
}

func GenerateId(prefix string) string {
	buff := bytes.Buffer{}
	u1 := uuid.NewString()
	u2 := UUID()
	if rand.Intn(2)&1 == 0 {
		buff.WriteString(u1[:4])
		buff.WriteString(u2[:4])
		buff.WriteString(u1[4:8])
		buff.WriteString(u2[4:8])
	} else {
		buff.WriteString(u1[4:8])
		buff.WriteString(u2[4:8])
		buff.WriteString(u1[:4])
		buff.WriteString(u2[:4])
	}
	// nodeId
	buff.WriteString(u2[len(u2)-4:])

	return prefix + "-" + buff.String()
}
