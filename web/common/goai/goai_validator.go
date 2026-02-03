package goai

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

type Validator interface {
	Validate(arg any) error
}

type defaultValidator struct {
	mu          sync.RWMutex
	compiled    map[string]*compiledFieldValidator
	propIndexes map[string][]int
}

type compiledFieldValidator struct {
	required bool
	nilable  bool
	rules    []compiledRule
}

type compiledRule func(v reflect.Value, fieldName string) error

func newDefaultValidator() *defaultValidator {
	return &defaultValidator{
		compiled:    make(map[string]*compiledFieldValidator),
		propIndexes: make(map[string][]int),
	}
}

func schemaHasValidation(s *Schema) bool {
	if s == nil {
		return false
	}
	if strings.TrimSpace(s.ValidationRules) != "" {
		return true
	}
	switch s.Type {
	case TypeObject:
		props := s.Properties.Map()
		for _, ref := range props {
			if schemaHasValidation(ref.Value) {
				return true
			}
		}
	case TypeArray:
		if s.Items != nil && s.Items.Value != nil {
			return schemaHasValidation(s.Items.Value)
		}
	}
	return false
}

func (dv *defaultValidator) verfFieldIndexes(cacheKey string, t reflect.Type, schema *Schema) []int {
	dv.mu.RLock()
	if idx, ok := dv.propIndexes[cacheKey]; ok {
		dv.mu.RUnlock()
		return idx
	}
	dv.mu.RUnlock()

	var (
		indexes  []int
		hasVerf  bool
		propMaps = schema.Properties.Map()
	)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		jsonName := field.Name
		if tag := field.Tag.Get("json"); tag != "" {
			s := strings.Split(strings.Trim(tag, ","), ",")
			if len(s) > 0 && s[0] == "-" {
				continue
			}
			if len(s) > 0 && s[0] != "" {
				jsonName = s[0]
			}
		}

		propRef, ok := propMaps[jsonName]
		if !ok || propRef.Value == nil {
			continue
		}
		if !schemaHasValidation(propRef.Value) {
			continue
		}

		hasVerf = true
		indexes = append(indexes, i)
	}

	if !hasVerf {
		dv.mu.Lock()
		dv.propIndexes[cacheKey] = nil
		dv.mu.Unlock()
		return nil
	}

	dv.mu.Lock()
	dv.propIndexes[cacheKey] = indexes
	dv.mu.Unlock()
	return indexes
}

func (dv *defaultValidator) ValidateField(cacheKey string, displayName string, value reflect.Value, schema *Schema) error {
	if schema == nil {
		return nil
	}
	tagValue := schema.ValidationRules
	if strings.TrimSpace(tagValue) == "" {
		return nil
	}

	dv.mu.RLock()
	compiled := dv.compiled[cacheKey]
	dv.mu.RUnlock()
	if compiled == nil {
		var err error
		compiled, err = compileFieldValidator(tagValue, value.Type())
		if err != nil {
			return err
		}
		dv.mu.Lock()
		dv.compiled[cacheKey] = compiled
		dv.mu.Unlock()
	}

	if isZero(value) && (compiled.nilable || !compiled.required) {
		return nil
	}
	if compiled.required {
		if err := requiredRule(value, displayName); err != nil {
			return err
		}
	}
	for _, rule := range compiled.rules {
		if err := rule(value, displayName); err != nil {
			return err
		}
	}
	return nil
}

// helper funcs used by validator (duplicated minimal set to avoid extra imports)
func isZero(v reflect.Value) bool {
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr) {
		if v.IsNil() {
			return true
		}
		v = v.Elem()
	}
	return !v.IsValid() || v.IsZero()
}

