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

 package telemetry

import (
	"context"
	"database/sql"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/uptrace/uptrace-go/uptrace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.opentelemetry.io/otel/trace"
	"runtime"
	"strings"
)

type QueryHook struct {
	attrs             []attribute.KeyValue
	formatQueries     bool
	tracer            trace.Tracer
	meter             metric.Meter
	spanNameFormatter func(*bun.QueryEvent) string
}

var _ bun.QueryHook = (*QueryHook)(nil)

func NewQueryHook(opts ...Option) *QueryHook {
	h := new(QueryHook)
	for _, opt := range opts {
		opt(h)
	}
	if h.tracer == nil {
		h.tracer = otel.Tracer("github.com/uptrace/bun")
	}
	//if h.meter == nil {
	//	h.meter = otel.Meter("github.com/uptrace/bun")
	//}
	//h.queryHistogram, _ = h.meter.Int64Histogram(
	//	"go.sql.query_timing",
	//	metric.WithDescription("Timing of processed queries"),
	//	metric.WithUnit("milliseconds"),
	//)
	return h
}

//func (h *QueryHook) Init(db *bun.DB) {
//labels := make([]attribute.KeyValue, 0, len(h.attrs)+1)
//labels = append(labels, h.attrs...)
//if sys := dbSystem(db); sys.Valid() {
//	labels = append(labels, sys)
//}

//otelsql.ReportDBStatsMetrics(db.DB, otelsql.WithAttributes(labels...))
//}

func (h *QueryHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	traceId := ctx.Value("traceId").(string)
	traceID, err := trace.TraceIDFromHex(traceId)
	if err != nil {
		logger.Error("trace.TraceIDFromHex failed. Error: %v", err)
	}
	ctx, _ = h.tracer.Start(trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
	})), "", trace.WithSpanKind(trace.SpanKindClient))
	return ctx
}

func (h *QueryHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	operation := event.Operation()
	dbOperation := semconv.DBOperationKey.String(operation)

	//labels := make([]attribute.KeyValue, 0, len(h.attrs)+2)
	//labels = append(labels, h.attrs...)
	//labels = append(labels, dbOperation)
	//if event.IQuery != nil {
	//	if tableName := event.IQuery.GetTableName(); tableName != "" {
	//		labels = append(labels, semconv.DBSQLTableKey.String(tableName))
	//	}
	//}

	//dur := time.Since(event.StartTime)
	//h.queryHistogram.Record(ctx, dur.Milliseconds(), labels...)

	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	name := operation
	if h.spanNameFormatter != nil {
		name = h.spanNameFormatter(event)
	}
	span.SetName(name)
	defer span.End()
	if !span.IsRecording() {
		return
	}

	query := h.eventQuery(event)
	fn, file, line := funcFileLine("github.com/uptrace/bun")

	attrs := make([]attribute.KeyValue, 0, 10)
	attrs = append(attrs, h.attrs...)
	attrs = append(attrs,
		dbOperation,
		semconv.DBStatementKey.String(query),
		semconv.CodeFunctionKey.String(fn),
		semconv.CodeFilepathKey.String(file),
		semconv.CodeLineNumberKey.Int(line),
	)

	if sys := dbSystem(event.DB); sys.Valid() {
		attrs = append(attrs, sys)
	}
	if event.Result != nil {
		rows, _ := event.Result.RowsAffected()
		attrs = append(attrs, attribute.Int64("db.rows_affected", rows))
	}

	switch event.Err {
	case nil, sql.ErrNoRows, sql.ErrTxDone:
		// ignore
	default:
		span.RecordError(event.Err)
		span.SetStatus(codes.Error, event.Err.Error())
	}

	span.SetAttributes(attrs...)
	logger.Trace("uptrace: %s\n", uptrace.TraceURL(span))
}

func funcFileLine(pkg string) (string, string, int) {
	const depth = 16
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	ff := runtime.CallersFrames(pcs[:n])

	var fn, file string
	var line int
	for {
		f, ok := ff.Next()
		if !ok {
			break
		}
		fn, file, line = f.Function, f.File, f.Line
		if !strings.Contains(fn, pkg) {
			break
		}
	}

	if ind := strings.LastIndexByte(fn, '/'); ind != -1 {
		fn = fn[ind+1:]
	}

	return fn, file, line
}

func (h *QueryHook) eventQuery(event *bun.QueryEvent) string {
	const softQueryLimit = 8000
	const hardQueryLimit = 16000

	var query string

	if h.formatQueries && len(event.Query) <= softQueryLimit {
		query = event.Query
	} else {
		//query = unformattedQuery(event)
		query = event.Query
	}

	if len(query) > hardQueryLimit {
		query = query[:hardQueryLimit]
	}

	return query
}

//func unformattedQuery(event *bun.QueryEvent) string {
//	if event.IQuery != nil {
//		if b, err := event.IQuery.AppendQuery(schema.NewNopFormatter(), nil); err == nil {
//			return internal.String(b)
//		}
//	}
//	return string(event.QueryTemplate)
//}

func dbSystem(db *bun.DB) attribute.KeyValue {
	switch db.Dialect().Name() {
	case dialect.PG:
		return semconv.DBSystemPostgreSQL
	case dialect.MySQL:
		return semconv.DBSystemMySQL
	case dialect.SQLite:
		return semconv.DBSystemSqlite
	default:
		return attribute.KeyValue{}
	}
}
