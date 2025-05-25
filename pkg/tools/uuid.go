package tools

import (
	"bytes"
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"strings"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

func UUID() string {
	u, _ := uuid.NewUUID()
	return strings.Replace(u.String(), "-", "", 4)
}

func UptraceUUID() trace.TraceID {
	tid := trace.TraceID{}
	var rngSeed int64
	_ = binary.Read(crand.Reader, binary.LittleEndian, &rngSeed)
	randSource := rand.New(rand.NewSource(rngSeed))
	randSource.Read(tid[:])
	return tid
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
