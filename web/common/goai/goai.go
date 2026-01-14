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
)

const (
	TypeInteger = `integer`
	TypeNumber  = `number`
	TypeBoolean = `boolean`
	TypeArray   = `array`
	TypeString  = `string`
	TypeFile    = `file`
	TypeObject  = `object`

	FormatInt32    = `int32`
	FormatInt64    = `int64`
	FormatDouble   = `double`
	FormatByte     = `byte`
	FormatBinary   = `binary`
	FormatDate     = `date`
	FormatDateTime = `date-time`
	FormatPassword = `password`
)

const (
	ParameterInHeader = `header`
	ParameterInPath   = `path`
	ParameterInQuery  = `query`
	ParameterInCookie = `cookie`
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

var (
	defaultReadContentTypes  = []string{`application/json`}
	defaultWriteContentTypes = []string{`application/json`}
)

type OpenApiV3 struct {
	Config     Config     `json:"-"`
	Info       Info       `json:"info"`
	OpenAPI    string     `json:"openapi"`
	Components Components `json:"components,omitempty"`
	Paths      Paths      `json:"paths"`

	validator *defaultValidator
}

type AddInput struct {
	Path   string
	Prefix string
	Method string
	Object any
}

func New() *OpenApiV3 {
	oai := &OpenApiV3{
		Paths: make(Paths),
		Components: Components{
			Schemas: createSchemas(),
		},
	}
	oai.validator = newDefaultValidator()
	oai.fillWithDefaultValue()
	return oai
}

func (oai *OpenApiV3) Add(in AddInput) error {
	reflectValue := reflect.ValueOf(in.Object)
	for reflectValue.Kind() == reflect.Pointer {
		reflectValue = reflectValue.Elem()
	}
	switch reflectValue.Kind() {
	case reflect.Struct:
		return oai.addSchema(in.Object)
	case reflect.Func:
		return oai.addPath(addPathInput{
			Path:     in.Path,
			Prefix:   in.Prefix,
			Method:   in.Method,
			Function: in.Object,
		})
	default:
		return fmt.Errorf("unsupported parameter type %s, only struct/function type is supported", reflect.TypeOf(in.Object).String())
	}
}

func (oai OpenApiV3) String() string {
	b, _ := json.Marshal(oai)
	return string(b)
}

func (oai *OpenApiV3) golangTypeToOAIType(t reflect.Type) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String:
		return TypeString
	case reflect.Struct:
		return TypeObject
	case reflect.Slice, reflect.Array:
		if t.String() == `[]uint8` {
			return TypeString
		}
		return TypeArray
	case reflect.Bool:
		return TypeBoolean
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return TypeInteger
	case reflect.Float32, reflect.Float64:
		return TypeNumber
	default:
		return TypeObject
	}
}

func (oai *OpenApiV3) golangTypeToOAIFormat(t reflect.Type) string {
	format := t.String()
	switch strings.TrimLeft(format, "*") {
	case `[]uint8`:
		return FormatBinary
	default:
		if oai.isEmbeddedStructDefinition(t) {
			return `EmbeddedStructDefinition`
		}
		return format
	}
}

func (oai *OpenApiV3) golangTypeToSchemaName(t reflect.Type) string {
	var (
		pkgPath    string
		schemaName = strings.TrimLeft(t.String(), "*")
	)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	schemaName = strings.Replace(schemaName, `/`, `.`, -1)
	if pkgPath = t.PkgPath(); pkgPath != "" && pkgPath != "." {
		if !oai.Config.IgnorePkgPath {
			schemaName = strings.Replace(pkgPath, `/`, `.`, -1) + schemaName[strings.Index(schemaName, "."):]
		}
	}
	schemaName = strings.Replace(schemaName, ` `, ``, -1)
	schemaName = strings.Replace(schemaName, `{`, ``, -1)
	schemaName = strings.Replace(schemaName, `}`, ``, -1)
	schemaName = strings.Replace(schemaName, `[`, `.`, -1)
	schemaName = strings.Replace(schemaName, `]`, `.`, -1)
	return schemaName
}

