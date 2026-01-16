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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/caiflower/common-tools/web/app"
	"github.com/caiflower/common-tools/web/common/e"
	"github.com/caiflower/common-tools/web/common/resp"
	"github.com/stretchr/testify/assert"
)

// TestUser is a test struct for testing schema generation
type TestUser struct {
	ID     int    `json:"id" description:"User ID"`
	Name   string `json:"name" description:"User name" verf:"required|len:2,100"`
	Email  string `json:"email" description:"User email" verf:"required|reg:^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$"`
	Age    int    `json:"age" description:"User age" verf:"between:0,120"`
	Active bool   `json:"active" description:"Is user active"`
	Role   string `json:"role" description:"User role" verf:"inList:admin,user,guest"`
}

// TestCreateUserRequest is a test request struct
type TestCreateUserRequest struct {
	Name     string `json:"name" description:"User name" verf:"required"`
	Email    string `json:"email" description:"User email" verf:"required"`
	Password string `json:"password" description:"User password" verf:"required|len:6,20"`
}

// TestCreateUserResponse is a test response struct
type TestCreateUserResponse struct {
	User  TestUser `json:"user" description:"Created user"`
	Token string   `json:"token" description:"Authentication token"`
}

func TestNewOpenApiV3(t *testing.T) {
	oai := New()
	assert.NotNil(t, oai)
	assert.NotNil(t, oai.Config)
	assert.NotNil(t, oai.Paths)
	assert.NotNil(t, oai.Components)
}

func TestAddSchema(t *testing.T) {
	oai := New()
	user := TestUser{}

	err := oai.Add(AddInput{Object: user})
	assert.NoError(t, err)

	// Check if schema was added
	schemaRef := oai.Components.Schemas.Get("github.com.caiflower.common-tools.web.common.goai.TestUser")
	assert.NotNil(t, schemaRef)
	assert.NotNil(t, schemaRef.Value)
	assert.Equal(t, TypeObject, schemaRef.Value.Type)
	assert.NotNil(t, schemaRef.Value.Properties)
}

func TestStructToSchema(t *testing.T) {
	oai := New()
	user := TestUser{}

	schema, err := oai.structToSchema(user)
	assert.NoError(t, err)
	assert.NotNil(t, schema)
	assert.Equal(t, TypeObject, schema.Type)
	assert.NotNil(t, schema.Properties)

	// Debug: print all properties
	fmt.Printf("schema.Properties: %v\n", schema.Properties)
	fmt.Printf("schema.Properties.Map(): %v\n", schema.Properties.Map())

	// Check properties
	props := schema.Properties.Map()
	assert.Len(t, props, 6)

	// Check Name property
	nameProp := props["name"]
	assert.NotNil(t, nameProp.Value)
	assert.Equal(t, TypeString, nameProp.Value.Type)
	assert.Equal(t, "User name", nameProp.Value.Description)
	assert.Equal(t, "required|len:2,100", nameProp.Value.ValidationRules)
}

func TestNewSchemaRefWithGolangType(t *testing.T) {
	oai := New()

	// Test with int
	schemaRef, err := oai.newSchemaRefWithGolangType(reflect.TypeOf(0), reflect.StructTag(""))
	assert.NoError(t, err)
	assert.NotNil(t, schemaRef)
	assert.Equal(t, TypeInteger, schemaRef.Value.Type)

	// Test with string
	schemaRef, err = oai.newSchemaRefWithGolangType(reflect.TypeOf(""), reflect.StructTag(""))
	assert.NoError(t, err)
	assert.NotNil(t, schemaRef)
	assert.Equal(t, TypeString, schemaRef.Value.Type)

	// Test with struct
	schemaRef, err = oai.newSchemaRefWithGolangType(reflect.TypeOf(TestUser{}), reflect.StructTag(""))
	assert.NoError(t, err)
	assert.NotNil(t, schemaRef)
	assert.Equal(t, TypeObject, schemaRef.Value.Type)
}

func TestGolangTypeToSchemaName(t *testing.T) {
	oai := New()

	// Test with basic types
	name := oai.golangTypeToSchemaName(reflect.TypeOf(0))
	assert.Equal(t, "int", name)

	name = oai.golangTypeToSchemaName(reflect.TypeOf(""))
	assert.Equal(t, "string", name)

	// Test with struct
	name = oai.golangTypeToSchemaName(reflect.TypeOf(TestUser{}))
	assert.Equal(t, "github.com.caiflower.common-tools.web.common.goai.TestUser", name)
}

