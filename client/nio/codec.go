package nio

import (
	"bytes"
	"encoding/binary"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
)

type ICodec interface {
	// Encode 编码，将数据编码为字节
	Encode(msg *Msg) *bytes.Buffer

	// Decode 解码，将数据解码为需要的结构体，返回true代表解码成功，false代表发生断包解码工作还在进行中
	Decode(msg *Msg, reader *bytes.Buffer) bool
}

type Msg struct {

	// 消息类型：占用1字节，最大值255
	flag uint8

	// 消息总长度：占用4字节，最大值4294967295
	length uint32

	// 消息体
	body interface{}

	// body对应的[]byte
	bytes []byte
}

type gzipCodec struct {
	logger logger.ILog
}

func GetZipCodec(logger logger.ILog) *gzipCodec {
	return &gzipCodec{logger: logger}
}

func (gc *gzipCodec) Encode(msg *Msg) *bytes.Buffer {
	buf := new(bytes.Buffer)

	var gzipBytes []byte
	var err error

	if msg.body == nil {
		msg.length = 1
	} else {
		msg.bytes, err = tools.Marshal(msg.body)
		if err != nil {
			gc.logger.Error("encode msg error: %s", err.Error())
		}
		gzipBytes, err = tools.Gzip(msg.bytes)
		if err != nil {
			gc.logger.Error("encode msg error: %s", err.Error())
		}
		msg.length = uint32(len(gzipBytes) + 1)
	}

	if err = binary.Write(buf, binary.BigEndian, msg.length); err != nil {
		gc.logger.Error("encode msg error: %s", err.Error())
	}

	if err = binary.Write(buf, binary.BigEndian, msg.flag); err != nil {
		gc.logger.Error("encode msg error: %s", err.Error())
	}
	if gzipBytes != nil && len(gzipBytes) != 0 {
		buf.Write(gzipBytes)
	}

	return buf
}

func (gc *gzipCodec) Decode(msg *Msg, reader *bytes.Buffer) bool {
	if reader == nil || msg == nil {
		gc.logger.Warn("decode msg failed. msg or reader is nil. ")
		return false
	}

	// 如果流中数据足够
	if reader.Len() >= 5 {

		// 读取4个字节
		var length uint32
		binary.Read(reader, binary.BigEndian, &length)
		msg.length = length

		// 读取1个字节
		var flag uint8
		binary.Read(reader, binary.BigEndian, &flag)
		msg.flag = flag

		// 流中数据不够，暂时返回
	} else {
		return false
	}

	// 说明这条消息，只有消息头，可以返回了
	if msg.length == 1 {
		return true
	}

	var err error
	if msg.body == nil && reader.Len() == int(msg.length-1) {
		gizBytes := make([]byte, msg.length-1)
		_, err = reader.Read(gizBytes)
		if err != nil {
			gc.logger.Error("decode msg error: %s", err.Error())
		}

		msg.bytes, err = tools.Gunzip(gizBytes)
		if err != nil {
			gc.logger.Error("decode msg error: %s", err.Error())
		}
		return true
	} else {
		return false
	}

}
