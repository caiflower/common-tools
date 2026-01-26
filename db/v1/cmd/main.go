package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"go/format"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/caiflower/common-tools/pkg/tools"
	_ "github.com/go-sql-driver/mysql"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/schema"
)

// 使用例子 //go:generate go run -mod=mod github.com/caiflower/common-tools/db/v1/cmd@release-v0.1.0 -dsn 'mysql:root:root@tcp(127.0.0.1:3306)/test_db' pkg "github.com/caiflower/common-tools/dao" -tables test_table -dao_out ./dao

type options struct {
	Dialect   string
	Host      string
	Port      string
	User      string
	Password  string
	DBName    string
	Schema    string
	Charset   string
	Tables    []string
	StructOut string
	DaoOut    string
	Plural    bool
	DSN       string
	Timeout   int
	Keyword   string
	Pkg       string
	// FilterDisableZeroValue bool
}

type columnMeta struct {
	ColumnName     string
	DataType       string
	ColumnType     string
	IsNullable     bool
	IsPrimary      bool
	AutoIncrement  bool
	ColumnDefault  sql.NullString
	ColumnComment  sql.NullString
	GoName         string
	GoType         string
	JSONTag        string
	BunTag         string
	NeedTime       bool
	NeedJSONRaw    bool
	NeedSQLPkg     bool
	OrigColumnType string
}

type tableMeta struct {
	TableName  string
	StructName string
	Columns    []columnMeta
	HasPrimary bool
	PrimaryCol columnMeta
}

func main() {
	opts := parseOptions()
	if err := run(opts); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "generator failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("generate success")
}

func parseOptions() options {
	var opt options
	flag.StringVar(&opt.Dialect, "dialect", "mysql", "数据库类型: mysql 或 postgres")
	flag.StringVar(&opt.DSN, "dsn", "", "完整DSN，设置后优先使用（例如 mysql:user:pass@tcp(host:3306)/db?charset=utf8mb4&parseTime=true&loc=Local）")
	flag.StringVar(&opt.Host, "host", "127.0.0.1", "数据库主机")
	flag.StringVar(&opt.Port, "port", "", "数据库端口，mysql默认3306，postgres默认5432")
	flag.StringVar(&opt.User, "user", "root", "数据库用户")
	flag.StringVar(&opt.Password, "password", "", "数据库密码")
	flag.StringVar(&opt.DBName, "db", "", "数据库名（必填）")
	flag.StringVar(&opt.Schema, "schema", "", "数据库schema，postgres默认public，mysql可留空")
	flag.StringVar(&opt.Charset, "charset", "utf8mb4", "字符集(mysql)")
	flag.StringVar(&opt.DaoOut, "dao_out", "./dao", "Dao 输出目录路径")
	flag.StringVar(&opt.Pkg, "pkg", "", "Dao 包路径 (例如: github.com/caiflower/common-tools/dao)")
	flag.BoolVar(&opt.Plural, "plural", false, "保留复数表名，默认关闭（与bun一致）")
	flag.IntVar(&opt.Timeout, "timeout", 10, "执行超时时间")
	flag.StringVar(&opt.Keyword, "keyword", "id", "关键字，设置之后不遵循驼峰命名规则，用逗号分隔。 例如：id,uuid")
	// flag.BoolVar(&opt.FilterDisableZeroValue, "fiter_disable_zero_value", false, "保留复数表名，默认关闭（与bun一致）")
	tables := flag.String("tables", "", "待生成的表名，多个以逗号分隔（必填）")
	flag.Parse()

	if *tables == "" {
		exitUsage("tables 不能为空，例如: -tables=user,order")
	}
	for _, t := range strings.Split(*tables, ",") {
		name := strings.TrimSpace(t)
		if name != "" {
			opt.Tables = append(opt.Tables, name)
		}
	}
	if opt.DBName == "" && opt.DSN != "" {
		opt.DBName = inferDBNameFromDSN(opt.Dialect, opt.DSN)
	}
	if opt.DSN == "" && opt.DBName == "" {
		exitUsage("db 不能为空")
	}
	if opt.Port == "" {
		if opt.Dialect == "postgres" {
			opt.Port = "5432"
		} else {
			opt.Port = "3306"
		}
	}
	if opt.Schema == "" && opt.Dialect == "postgres" {
		opt.Schema = "public"
	}
	if opt.Pkg == "" {
		exitUsage("pkg 不能为空")
	}
	opt.Dialect = strings.ToLower(opt.Dialect)
	return opt
}

