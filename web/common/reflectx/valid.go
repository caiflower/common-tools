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

package reflectx

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/caiflower/common-tools/pkg/basic"
)

const (
	verf = "verf"
)

const (
	validationRuleKeyForRequired = `required`
	validationRuleKeyForNilable  = `nilable`
	validationRuleKeyForInList   = `inList:`
	validationRuleKeyForReg      = `reg:`
	validationRuleKeyForBetween  = `between:`
	validationRuleKeyForLen      = `len:`
	validationRuleKeyForItemLen  = `itemLen:`
)

type compiledFieldValidator struct {
	required bool
	nilable  bool
	rules    []compiledRule
}

type compiledRule func(v reflect.Value, fieldName string) error

var validMap = make(map[string]*compiledFieldValidator)

type BuildValidData struct {
	PackageName   string
	alreadyStruct map[string]struct{}
}

func BuildValid(structField reflect.StructField, vValue reflect.Value, v interface{}) (err error) {
	data := v.(BuildValidData)
	if data.alreadyStruct == nil {
		data.alreadyStruct = make(map[string]struct{})
	}

	switch vValue.Kind() {
	case reflect.Ptr:
		pValue := reflect.New(structField.Type.Elem()).Elem()
		switch pValue.Kind() {
		case reflect.Struct:
			newValue := vValue
			if vValue.IsZero() {
				newValue = reflect.New(structField.Type.Elem())
			}
			structName := structField.Type.String()
			if _, ok := data.alreadyStruct[structName]; ok {
				return
			}
			data.alreadyStruct[structName] = struct{}{}

			if isTime(structField, newValue) {

			} else {
				for i := 0; i < pValue.NumField(); i++ {
					field := newValue.Elem().Field(i)
					fieldStruct := pValue.Type().Field(i)
					dataTmp := data.PackageName
					if !structField.Anonymous {
						dataTmp = dataTmp + "." + structField.Name
					}
					if err = BuildValid(fieldStruct, field, BuildValidData{
						PackageName:   dataTmp,
						alreadyStruct: data.alreadyStruct,
					}); err != nil {
						return
					}
				}
			}
		default:
		}
	case reflect.Struct:
		t := structField.Type
		if _, ok := data.alreadyStruct[t.String()]; ok {
			return
		}
		data.alreadyStruct[t.String()] = struct{}{}

		if !isTime(structField, vValue) {
			for i := 0; i < t.NumField(); i++ {
				fieldStruct := t.Field(i)
				dataTmp := data.PackageName
				if !structField.Anonymous {
					dataTmp = dataTmp + "." + structField.Name
				}
				if err = BuildValid(fieldStruct, vValue.Field(i), BuildValidData{
					PackageName:   dataTmp,
					alreadyStruct: data.alreadyStruct,
				}); err != nil {
					return
				}
			}
			return
		}
	default:

	}

	fieldName := data.PackageName + "." + structField.Name
	if _, ok := validMap[fieldName]; ok {
		return
	}

	tagValue := structField.Tag.Get(verf)
	if strings.TrimSpace(tagValue) == "" {
		return nil
	}

	validator, buildErr := compileFieldValidator(tagValue, structField.Type)
	if buildErr != nil {
		panic(buildErr.Error())
	}

	if validator.required || validator.nilable || len(validator.rules) > 0 {
		validMap[fieldName] = validator
	}

	return nil
}

type ValidObject struct {
	PkgPath   string
	FiledName string
}