func (oai *OpenApiV3) isEmbeddedStructDefinition(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Kind() == reflect.Struct && t.NumField() > 0 && t.Field(0).Anonymous
}

func formatRefToBytes(ref string) []byte {
	return []byte(fmt.Sprintf(`{"$ref":"#/components/schemas/%s"}`, ref))
}

func isValidParameterName(key string) bool {
	return key != "-"
}

type addPathInput struct {
	Path     string
	Prefix   string
	Method   string
	Function any
}

func (oai *OpenApiV3) addPath(in addPathInput) error {
	if oai.Paths == nil {
		oai.Paths = make(Paths)
	}

	var reflectType = reflect.TypeOf(in.Function)
	if reflectType.NumIn() < 1 || reflectType.NumOut() < 1 {
		return fmt.Errorf("unsupported function %s for OpenAPI Path register, there should be input & output structures", reflectType.String())
	}

	var (
		inputObject  reflect.Value
		outputObject reflect.Value
	)

	// Determine which parameter is the actual request struct
	var inputType reflect.Type
	if reflectType.NumIn() == 1 {
		// Single parameter: it's the request struct
		inputType = reflectType.In(0)
	} else {
		// Multiple parameters: check if first is context
		firstParamType := reflectType.In(0)
		if firstParamType.Kind() == reflect.Pointer {
			// Check if it's a context type by name or type
			elemType := firstParamType.Elem()
			if elemType.Name() == "RequestCtx" || elemType.Name() == "Context" ||
				strings.Contains(elemType.PkgPath(), "/app") {
				// First parameter is context, use second parameter as request
				inputType = reflectType.In(1)
			} else {
				// First parameter is not context, use it as request
				inputType = firstParamType
			}
		} else {
			// First parameter is not a pointer, use it as request
			inputType = firstParamType
		}
	}

	if inputType.Kind() == reflect.Pointer {
		inputObject = reflect.New(inputType.Elem()).Elem()
	} else {
		inputObject = reflect.New(inputType).Elem()
	}

	if inputObject.Kind() != reflect.Struct {
		return fmt.Errorf("unsupported function %s for OpenAPI Path register, request parameter is not a struct", reflectType.String())
	}

	// Use the dereferenced type for field iteration
	if inputType.Kind() == reflect.Pointer {
		inputType = inputType.Elem()
	}

	outputType := reflectType.Out(0)
	if outputType.Kind() == reflect.Pointer {
		outputObject = reflect.New(outputType.Elem()).Elem()
	} else {
		outputObject = reflect.New(outputType).Elem()
	}

	var (
		path                 = Path{XExtensions: make(XExtension)}
		inputStructTypeName  = oai.golangTypeToSchemaName(inputObject.Type())
		outputStructTypeName = oai.golangTypeToSchemaName(outputObject.Type())
		operation            = Operation{
			Responses:   make(Responses),
			XExtensions: make(XExtension),
		}
	)

	if in.Path == "" {
		in.Path = "/"
	}

	if in.Prefix != "" {
		if !strings.HasPrefix(in.Prefix, "/") {
			in.Prefix = "/" + in.Prefix
		}
		in.Path = strings.TrimRight(in.Prefix, "/") + "/" + strings.TrimLeft(in.Path, "/")
	}

	if v, ok := oai.Paths[in.Path]; ok {
		path = v
	}

	if in.Method == "" {
		in.Method = "POST"
	}

	if err := oai.addSchema(inputObject.Interface()); err != nil {
		return err
	}

	if err := oai.addSchema(outputObject.Interface()); err != nil {
		return err
	}

	operation.Summary = inputStructTypeName
	operation.Description = "API endpoint for " + inputStructTypeName
	// operation.OperationID = strings.ReplaceAll(inputStructTypeName, ".", "_")

	oai.collectParameters(inputType, &operation)

	if in.Method != "GET" && in.Method != "DELETE" {
		requestBody := RequestBody{
			Content: make(map[string]MediaType),
		}

		contentTypes := oai.Config.ReadContentTypes
		for _, v := range contentTypes {
			schemaRef, err := oai.getRequestSchemaRef(getRequestSchemaRefInput{
				BusinessStructName: inputStructTypeName,
				RequestObject:      oai.Config.CommonRequest,
				RequestDataField:   oai.Config.CommonRequestDataField,
			})
			if err != nil {
				return err
			}
			example := oai.generateRequestExample(inputObject.Type(), oai.Config.CommonRequest, oai.Config.CommonRequestDataField)
			requestBody.Content[v] = MediaType{Schema: schemaRef, Example: example}
		}

		operation.RequestBody = &RequestBodyRef{Value: &requestBody}
	}

	response, err := oai.getResponseFromObject(outputObject.Interface(), true, outputStructTypeName)
	if err != nil {
		return err
	}
	operation.Responses["200"] = ResponseRef{Value: response}

	oai.removeOperationDuplicatedProperties(&operation)

	switch strings.ToUpper(in.Method) {
	case "GET":
		operation.RequestBody = nil
		path.Get = &operation
	case "PUT":
		path.Put = &operation
	case "POST":
		path.Post = &operation
	case "DELETE":
		operation.RequestBody = nil
		path.Delete = &operation
	case "PATCH":
		path.Patch = &operation
	default:
		return fmt.Errorf("invalid method %s", in.Method)
	}

	oai.Paths[in.Path] = path
	return nil
}

