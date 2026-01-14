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

package goai

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/caiflower/common-tools/pkg/tools"
)

type Schema struct {
	OneOf                SchemaRefs     `json:"oneOf,omitempty"`
	AnyOf                SchemaRefs     `json:"anyOf,omitempty"`
	AllOf                SchemaRefs     `json:"allOf,omitempty"`
	Not                  *SchemaRef     `json:"not,omitempty"`
	Type                 string         `json:"type,omitempty"`
	Title                string         `json:"title,omitempty"`
	Format               string         `json:"format,omitempty"`
	Description          string         `json:"description,omitempty"`
	Enum                 []any          `json:"enum,omitempty"`
	Default              any            `json:"default,omitempty"`
	Example              any            `json:"example,omitempty"`
	ExternalDocs         *ExternalDoc   `json:"externalDocs,omitempty"`
	UniqueItems          bool           `json:"uniqueItems,omitempty"`
	ExclusiveMin         bool           `json:"exclusiveMinimum,omitempty"`
	ExclusiveMax         bool           `json:"exclusiveMaximum,omitempty"`
	Nullable             bool           `json:"nullable,omitempty"`
	ReadOnly             bool           `json:"readOnly,omitempty"`
	WriteOnly            bool           `json:"writeOnly,omitempty"`
	AllowEmptyValue      bool           `json:"allowEmptyValue,omitempty"`
	XML                  any            `json:"xml,omitempty"`
	Deprecated           bool           `json:"deprecated,omitempty"`
	Min                  *float64       `json:"minimum,omitempty"`
	Max                  *float64       `json:"maximum,omitempty"`
	MultipleOf           *float64       `json:"multipleOf,omitempty"`
	MinLength            uint64         `json:"minLength,omitempty"`
	MaxLength            *uint64        `json:"maxLength,omitempty"`
	Pattern              string         `json:"pattern,omitempty"`
	MinItems             uint64         `json:"minItems,omitempty"`
	MaxItems             *uint64        `json:"maxItems,omitempty"`
	Items                *SchemaRef     `json:"items,omitempty"`
	Required             []string       `json:"required,omitempty"`
	Properties           Schemas        `json:"properties,omitempty"`
	MinProps             uint64         `json:"minProperties,omitempty"`
	MaxProps             *uint64        `json:"maxProperties,omitempty"`
	AdditionalProperties *SchemaRef     `json:"additionalProperties,omitempty"`
	Discriminator        *Discriminator `json:"discriminator,omitempty"`
	XExtensions          XExtension     `json:"-"`
	ValidationRules      string         `json:"x-validation,omitempty"`
}

// Clone creates a deep copy of the Schema
func (s *Schema) Clone() *Schema {
	if s == nil {
		return nil
	}
	clone := *s
	// Clone slices
	clone.OneOf = make(SchemaRefs, len(s.OneOf))
	copy(clone.OneOf, s.OneOf)
	clone.AnyOf = make(SchemaRefs, len(s.AnyOf))
	copy(clone.AnyOf, s.AnyOf)
	clone.AllOf = make(SchemaRefs, len(s.AllOf))
	copy(clone.AllOf, s.AllOf)
	// Clone Not if present
	if s.Not != nil {
		clone.Not = &SchemaRef{
			Ref:   s.Not.Ref,
			Value: s.Not.Value.Clone(),
		}
	}
	// Clone Enum
	clone.Enum = make([]any, len(s.Enum))
	copy(clone.Enum, s.Enum)
	return &clone
}

func (s Schema) MarshalJSON() ([]byte, error) {
	var (
		b   []byte
		m   map[string]json.RawMessage
		err error
	)
	type tempSchema Schema // To prevent JSON marshal recursion error.
	if b, err = json.Marshal(tempSchema(s)); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	for k, v := range s.XExtensions {
		if b, err = json.Marshal(v); err != nil {
			return nil, err
		}
		m[k] = b
	}
	return json.Marshal(m)
}

type Discriminator struct {
	PropertyName string            `json:"propertyName"`
	Mapping      map[string]string `json:"mapping,omitempty"`
}

type SchemaRef struct {
	Ref         string  `json:"$ref,omitempty"`
	Value       *Schema `json:",omitempty"`
	Description string  `json:",omitempty"`
}

func (r SchemaRef) MarshalJSON() ([]byte, error) {
	if r.Ref != "" {
		return formatRefAndDescToBytes(r.Ref, r.Description), nil
	}
	return json.Marshal(r.Value)
}

func formatRefAndDescToBytes(ref, desc string) []byte {
	return []byte(fmt.Sprintf(`{"$ref":"#/components/schemas/%s","description":"%s"}`, ref, desc))
}

