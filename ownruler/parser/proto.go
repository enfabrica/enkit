// This file provides a parser for ascii protocol buffers.
//
// See ../proto/owners.proto for details. Given protocol buffer library
// limitations, this parser cannot annotate correctly the file with line
// number information.
//
// Use ParseProtoOwners as your entry point.
//
package parser

import (
	"github.com/enfabrica/enkit/ownruler/proto"
	"google.golang.org/protobuf/encoding/prototext"
	"io"
	"io/ioutil"
)

func ParseProtoOwners(path string, input io.Reader) (*proto.Owners, error) {
	var config proto.Owners
	data, err := ioutil.ReadAll(input)
	if err != nil {
		return nil, err
	}
	if err := prototext.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	config.Location = path
	return &config, nil
}