func TestSchemaClone(t *testing.T) {
	schema := &Schema{
		Type:        TypeString,
		Title:       "Test Schema",
		Description: "Test description",
		Enum:        []any{"value1", "value2", "value3"},
	}

	clone := schema.Clone()
	assert.NotNil(t, clone)
	assert.Equal(t, schema.Type, clone.Type)
	assert.Equal(t, schema.Title, clone.Title)
	assert.Equal(t, schema.Description, clone.Description)
	assert.Equal(t, schema.Enum, clone.Enum)

	// Modify clone to ensure it's a deep copy
	clone.Title = "Modified Title"
	assert.NotEqual(t, schema.Title, clone.Title)
}

func TestAddAPI(t *testing.T) {
	oai := New()

	// Test function
	funcToWrap := func(req TestCreateUserRequest) (TestCreateUserResponse, error) {
		return TestCreateUserResponse{}, nil
	}

	err := oai.Add(AddInput{
		Method: "POST",
		Path:   "/users",
		Object: funcToWrap,
	})
	assert.NoError(t, err)
	fmt.Println(oai.String())
	assert.NotEmpty(t, oai.String())

	// Check if path was added
	path := oai.Paths["/users"]
	assert.NotNil(t, path)
	assert.NotNil(t, path.Post)
	assert.Equal(t, "github.com.caiflower.common-tools.web.common.goai.TestCreateUserRequest", path.Post.Summary)
	assert.Equal(t, "API endpoint for github.com.caiflower.common-tools.web.common.goai.TestCreateUserRequest", path.Post.Description)
}