func inferDBNameFromDSN(dialect, dsn string) string {
	switch dialect {
	case "mysql":
		// pattern: user:pass@tcp(host:port)/dbname?params
		if idx := strings.LastIndex(dsn, "/"); idx >= 0 {
			rest := dsn[idx+1:]
			if rest == "" {
				return ""
			}
			if q := strings.Index(rest, "?"); q >= 0 {
				return rest[:q]
			}
			return rest
		}
	case "postgres":
		// prefer url form postgres://user:pass@host:port/dbname?params
		if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
			if u, err := url.Parse(dsn); err == nil {
				name := strings.TrimPrefix(u.Path, "/")
				if name != "" {
					return name
				}
				if v := u.Query().Get("dbname"); v != "" {
					return v
				}
			}
		}
		// kv style: host=... dbname=... user=...
		fields := strings.Fields(dsn)
		for _, f := range fields {
			if strings.HasPrefix(f, "dbname=") {
				return strings.TrimPrefix(f, "dbname=")
			}
		}
	}
	return ""
}

func run(opts options) error {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*time.Duration(opts.Timeout))
	defer cancel()
	db, err := connectDB(ctx, opts)
	if err != nil {
		return err
	}
	defer db.Close()

	if !opts.Plural {
		// 保持表名原样
		schema.SetTableNameInflector(func(tableName string) string {
			return tableName
		})
	}

	daoPkgName := getPkgName(opts.Pkg)
	if daoPkgName == "" {
		daoPkgName = filepath.Base(filepath.Clean(opts.DaoOut))
	}

	modelOutDir := filepath.Join(opts.DaoOut, "model")

	var allTables []tableMeta
	for _, tbl := range opts.Tables {
		cols, err := fetchColumns(ctx, db, opts, tbl)
		if err != nil {
			return fmt.Errorf("fetch columns for %s: %w", tbl, err)
		}
		tmeta := buildTableMeta(tbl, cols)
		allTables = append(allTables, tmeta)

		// Generate model file in model subdirectory
		modelContent := renderModelFile("model", []tableMeta{tmeta})
		modelPath := filepath.Join(modelOutDir, fmt.Sprintf("%s.go", tbl))
		if err := writeFormatted(modelPath, modelContent); err != nil {
			return fmt.Errorf("write model file %s: %w", modelPath, err)
		}

		// Generate dao file only if it doesn't exist
		daoPath := filepath.Join(opts.DaoOut, fmt.Sprintf("%s.go", tbl))
		if _, err := os.Stat(daoPath); os.IsNotExist(err) {
			daoContent := renderDaoFile(opts, daoPkgName, opts.Pkg, []tableMeta{tmeta})
			if err := writeFormatted(daoPath, daoContent); err != nil {
				return fmt.Errorf("write dao file %s: %w", daoPath, err)
			}
		}
	}

	return nil
}

func connectDB(ctx context.Context, opt options) (*bun.DB, error) {
	switch opt.Dialect {
	case "mysql":
		dsn := normalizeMySQLDSN(opt.DSN)
		if dsn == "" {
			dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=true&loc=Local",
				opt.User, opt.Password, opt.Host, opt.Port, opt.DBName, opt.Charset)
		}
		sqldb, err := sql.Open("mysql", dsn)
		if err != nil {
			return nil, err
		}
		if err := sqldb.PingContext(ctx); err != nil {
			return nil, err
		}
		return bun.NewDB(sqldb, mysqldialect.New()), nil
	case "postgres":
		dsn := opt.DSN
		if dsn == "" {
			dsn = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
				opt.User, opt.Password, opt.Host, opt.Port, opt.DBName)
		}
		connector := pgdriver.NewConnector(pgdriver.WithDSN(dsn))
		sqldb := sql.OpenDB(connector)
		if err := sqldb.PingContext(ctx); err != nil {
			return nil, err
		}
		return bun.NewDB(sqldb, pgdialect.New()), nil
	default:
		return nil, fmt.Errorf("unsupported dialect %s", opt.Dialect)
	}
}