func (oai *OpenApiV3) removeOperationDuplicatedProperties(operation *Operation) {
	if len(operation.Parameters) == 0 {
		return
	}

	var duplicatedParameterNames []any
	for _, parameter := range operation.Parameters {
		duplicatedParameterNames = append(duplicatedParameterNames, parameter.Value.Name)
	}

	if operation.RequestBody == nil || operation.RequestBody.Value == nil {
		return
	}

	for _, requestBodyContent := range operation.RequestBody.Value.Content {
		if requestBodyContent.Schema == nil {
			continue
		}

		if requestBodyContent.Schema.Ref != "" {
			if schema := oai.Components.Schemas.Get(requestBodyContent.Schema.Ref); schema != nil {
				newSchema := schema.Value.Clone()
				requestBodyContent.Schema.Ref = ""
				requestBodyContent.Schema.Value = newSchema
				newSchema.Required = oai.removeItemsFromArray(newSchema.Required, duplicatedParameterNames)
				newSchema.Properties.Removes(duplicatedParameterNames)
				if newSchema.Properties.refs == nil || len(newSchema.Properties.refs) == 0 {
					operation.RequestBody = nil
				}
				continue
			}
		}

		if requestBodyContent.Schema.Value != nil {
			requestBodyContent.Schema.Value.Required = oai.removeItemsFromArray(requestBodyContent.Schema.Value.Required, duplicatedParameterNames)
			requestBodyContent.Schema.Value.Properties.Removes(duplicatedParameterNames)
			continue
		}
	}
}

func (oai *OpenApiV3) removeItemsFromArray(array []string, items []any) []string {
	result := make([]string, 0, len(array))
	for _, item := range array {
		found := false
		for _, i := range items {
			if value, ok := i.(string); ok && value == item {
				found = true
				break
			}
		}
		if !found {
			result = append(result, item)
		}
	}
	return result
}

type getRequestSchemaRefInput struct {
	BusinessStructName string
	RequestObject      any
	RequestDataField   string
}

type getResponseSchemaRefInput struct {
	BusinessStructName string
	ResponseObject     any
	ResponseDataField  string
}