func TestOpenApiV3_Add(t *testing.T) {
	type CommonReq struct {
		AppId      int64  `verf:"required" path:"appId" description:"应用Id"`
		ResourceId string `json:"resourceId" query:"resourceId" description:"资源Id"`
	}
	type SetSpecInfo struct {
		StorageType string   `verf:"required|inList:CLOUD_PREMIUM,CLOUD_SSD,CLOUD_HSSD" description:"StorageType"`
		Shards      int32    `description:"shards 分片数"`
		Params      []string `description:"默认参数(json 串-ClickHouseParams)"`
	}
	type CreateResourceReq struct {
		CommonReq
		Name     string                  `description:"实例名称"`
		Product  string                  `description:"业务类型"`
		Region   string                  `verf:"required" description:"区域"`
		SetMap   map[string]*SetSpecInfo `verf:"required" description:"配置Map"`
		SetSlice []SetSpecInfo           `verf:"required" description:"配置Slice"`
	}

	type CreateResourceRes struct {
		FlowId int64 `description:"创建实例流程id"`
	}

	f := func(ctx *app.RequestContext, req *CreateResourceReq) (res *CreateResourceRes, err error) {
		return
	}

	var (
		err error
		oai = New()
	)

	err = oai.Add(AddInput{
		Path:   "/test1/{appId}",
		Method: http.MethodPut,
		Object: f,
	})
	assert.Nil(t, err)

	err = oai.Add(AddInput{
		Path:   "/test1/{appId}",
		Method: http.MethodPost,
		Object: f,
	})
	assert.Nil(t, err)

	err = oai.Add(AddInput{
		Path:   "/test2/{appId}",
		Method: http.MethodGet,
		Object: f,
	})
	assert.Nil(t, err)

	jsonStr := oai.String()
	assert.NotEmpty(t, jsonStr)

	fmt.Println(oai.String())

	assert.Len(t, oai.Paths, 2, "Should have 2 paths")

	test1Path := oai.Paths["/test1/{appId}"]
	assert.NotNil(t, test1Path, "Path /test1/{appId} should exist")

	assert.NotNil(t, test1Path.Put, "PUT method should exist for /test1/{appId}")
	assert.Equal(t, "github.com.caiflower.common-tools.web.common.goai.CreateResourceReq", test1Path.Put.Summary)
	assert.Equal(t, "API endpoint for github.com.caiflower.common-tools.web.common.goai.CreateResourceReq", test1Path.Put.Description)

	putParams := test1Path.Put.Parameters
	assert.Len(t, putParams, 2, "PUT should have 2 parameters")
	assert.Equal(t, "appId", putParams[0].Value.Name)
	assert.Equal(t, ParameterInPath, putParams[0].Value.In)
	assert.Equal(t, "应用Id", putParams[0].Value.Description)
	assert.True(t, putParams[0].Value.Required)
	assert.Equal(t, "resourceId", putParams[1].Value.Name)
	assert.Equal(t, ParameterInQuery, putParams[1].Value.In)
	assert.Equal(t, "资源Id", putParams[1].Value.Description)

	putRequestBody := test1Path.Put.RequestBody
	assert.NotNil(t, putRequestBody, "PUT should have request body")
	putContent := putRequestBody.Value.Content["application/json"]
	assert.NotNil(t, putContent)
	putSchema := putContent.Schema.Value
	assert.NotNil(t, putSchema)
	assert.Equal(t, TypeObject, putSchema.Type)
	putProperties := putSchema.Properties.Map()
	assert.Contains(t, putProperties, "CommonReq")
	assert.Contains(t, putProperties, "Name")
	assert.Contains(t, putProperties, "Region")
	assert.Contains(t, putProperties, "SetMap")
	assert.Contains(t, putProperties, "SetSlice")

	putResponse := test1Path.Put.Responses["200"]
	assert.NotNil(t, putResponse, "PUT should have 200 response")
	putResponseContent := putResponse.Value.Content["application/json"]
	assert.NotNil(t, putResponseContent)
	putResponseSchema := putResponseContent.Schema.Value
	assert.NotNil(t, putResponseSchema)
	assert.Equal(t, TypeObject, putResponseSchema.Type)
	putResponseProperties := putResponseSchema.Properties.Map()
	assert.Contains(t, putResponseProperties, "FlowId")

	assert.NotNil(t, test1Path.Post, "POST method should exist for /test1/{appId}")
	assert.Equal(t, "github.com.caiflower.common-tools.web.common.goai.CreateResourceReq", test1Path.Post.Summary)
	assert.Equal(t, "API endpoint for github.com.caiflower.common-tools.web.common.goai.CreateResourceReq", test1Path.Post.Description)

	postParams := test1Path.Post.Parameters
	assert.Len(t, postParams, 2, "POST should have 2 parameters")
	assert.Equal(t, "appId", postParams[0].Value.Name)
	assert.Equal(t, ParameterInPath, postParams[0].Value.In)
	assert.Equal(t, "resourceId", postParams[1].Value.Name)
	assert.Equal(t, ParameterInQuery, postParams[1].Value.In)

	postRequestBody := test1Path.Post.RequestBody
	assert.NotNil(t, postRequestBody, "POST should have request body")
	postContent := postRequestBody.Value.Content["application/json"]
	assert.NotNil(t, postContent)
	postExample := postContent.Example
	assert.NotNil(t, postExample)
	postExampleMap, ok := postExample.(map[string]any)
	assert.True(t, ok, "Example should be a map")
	assert.Contains(t, postExampleMap, "CommonReq")
	assert.Contains(t, postExampleMap, "Name")
	assert.Contains(t, postExampleMap, "Region")
	assert.Contains(t, postExampleMap, "SetMap")
	assert.Contains(t, postExampleMap, "SetSlice")

	postResponse := test1Path.Post.Responses["200"]
	assert.NotNil(t, postResponse, "POST should have 200 response")
	postResponseContent := postResponse.Value.Content["application/json"]
	assert.NotNil(t, postResponseContent)
	postResponseSchema := postResponseContent.Schema.Value
	assert.NotNil(t, postResponseSchema)
	assert.Equal(t, TypeObject, postResponseSchema.Type)
	postResponseProperties := postResponseSchema.Properties.Map()
	assert.Contains(t, postResponseProperties, "FlowId")

	test2Path := oai.Paths["/test2/{appId}"]
	assert.NotNil(t, test2Path, "Path /test2/{appId} should exist")

	assert.NotNil(t, test2Path.Get, "GET method should exist for /test2/{appId}")
	assert.Equal(t, "github.com.caiflower.common-tools.web.common.goai.CreateResourceReq", test2Path.Get.Summary)
	assert.Equal(t, "API endpoint for github.com.caiflower.common-tools.web.common.goai.CreateResourceReq", test2Path.Get.Description)

	getParams := test2Path.Get.Parameters
	assert.Len(t, getParams, 2, "GET should have 2 parameters")
	assert.Equal(t, "appId", getParams[0].Value.Name)
	assert.Equal(t, ParameterInPath, getParams[0].Value.In)
	assert.Equal(t, "resourceId", getParams[1].Value.Name)
	assert.Equal(t, ParameterInQuery, getParams[1].Value.In)

	getRequestBody := test2Path.Get.RequestBody
	assert.Nil(t, getRequestBody, "GET should not have request body")

	getResponse := test2Path.Get.Responses["200"]
	assert.NotNil(t, getResponse, "GET should have 200 response")
	getResponseContent := getResponse.Value.Content["application/json"]
	assert.NotNil(t, getResponseContent)
	getResponseSchema := getResponseContent.Schema.Value
	assert.NotNil(t, getResponseSchema)
	assert.Equal(t, TypeObject, getResponseSchema.Type)
	getResponseProperties := getResponseSchema.Properties.Map()
	assert.Contains(t, getResponseProperties, "FlowId")

	schemas := oai.Components.Schemas.Map()
	assert.Contains(t, schemas, "github.com.caiflower.common-tools.web.common.goai.CreateResourceReq")
	assert.Contains(t, schemas, "github.com.caiflower.common-tools.web.common.goai.CreateResourceRes")
	assert.Contains(t, schemas, "github.com.caiflower.common-tools.web.common.goai.SetSpecInfo")
	assert.Contains(t, schemas, "github.com.caiflower.common-tools.web.common.goai.CommonReq")

	createResourceReqSchema := schemas["github.com.caiflower.common-tools.web.common.goai.CreateResourceReq"].Value
	assert.NotNil(t, createResourceReqSchema)
	assert.Equal(t, TypeObject, createResourceReqSchema.Type)
	reqProperties := createResourceReqSchema.Properties.Map()
	assert.Contains(t, reqProperties, "CommonReq")
	assert.Contains(t, reqProperties, "Name")
	assert.Contains(t, reqProperties, "Region")
	assert.Contains(t, reqProperties, "SetMap")
	assert.Contains(t, reqProperties, "SetSlice")

	setSpecInfoSchema := schemas["github.com.caiflower.common-tools.web.common.goai.SetSpecInfo"].Value
	assert.NotNil(t, setSpecInfoSchema)
	assert.Equal(t, TypeObject, setSpecInfoSchema.Type)
	setSpecProperties := setSpecInfoSchema.Properties.Map()
	assert.Contains(t, setSpecProperties, "StorageType")
	assert.Contains(t, setSpecProperties, "Shards")
	assert.Contains(t, setSpecProperties, "Params")

	storageTypeEnum := setSpecProperties["StorageType"]
	assert.NotNil(t, storageTypeEnum)
	storageTypeValue := storageTypeEnum.Value
	assert.NotNil(t, storageTypeValue)
	assert.Contains(t, storageTypeValue.Enum, "CLOUD_PREMIUM")
	assert.Contains(t, storageTypeValue.Enum, "CLOUD_SSD")
	assert.Contains(t, storageTypeValue.Enum, "CLOUD_HSSD")

	assert.Contains(t, setSpecInfoSchema.Required, "StorageType")
}

