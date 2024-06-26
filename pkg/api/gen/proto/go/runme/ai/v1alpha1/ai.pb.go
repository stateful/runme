// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        (unknown)
// source: runme/ai/v1alpha1/ai.proto

package aiv1alpha1

import (
	v1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/parser/v1"
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

type GenerateCellsRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Notebook *v1.Notebook `protobuf:"bytes,1,opt,name=notebook,proto3" json:"notebook,omitempty"`
}

func (x *GenerateCellsRequest) Reset() {
	*x = GenerateCellsRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_runme_ai_v1alpha1_ai_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GenerateCellsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GenerateCellsRequest) ProtoMessage() {}

func (x *GenerateCellsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_runme_ai_v1alpha1_ai_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GenerateCellsRequest.ProtoReflect.Descriptor instead.
func (*GenerateCellsRequest) Descriptor() ([]byte, []int) {
	return file_runme_ai_v1alpha1_ai_proto_rawDescGZIP(), []int{0}
}

func (x *GenerateCellsRequest) GetNotebook() *v1.Notebook {
	if x != nil {
		return x.Notebook
	}
	return nil
}

type GenerateCellsResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Cells []*v1.Cell `protobuf:"bytes,1,rep,name=cells,proto3" json:"cells,omitempty"`
}

func (x *GenerateCellsResponse) Reset() {
	*x = GenerateCellsResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_runme_ai_v1alpha1_ai_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GenerateCellsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GenerateCellsResponse) ProtoMessage() {}