func CheckParam(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	tagValue := structField.Tag.Get(verf)
	hasRequired, hasNilable := hasRule(tagValue, validationRuleKeyForRequired), hasRule(tagValue, validationRuleKeyForNilable)
	if (tagValue == "" || hasNilable || !hasRequired) && vValue.IsZero() {
		return nil
	}

	object := data.(ValidObject)
	switch vValue.Kind() {
	case reflect.Ptr:
		// 获取指针指向的值
		pValue := reflect.New(structField.Type.Elem()).Elem()
		switch pValue.Kind() {
		case reflect.Struct:
			fieldName := object.PkgPath + "." + object.FiledName + "." + structField.Name
			if fieldValidator, ok := validMap[fieldName]; ok && fieldValidator.required && vValue.IsNil() {
				return fmt.Errorf("%s is missing", object.FiledName+"."+structField.Name)
			}

			newValue := vValue
			if vValue.IsZero() {
				newValue = reflect.New(structField.Type.Elem())
			}

			if isTime(structField, newValue) {
			} else {
				for i := 0; i < pValue.NumField(); i++ {
					field := newValue.Elem().Field(i)
					fieldStruct := pValue.Type().Field(i)
					objectTmp := object
					if !structField.Anonymous {
						objectTmp.FiledName += "." + structField.Name
					}
					if err = CheckParam(fieldStruct, field, objectTmp); err != nil {
						return
					}
				}
			}

		default:
		}
	case reflect.Struct:
		t := structField.Type
		if isTime(structField, vValue) {
		} else {
			for i := 0; i < t.NumField(); i++ {
				fieldStruct := t.Field(i)
				objectTmp := object
				if !structField.Anonymous {
					objectTmp.FiledName += "." + structField.Name
				}
				if err = CheckParam(fieldStruct, vValue.Field(i), objectTmp); err != nil {
					return
				}
			}
			return
		}
	default:
	}

	fieldName := object.PkgPath + "." + object.FiledName + "." + structField.Name
	if fieldValidator, ok := validMap[fieldName]; ok {
		fieldDisplayName := object.FiledName + "." + structField.Name
		if vValue.IsZero() && (fieldValidator.nilable || !fieldValidator.required) {
			return nil
		}

		if fieldValidator.required {
			if err := requiredRule(vValue, fieldDisplayName); err != nil {
				return err
			}
		}
		for _, rule := range fieldValidator.rules {
			if err := rule(vValue, fieldDisplayName); err != nil {
				return err
			}
		}
	}

	return
}

func isTime(structField reflect.StructField, value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Ptr:
		return reflect.New(structField.Type.Elem()).Type().AssignableTo(reflect.TypeOf(new(basic.TimeStandard))) || reflect.New(structField.Type.Elem()).Type().AssignableTo(reflect.TypeOf(new(basic.Time))) || reflect.New(structField.Type.Elem()).Type().AssignableTo(reflect.TypeOf(new(time.Time)))
	case reflect.Struct:
		return reflect.New(structField.Type).Type().AssignableTo(reflect.TypeOf(new(basic.TimeStandard))) || reflect.New(structField.Type).Type().AssignableTo(reflect.TypeOf(new(basic.Time))) || reflect.New(structField.Type).Type().AssignableTo(reflect.TypeOf(new(time.Time)))
	default:
		return false
	}
}