func (oai *OpenApiV3) getRequestSchemaRef(in getRequestSchemaRefInput) (*SchemaRef, error) {
	if in.RequestObject == nil {
		return &SchemaRef{
			Ref: in.BusinessStructName,
		}, nil
	}

	requestType := reflect.TypeOf(in.RequestObject)
	for requestType.Kind() == reflect.Pointer {
		requestType = requestType.Elem()
	}

	if requestType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("CommonRequest must be a struct type")
	}

	schema := &Schema{
		Type:       TypeObject,
		Properties: createSchemas(),
	}

	for i := 0; i < requestType.NumField(); i++ {
		field := requestType.Field(i)
		fieldName := field.Name

		jsonTag := field.Tag.Get("json")
		if jsonTag != "" {
			tagParts := strings.Split(jsonTag, ",")
			if tagParts[0] != "" {
				fieldName = tagParts[0]
			}
		}

		if fieldName == "" {
			continue
		}

		var fieldSchemaRef *SchemaRef
		var err error

		if fieldName == in.RequestDataField {
			fieldSchemaRef = &SchemaRef{
				Ref: in.BusinessStructName,
			}
		} else {
			fieldSchemaRef, err = oai.newSchemaRefWithGolangType(field.Type, field.Tag)
		}

		if err != nil {
			return nil, err
		}

		schema.Properties.Set(fieldName, *fieldSchemaRef)
	}

	return &SchemaRef{
		Value: schema,
	}, nil
}

func (oai *OpenApiV3) getResponseSchemaRef(in getResponseSchemaRefInput) (*SchemaRef, error) {
	if in.ResponseObject == nil {
		return &SchemaRef{
			Ref: in.BusinessStructName,
		}, nil
	}

	responseType := reflect.TypeOf(in.ResponseObject)
	for responseType.Kind() == reflect.Pointer {
		responseType = responseType.Elem()
	}

	if responseType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("CommonResponse must be a struct type")
	}

	schema := &Schema{
		Type:       TypeObject,
		Properties: createSchemas(),
	}

	for i := 0; i < responseType.NumField(); i++ {
		field := responseType.Field(i)
		fieldName := field.Name

		jsonTag := field.Tag.Get("json")
		if jsonTag != "" {
			tagParts := strings.Split(jsonTag, ",")
			if tagParts[0] != "" {
				fieldName = tagParts[0]
			}
		}

		if fieldName == "" {
			continue
		}

		var fieldSchemaRef *SchemaRef
		var err error

		if fieldName == in.ResponseDataField {
			fieldSchemaRef = &SchemaRef{
				Ref: in.BusinessStructName,
			}
		} else {
			fieldSchemaRef, err = oai.newSchemaRefWithGolangType(field.Type, field.Tag)
		}

		if err != nil {
			return nil, err
		}

		schema.Properties.Set(fieldName, *fieldSchemaRef)
	}

	return &SchemaRef{
		Value: schema,
	}, nil
}

func (oai *OpenApiV3) getResponseFromObject(object any, isDefault bool, businessStructName string) (*Response, error) {
	response := &Response{
		Description: "Success",
		Content:     make(map[string]MediaType),
	}

	var schemaRef *SchemaRef
	var err error

	if oai.Config.CommonResponse != nil && oai.Config.CommonResponseDataField != "" {
		schemaRef, err = oai.getResponseSchemaRef(getResponseSchemaRefInput{
			BusinessStructName: businessStructName,
			ResponseObject:     oai.Config.CommonResponse,
			ResponseDataField:  oai.Config.CommonResponseDataField,
		})
	} else {
		schemaRef, err = oai.newSchemaRefWithGolangType(reflect.TypeOf(object), reflect.StructTag(""))
	}

	if err != nil {
		return nil, err
	}

	contentTypes := oai.Config.WriteContentTypes
	for _, v := range contentTypes {
		response.Content[v] = MediaType{Schema: schemaRef}
	}

	return response, nil
}