func fetchColumns(ctx context.Context, db *bun.DB, opt options, table string) ([]columnMeta, error) {
	switch opt.Dialect {
	case "mysql":
		cols, err := fetchMySQLColumns(ctx, db, opt.DBName, table, strings.Split(opt.Keyword, ","))
		if err != nil {
			return nil, err
		}
		if len(cols) == 0 {
			return nil, fmt.Errorf("no columns found for table %s in schema %s, please check -db/-dsn/-tables", table, opt.DBName)
		}
		return cols, nil
	case "postgres":
		cols, err := fetchPostgresColumns(ctx, db, opt.Schema, table, strings.Split(opt.Keyword, ","))
		if err != nil {
			return nil, err
		}
		if len(cols) == 0 {
			return nil, fmt.Errorf("no columns found for table %s in schema %s, please check -schema/-dsn/-tables", table, opt.Schema)
		}
		return cols, nil
	default:
		return nil, fmt.Errorf("unsupported dialect %s", opt.Dialect)
	}
}

type mysqlColumnRow struct {
	ColumnName    string         `bun:"column_name"`
	DataType      string         `bun:"data_type"`
	ColumnType    string         `bun:"column_type"`
	IsNullable    string         `bun:"is_nullable"`
	ColumnKey     sql.NullString `bun:"column_key"`
	ColumnDefault sql.NullString `bun:"column_default"`
	Extra         sql.NullString `bun:"extra"`
	ColumnComment sql.NullString `bun:"column_comment"`
}

func fetchMySQLColumns(ctx context.Context, db *bun.DB, schema, table string, keywords []string) ([]columnMeta, error) {
	var rows []mysqlColumnRow
	err := db.NewSelect().
		TableExpr("information_schema.COLUMNS").
		ColumnExpr("COLUMN_NAME as column_name").
		ColumnExpr("DATA_TYPE as data_type").
		ColumnExpr("COLUMN_TYPE as column_type").
		ColumnExpr("IS_NULLABLE as is_nullable").
		ColumnExpr("COLUMN_KEY as column_key").
		ColumnExpr("COLUMN_DEFAULT as column_default").
		ColumnExpr("EXTRA as extra").
		ColumnExpr("COLUMN_COMMENT as column_comment").
		Where("TABLE_SCHEMA = ?", schema).
		Where("TABLE_NAME = ?", table).
		OrderExpr("ORDINAL_POSITION").
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	var result []columnMeta
	for _, r := range rows {
		m := columnMeta{
			ColumnName:     r.ColumnName,
			DataType:       strings.ToLower(r.DataType),
			ColumnType:     strings.ToLower(r.ColumnType),
			IsNullable:     strings.ToUpper(r.IsNullable) == "YES",
			IsPrimary:      strings.ToUpper(r.ColumnKey.String) == "PRI",
			AutoIncrement:  strings.Contains(strings.ToLower(r.Extra.String), "auto_increment"),
			ColumnDefault:  r.ColumnDefault,
			ColumnComment:  r.ColumnComment,
			OrigColumnType: r.ColumnType,
		}
		fillGoType(&m, "mysql", keywords)
		result = append(result, m)
	}
	return result, nil
}

type pgColumnRow struct {
	ColumnName    string         `bun:"column_name"`
	DataType      string         `bun:"data_type"`
	IsNullable    string         `bun:"is_nullable"`
	ColumnDefault sql.NullString `bun:"column_default"`
	UdtName       sql.NullString `bun:"udt_name"`
	CharacterMax  sql.NullInt64  `bun:"character_maximum_length"`
	NumericPrec   sql.NullInt64  `bun:"numeric_precision"`
	NumericScale  sql.NullInt64  `bun:"numeric_scale"`
	IsPrimary     bool           `bun:"is_primary"`
}

