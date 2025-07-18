package patch

import (
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
)

func fieldOptions(f *protogen.Field) *Options {
	return proto.GetExtension(f.Desc.Options(), E_Field).(*Options)
}