func requiredRule(v reflect.Value, fieldName string) error {
	if !v.IsValid() {
		return fmt.Errorf("%s is missing", fieldName)
	}
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return fmt.Errorf("%s is missing", fieldName)
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.String:
		if v.String() == "" {
			return fmt.Errorf("%s is missing", fieldName)
		}
	case reflect.Slice, reflect.Array:
		if v.Len() == 0 {
			return fmt.Errorf("%s is missing", fieldName)
		}
		if v.Type().Elem().Kind() == reflect.String {
			for i := 0; i < v.Len(); i++ {
				if v.Index(i).String() == "" {
					return fmt.Errorf("%s[%d] is missing", fieldName, i)
				}
			}
		}
	default:
	}
	return nil
}

func compileFieldValidator(tagValue string, fieldType reflect.Type) (*compiledFieldValidator, error) {
	rules := strings.Split(tagValue, "|")
	validator := &compiledFieldValidator{}
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		switch {
		case rule == "required":
			validator.required = true
		case rule == "nilable":
			validator.nilable = true
		case strings.HasPrefix(rule, "inList:"):
			enumValues := stringsSplitAndTrim(rule[len("inList:"):], ",")
			if len(enumValues) == 0 {
				return nil, fmt.Errorf("%s tag[%s] value is not vaild", fieldType.String(), "inList")
			}
			validator.rules = append(validator.rules, inListRule(enumValues, fieldType))
		case strings.HasPrefix(rule, "reg:"):
			pattern := strings.TrimSpace(rule[len("reg:"):])
			if pattern == "" {
				return nil, fmt.Errorf("%s tag[%s] value is not vaild", fieldType.String(), "reg")
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("%s tag[%s] value is not vaild", fieldType.String(), "reg")
			}
			validator.rules = append(validator.rules, regRule(re, pattern))
		case strings.HasPrefix(rule, "between:"):
			low, high, err := parseTwoNumbers(rule[len("between:"):], "between", fieldType.String())
			if err != nil {
				return nil, err
			}
			validator.rules = append(validator.rules, betweenRule(low, high, rule))
		case strings.HasPrefix(rule, "len:"):
			min, max, err := parseLenRule(rule[len("len:"):], "len", fieldType.String())
			if err != nil {
				return nil, err
			}
			validator.rules = append(validator.rules, lenRule(min, max))
		case strings.HasPrefix(rule, "itemLen:"):
			min, max, err := parseLenRule(rule[len("itemLen:"):], "itemLen", fieldType.String())
			if err != nil {
				return nil, err
			}
			validator.rules = append(validator.rules, itemLenRule(min, max))
		default:
		}
	}
	if validator.required && validator.nilable {
		return nil, fmt.Errorf("%s tag[%s] value is not vaild", fieldType.String(), "verf")
	}
	return validator, nil
}