func (oai *OpenApiV3) newSchemaRefWithGolangType(t reflect.Type, tag reflect.StructTag) (*SchemaRef, error) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	schema := &Schema{
		Type: oai.golangTypeToOAIType(t),
	}

	schema.Format = oai.golangTypeToOAIFormat(t)

	if tag != "" {
		if desc := tag.Get("description"); desc != "" {
			schema.Description = desc
		}
		if example := tag.Get("example"); example != "" {
			schema.Example = example
		}
	}

	if schema.Type == TypeObject {
		if t.Kind() == reflect.Struct && t.String() != "time.Time" {
			schemaName := oai.golangTypeToSchemaName(t)
			// Check if schema already exists
			if existingSchema := oai.Components.Schemas.Get(schemaName); existingSchema != nil && existingSchema.Value != nil {
				return &SchemaRef{
					Ref:   schemaName,
					Value: existingSchema.Value.Clone(),
				}, nil
			}
			// If schema doesn't exist, create it
			obj := reflect.New(t).Elem().Interface()
			objSchema, err := oai.structToSchema(obj)
			if err != nil {
				return nil, err
			}
			// Add schema to components
			oai.Components.Schemas.Set(schemaName, SchemaRef{
				Value: objSchema,
			})
			return &SchemaRef{
				Ref:   schemaName,
				Value: objSchema.Clone(),
			}, nil
		}
		if t.Kind() == reflect.Map {
			// For map types, we need to create schema for the value type if it's a struct
			valueType := t.Elem()
			for valueType.Kind() == reflect.Pointer {
				valueType = valueType.Elem()
			}
			if valueType.Kind() == reflect.Struct && valueType.String() != "time.Time" {
				schemaName := oai.golangTypeToSchemaName(valueType)
				// Check if schema already exists
				if existingSchema := oai.Components.Schemas.Get(schemaName); existingSchema != nil && existingSchema.Value != nil {
					schema.AdditionalProperties = &SchemaRef{
						Ref:   schemaName,
						Value: existingSchema.Value.Clone(),
					}
					return &SchemaRef{
						Value: schema,
					}, nil
				}
				// If schema doesn't exist, create it
				obj := reflect.New(valueType).Elem().Interface()
				objSchema, err := oai.structToSchema(obj)
				if err != nil {
					return nil, err
				}
				// Add schema to components
				oai.Components.Schemas.Set(schemaName, SchemaRef{
					Value: objSchema,
				})
				schema.AdditionalProperties = &SchemaRef{
					Ref:   schemaName,
					Value: objSchema.Clone(),
				}
				return &SchemaRef{
					Value: schema,
				}, nil
			}
		}
	}

	if schema.Type == TypeArray {
		elemType := t.Elem()
		for elemType.Kind() == reflect.Pointer {
			elemType = elemType.Elem()
		}
		if elemType.Kind() == reflect.Struct && elemType.String() != "time.Time" {
			schemaName := oai.golangTypeToSchemaName(elemType)
			// Check if schema already exists
			if existingSchema := oai.Components.Schemas.Get(schemaName); existingSchema != nil && existingSchema.Value != nil {
				schema.Items = &SchemaRef{
					Ref:   schemaName,
					Value: existingSchema.Value.Clone(),
				}
			} else {
				// If schema doesn't exist, create it
				obj := reflect.New(elemType).Elem().Interface()
				objSchema, err := oai.structToSchema(obj)
				if err != nil {
					return nil, err
				}
				// Add schema to components
				oai.Components.Schemas.Set(schemaName, SchemaRef{
					Value: objSchema,
				})
				schema.Items = &SchemaRef{
					Ref:   schemaName,
					Value: objSchema.Clone(),
				}
			}
		} else {
			subSchema, err := oai.newSchemaRefWithGolangType(elemType, reflect.StructTag(""))
			if err != nil {
				return nil, err
			}
			schema.Items = subSchema
		}
	}

	return &SchemaRef{
		Value: schema,
	}, nil
}