type SchemaRefs []*SchemaRef

type Schemas struct {
	refs map[string]SchemaRef
}

func createSchemas() Schemas {
	return Schemas{
		refs: make(map[string]SchemaRef),
	}
}

func (s *Schemas) Get(key string) *SchemaRef {
	if v, ok := s.refs[key]; ok {
		return &v
	}
	return nil
}

func (s *Schemas) Set(key string, ref SchemaRef) {
	s.refs[key] = ref
}

func (s *Schemas) Map() map[string]SchemaRef {
	return s.refs
}

func (s Schemas) MarshalJSON() ([]byte, error) {
	if len(s.refs) == 0 {
		return []byte("{}"), nil
	}
	r, err := tools.Marshal(s.refs)
	return r, err
}

func (s *Schemas) Clone() Schemas {
	newSchemas := createSchemas()
	for k, v := range s.refs {
		newSchemas.Set(k, v)
	}
	return newSchemas
}

func (s *Schemas) Iterator(fn func(key string, ref SchemaRef) bool) {
	for k, v := range s.refs {
		if !fn(k, v) {
			break
		}
	}
}

func (s *Schemas) Removes(items []any) {
	for _, item := range items {
		if key, ok := item.(string); ok {
			delete(s.refs, key)
		}
	}
}

func (oai *OpenApiV3) addSchema(object ...any) error {
	for _, v := range object {
		if err := oai.doAddSchemaSingle(v); err != nil {
			return err
		}
	}
	return nil
}

func (oai *OpenApiV3) doAddSchemaSingle(object any) error {
	if oai.Components.Schemas.refs == nil {
		oai.Components.Schemas = createSchemas()
	}

	var (
		reflectType    = reflect.TypeOf(object)
		structTypeName = oai.golangTypeToSchemaName(reflectType)
	)

	if oai.Components.Schemas.Get(structTypeName) != nil {
		return nil
	}

	schema, err := oai.structToSchema(object)
	if err != nil {
		return err
	}

	oai.Components.Schemas.Set(structTypeName, SchemaRef{
		Ref:   "",
		Value: schema,
	})
	return nil
}