func TestCommonResponse(t *testing.T) {
	oai := New()

	oai.Config.CommonResponse = &resp.Result{}
	oai.Config.CommonResponseDataField = "Data"

	type CreateUserRequest struct {
		Name  string `json:"name" description:"User name" verf:"required"`
		Email string `json:"email" description:"User email" verf:"required"`
	}

	type CreateUserResponse struct {
		User  TestUser `json:"user" description:"Created user"`
		Token string   `json:"token" description:"Authentication token"`
	}

	f := func(req CreateUserRequest) (CreateUserResponse, error) {
		return CreateUserResponse{}, nil
	}

	err := oai.Add(AddInput{
		Path:   "/users",
		Method: http.MethodPost,
		Object: f,
	})
	assert.Nil(t, err)

	jsonStr := oai.String()
	assert.NotEmpty(t, jsonStr)

	fmt.Println("OpenAPI JSON with CommonResponse:")
	fmt.Println(jsonStr)

	path := oai.Paths["/users"]
	assert.NotNil(t, path)
	assert.NotNil(t, path.Post)

	response := path.Post.Responses["200"]
	assert.NotNil(t, response)
	assert.NotNil(t, response.Value)

	content := response.Value.Content["application/json"]
	assert.NotNil(t, content)
	assert.NotNil(t, content.Schema)

	schema := content.Schema.Value
	assert.NotNil(t, schema)
	assert.Equal(t, TypeObject, schema.Type)

	properties := schema.Properties.Map()
	assert.Contains(t, properties, "Data")
	assert.Contains(t, properties, "RequestId")

	dataRef := properties["Data"]
	assert.NotNil(t, dataRef)
	assert.Equal(t, "github.com.caiflower.common-tools.web.common.goai.CreateUserResponse", dataRef.Ref)

	dataSchema := oai.Components.Schemas.Get(dataRef.Ref)
	assert.NotNil(t, dataSchema)
	assert.NotNil(t, dataSchema.Value)
	assert.Equal(t, TypeObject, dataSchema.Value.Type)

	dataProperties := dataSchema.Value.Properties.Map()
	assert.Contains(t, dataProperties, "user")
	assert.Contains(t, dataProperties, "token")
}