func fetchPostgresColumns(ctx context.Context, db *bun.DB, schema, table string, keywords []string) ([]columnMeta, error) {
	var rows []pgColumnRow
	err := db.NewSelect().
		TableExpr("information_schema.columns AS c").
		ColumnExpr("c.column_name").
		ColumnExpr("c.data_type").
		ColumnExpr("c.is_nullable").
		ColumnExpr("c.column_default").
		ColumnExpr("c.udt_name").
		ColumnExpr("c.character_maximum_length").
		ColumnExpr("c.numeric_precision").
		ColumnExpr("c.numeric_scale").
		ColumnExpr(`EXISTS (
	        SELECT 1 FROM information_schema.table_constraints tc
	        JOIN information_schema.key_column_usage kcu
	          ON tc.constraint_name = kcu.constraint_name
	         AND tc.table_schema = kcu.table_schema
	         AND tc.table_name = kcu.table_name
	       WHERE tc.constraint_type = 'PRIMARY KEY'
	         AND tc.table_schema = c.table_schema
	         AND tc.table_name = c.table_name
	         AND kcu.column_name = c.column_name
	      ) AS is_primary`).
		Where("c.table_schema = ?", schema).
		Where("c.table_name = ?", table).
		OrderExpr("c.ordinal_position").
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	var result []columnMeta
	for _, r := range rows {
		m := columnMeta{
			ColumnName:     r.ColumnName,
			DataType:       strings.ToLower(r.DataType),
			ColumnType:     strings.ToLower(r.UdtName.String),
			IsNullable:     strings.ToUpper(r.IsNullable) == "YES",
			IsPrimary:      r.IsPrimary,
			AutoIncrement:  strings.HasPrefix(strings.ToLower(r.ColumnDefault.String), "nextval"),
			ColumnDefault:  r.ColumnDefault,
			ColumnComment:  sql.NullString{},
			OrigColumnType: r.UdtName.String,
		}
		fillGoType(&m, "postgres", keywords)
		result = append(result, m)
	}
	return result, nil
}

func normalizeMySQLDSN(dsn string) string {
	if strings.HasPrefix(dsn, "mysql:") {
		return strings.TrimPrefix(dsn, "mysql:")
	}
	return dsn
}

func fillGoType(m *columnMeta, dialect string, keywords []string) {
	colType := strings.ToLower(m.ColumnType)
	dataType := strings.ToLower(m.DataType)

	switch dialect {
	case "mysql":
		switch dataType {
		case "tinyint":
			if strings.Contains(m.ColumnType, "(1)") {
				m.GoType = baseType("bool", m.IsNullable, false)
			} else {
				m.GoType = baseType("int8", m.IsNullable, false)
			}
		case "smallint":
			m.GoType = baseType("int16", m.IsNullable, false)
		case "mediumint", "int", "integer":
			m.GoType = baseType("int32", m.IsNullable, false)
		case "bigint":
			m.GoType = baseType("int64", m.IsNullable, false)
		case "float":
			m.GoType = baseType("float32", m.IsNullable, false)
		case "double", "real":
			m.GoType = baseType("float64", m.IsNullable, false)
		case "decimal", "numeric":
			m.GoType = baseType("string", m.IsNullable, false)
		case "bit":
			m.GoType = baseType("bool", m.IsNullable, false)
		case "datetime", "timestamp", "date", "time":
			m.GoType = baseType("basic.Time", m.IsNullable, true)
			m.NeedTime = true
		case "json":
			m.GoType = baseType("json.RawMessage", m.IsNullable, false)
			m.NeedJSONRaw = true
		case "binary", "varbinary", "blob", "tinyblob", "mediumblob", "longblob":
			m.GoType = baseType("[]byte", false, false)
		default:
			m.GoType = baseType("string", m.IsNullable, false)
		}
	case "postgres":
		switch dataType {
		case "smallint":
			m.GoType = baseType("int16", m.IsNullable, false)
		case "integer":
			m.GoType = baseType("int32", m.IsNullable, false)
		case "bigint":
			m.GoType = baseType("int64", m.IsNullable, false)
		case "real":
			m.GoType = baseType("float32", m.IsNullable, false)
		case "double precision":
			m.GoType = baseType("float64", m.IsNullable, false)
		case "numeric", "decimal":
			m.GoType = baseType("string", m.IsNullable, false)
		case "boolean":
			m.GoType = baseType("bool", m.IsNullable, false)
		case "timestamp without time zone", "timestamp with time zone", "date", "time":
			m.GoType = baseType("time.Time", m.IsNullable, true)
			m.NeedTime = true
		case "json", "jsonb":
			m.GoType = baseType("json.RawMessage", m.IsNullable, false)
			m.NeedJSONRaw = true
		case "bytea":
			m.GoType = baseType("[]byte", false, false)
		case "uuid":
			m.GoType = baseType("string", m.IsNullable, false)
		default:
			if strings.HasSuffix(colType, "[]") {
				m.GoType = baseType("[]byte", false, false)
			} else {
				m.GoType = baseType("string", m.IsNullable, false)
			}
		}
	}
	m.GoName, _ = toCamel(m.ColumnName, keywords)
	m.JSONTag = toLowerCamel(m.ColumnName, keywords)
	m.BunTag = buildBunTag(m)
	if strings.HasPrefix(m.GoType, "sql.") {
		m.NeedSQLPkg = true
	}
}