func (x *GenerateCellsResponse) ProtoReflect() protoreflect.Message {
	mi := &file_runme_ai_v1alpha1_ai_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GenerateCellsResponse.ProtoReflect.Descriptor instead.
func (*GenerateCellsResponse) Descriptor() ([]byte, []int) {
	return file_runme_ai_v1alpha1_ai_proto_rawDescGZIP(), []int{1}
}

func (x *GenerateCellsResponse) GetCells() []*v1.Cell {
	if x != nil {
		return x.Cells
	}
	return nil
}

var File_runme_ai_v1alpha1_ai_proto protoreflect.FileDescriptor

var file_runme_ai_v1alpha1_ai_proto_rawDesc = []byte{
	0x0a, 0x1a, 0x72, 0x75, 0x6e, 0x6d, 0x65, 0x2f, 0x61, 0x69, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70,
	0x68, 0x61, 0x31, 0x2f, 0x61, 0x69, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x11, 0x72, 0x75,
	0x6e, 0x6d, 0x65, 0x2e, 0x61, 0x69, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x1a,
	0x1c, 0x72, 0x75, 0x6e, 0x6d, 0x65, 0x2f, 0x70, 0x61, 0x72, 0x73, 0x65, 0x72, 0x2f, 0x76, 0x31,
	0x2f, 0x70, 0x61, 0x72, 0x73, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x4d, 0x0a,
	0x14, 0x47, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x43, 0x65, 0x6c, 0x6c, 0x73, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x35, 0x0a, 0x08, 0x6e, 0x6f, 0x74, 0x65, 0x62, 0x6f, 0x6f,
	0x6b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x72, 0x75, 0x6e, 0x6d, 0x65, 0x2e,
	0x70, 0x61, 0x72, 0x73, 0x65, 0x72, 0x2e, 0x76, 0x31, 0x2e, 0x4e, 0x6f, 0x74, 0x65, 0x62, 0x6f,
	0x6f, 0x6b, 0x52, 0x08, 0x6e, 0x6f, 0x74, 0x65, 0x62, 0x6f, 0x6f, 0x6b, 0x22, 0x44, 0x0a, 0x15,
	0x47, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x43, 0x65, 0x6c, 0x6c, 0x73, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x2b, 0x0a, 0x05, 0x63, 0x65, 0x6c, 0x6c, 0x73, 0x18, 0x01,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x15, 0x2e, 0x72, 0x75, 0x6e, 0x6d, 0x65, 0x2e, 0x70, 0x61, 0x72,
	0x73, 0x65, 0x72, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x65, 0x6c, 0x6c, 0x52, 0x05, 0x63, 0x65, 0x6c,
	0x6c, 0x73, 0x32, 0x71, 0x0a, 0x09, 0x41, 0x49, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12,
	0x64, 0x0a, 0x0d, 0x47, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x43, 0x65, 0x6c, 0x6c, 0x73,
	0x12, 0x27, 0x2e, 0x72, 0x75, 0x6e, 0x6d, 0x65, 0x2e, 0x61, 0x69, 0x2e, 0x76, 0x31, 0x61, 0x6c,
	0x70, 0x68, 0x61, 0x31, 0x2e, 0x47, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x43, 0x65, 0x6c,
	0x6c, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x28, 0x2e, 0x72, 0x75, 0x6e, 0x6d,
	0x65, 0x2e, 0x61, 0x69, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x47, 0x65,
	0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x43, 0x65, 0x6c, 0x6c, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x4d, 0x5a, 0x4b, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e,
	0x63, 0x6f, 0x6d, 0x2f, 0x73, 0x74, 0x61, 0x74, 0x65, 0x66, 0x75, 0x6c, 0x2f, 0x72, 0x75, 0x6e,
	0x6d, 0x65, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x67, 0x65, 0x6e, 0x2f, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x67, 0x6f, 0x2f, 0x72, 0x75, 0x6e, 0x6d, 0x65, 0x2f, 0x61, 0x69,
	0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x3b, 0x61, 0x69, 0x76, 0x31, 0x61, 0x6c,
	0x70, 0x68, 0x61, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_runme_ai_v1alpha1_ai_proto_rawDescOnce sync.Once
	file_runme_ai_v1alpha1_ai_proto_rawDescData = file_runme_ai_v1alpha1_ai_proto_rawDesc
)

func file_runme_ai_v1alpha1_ai_proto_rawDescGZIP() []byte {
	file_runme_ai_v1alpha1_ai_proto_rawDescOnce.Do(func() {
		file_runme_ai_v1alpha1_ai_proto_rawDescData = protoimpl.X.CompressGZIP(file_runme_ai_v1alpha1_ai_proto_rawDescData)
	})
	return file_runme_ai_v1alpha1_ai_proto_rawDescData
}

var file_runme_ai_v1alpha1_ai_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_runme_ai_v1alpha1_ai_proto_goTypes = []any{
	(*GenerateCellsRequest)(nil),  // 0: runme.ai.v1alpha1.GenerateCellsRequest
	(*GenerateCellsResponse)(nil), // 1: runme.ai.v1alpha1.GenerateCellsResponse
	(*v1.Notebook)(nil),           // 2: runme.parser.v1.Notebook
	(*v1.Cell)(nil),               // 3: runme.parser.v1.Cell
}
var file_runme_ai_v1alpha1_ai_proto_depIdxs = []int32{
	2, // 0: runme.ai.v1alpha1.GenerateCellsRequest.notebook:type_name -> runme.parser.v1.Notebook
	3, // 1: runme.ai.v1alpha1.GenerateCellsResponse.cells:type_name -> runme.parser.v1.Cell
	0, // 2: runme.ai.v1alpha1.AIService.GenerateCells:input_type -> runme.ai.v1alpha1.GenerateCellsRequest
	1, // 3: runme.ai.v1alpha1.AIService.GenerateCells:output_type -> runme.ai.v1alpha1.GenerateCellsResponse
	3, // [3:4] is the sub-list for method output_type
	2, // [2:3] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_runme_ai_v1alpha1_ai_proto_init() }
func file_runme_ai_v1alpha1_ai_proto_init() {
	if File_runme_ai_v1alpha1_ai_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_runme_ai_v1alpha1_ai_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*GenerateCellsRequest); i {
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
		file_runme_ai_v1alpha1_ai_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*GenerateCellsResponse); i {
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
			RawDescriptor: file_runme_ai_v1alpha1_ai_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_runme_ai_v1alpha1_ai_proto_goTypes,
		DependencyIndexes: file_runme_ai_v1alpha1_ai_proto_depIdxs,
		MessageInfos:      file_runme_ai_v1alpha1_ai_proto_msgTypes,
	}.Build()
	File_runme_ai_v1alpha1_ai_proto = out.File
	file_runme_ai_v1alpha1_ai_proto_rawDesc = nil
	file_runme_ai_v1alpha1_ai_proto_goTypes = nil
	file_runme_ai_v1alpha1_ai_proto_depIdxs = nil
}