func compileFieldValidator(tagValue string, fieldType reflect.Type) (*compiledFieldValidator, error) {
	rules := stringsSplitAndTrim(tagValue, "|")
	validator := &compiledFieldValidator{}

	for _, rule := range rules {
		switch {
		case rule == validationRuleKeyForRequired:
			validator.required = true
		case rule == validationRuleKeyForNilable:
			validator.nilable = true
		case strings.HasPrefix(rule, validationRuleKeyForInList):
			enumValues := stringsSplitAndTrim(rule[len(validationRuleKeyForInList):], ",")
			if len(enumValues) == 0 {
				return nil, fmt.Errorf("%s tag[%s] value is not vaild", fieldType.String(), validationRuleKeyForInList)
			}
			validator.rules = append(validator.rules, inListRule(enumValues, fieldType, rule))
		case strings.HasPrefix(rule, validationRuleKeyForReg):
			pattern := strings.TrimSpace(rule[len(validationRuleKeyForReg):])
			if pattern == "" {
				return nil, fmt.Errorf("%s tag[%s] value is not vaild", fieldType.String(), validationRuleKeyForReg)
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("%s tag[%s] value is not vaild", fieldType.String(), validationRuleKeyForReg)
			}
			validator.rules = append(validator.rules, regRule(re, pattern))
		case strings.HasPrefix(rule, validationRuleKeyForBetween):
			low, high, err := parseTwoNumbers(rule[len(validationRuleKeyForBetween):], validationRuleKeyForBetween, fieldType.String())
			if err != nil {
				return nil, err
			}
			validator.rules = append(validator.rules, betweenRule(low, high, rule))
		case strings.HasPrefix(rule, validationRuleKeyForLen):
			min, max, err := parseLenRule(rule[len(validationRuleKeyForLen):], validationRuleKeyForLen, fieldType.String())
			if err != nil {
				return nil, err
			}
			validator.rules = append(validator.rules, lenRule(min, max))
		case strings.HasPrefix(rule, validationRuleKeyForItemLen):
			min, max, err := parseLenRule(rule[len(validationRuleKeyForItemLen):], validationRuleKeyForItemLen, fieldType.String())
			if err != nil {
				return nil, err
			}
			validator.rules = append(validator.rules, itemLenRule(min, max))
		default:
		}
	}

	if validator.required && validator.nilable {
		return nil, fmt.Errorf("%s tag[%s] value is not vaild", fieldType.String(), verf)
	}

	return validator, nil
}

func hasRule(tagValue string, key string) bool {
	for _, r := range stringsSplitAndTrim(tagValue, "|") {
		if r == key {
			return true
		}
	}
	return false
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

func inListRule(enumValues []string, fieldType reflect.Type, rawRule string) compiledRule {
	for fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}

	switch fieldType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		set := make(map[int64]struct{}, len(enumValues))
		for _, s := range enumValues {
			v, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				continue
			}
			set[v] = struct{}{}
		}
		return func(v reflect.Value, fieldName string) error {
			v = derefValue(v)
			switch v.Kind() {
			case reflect.Slice, reflect.Array:
				for i := 0; i < v.Len(); i++ {
					ev := derefValue(v.Index(i))
					if ev.Kind() == reflect.Struct || ev.Kind() == reflect.Interface || ev.Kind() == reflect.Ptr {
						continue
					}
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
			if err != nil {
				continue
			}
			set[v] = struct{}{}
		}
		return func(v reflect.Value, fieldName string) error {
			v = derefValue(v)
			switch v.Kind() {
			case reflect.Slice, reflect.Array:
				for i := 0; i < v.Len(); i++ {
					ev := derefValue(v.Index(i))
					if ev.Kind() == reflect.Struct || ev.Kind() == reflect.Interface || ev.Kind() == reflect.Ptr {
						continue
					}
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
			if err != nil {
				continue
			}
			set[v] = struct{}{}
		}
		return func(v reflect.Value, fieldName string) error {
			v = derefValue(v)
			switch v.Kind() {
			case reflect.Slice, reflect.Array:
				for i := 0; i < v.Len(); i++ {
					ev := derefValue(v.Index(i))
					if ev.Kind() == reflect.Struct || ev.Kind() == reflect.Interface || ev.Kind() == reflect.Ptr {
						continue
					}
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
					if ev.Kind() == reflect.Struct || ev.Kind() == reflect.Interface || ev.Kind() == reflect.Ptr {
						continue
					}
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
	splits := stringsSplitAndTrim(rawRule[len(validationRuleKeyForBetween):], ",")
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

func stringsSplitAndTrim(s string, sep string) []string {
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
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
		return len(v.String()), true
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 {
			return v.Len(), true
		}
		if v.Type().Elem().Kind() == reflect.String {
			total := 0
			for i := 0; i < v.Len(); i++ {
				total += len(v.Index(i).String())
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