func baseType(base string, nullable bool, needTime bool) string {
	//if nullable {
	//	if strings.HasPrefix(base, "[]") {
	//		return base
	//	}
	//	return "*" + base
	//}
	return base
}

func buildBunTag(m *columnMeta) string {
	var parts []string
	parts = append(parts, m.ColumnName)
	if m.IsPrimary {
		parts = append(parts, "pk")
	}
	if m.AutoIncrement {
		parts = append(parts, "autoincrement")
	}
	if !m.IsNullable {
		parts = append(parts, "notnull")
	}
	return strings.Join(parts, ",")
}

func buildTableMeta(table string, cols []columnMeta) tableMeta {
	t := tableMeta{TableName: table}
	t.StructName, _ = toCamel(table, []string{})
	t.Columns = cols
	for _, c := range cols {
		if c.IsPrimary && !t.HasPrimary {
			t.HasPrimary = true
			t.PrimaryCol = c
		}
	}
	return t
}

func renderModelFile(pkg string, tables []tableMeta) string {
	imports := collectImports(tables, false)
	body := renderStructBlocks(tables)
	return renderModelFileWithMarker(pkg, imports, body)
}

func renderDaoFile(opts options, pkgName string, fullPkg string, tables []tableMeta) string {
	imports := collectDaoImports(tables, fullPkg+"/model")
	body := renderDaoBlocks(opts, tables)
	return renderFile(pkgName, imports, body)
}

func collectImports(tables []tableMeta, includeDAO bool) map[string]struct{} {
	imports := map[string]struct{}{
		"github.com/uptrace/bun": {},
	}
	for _, t := range tables {
		for _, c := range t.Columns {
			if c.NeedTime {
				imports["github.com/caiflower/common-tools/pkg/basic"] = struct{}{}
			}
			if c.NeedJSONRaw {
				imports["encoding/json"] = struct{}{}
			}
		}
	}
	if includeDAO {
		imports["github.com/caiflower/common-tools/db/v1"] = struct{}{}
	}
	return imports
}

func collectDaoImports(tables []tableMeta, modelPkg string) map[string]struct{} {
	imports := map[string]struct{}{
		"github.com/caiflower/common-tools/db/v1": {},
		"github.com/uptrace/bun":                  {},
		modelPkg:                                  {},
		"context":                                 {},
	}
	for _, t := range tables {
		for _, c := range t.Columns {
			if c.NeedJSONRaw {
				imports["encoding/json"] = struct{}{}
			}
		}
	}
	return imports
}

func renderFile(pkgName string, imports map[string]struct{}, body string) string {
	var importList []string
	for imp := range imports {
		importList = append(importList, imp)
	}
	sort.Strings(importList)

	var b strings.Builder
	b.WriteString("package " + pkgName + "\n\n")

	if len(importList) > 0 {
		b.WriteString("import (\n")
		for _, imp := range importList {
			b.WriteString(fmt.Sprintf("\t\"%s\"\n", imp))
		}
		b.WriteString(")\n\n")
	}
	b.WriteString(body)
	return b.String()
}

func renderModelFileWithMarker(pkg string, imports map[string]struct{}, body string) string {
	var importList []string
	for imp := range imports {
		importList = append(importList, imp)
	}
	sort.Strings(importList)

	var b strings.Builder
	b.WriteString("// Code generated by caiflower generator; DO NOT EDIT.\n")
	b.WriteString("package " + pkg + "\n\n")

	if len(importList) > 0 {
		b.WriteString("import (\n")
		for _, imp := range importList {
			b.WriteString(fmt.Sprintf("\t\"%s\"\n", imp))
		}
		b.WriteString(")\n\n")
	}
	b.WriteString(body)
	return b.String()
}

