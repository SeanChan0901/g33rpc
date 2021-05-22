package serializer

import "io"

type Header struct {
	ServiceMethod string
	Seq uint64
	Error string
}

type Serializer interface {
	io.Closer
	ReadHeader(*Header) error
	ReadBody(interface{}) error
	Write(*Header, interface{}) error
}

type NewSerializer func(io.ReadWriteCloser) Serializer

type Type string

const (
	GobType Type = "application/gob"
	JsonType Type = "application/json"
)

var NewSerializerMap map[Type]NewSerializer

func init() {
	NewSerializerMap = make(map[Type]NewSerializer)
	NewSerializerMap[GobType] = NewGobSerializer
}