func isArray(v any) bool {
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Kind() == reflect.Slice || t.Kind() == reflect.Array
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
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

func (oai *OpenApiV3) generateEmptyExample(t reflect.Type) any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String:
		return ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return 0
	case reflect.Float32, reflect.Float64:
		return 0.0
	case reflect.Bool:
		return false
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return ""
		}
		elemType := t.Elem()
		for elemType.Kind() == reflect.Pointer {
			elemType = elemType.Elem()
		}
		if elemType.Kind() == reflect.Struct && elemType.String() != "time.Time" {
			return []any{oai.generateEmptyExample(t.Elem())}
		}
		return []any{}
	case reflect.Map:
		valueType := t.Elem()
		for valueType.Kind() == reflect.Pointer {
			valueType = valueType.Elem()
		}
		if valueType.Kind() == reflect.Struct && valueType.String() != "time.Time" {
			return map[string]any{"key": oai.generateEmptyExample(t.Elem())}
		}
		return map[string]any{}
	case reflect.Struct:
		if t.String() == "time.Time" {
			return ""
		}
		result := make(map[string]any)
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !unicode.IsUpper(rune(field.Name[0])) {
				continue
			}

			fieldName := field.Name
			jsonTag := field.Tag.Get("json")
			if jsonTag != "" {
				fieldName = strings.Split(strings.Trim(jsonTag, ","), ",")[0]
			}
			if fieldName == "-" {
				continue
			}

			result[fieldName] = oai.generateEmptyExample(field.Type)
		}
		return result
	default:
		return nil
	}
}

func (oai *OpenApiV3) generateRequestExample(businessType reflect.Type, commonRequest any, dataField string) any {
	if commonRequest == nil || dataField == "" {
		return oai.generateEmptyExample(businessType)
	}

	requestType := reflect.TypeOf(commonRequest)
	for requestType.Kind() == reflect.Pointer {
		requestType = requestType.Elem()
	}

	if requestType.Kind() != reflect.Struct {
		return oai.generateEmptyExample(businessType)
	}

	result := make(map[string]any)
	for i := 0; i < requestType.NumField(); i++ {
		field := requestType.Field(i)
		fieldName := field.Name

		jsonTag := field.Tag.Get("json")
		if jsonTag != "" {
			tagParts := strings.Split(jsonTag, ",")
			if tagParts[0] != "" {
				fieldName = tagParts[0]
			}
		}

		if fieldName == "" || fieldName == "-" {
			continue
		}

		if fieldName == dataField {
			result[fieldName] = oai.generateEmptyExample(businessType)
		} else {
			result[fieldName] = oai.generateEmptyExample(field.Type)
		}
	}

	return result
}

func (oai *OpenApiV3) collectParameters(t reflect.Type, operation *Operation) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	oai.collectParametersRecursive(t, operation, make(map[string]bool))
}

func (oai *OpenApiV3) collectParametersRecursive(t reflect.Type, operation *Operation, visited map[string]bool) {
	if visited[t.String()] {
		return
	}
	visited[t.String()] = true

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !unicode.IsUpper(rune(field.Name[0])) {
			continue
		}

		// Handle embedded structs
		if field.Anonymous {
			oai.collectParametersRecursive(field.Type, operation, visited)
			continue
		}

		paramName := field.Name
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" {
			paramName = strings.Split(strings.Trim(jsonTag, ","), ",")[0]
		}

		parameter := Parameter{
			Name:        paramName,
			Description: field.Tag.Get("description"),
			Required:    strings.Contains(field.Tag.Get("verf"), "required"),
			Schema:      &SchemaRef{},
			XExtensions: make(XExtension),
		}

		if field.Tag.Get("header") != "" {
			parameter.In = ParameterInHeader
		} else if field.Tag.Get("path") != "" {
			parameter.In = ParameterInPath
			parameter.Required = true
		} else if field.Tag.Get("query") != "" || field.Tag.Get("in") == "query" {
			parameter.In = ParameterInQuery
		} else {
			continue
		}

		schemaRef, err := oai.newSchemaRefWithGolangType(field.Type, field.Tag)
		if err != nil {
			continue
		}
		parameter.Schema = schemaRef

		operation.Parameters = append(operation.Parameters, &ParameterRef{Value: &parameter})
	}
}