func renderStructBlocks(tables []tableMeta) string {
	var b strings.Builder
	for _, t := range tables {
		b.WriteString(fmt.Sprintf("// %s generate from table %s\n", t.StructName, t.TableName))
		b.WriteString(fmt.Sprintf("type %s struct {\n", t.StructName))
		b.WriteString(fmt.Sprintf("\tbun.BaseModel `bun:\"table:%s\"`\n", t.TableName))
		for _, c := range t.Columns {
			comment := ""
			if c.ColumnComment.Valid && strings.TrimSpace(c.ColumnComment.String) != "" {
				comment = " // " + strings.TrimSpace(c.ColumnComment.String)
			}
			b.WriteString(fmt.Sprintf("\t%s %s `bun:\"%s\" json:\"%s\"`%s\n",
				c.GoName, c.GoType, c.BunTag, c.JSONTag, comment))
		}
		b.WriteString("}\n\n")

		filterName := t.StructName + "Filter"
		hasStatus := false
		hasID := false
		for _, c := range t.Columns {
			if c.ColumnName == "status" {
				hasStatus = true
			}
			if c.ColumnName == "id" {
				hasID = true
			}
		}

		b.WriteString(fmt.Sprintf("type %s struct {\n", filterName))
		b.WriteString("\tPage int `json:\"page\"`\n")
		b.WriteString("\tPageSize int `json:\"pageSize\"`\n")
		b.WriteString("\tDisablePage bool `json:\"disablePage\"`\n")
		b.WriteString("\tOrders []string `json:\"orders,omitempty\"`\n\n")
		for _, c := range t.Columns {
			b.WriteString(fmt.Sprintf("\t%s *%s `json:\"%s,omitempty\"`\n", c.GoName, c.GoType, c.JSONTag))
		}
		b.WriteString("}\n\n")

		b.WriteString(fmt.Sprintf("func (f *%s) GetPage() (offset int, limit int, disable bool) {\n", filterName))
		b.WriteString("\tif f.DisablePage {\n\t\treturn 0, 0, true\n\t}\n")
		b.WriteString("\tpage := f.Page\n\tif page <= 0 {\n\t\tpage = 1\n\t}\n")
		b.WriteString("\tsize := f.PageSize\n\tif size <= 0 {\n\t\tsize = 10\n\t}\n")
		b.WriteString("\treturn (page - 1) * size, size, false\n")
		b.WriteString("}\n\n")

		b.WriteString(fmt.Sprintf("func (f *%s) Filter(db bun.IDB) *bun.SelectQuery {\n", filterName))
		b.WriteString("\tq := db.NewSelect()\n")
		if hasStatus {
			b.WriteString("\tif f.Status == nil {\n\t\tq.Where(\"status>0\")\n\t}\n")
		}
		for _, c := range t.Columns {
			field := "f." + c.GoName
			switch c.GoType {
			case "string":
				b.WriteString(fmt.Sprintf("\tif %s != nil {\n\t\tq.Where(\"%s = ?\", *%s)\n\t}\n", field, c.ColumnName, field))
			default:
				b.WriteString(fmt.Sprintf("\tif %s != nil {\n\t\tq.Where(\"%s = ?\", *%s)\n\t}\n", field, c.ColumnName, field))
			}
		}
		b.WriteString("\tif len(f.Orders) > 0 {\n\t\tq.Order(f.Orders...)\n\t} else {\n")
		if hasID {
			b.WriteString("\t\tq.Order(\"id desc\")\n")
		}
		b.WriteString("\t}\n\treturn q\n")
		b.WriteString("}\n\n")
	}
	return b.String()
}