func (oai *OpenApiV3) structToSchema(object any) (*Schema, error) {
	schema := &Schema{
		Properties:  createSchemas(),
		XExtensions: make(XExtension),
	}

	if err := oai.tagMapToSchema(reflect.TypeOf(object), schema); err != nil {
		return nil, err
	}

	if schema.Type != "" && schema.Type != TypeObject {
		return schema, nil
	}

	if isArray(object) {
		schema.Type = TypeArray
		subSchemaRef, err := oai.newSchemaRefWithGolangType(reflect.TypeOf(object).Elem(), reflect.StructTag(""))
		if err != nil {
			return nil, err
		}
		schema.Items = subSchemaRef
		if len(schema.Enum) > 0 {
			schema.Items.Value.Enum = schema.Enum
			schema.Enum = nil
		}
		return schema, nil
	}

	schema.Type = TypeObject
	t := reflect.TypeOf(object)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		structField := t.Field(i)
		if !unicode.IsUpper(rune(structField.Name[0])) {
			continue
		}

		fieldName := structField.Name
		jsonTag := structField.Tag.Get("json")
		if jsonTag != "" {
			fieldName = strings.Split(strings.Trim(jsonTag, ","), ",")[0]
		}

		schemaRef, err := oai.newSchemaRefWithGolangType(structField.Type, structField.Tag)
		if err != nil {
			return nil, err
		}

		verfTag := strings.TrimSpace(structField.Tag.Get("verf"))
		validateTag := strings.TrimSpace(structField.Tag.Get("validate"))
		if schemaRef.Value != nil {
			schemaRef.Value.ValidationRules = mergeValidationRules(verfTag, validateTag)
		}

		schema.Properties.Set(fieldName, *schemaRef)
	}

	var ignoreProperties []any
	schema.Properties.Iterator(func(key string, ref SchemaRef) bool {
		if ref.Value != nil && ref.Value.ValidationRules != "" {
			if strings.Contains(ref.Value.ValidationRules, validationRuleKeyForNilable) {
				ref.Value.Nullable = true
			}
			if strings.Contains(ref.Value.ValidationRules, validationRuleKeyForRequired) && !strings.Contains(ref.Value.ValidationRules, validationRuleKeyForNilable) {
				schema.Required = append(schema.Required, key)
			}

			rules := stringsSplitAndTrim(ref.Value.ValidationRules, "|")
			for _, rule := range rules {
				if strings.HasPrefix(rule, validationRuleKeyForInList) {
					enumArray := stringsSplitAndTrim(rule[len(validationRuleKeyForInList):], ",")
					var enumValues []any
					for _, e := range enumArray {
						if isNumeric(e) {
							enumValues = append(enumValues, tools.ToInt(e))
						} else {
							enumValues = append(enumValues, e)
						}
					}
					ref.Value.Enum = enumValues
				}

				if strings.HasPrefix(rule, validationRuleKeyForReg) {
					ref.Value.Pattern = rule[len(validationRuleKeyForReg):]
				}

				if strings.HasPrefix(rule, validationRuleKeyForLen) {
					lenRule := stringsSplitAndTrim(rule[len(validationRuleKeyForLen):], ",")
					if len(lenRule) >= 1 && len(lenRule) <= 2 {
						var (
							minSet bool
							maxSet bool
							minVal uint64
							maxVal uint64
						)
						if lenRule[0] != "" {
							minSet = true
							minVal = tools.ToUint64(lenRule[0])
						}
						if len(lenRule) == 2 && lenRule[1] != "" {
							maxSet = true
							maxVal = tools.ToUint64(lenRule[1])
						}
						switch ref.Value.Type {
						case TypeString:
							if minSet {
								ref.Value.MinLength = minVal
							}
							if maxSet {
								ref.Value.MaxLength = &maxVal
							}
						case TypeArray:
							if minSet {
								ref.Value.MinItems = minVal
							}
							if maxSet {
								ref.Value.MaxItems = &maxVal
							}
						case TypeObject:
							if minSet {
								ref.Value.MinProps = minVal
							}
							if maxSet {
								ref.Value.MaxProps = &maxVal
							}
						default:
							if minSet {
								ref.Value.MinLength = minVal
							}
							if maxSet {
								ref.Value.MaxLength = &maxVal
							}
						}
					}
				}

				if strings.HasPrefix(rule, validationRuleKeyForBetween) {
					if ref.Value.Type == TypeInteger || ref.Value.Type == TypeNumber {
						betweenRule := stringsSplitAndTrim(rule[len(validationRuleKeyForBetween):], ",")
						if len(betweenRule) == 2 {
							min := tools.ToFloat64(betweenRule[0])
							max := tools.ToFloat64(betweenRule[1])
							ref.Value.Min = &min
							ref.Value.Max = &max
						}
					}
				}

				if strings.HasPrefix(rule, validationRuleKeyForItemLen) {
					if ref.Value.Type != TypeArray || ref.Value.Items == nil || ref.Value.Items.Value == nil {
						continue
					}
					itemLenRule := stringsSplitAndTrim(rule[len(validationRuleKeyForItemLen):], ",")
					if len(itemLenRule) >= 1 && len(itemLenRule) <= 2 {
						var (
							minSet bool
							maxSet bool
							minVal uint64
							maxVal uint64
						)
						if itemLenRule[0] != "" {
							minSet = true
							minVal = tools.ToUint64(itemLenRule[0])
						}
						if len(itemLenRule) == 2 && itemLenRule[1] != "" {
							maxSet = true
							maxVal = tools.ToUint64(itemLenRule[1])
						}
						if ref.Value.Items.Value.Type == TypeString {
							if minSet {
								ref.Value.Items.Value.MinLength = minVal
							}
							if maxSet {
								ref.Value.Items.Value.MaxLength = &maxVal
							}
						}
					}
				}
			}
		}
		if !isValidParameterName(key) {
			ignoreProperties = append(ignoreProperties, key)
		}
		return true
	})

	if len(ignoreProperties) > 0 {
		schema.Properties.Removes(ignoreProperties)
	}

	return schema, nil
}

func mergeValidationRules(verfTag string, validateTag string) string {
	if verfTag == "" {
		return validateTag
	}
	if validateTag == "" {
		return verfTag
	}
	a := stringsSplitAndTrim(verfTag, "|")
	b := stringsSplitAndTrim(validateTag, "|")
	distinct := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, r := range a {
		if _, ok := distinct[r]; ok {
			continue
		}
		distinct[r] = struct{}{}
		out = append(out, r)
	}
	for _, r := range b {
		if _, ok := distinct[r]; ok {
			continue
		}
		distinct[r] = struct{}{}
		out = append(out, r)
	}
	return strings.Join(out, "|")
}

func (oai *OpenApiV3) tagMapToSchema(t reflect.Type, schema *Schema) error {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		verfTag := field.Tag.Get("verf")
		validateTag := field.Tag.Get("validate")
		if strings.TrimSpace(verfTag) != "" || strings.TrimSpace(validateTag) != "" {
			schema.ValidationRules = mergeValidationRules(strings.TrimSpace(verfTag), strings.TrimSpace(validateTag))
			break
		}
	}

	return nil
}