func inListRule(enumValues []string, fieldType reflect.Type) compiledRule {
	for fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}
	switch fieldType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		set := make(map[int64]struct{}, len(enumValues))
		for _, s := range enumValues {
			v, err := strconv.ParseInt(s, 10, 64)
			if err == nil {
				set[v] = struct{}{}
			}
		}
		return func(v reflect.Value, fieldName string) error {
			v = derefValue(v)
			switch v.Kind() {
			case reflect.Slice, reflect.Array:
				for i := 0; i < v.Len(); i++ {
					ev := derefValue(v.Index(i))
					val := ev.Int()
					if _, ok := set[val]; !ok {
						return fmt.Errorf("%s[%d] '%d' is not in %v", fieldName, i, val, enumValues)
					}
				}
			default:
				val := v.Int()
				if _, ok := set[val]; !ok {
					return fmt.Errorf("%s is not in %v", fieldName, enumValues)
				}
			}
			return nil
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		set := make(map[uint64]struct{}, len(enumValues))
		for _, s := range enumValues {
			v, err := strconv.ParseUint(s, 10, 64)
			if err == nil {
				set[v] = struct{}{}
			}
		}
		return func(v reflect.Value, fieldName string) error {
			v = derefValue(v)
			switch v.Kind() {
			case reflect.Slice, reflect.Array:
				for i := 0; i < v.Len(); i++ {
					ev := derefValue(v.Index(i))
					val := ev.Uint()
					if _, ok := set[val]; !ok {
						return fmt.Errorf("%s[%d] '%d' is not in %v", fieldName, i, val, enumValues)
					}
				}
			default:
				val := v.Uint()
				if _, ok := set[val]; !ok {
					return fmt.Errorf("%s is not in %v", fieldName, enumValues)
				}
			}
			return nil
		}
	case reflect.Float32, reflect.Float64:
		set := make(map[float64]struct{}, len(enumValues))
		for _, s := range enumValues {
			v, err := strconv.ParseFloat(s, 64)
			if err == nil {
				set[v] = struct{}{}
			}
		}
		return func(v reflect.Value, fieldName string) error {
			v = derefValue(v)
			switch v.Kind() {
			case reflect.Slice, reflect.Array:
				for i := 0; i < v.Len(); i++ {
					ev := derefValue(v.Index(i))
					val := ev.Float()
					if _, ok := set[val]; !ok {
						return fmt.Errorf("%s[%d] '%v' is not in %v", fieldName, i, val, enumValues)
					}
				}
			default:
				val := v.Float()
				if _, ok := set[val]; !ok {
					return fmt.Errorf("%s is not in %v", fieldName, enumValues)
				}
			}
			return nil
		}
	default:
		set := make(map[string]struct{}, len(enumValues))
		for _, s := range enumValues {
			set[s] = struct{}{}
		}
		return func(v reflect.Value, fieldName string) error {
			v = derefValue(v)
			switch v.Kind() {
			case reflect.Struct, reflect.Interface, reflect.Ptr:
				return nil
			case reflect.Slice, reflect.Array:
				for i := 0; i < v.Len(); i++ {
					ev := derefValue(v.Index(i))
					val := valueToString(ev)
					if _, ok := set[val]; !ok {
						return fmt.Errorf("%s[%d] '%s' is not in %v", fieldName, i, val, enumValues)
					}
				}
				return nil
			default:
				val := valueToString(v)
				if _, ok := set[val]; !ok {
					return fmt.Errorf("%s is not in %v", fieldName, enumValues)
				}
				return nil
			}
		}
	}
}

func regRule(re *regexp.Regexp, pattern string) compiledRule {
	return func(v reflect.Value, fieldName string) error {
		v = derefValue(v)
		switch v.Kind() {
		case reflect.Struct, reflect.Interface, reflect.Ptr:
			return nil
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				ev := derefValue(v.Index(i))
				val := valueToString(ev)
				if !re.MatchString(val) {
					return fmt.Errorf("%s[%d] '%s' is not match %v", fieldName, i, val, pattern)
				}
			}
			return nil
		default:
			val := valueToString(v)
			if !re.MatchString(val) {
				return fmt.Errorf("%s is not match %v", fieldName, pattern)
			}
			return nil
		}
	}
}

func betweenRule(low float64, high float64, rawRule string) compiledRule {
	splits := stringsSplitAndTrim(rawRule[len("between:"):], ",")
	lowStr, highStr := "", ""
	if len(splits) >= 2 {
		lowStr, highStr = splits[0], splits[1]
	}
	return func(v reflect.Value, fieldName string) error {
		v = derefValue(v)
		switch v.Kind() {
		case reflect.Struct, reflect.Interface, reflect.Ptr:
			return nil
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				ev := derefValue(v.Index(i))
				val, ok := valueToFloat64(ev)
				if !ok {
					continue
				}
				if val < low || val > high {
					return fmt.Errorf("%s[%d] '%s' is not between %s and %s", fieldName, i, valueToString(ev), lowStr, highStr)
				}
			}
			return nil
		default:
			val, ok := valueToFloat64(v)
			if !ok {
				return nil
			}
			if val < low || val > high {
				return fmt.Errorf("%s is not between %s and %s", fieldName, lowStr, highStr)
			}
			return nil
		}
	}
}

