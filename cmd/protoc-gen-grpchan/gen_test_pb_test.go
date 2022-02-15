// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0-devel
// 	protoc        v3.14.0
// source: gen_test.proto

package main

import (
	proto "github.com/golang/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// This is a compile-time assertion that a sufficiently up-to-date version
// of the legacy proto package is being used.
const _ = proto.ProtoPackageIsVersion4

type Test struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id   int64  `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	Name string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
}

func (x *Test) Reset() {
	*x = Test{}
	if protoimpl.UnsafeEnabled {
		mi := &file_gen_test_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Test) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Test) ProtoMessage() {}

func (x *Test) ProtoReflect() protoreflect.Message {
	mi := &file_gen_test_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Test.ProtoReflect.Descriptor instead.
func (*Test) Descriptor() ([]byte, []int) {
	return file_gen_test_proto_rawDescGZIP(), []int{0}
}

func (x *Test) GetId() int64 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *Test) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

var File_gen_test_proto protoreflect.FileDescriptor

var file_gen_test_proto_rawDesc = []byte{
	0x0a, 0x0e, 0x67, 0x65, 0x6e, 0x5f, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x04, 0x6d, 0x61, 0x69, 0x6e, 0x22, 0x2a, 0x0a, 0x04, 0x54, 0x65, 0x73, 0x74, 0x12, 0x0e,
	0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x02, 0x69, 0x64, 0x12, 0x12,
	0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61,
	0x6d, 0x65, 0x32, 0xb1, 0x05, 0x0a, 0x0b, 0x54, 0x65, 0x73, 0x74, 0x53, 0x74, 0x72, 0x65, 0x61,
	0x6d, 0x73, 0x12, 0x20, 0x0a, 0x06, 0x55, 0x6e, 0x61, 0x72, 0x79, 0x31, 0x12, 0x0a, 0x2e, 0x6d,
	0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e,
	0x54, 0x65, 0x73, 0x74, 0x12, 0x20, 0x0a, 0x06, 0x55, 0x6e, 0x61, 0x72, 0x79, 0x32, 0x12, 0x0a,
	0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x6d, 0x61, 0x69,
	0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x12, 0x23, 0x0a, 0x07, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d,
	0x31, 0x12, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e,
	0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x28, 0x01, 0x12, 0x20, 0x0a, 0x06, 0x55,
	0x6e, 0x61, 0x72, 0x79, 0x33, 0x12, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73,
	0x74, 0x1a, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x12, 0x25, 0x0a,
	0x07, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x32, 0x12, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e,
	0x54, 0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74,
	0x28, 0x01, 0x30, 0x01, 0x12, 0x20, 0x0a, 0x06, 0x55, 0x6e, 0x61, 0x72, 0x79, 0x34, 0x12, 0x0a,
	0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x6d, 0x61, 0x69,
	0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x12, 0x20, 0x0a, 0x06, 0x55, 0x6e, 0x61, 0x72, 0x79, 0x35,
	0x12, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x6d,
	0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x12, 0x23, 0x0a, 0x07, 0x53, 0x74, 0x72, 0x65,
	0x61, 0x6d, 0x33, 0x12, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x1a,
	0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x30, 0x01, 0x12, 0x20, 0x0a,
	0x06, 0x55, 0x6e, 0x61, 0x72, 0x79, 0x36, 0x12, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54,
	0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x12,
	0x20, 0x0a, 0x06, 0x55, 0x6e, 0x61, 0x72, 0x79, 0x37, 0x12, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e,
	0x2e, 0x54, 0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73,
	0x74, 0x12, 0x20, 0x0a, 0x06, 0x55, 0x6e, 0x61, 0x72, 0x79, 0x38, 0x12, 0x0a, 0x2e, 0x6d, 0x61,
	0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54,
	0x65, 0x73, 0x74, 0x12, 0x23, 0x0a, 0x07, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x34, 0x12, 0x0a,
	0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x6d, 0x61, 0x69,
	0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x28, 0x01, 0x12, 0x23, 0x0a, 0x07, 0x53, 0x74, 0x72, 0x65,
	0x61, 0x6d, 0x35, 0x12, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x1a,
	0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x30, 0x01, 0x12, 0x20, 0x0a,
	0x06, 0x55, 0x6e, 0x61, 0x72, 0x79, 0x39, 0x12, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54,
	0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x12,
	0x21, 0x0a, 0x07, 0x55, 0x6e, 0x61, 0x72, 0x79, 0x31, 0x30, 0x12, 0x0a, 0x2e, 0x6d, 0x61, 0x69,
	0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65,
	0x73, 0x74, 0x12, 0x21, 0x0a, 0x07, 0x55, 0x6e, 0x61, 0x72, 0x79, 0x31, 0x31, 0x12, 0x0a, 0x2e,
	0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e,
	0x2e, 0x54, 0x65, 0x73, 0x74, 0x12, 0x25, 0x0a, 0x07, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x36,
	0x12, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x6d,
	0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x28, 0x01, 0x30, 0x01, 0x12, 0x25, 0x0a, 0x07,
	0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x37, 0x12, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54,
	0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x28,
	0x01, 0x30, 0x01, 0x12, 0x21, 0x0a, 0x07, 0x55, 0x6e, 0x61, 0x72, 0x79, 0x31, 0x32, 0x12, 0x0a,
	0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x6d, 0x61, 0x69,
	0x6e, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x42, 0x08, 0x5a, 0x06, 0x2e, 0x3b, 0x6d, 0x61, 0x69, 0x6e,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_gen_test_proto_rawDescOnce sync.Once
	file_gen_test_proto_rawDescData = file_gen_test_proto_rawDesc
)

func file_gen_test_proto_rawDescGZIP() []byte {
	file_gen_test_proto_rawDescOnce.Do(func() {
		file_gen_test_proto_rawDescData = protoimpl.X.CompressGZIP(file_gen_test_proto_rawDescData)
	})
	return file_gen_test_proto_rawDescData
}

var file_gen_test_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_gen_test_proto_goTypes = []interface{}{
	(*Test)(nil), // 0: main.Test
}
var file_gen_test_proto_depIdxs = []int32{
	0,  // 0: main.TestStreams.Unary1:input_type -> main.Test
	0,  // 1: main.TestStreams.Unary2:input_type -> main.Test
	0,  // 2: main.TestStreams.Stream1:input_type -> main.Test
	0,  // 3: main.TestStreams.Unary3:input_type -> main.Test
	0,  // 4: main.TestStreams.Stream2:input_type -> main.Test
	0,  // 5: main.TestStreams.Unary4:input_type -> main.Test
	0,  // 6: main.TestStreams.Unary5:input_type -> main.Test
	0,  // 7: main.TestStreams.Stream3:input_type -> main.Test
	0,  // 8: main.TestStreams.Unary6:input_type -> main.Test
	0,  // 9: main.TestStreams.Unary7:input_type -> main.Test
	0,  // 10: main.TestStreams.Unary8:input_type -> main.Test
	0,  // 11: main.TestStreams.Stream4:input_type -> main.Test
	0,  // 12: main.TestStreams.Stream5:input_type -> main.Test
	0,  // 13: main.TestStreams.Unary9:input_type -> main.Test
	0,  // 14: main.TestStreams.Unary10:input_type -> main.Test
	0,  // 15: main.TestStreams.Unary11:input_type -> main.Test
	0,  // 16: main.TestStreams.Stream6:input_type -> main.Test
	0,  // 17: main.TestStreams.Stream7:input_type -> main.Test
	0,  // 18: main.TestStreams.Unary12:input_type -> main.Test
	0,  // 19: main.TestStreams.Unary1:output_type -> main.Test
	0,  // 20: main.TestStreams.Unary2:output_type -> main.Test
	0,  // 21: main.TestStreams.Stream1:output_type -> main.Test
	0,  // 22: main.TestStreams.Unary3:output_type -> main.Test
	0,  // 23: main.TestStreams.Stream2:output_type -> main.Test
	0,  // 24: main.TestStreams.Unary4:output_type -> main.Test
	0,  // 25: main.TestStreams.Unary5:output_type -> main.Test
	0,  // 26: main.TestStreams.Stream3:output_type -> main.Test
	0,  // 27: main.TestStreams.Unary6:output_type -> main.Test
	0,  // 28: main.TestStreams.Unary7:output_type -> main.Test
	0,  // 29: main.TestStreams.Unary8:output_type -> main.Test
	0,  // 30: main.TestStreams.Stream4:output_type -> main.Test
	0,  // 31: main.TestStreams.Stream5:output_type -> main.Test
	0,  // 32: main.TestStreams.Unary9:output_type -> main.Test
	0,  // 33: main.TestStreams.Unary10:output_type -> main.Test
	0,  // 34: main.TestStreams.Unary11:output_type -> main.Test
	0,  // 35: main.TestStreams.Stream6:output_type -> main.Test
	0,  // 36: main.TestStreams.Stream7:output_type -> main.Test
	0,  // 37: main.TestStreams.Unary12:output_type -> main.Test
	19, // [19:38] is the sub-list for method output_type
	0,  // [0:19] is the sub-list for method input_type
	0,  // [0:0] is the sub-list for extension type_name
	0,  // [0:0] is the sub-list for extension extendee
	0,  // [0:0] is the sub-list for field type_name
}

func init() { file_gen_test_proto_init() }
func file_gen_test_proto_init() {
	if File_gen_test_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_gen_test_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Test); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_gen_test_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_gen_test_proto_goTypes,
		DependencyIndexes: file_gen_test_proto_depIdxs,
		MessageInfos:      file_gen_test_proto_msgTypes,
	}.Build()
	File_gen_test_proto = out.File
	file_gen_test_proto_rawDesc = nil
	file_gen_test_proto_goTypes = nil
	file_gen_test_proto_depIdxs = nil
}
