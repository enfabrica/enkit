package protocfg

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/google/go-jsonnet"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

type Config[T any, PT interface {
	*T
	proto.Message
}] struct {
	Reader MessageReader
	Parser MessageParser[T, PT]

	err error
}

func FromFile[T any, PT interface {
	*T
	proto.Message
}](filename string) *Config[T, PT] {
	ext := filepath.Ext(filename)
	switch ext {
	case ".jsonnet":
		return &Config[T, PT]{
			Reader: NewJsonnetFileReader(filename),
			Parser: Json[T, PT](),
		}
	case ".json":
		return &Config[T, PT]{
			Reader: NewFileReader(filename),
			Parser: Json[T, PT](),
		}
	case ".textproto", ".prototext":
		return &Config[T, PT]{
			Reader: NewFileReader(filename),
			Parser: TextProto[T, PT](),
		}
	case ".pb":
		return &Config[T, PT]{
			Reader: NewFileReader(filename),
			Parser: BinaryProto[T, PT](),
		}
	default:
		return &Config[T, PT]{
			err: fmt.Errorf("cannot infer parser from file extension %q", ext),
		}
	}
}

func (c *Config[T, PT]) Load() (*T, error) {
	if c.err != nil {
		return nil, c.err
	}

	msg, err := c.Reader.ReadMessage()
	if err != nil {
		return nil, err
	}
	parsed, err := c.Parser.Parse(msg)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func (c *Config[T, PT]) LoadOnSignals(sigs ...os.Signal) (<-chan *T, error) {
	configChan := make(chan *T)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, sigs...)
	readAndSend := func() {
		cfg, err := c.Load()
		if err != nil {
			// TODO: log
		} else {
			configChan <- cfg
		}
	}
	go func() {
		readAndSend()
		for {
			_ = <-sigChan
			readAndSend()
		}
	}()
	return configChan, nil
}

type MessageReader interface {
	ReadMessage() ([]byte, error)
}

type FileReader struct {
	Filename string
}

func NewFileReader(filename string) MessageReader {
	return &FileReader{Filename: filename}
}

func (r *FileReader) ReadMessage() ([]byte, error) {
	return os.ReadFile(r.Filename)
}

type JsonnetFileReader struct {
	Filename string
}

func NewJsonnetFileReader(filename string) MessageReader {
	return &JsonnetFileReader{Filename: filename}
}

func (r *JsonnetFileReader) ReadMessage() ([]byte, error) {
	vm := jsonnet.MakeVM()
	jsonStr, err := vm.EvaluateFile(r.Filename)
	if err != nil {
		return nil, err
	}
	return []byte(jsonStr), nil
}

type IOReader struct {
	Reader io.ReadCloser
}

func (r *IOReader) ReadMessage() ([]byte, error) {
	defer r.Reader.Close()
	return io.ReadAll(r.Reader)
}

type MessageParser[T any, PT interface {
	*T
	proto.Message
}] interface {
	Parse([]byte) (*T, error)
}

type unmarshalFuncParser[T any, PT interface {
	*T
	proto.Message
}] struct {
	unmarshalFunc func([]byte, proto.Message) error
}

func (p *unmarshalFuncParser[T, PT]) Parse(contents []byte) (*T, error) {
	msg := PT(new(T))
	if err := p.unmarshalFunc(contents, msg); err != nil {
		return nil, err
	}
	return (*T)(msg), nil
}

func Json[T any, PT interface {
	*T
	proto.Message
}]() MessageParser[T, PT] {
	return &unmarshalFuncParser[T, PT]{unmarshalFunc: protojson.Unmarshal}
}

func BinaryProto[T any, PT interface {
	*T
	proto.Message
}]() MessageParser[T, PT] {
	return &unmarshalFuncParser[T, PT]{unmarshalFunc: proto.Unmarshal}
}

func TextProto[T any, PT interface {
	*T
	proto.Message
}]() MessageParser[T, PT] {
	return &unmarshalFuncParser[T, PT]{unmarshalFunc: prototext.Unmarshal}
}