func lenRule(min *int, max *int) compiledRule {
	return func(v reflect.Value, fieldName string) error {
		length, ok := valueLen(v)
		if !ok {
			return nil
		}
		if min != nil && length < *min {
			return fmt.Errorf("%s len is less than %d", fieldName, *min)
		}
		if max != nil && length > *max {
			return fmt.Errorf("%s len is greater than %d", fieldName, *max)
		}
		return nil
	}
}

func itemLenRule(min *int, max *int) compiledRule {
	return func(v reflect.Value, fieldName string) error {
		v = derefValue(v)
		switch v.Kind() {
		case reflect.Struct, reflect.Interface, reflect.Ptr:
			return nil
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				item := v.Index(i)
				length, ok := valueLen(item)
				if !ok {
					continue
				}
				if min != nil && length < *min {
					return fmt.Errorf("%s[%d] len is less than %d", fieldName, i, *min)
				}
				if max != nil && length > *max {
					return fmt.Errorf("%s[%d] len is greater than %d", fieldName, i, *max)
				}
			}
			return nil
		default:
			length, ok := valueLen(v)
			if !ok {
				return nil
			}
			if min != nil && length < *min {
				return fmt.Errorf("%s len is less than %d", fieldName, *min)
			}
			if max != nil && length > *max {
				return fmt.Errorf("%s len is greater than %d", fieldName, *max)
			}
			return nil
		}
	}
}

func parseTwoNumbers(value string, tag string, fieldName string) (float64, float64, error) {
	splits := stringsSplitAndTrim(value, ",")
	if len(splits) != 2 {
		return 0, 0, fmt.Errorf("%s tag[%s] value is not vaild", fieldName, tag)
	}
	low, err := strconv.ParseFloat(splits[0], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("%s tag[%s] value is not vaild", fieldName, tag)
	}
	high, err := strconv.ParseFloat(splits[1], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("%s tag[%s] value is not vaild", fieldName, tag)
	}
	return low, high, nil
}

func parseLenRule(value string, tag string, fieldName string) (*int, *int, error) {
	splits := strings.Split(value, ",")
	if len(splits) > 2 {
		return nil, nil, fmt.Errorf("%s tag[%s] value is not vaild", fieldName, tag)
	}
	var minPtr *int
	var maxPtr *int
	if len(splits) >= 1 && strings.TrimSpace(splits[0]) != "" {
		min, err := strconv.Atoi(strings.TrimSpace(splits[0]))
		if err != nil {
			return nil, nil, fmt.Errorf("%s tag[%s] value is not vaild", fieldName, tag)
		}
		minPtr = &min
	}
	if len(splits) == 2 && strings.TrimSpace(splits[1]) != "" {
		max, err := strconv.Atoi(strings.TrimSpace(splits[1]))
		if err != nil {
			return nil, nil, fmt.Errorf("%s tag[%s] value is not vaild", fieldName, tag)
		}
		maxPtr = &max
	}
	return minPtr, maxPtr, nil
}

func derefValue(v reflect.Value) reflect.Value {
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr) {
		if v.IsNil() {
			return v
		}
		v = v.Elem()
	}
	return v
}