func TestCommonRequest(t *testing.T) {
	oai := New()

	oai.Config.CommonRequest = &resp.Result{}
	oai.Config.CommonRequestDataField = "Data"

	type CreateUserRequest struct {
		Name  string `json:"name" description:"User name" verf:"required"`
		Email string `json:"email" description:"User email" verf:"required"`
	}

	type CreateUserResponse struct {
		User  TestUser `json:"user" description:"Created user"`
		Token string   `json:"token" description:"Authentication token"`
	}

	f := func(req CreateUserRequest) (CreateUserResponse, error) {
		return CreateUserResponse{}, nil
	}

	err := oai.Add(AddInput{
		Path:   "/users",
		Method: http.MethodPost,
		Object: f,
	})
	assert.Nil(t, err)

	jsonStr := oai.String()
	assert.NotEmpty(t, jsonStr)

	fmt.Println("OpenAPI JSON with CommonRequest:")
	fmt.Println(jsonStr)

	path := oai.Paths["/users"]
	assert.NotNil(t, path)
	assert.NotNil(t, path.Post)

	requestBody := path.Post.RequestBody
	assert.NotNil(t, requestBody)
	assert.NotNil(t, requestBody.Value)

	content := requestBody.Value.Content["application/json"]
	assert.NotNil(t, content)
	assert.NotNil(t, content.Schema)

	schema := content.Schema.Value
	assert.NotNil(t, schema)
	assert.Equal(t, TypeObject, schema.Type)

	properties := schema.Properties.Map()
	assert.Contains(t, properties, "Data")
	assert.Contains(t, properties, "RequestId")

	dataRef := properties["Data"]
	assert.NotNil(t, dataRef)
	assert.Equal(t, "github.com.caiflower.common-tools.web.common.goai.CreateUserRequest", dataRef.Ref)

	dataSchema := oai.Components.Schemas.Get(dataRef.Ref)
	assert.NotNil(t, dataSchema)
	assert.NotNil(t, dataSchema.Value)
	assert.Equal(t, TypeObject, dataSchema.Value.Type)

	dataProperties := dataSchema.Value.Properties.Map()
	assert.Contains(t, dataProperties, "name")
	assert.Contains(t, dataProperties, "email")
}

