package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type SearchRequest struct {
	// 每个字段都有四块组成: 字段规则，类型，名称，编号
	Query         string   `protobuf:"bytes,1,opt,name=query,proto3" json:"query,omitempty"`
	PageNumber    int32    `protobuf:"varint,2,opt,name=page_number,json=pageNumber,proto3" json:"page_number,omitempty"`
	ResultPerPage int32    `protobuf:"varint,3,opt,name=result_per_page,json=resultPerPage,proto3" json:"result_per_page,omitempty"`
	Hobby         []string `protobuf:"bytes,4,rep,name=hobby,proto3" json:"hobby,omitempty"`
}

func TestUnmarshal(t *testing.T) {
	req := &SearchRequest{}
	jsonStr := "{\"query\":\"query\",\"page_number\":1,\"result_per_page\":2,\"hobby\":[\"hobby1\",\"hobby2\"]}"
	_ = Unmarshal([]byte(jsonStr), req)
	assert.Equal(t, req.Query, "query")
	assert.EqualValues(t, req.PageNumber, 1)
	assert.EqualValues(t, req.ResultPerPage, 2)
	assert.Equal(t, req.Hobby, []string{"hobby1", "hobby2"})
}

func TestMarshal(t *testing.T) {
	req := &SearchRequest{
		Query:         "query",
		PageNumber:    1,
		ResultPerPage: 2,
		Hobby:         []string{"hobby1", "hobby2"},
	}
	jsonStr := "{\"query\":\"query\",\"page_number\":1,\"result_per_page\":2,\"hobby\":[\"hobby1\",\"hobby2\"]}"
	marshal, _ := Marshal(req)
	assert.Equal(t, jsonStr, string(marshal))
}