func renderDaoBlocks(opts options, tables []tableMeta) string {
	var b strings.Builder
	for _, t := range tables {
		daoName := strings.ToLower(t.StructName[:1]) + t.StructName[1:] + "DAO"
		daoName1 := t.StructName + "DAO"
		hasStatus := false
		tableNameConst := "TableNameOf" + t.StructName
		for _, c := range t.Columns {
			if c.ColumnName == "status" {
				hasStatus = true
			}
		}

		// Generate interface file
		interfaceContent := renderInterfaceFile(tables, t.HasPrimary)
		b.WriteString(interfaceContent)

		b.WriteString(fmt.Sprintf("const %s = \"%s\"\n\n", tableNameConst, t.TableName))
		b.WriteString(fmt.Sprintf("type %s struct {\n\tClient *dbv1.Client `autowired:\"\"`\n}\n\n", daoName))

		b.WriteString(fmt.Sprintf("// New%sWithClient new client with db client \n", daoName1))
		b.WriteString(fmt.Sprintf("func New%sWithClient(db *dbv1.Client) %s {\n\treturn &%s{Client: db}\n}\n\n", daoName1, daoName1, daoName))

		b.WriteString(fmt.Sprintf("// New%s new client \n", daoName1))
		b.WriteString(fmt.Sprintf("func New%s() %s {\n\treturn &%s{}\n}\n\n", daoName1, daoName1, daoName))

		b.WriteString(fmt.Sprintf("// GetClient get the db client\n"))
		b.WriteString(fmt.Sprintf("func (d *%s) GetClient() (dbv1.DB) {\n", daoName))
		b.WriteString("\treturn d.Client\n}\n\n")

		b.WriteString(fmt.Sprintf("// Insert create a new record\n"))
		b.WriteString(fmt.Sprintf("func (d *%s) Insert(ctx context.Context, data *model.%s, tx ...*bun.Tx) (int64, error) {\n", daoName, t.StructName))
		b.WriteString("\treturn d.Client.Insert(ctx, data, tx...)\n}\n\n")

		b.WriteString(fmt.Sprintf("// QueryPage query by page\n"))
		b.WriteString(fmt.Sprintf("func (d *%s) QueryPage(ctx context.Context, filter *model.%sFilter) (res []model.%s, cnt int, err error) {\n", daoName, t.StructName, t.StructName))
		b.WriteString(fmt.Sprintf("\tres = make([]model.%s, 0)\n\tcnt, err = d.Client.QueryPage(ctx, &res, filter)\n\treturn\n}\n\n", t.StructName))

		if t.HasPrimary {
			pkType := t.PrimaryCol.GoType
			pkName := t.PrimaryCol.GoName
			if strings.HasPrefix(pkType, "*") {
				pkType = strings.TrimPrefix(pkType, "*")
			}
			b.WriteString(fmt.Sprintf("// GetBy%s get by primaryKey, return nil if not found\n", pkName))
			b.WriteString(fmt.Sprintf("func (d *%s) GetBy%s(ctx context.Context, id %s) (*model.%s, error) {\n", daoName, pkName, pkType, t.StructName))
			b.WriteString(fmt.Sprintf("\tmodel := new(model.%s)\n", t.StructName))
			b.WriteString(fmt.Sprintf("\terr := d.Client.GetSelect(model).Where(\"%s = ?\", id).Limit(1).Scan(ctx)\n", t.PrimaryCol.ColumnName))
			b.WriteString("\tif d.Client.ParseErr(err) == nil {\n\t\treturn nil, nil\n\t}\n")
			b.WriteString("\treturn model, err\n}\n\n")

			b.WriteString(fmt.Sprintf("// UpdateBy%s update record primaryKey\n", pkName))
			b.WriteString(fmt.Sprintf("func (d *%s) UpdateBy%s(ctx context.Context, data *model.%s, tx ...*bun.Tx) (int64, error) {\n", daoName, pkName, t.StructName))
			b.WriteString(fmt.Sprintf("\treturn d.Client.GetRowsAffected(d.Client.GetUpdate(data, tx...).Where(\"%s = ?\", data.%s).Exec(ctx))\n", t.PrimaryCol.ColumnName, t.PrimaryCol.GoName))
			b.WriteString("}\n\n")

			b.WriteString(fmt.Sprintf("// DeleteBy%s physically delete record by primaryKey\n", pkName))
			b.WriteString(fmt.Sprintf("func (d *%s) DeleteBy%s(ctx context.Context, id %s, tx ...*bun.Tx) (int64, error) {\n", daoName, pkName, pkType))
			b.WriteString(fmt.Sprintf("\tresult, err := d.Client.DB.NewDelete().Table(%s).Where(\"%s = ?\", id).Exec(ctx)\n", tableNameConst, t.PrimaryCol.ColumnName))
			b.WriteString("\tif d.Client.ParseErr(err) == nil {\n\t\treturn 0, nil\n\t}\n")
			b.WriteString("\treturn result.RowsAffected()\n}\n\n")

			if hasStatus {
				b.WriteString(fmt.Sprintf("// SoftDeleteBy%s logically delete record by primaryKey (set status=-1)\n", pkName))
				b.WriteString(fmt.Sprintf("func (d *%s) SoftDeleteBy%s(ctx context.Context, id %s, tx ...*bun.Tx) (int64, error) {\n", daoName, pkName, pkType))
				b.WriteString(fmt.Sprintf("\tresult, err := d.Client.DB.NewUpdate().Table(%s).Set(\"status = ?\", -1).Where(\"%s = ?\", id).Exec(ctx)\n", tableNameConst, t.PrimaryCol.ColumnName))
				b.WriteString("\tif d.Client.ParseErr(err) == nil {\n\t\treturn 0, nil\n\t}\n")
				b.WriteString("\treturn result.RowsAffected()\n}\n\n")
			}
		}

	}
	return b.String()
}