func TestCommonRequestAndResponse(t *testing.T) {
	oai := New()

	oai.Config.CommonRequest = &resp.Result{}
	oai.Config.CommonRequestDataField = "Data"
	oai.Config.CommonResponse = &resp.Result{}
	oai.Config.CommonResponseDataField = "Data"

	type CreateUserRequest struct {
		Name  string `json:"name" description:"User name" verf:"required"`
		Email string `json:"email" description:"User email" verf:"required"`
	}

	type CreateUserResponse struct {
		User  TestUser `json:"user" description:"Created user"`
		Token string   `json:"token" description:"Authentication token"`
	}

	f := func(req CreateUserRequest) (CreateUserResponse, error) {
		return CreateUserResponse{}, nil
	}

	err := oai.Add(AddInput{
		Path:   "/users",
		Method: http.MethodPost,
		Object: f,
	})
	assert.Nil(t, err)

	jsonStr := oai.String()
	assert.NotEmpty(t, jsonStr)

	fmt.Println("OpenAPI JSON with CommonRequest and CommonResponse:")
	fmt.Println(jsonStr)

	path := oai.Paths["/users"]
	assert.NotNil(t, path)
	assert.NotNil(t, path.Post)

	requestBody := path.Post.RequestBody
	assert.NotNil(t, requestBody)
	assert.NotNil(t, requestBody.Value)

	requestContent := requestBody.Value.Content["application/json"]
	assert.NotNil(t, requestContent)
	assert.NotNil(t, requestContent.Schema)

	requestSchema := requestContent.Schema.Value
	assert.NotNil(t, requestSchema)
	assert.Equal(t, TypeObject, requestSchema.Type)

	requestProperties := requestSchema.Properties.Map()
	assert.Contains(t, requestProperties, "Data")
	assert.Contains(t, requestProperties, "RequestId")

	response := path.Post.Responses["200"]
	assert.NotNil(t, response)
	assert.NotNil(t, response.Value)

	responseContent := response.Value.Content["application/json"]
	assert.NotNil(t, responseContent)
	assert.NotNil(t, responseContent.Schema)

	responseSchema := responseContent.Schema.Value
	assert.NotNil(t, responseSchema)
	assert.Equal(t, TypeObject, responseSchema.Type)

	responseProperties := responseSchema.Properties.Map()
	assert.Contains(t, responseProperties, "Data")
	assert.Contains(t, responseProperties, "RequestId")

	requestDataRef := requestProperties["Data"]
	assert.NotNil(t, requestDataRef)
	assert.Equal(t, "github.com.caiflower.common-tools.web.common.goai.CreateUserRequest", requestDataRef.Ref)

	responseDataRef := responseProperties["Data"]
	assert.NotNil(t, responseDataRef)
	assert.Equal(t, "github.com.caiflower.common-tools.web.common.goai.CreateUserResponse", responseDataRef.Ref)
}