func valueToString(v reflect.Value) string {
	v = derefValue(v)
	if !v.IsValid() {
		return ""
	}
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64)
	case reflect.Bool:
		if v.Bool() {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func valueToFloat64(v reflect.Value) (float64, bool) {
	v = derefValue(v)
	if !v.IsValid() {
		return 0, false
	}
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint()), true
	case reflect.Float32, reflect.Float64:
		return v.Float(), true
	case reflect.String:
		f, err := strconv.ParseFloat(v.String(), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func valueLen(v reflect.Value) (int, bool) {
	v = derefValue(v)
	if !v.IsValid() {
		return 0, false
	}
	switch v.Kind() {
	case reflect.String:
		return utf8.RuneCountInString(v.String()), true
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 {
			return v.Len(), true
		}
		if v.Type().Elem().Kind() == reflect.String {
			total := 0
			for i := 0; i < v.Len(); i++ {
				total += utf8.RuneCountInString(v.Index(i).String())
			}
			return total, true
		}
		return v.Len(), true
	case reflect.Map:
		return v.Len(), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Bool:
		return len(valueToString(v)), true
	default:
		return 0, false
	}
}

func (oai *OpenApiV3) Validate(arg any) error {
	if arg == nil {
		return nil
	}
	if oai.validator == nil {
		oai.validator = newDefaultValidator()
	}

	val := reflect.ValueOf(arg)
	for val.IsValid() && (val.Kind() == reflect.Interface || val.Kind() == reflect.Pointer) {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}
	if !val.IsValid() || val.Kind() != reflect.Struct {
		return nil
	}

	schemaName := oai.golangTypeToSchemaName(val.Type())
	if oai.Components.Schemas.Get(schemaName) == nil {
		_ = oai.addSchema(val.Interface())
	}

	return oai.validateStructWithSchema(val, schemaName, val.Type().Name())
}

func (oai *OpenApiV3) validateStructWithSchema(v reflect.Value, schemaName string, displayPrefix string) error {
	schemaRef := oai.Components.Schemas.Get(schemaName)
	if schemaRef == nil || schemaRef.Value == nil {
		return nil
	}
	schema := schemaRef.Value
	if schema.Type != TypeObject {
		return nil
	}

	t := v.Type()
	indexKey := schemaName
	verfIndexes := oai.validator.verfFieldIndexes(indexKey, t, schema)
	if len(verfIndexes) == 0 {
		return nil
	}

	for _, i := range verfIndexes {
		field := t.Field(i)

		if field.Anonymous {
			fv := derefValue(v.Field(i))
			ft := field.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if fv.IsValid() && ft.Kind() == reflect.Struct && ft.String() != "time.Time" {
				if err := oai.validateStructWithSchema(fv, schemaName, displayPrefix); err != nil {
					return err
				}
			}
			continue
		}

		jsonName := field.Name
		if tag := field.Tag.Get("json"); tag != "" {
			s := strings.Split(strings.Trim(tag, ","), ",")
			if len(s) > 0 && s[0] == "-" {
				continue
			}
			if len(s) > 0 && s[0] != "" {
				jsonName = s[0]
			}
		}

		propRef := schema.Properties.Get(jsonName)
		if propRef == nil {
			continue
		}
		propSchema := propRef.Value

		displayName := displayPrefix + "." + field.Name
		cacheKey := schemaName + "." + jsonName
		fieldVal := v.Field(i)
		if err := oai.validator.ValidateField(cacheKey, displayName, fieldVal, propSchema); err != nil {
			return err
		}

		fv := derefValue(fieldVal)
		if !fv.IsValid() || (fv.Kind() == reflect.Pointer && fv.IsNil()) {
			continue
		}

		switch fv.Kind() {
		case reflect.Struct:
			ft := fv.Type()
			if ft.String() == "time.Time" {
				continue
			}
			nestedSchemaName := propRef.Ref
			if nestedSchemaName == "" {
				nestedSchemaName = oai.golangTypeToSchemaName(ft)
			}
			if err := oai.validateStructWithSchema(fv, nestedSchemaName, displayName); err != nil {
				return err
			}
		case reflect.Slice, reflect.Array:
			if propSchema == nil || propSchema.Items == nil {
				continue
			}
			itemSchemaName := propSchema.Items.Ref
			for j := 0; j < fv.Len(); j++ {
				item := derefValue(fv.Index(j))
				if !item.IsValid() {
					continue
				}
				if item.Kind() == reflect.Struct && item.Type().String() != "time.Time" {
					if itemSchemaName == "" {
						itemSchemaName = oai.golangTypeToSchemaName(item.Type())
					}
					if err := oai.validateStructWithSchema(item, itemSchemaName, fmt.Sprintf("%s[%d]", displayName, j)); err != nil {
						return err
					}
				}
			}
		default:
		}
	}

	return nil
}
