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
	"runtime"
	"strings"
	"sync"
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

	operationIDCounter map[string]int

	validator *defaultValidator
	mu        sync.RWMutex
}

type AddInput struct {
	Path        string
	Method      string
	Object      any
	OperationID string
}

func New() *OpenApiV3 {
	oai := &OpenApiV3{
		Paths: make(Paths),
		Components: Components{
			Schemas: createSchemas(),
		},
		operationIDCounter: make(map[string]int),
	}
	oai.validator = newDefaultValidator()
	oai.fillWithDefaultValue()
	return oai
}

var (
	defaultMu  sync.RWMutex
	defaultOAI = New()
)

func Default() *OpenApiV3 {
	defaultMu.RLock()
	o := defaultOAI
	defaultMu.RUnlock()
	return o
}

func SetDefault(o *OpenApiV3) {
	if o == nil {
		return
	}
	defaultMu.Lock()
	defaultOAI = o
	defaultMu.Unlock()
}

// Add registers a struct schema or a handler function into OpenAPI.
func (oai *OpenApiV3) Add(in AddInput) error {
	oai.mu.Lock()
	defer oai.mu.Unlock()

	if in.Object == nil {
		return fmt.Errorf("unsupported parameter type <nil>, only struct/function type is supported")
	}
	reflectValue := reflect.ValueOf(in.Object)
	for reflectValue.Kind() == reflect.Pointer {
		reflectValue = reflectValue.Elem()
	}
	switch reflectValue.Kind() {
	case reflect.Struct:
		return oai.addSchema(in.Object)
	case reflect.Func:
		return oai.addPath(addPathInput{
			Path:        in.Path,
			Method:      in.Method,
			Function:    in.Object,
			OperationID: in.OperationID,
		})
	default:
		return fmt.Errorf("unsupported parameter type %s, only struct/function type is supported", reflect.TypeOf(in.Object).String())
	}
}

func (oai OpenApiV3) String() string {
	oai.mu.RLock()
	defer oai.mu.RUnlock()

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
	Path        string
	Method      string
	Function    any
	OperationID string
}

// parsedHandlerSignature is a normalized view of a Go handler function signature.
type parsedHandlerSignature struct {
	requestType       reflect.Type
	requestStructType reflect.Type
	responseType      reflect.Type
	hasError          bool
}