func TestOpenApiV3_Add_FunctionSignatureVariations(t *testing.T) {
	oai := New()
	oai.Config.CommonResponse = &resp.Result{}
	oai.Config.CommonResponseDataField = "Data"

	type CreateUserRequest struct {
		Name string `json:"name" description:"name" verf:"required"`
	}

	type CreateUserResponse struct {
		ID int `json:"id" description:"id"`
	}

	type CreateResourceReq struct {
		Name string `json:"name"`
	}

	type CreateResourceRes struct {
		FlowId int64 `json:"flowId"`
	}

	assertWrapped := func(op *Operation) map[string]SchemaRef {
		t.Helper()

		assert.NotNil(t, op)
		resp200 := op.Responses["200"]
		assert.NotNil(t, resp200.Value)
		content := resp200.Value.Content["application/json"]
		assert.NotNil(t, content.Schema)
		assert.NotNil(t, content.Schema.Value)
		props := content.Schema.Value.Properties.Map()
		assert.Contains(t, props, "Data")
		return props
	}

	t.Run("func(req CreateUserRequest)", func(t *testing.T) {
		f := func(req CreateUserRequest) {}
		err := oai.Add(AddInput{Path: "/sig/1", Method: http.MethodPost, Object: f})
		assert.NoError(t, err)

		props := assertWrapped(oai.Paths["/sig/1"].Post)
		dataRef := props["Data"]
		assert.Equal(t, "", dataRef.Ref)
		assert.NotNil(t, dataRef.Value)
		assert.Equal(t, TypeObject, dataRef.Value.Type)
	})

	t.Run("func(req CreateUserRequest) (CreateUserResponse, error)", func(t *testing.T) {
		f := func(req CreateUserRequest) (CreateUserResponse, error) { return CreateUserResponse{}, nil }
		err := oai.Add(AddInput{Path: "/sig/2", Method: http.MethodPost, Object: f})
		assert.NoError(t, err)

		props := assertWrapped(oai.Paths["/sig/2"].Post)
		dataRef := props["Data"]
		assert.Equal(t, "github.com.caiflower.common-tools.web.common.goai.CreateUserResponse", dataRef.Ref)
		assert.NotNil(t, oai.Components.Schemas.Get(dataRef.Ref))
		assert.NotNil(t, oai.Paths["/sig/2"].Post.Responses["400"].Value)
		assert.NotNil(t, oai.Paths["/sig/2"].Post.Responses["500"].Value)
	})

	t.Run("func(ctx, req) (res, err)", func(t *testing.T) {
		f := func(ctx *app.RequestContext, req *CreateResourceReq) (res *CreateResourceRes, err error) {
			return nil, nil
		}
		err := oai.Add(AddInput{Path: "/sig/3", Method: http.MethodPost, Object: f})
		assert.NoError(t, err)

		props := assertWrapped(oai.Paths["/sig/3"].Post)
		dataRef := props["Data"]
		assert.Equal(t, "github.com.caiflower.common-tools.web.common.goai.CreateResourceRes", dataRef.Ref)
		assert.NotNil(t, oai.Paths["/sig/3"].Post.Responses["400"].Value)
	})

	t.Run("SayHelloWorld() string", func(t *testing.T) {
		f := func() string { return "ok" }
		err := oai.Add(AddInput{Path: "/sig/4", Method: http.MethodGet, Object: f})
		assert.NoError(t, err)

		props := assertWrapped(oai.Paths["/sig/4"].Get)
		dataRef := props["Data"]
		assert.Equal(t, "", dataRef.Ref)
		assert.NotNil(t, dataRef.Value)
		assert.Equal(t, TypeString, dataRef.Value.Type)
		_, has400 := oai.Paths["/sig/4"].Get.Responses["400"]
		assert.False(t, has400)
	})

	t.Run("ReturnError() e.ApiError", func(t *testing.T) {
		f := func() e.ApiError { return nil }
		err := oai.Add(AddInput{Path: "/sig/5", Method: http.MethodGet, Object: f})
		assert.NoError(t, err)

		props := assertWrapped(oai.Paths["/sig/5"].Get)
		dataRef := props["Data"]
		assert.Equal(t, "", dataRef.Ref)
		assert.NotNil(t, dataRef.Value)
		assert.Equal(t, TypeObject, dataRef.Value.Type)
		assert.NotNil(t, oai.Paths["/sig/5"].Get.Responses["400"].Value)
		assert.NotNil(t, oai.Paths["/sig/5"].Get.Responses["500"].Value)
	})

	t.Run("func(context.Context, req) (res, err)", func(t *testing.T) {
		f := func(ctx context.Context, req *CreateResourceReq) (res *CreateResourceRes, err error) {
			return nil, nil
		}
		err := oai.Add(AddInput{Path: "/sig/6", Method: http.MethodPost, Object: f})
		assert.NoError(t, err)

		props := assertWrapped(oai.Paths["/sig/6"].Post)
		dataRef := props["Data"]
		assert.Equal(t, "github.com.caiflower.common-tools.web.common.goai.CreateResourceRes", dataRef.Ref)
		assert.NotNil(t, oai.Paths["/sig/6"].Post.Responses["400"].Value)
	})

	// RepeatRequest is a request model used by signature-variation tests.
	type RepeatRequest struct {
		Value string `json:"value"`
	}

	t.Run("Repeat(req *base.RepeatRequest) string", func(t *testing.T) {
		f := func(req *RepeatRequest) string { return req.Value }
		err := oai.Add(AddInput{Path: "/sig/6", Method: http.MethodPost, Object: f})
		assert.NoError(t, err)

		props := assertWrapped(oai.Paths["/sig/6"].Post)
		dataRef := props["Data"]
		assert.Equal(t, "", dataRef.Ref)
		assert.NotNil(t, dataRef.Value)
		assert.Equal(t, TypeString, dataRef.Value.Type)
	})

	t.Run("Reject primitive input", func(t *testing.T) {
		f := func(x int) (string, error) { return "", nil }
		err := oai.Add(AddInput{Path: "/sig/bad1", Method: http.MethodPost, Object: f})
		assert.Error(t, err)
	})

	t.Run("Reject bad return signature", func(t *testing.T) {
		f := func(req CreateUserRequest) (string, int) { return "", 0 }
		err := oai.Add(AddInput{Path: "/sig/bad2", Method: http.MethodPost, Object: f})
		assert.Error(t, err)
	})

	t.Run("Reject nil object", func(t *testing.T) {
		err := oai.Add(AddInput{Path: "/sig/bad3", Method: http.MethodPost, Object: nil})
		assert.Error(t, err)
	})

	jsonStr := oai.String()
	assert.NotEmpty(t, jsonStr)
	var m map[string]any
	assert.NoError(t, json.Unmarshal([]byte(jsonStr), &m))
}