func renderInterfaceFile(tables []tableMeta, hasPrimary bool) string {
	var b strings.Builder
	for _, t := range tables {
		daoName := t.StructName + "DAO"
		pkType := t.PrimaryCol.GoType
		if strings.HasPrefix(pkType, "*") {
			pkType = strings.TrimPrefix(pkType, "*")
		}
		hasStatus := false
		for _, c := range t.Columns {
			if c.ColumnName == "status" {
				hasStatus = true
				break
			}
		}

		b.WriteString(fmt.Sprintf("type %s interface {\n", daoName))
		b.WriteString(fmt.Sprintf("\tGetClient() dbv1.DB\n"))
		b.WriteString(fmt.Sprintf("\tInsert(ctx context.Context, data *model.%s, tx ...*bun.Tx) (int64, error)\n", t.StructName))
		b.WriteString(fmt.Sprintf("\tQueryPage(ctx context.Context, filter *model.%sFilter) (res []model.%s, cnt int, err error)\n", t.StructName, t.StructName))
		if hasPrimary {
			b.WriteString(fmt.Sprintf("\tGetByID(ctx context.Context, id %s) (*model.%s, error)\n", pkType, t.StructName))
			b.WriteString(fmt.Sprintf("\tUpdateByID(ctx context.Context, data *model.%s, tx ...*bun.Tx) (int64, error)\n", t.StructName))
			b.WriteString(fmt.Sprintf("\tDeleteByID(ctx context.Context, id %s, tx ...*bun.Tx) (int64, error)\n", pkType))
			if hasStatus {
				b.WriteString(fmt.Sprintf("\tSoftDeleteByID(ctx context.Context, id %s, tx ...*bun.Tx) (int64, error)\n", pkType))
			}
		}
		b.WriteString("}\n\n")
	}

	return b.String()
}

func writeFormatted(path, content string) error {
	formatted, err := format.Source([]byte(content))
	if err != nil {
		formatted = []byte(content)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, formatted, 0o644)
}

func toCamel(s string, keywords []string) (string, bool) {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' ' || r == '.'
	})
	onlyKeyword := false

	for i, p := range parts {
		if p == "" {
			continue
		}
		if tools.StringSliceContains(keywords, strings.ToLower(p)) {
			parts[i] = strings.ToUpper(p)
			if len(parts) == 1 {
				onlyKeyword = true
			}
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
	}
	return strings.Join(parts, ""), onlyKeyword
}

func toLowerCamel(s string, keywords []string) string {
	camel, onlyKeyword := toCamel(s, keywords)
	if camel == "" {
		return ""
	}
	if !onlyKeyword {
		return strings.ToLower(camel[:1]) + camel[1:]
	}
	return strings.ToLower(camel)
}

func exitUsage(msg string) {
	_, _ = fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	flag.Usage()
	os.Exit(2)
}

func getPkgName(pkgPath string) string {
	if pkgPath == "" {
		return ""
	}
	parts := strings.Split(pkgPath, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