func (oai *OpenApiV3) addPath(in addPathInput) error {
	if oai.Paths == nil {
		oai.Paths = make(Paths)
	}

	reflectType := reflect.TypeOf(in.Function)
	if reflectType.Kind() != reflect.Func {
		return fmt.Errorf("unsupported parameter type %s, only function type is supported", reflectType.String())
	}

	sig, err := oai.parseHandlerSignature(reflectType)
	if err != nil {
		return err
	}

	var (
		inputObject         reflect.Value
		inputStructTypeName string
		inputTypeForParams  reflect.Type
	)
	if sig.requestStructType != nil {
		inputObject = reflect.New(sig.requestStructType).Elem()
		inputStructTypeName = oai.golangTypeToSchemaName(inputObject.Type())
		inputTypeForParams = sig.requestStructType
	}

	var (
		path      = Path{XExtensions: make(XExtension)}
		operation = Operation{
			Responses:   make(Responses),
			XExtensions: make(XExtension),
		}
	)

	if in.Path == "" {
		in.Path = "/"
	}

	if v, ok := oai.Paths[in.Path]; ok {
		path = v
	}

	if in.Method == "" {
		in.Method = "POST"
	}

	operationID := oai.generateOperationID(in.Function, in.OperationID)
	operation.Summary = operationID
	operation.Description = "API endpoint for " + operation.Summary
	operation.OperationID = operationID

	if inputTypeForParams != nil {
		oai.collectParameters(inputTypeForParams, &operation)
	}

	if in.Method != "GET" && sig.requestStructType != nil {
		requestBody := RequestBody{
			Content:  make(map[string]MediaType),
			Required: true,
		}

		contentTypes := oai.Config.ReadContentTypes
		for _, v := range contentTypes {
			schemaRef, err := oai.getRequestSchemaRef(getRequestSchemaRefInput{
				BusinessSchema:   SchemaRef{Ref: inputStructTypeName},
				RequestObject:    oai.Config.CommonRequest,
				RequestDataField: oai.Config.CommonRequestDataField,
			})
			if err != nil {
				return err
			}
			example := oai.generateRequestExample(inputObject.Type(), oai.Config.CommonRequest, oai.Config.CommonRequestDataField)
			requestBody.Content[v] = MediaType{Schema: schemaRef, Example: example}
		}

		operation.RequestBody = &RequestBodyRef{Value: &requestBody}
	}

	if sig.requestStructType != nil {
		if err = oai.addSchema(inputObject.Interface()); err != nil {
			return err
		}
	}

	outputSchemaRef := oai.outputSchemaRef(sig.responseType)
	response, err := oai.getResponseFromOutput(outputSchemaRef, sig.responseType)
	if err != nil {
		return err
	}
	operation.Responses["200"] = ResponseRef{Value: response}

	if sig.hasError {
		if errResponse, err := oai.getResponseFromOutput(outputSchemaRef, sig.responseType); err == nil {
			errResponse.Description = "Invalid parameter"
			operation.Responses["400"] = ResponseRef{Value: errResponse}
		}
		if errResponse, err := oai.getResponseFromOutput(outputSchemaRef, sig.responseType); err == nil {
			errResponse.Description = "Internal Server Error"
			operation.Responses["500"] = ResponseRef{Value: errResponse}
		}

	}

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

// parseHandlerSignature validates and extracts request/response types from a handler function.
func (oai *OpenApiV3) parseHandlerSignature(ft reflect.Type) (*parsedHandlerSignature, error) {
	if ft.Kind() != reflect.Func {
		return nil, fmt.Errorf("unsupported parameter type %s, only function type is supported", ft.String())
	}

	var (
		reqType  reflect.Type
		inOffset int
	)

	if ft.NumIn() >= 1 && isContextParamType(ft.In(0)) {
		inOffset = 1
	}

	switch ft.NumIn() - inOffset {
	case 0:
	case 1:
		reqType = ft.In(inOffset)
		for reqType.Kind() == reflect.Pointer {
			reqType = reqType.Elem()
		}
		if reqType.Kind() != reflect.Struct {
			return nil, fmt.Errorf("unsupported function %s for OpenAPI Path register, request parameter is not a struct", ft.String())
		}
	default:
		return nil, fmt.Errorf("unsupported function %s for OpenAPI Path register, too many input parameters", ft.String())
	}

	var (
		respType reflect.Type
		hasError bool
	)

	errorType := reflect.TypeOf((*error)(nil)).Elem()
	switch ft.NumOut() {
	case 0:
		respType = nil
	case 1:
		out0 := ft.Out(0)
		if out0.Implements(errorType) {
			hasError = true
			respType = nil
		} else {
			respType = out0
		}
	case 2:
		out0 := ft.Out(0)
		out1 := ft.Out(1)
		if !out1.Implements(errorType) {
			return nil, fmt.Errorf("unsupported function %s for OpenAPI Path register, second return value must be error", ft.String())
		}
		if out0.Implements(errorType) {
			return nil, fmt.Errorf("unsupported function %s for OpenAPI Path register, first return value must not be error", ft.String())
		}
		respType = out0
		hasError = true
	default:
		return nil, fmt.Errorf("unsupported function %s for OpenAPI Path register, too many return values", ft.String())
	}

	var reqStructType reflect.Type
	if reqType != nil {
		reqStructType = reqType
	}

	if respType != nil {
		for respType.Kind() == reflect.Pointer {
			respType = respType.Elem()
		}
	}

	return &parsedHandlerSignature{
		requestStructType: reqStructType,
		responseType:      respType,
		hasError:          hasError,
	}, nil
}

// isContextParamType reports whether the parameter is a web context type (e.g. *app.RequestContext).
func isContextParamType(t reflect.Type) bool {
	if t.Kind() == reflect.Interface && t.String() == "context.Context" {
		return true
	}
	if t.Kind() != reflect.Pointer {
		return false
	}
	elem := t.Elem()
	if elem.Name() == "RequestCtx" || elem.Name() == "Context" || elem.Name() == "RequestContext" {
		return true
	}
	return strings.Contains(elem.PkgPath(), "/app")
}

// outputSchemaRef converts a response Go type into an OpenAPI schema reference/value.
func (oai *OpenApiV3) outputSchemaRef(respType reflect.Type) SchemaRef {
	if respType == nil {
		return SchemaRef{Value: &Schema{Type: TypeObject}}
	}

	ref, err := oai.newSchemaRefWithGolangType(respType, reflect.StructTag(""))
	if err != nil || ref == nil {
		return SchemaRef{Value: &Schema{Type: TypeObject}}
	}
	return *ref
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
	BusinessSchema   SchemaRef
	RequestObject    any
	RequestDataField string
}

type getResponseSchemaRefInput struct {
	BusinessSchema    SchemaRef
	ResponseObject    any
	ResponseDataField string
}

func (oai *OpenApiV3) getRequestSchemaRef(in getRequestSchemaRefInput) (*SchemaRef, error) {
	if in.RequestObject == nil {
		bs := in.BusinessSchema
		return &bs, nil
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
			bs := in.BusinessSchema
			fieldSchemaRef = &bs
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
		bs := in.BusinessSchema
		return &bs, nil
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
			bs := in.BusinessSchema
			fieldSchemaRef = &bs
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

// getResponseFromOutput builds a success response schema (optionally wrapped by CommonResponse).
func (oai *OpenApiV3) getResponseFromOutput(businessSchema SchemaRef, outputType reflect.Type) (*Response, error) {
	response := &Response{
		Description: "Success",
		Content:     make(map[string]MediaType),
	}

	var schemaRef *SchemaRef
	var err error

	if oai.Config.CommonResponse != nil && oai.Config.CommonResponseDataField != "" {
		schemaRef, err = oai.getResponseSchemaRef(getResponseSchemaRefInput{
			BusinessSchema:    businessSchema,
			ResponseObject:    oai.Config.CommonResponse,
			ResponseDataField: oai.Config.CommonResponseDataField,
		})
	} else {
		if outputType == nil {
			schemaRef = &SchemaRef{Value: &Schema{Type: TypeObject}}
		} else {
			schemaRef, err = oai.newSchemaRefWithGolangType(outputType, reflect.StructTag(""))
		}
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

func (oai *OpenApiV3) ensureUniqueOperationID(base string) string {
	if oai.operationIDCounter == nil {
		oai.operationIDCounter = make(map[string]int)
	}
	if base == "" {
		base = "operation"
	}
	count := oai.operationIDCounter[base]
	id := base
	if count > 0 {
		id = fmt.Sprintf("%s_%d", base, count)
	}
	oai.operationIDCounter[base] = count + 1
	return id
}

func sanitizeOperationID(raw string) string {
	if raw == "" {
		return raw
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range raw {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteRune('_')
			lastUnderscore = true
		}
	}
	result := strings.Trim(b.String(), "_")
	return result
}

func (oai *OpenApiV3) generateOperationID(fn any, operationID string) string {
	if fn == nil {
		return oai.ensureUniqueOperationID("")
	}
	if operationID != "" {
		return oai.ensureUniqueOperationID(operationID)
	}

	fnName := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	fnName = sanitizeOperationID(fnName)
	return oai.ensureUniqueOperationID(fnName)
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

		pathTag := field.Tag.Get("path")
		queryTag := field.Tag.Get("query")
		headerTag := field.Tag.Get("header")
		paramName := field.Name
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" {
			paramName = strings.Split(strings.Trim(jsonTag, ","), ",")[0]
		}
		if headerTag != "" {
			paramName = headerTag
		}
		if queryTag != "" {
			paramName = queryTag
		}
		if pathTag != "" {
			paramName = pathTag
		}

		parameter := Parameter{
			Name:        paramName,
			Description: field.Tag.Get("description"),
			Required:    strings.Contains(field.Tag.Get("verf"), "required"),
			Schema:      &SchemaRef{},
			XExtensions: make(XExtension),
		}

		if headerTag != "" {
			parameter.In = ParameterInHeader
		} else if pathTag != "" {
			parameter.In = ParameterInPath
			parameter.Required = true
		} else if queryTag != "" || field.Tag.Get("in") == "query" {
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
