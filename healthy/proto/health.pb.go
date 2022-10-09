// Code generated by protoc-gen-go. DO NOT EDIT.
// source: health.proto

package healthcheck

import (
	context "context"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type HealthRequest struct {
	Name                 string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *HealthRequest) Reset()         { *m = HealthRequest{} }
func (m *HealthRequest) String() string { return proto.CompactTextString(m) }
func (*HealthRequest) ProtoMessage()    {}
func (*HealthRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_fdbebe66dda7cb29, []int{0}
}

func (m *HealthRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_HealthRequest.Unmarshal(m, b)
}
func (m *HealthRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_HealthRequest.Marshal(b, m, deterministic)
}
func (m *HealthRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_HealthRequest.Merge(m, src)
}
func (m *HealthRequest) XXX_Size() int {
	return xxx_messageInfo_HealthRequest.Size(m)
}
func (m *HealthRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_HealthRequest.DiscardUnknown(m)
}

var xxx_messageInfo_HealthRequest proto.InternalMessageInfo

func (m *HealthRequest) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

type HealthReply struct {
	Message              string   `protobuf:"bytes,1,opt,name=message,proto3" json:"message,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *HealthReply) Reset()         { *m = HealthReply{} }
func (m *HealthReply) String() string { return proto.CompactTextString(m) }
func (*HealthReply) ProtoMessage()    {}
func (*HealthReply) Descriptor() ([]byte, []int) {
	return fileDescriptor_fdbebe66dda7cb29, []int{1}
}

func (m *HealthReply) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_HealthReply.Unmarshal(m, b)
}
func (m *HealthReply) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_HealthReply.Marshal(b, m, deterministic)
}
func (m *HealthReply) XXX_Merge(src proto.Message) {
	xxx_messageInfo_HealthReply.Merge(m, src)
}
func (m *HealthReply) XXX_Size() int {
	return xxx_messageInfo_HealthReply.Size(m)
}
func (m *HealthReply) XXX_DiscardUnknown() {
	xxx_messageInfo_HealthReply.DiscardUnknown(m)
}

var xxx_messageInfo_HealthReply proto.InternalMessageInfo

func (m *HealthReply) GetMessage() string {
	if m != nil {
		return m.Message
	}
	return ""
}

func init() {
	proto.RegisterType((*HealthRequest)(nil), "healthcheck.HealthRequest")
	proto.RegisterType((*HealthReply)(nil), "healthcheck.HealthReply")
}

func init() { proto.RegisterFile("health.proto", fileDescriptor_fdbebe66dda7cb29) }

var fileDescriptor_fdbebe66dda7cb29 = []byte{
	// 172 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0xc9, 0x48, 0x4d, 0xcc,
	0x29, 0xc9, 0xd0, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0xe2, 0x86, 0xf0, 0x92, 0x33, 0x52, 0x93,
	0xb3, 0x95, 0x94, 0xb9, 0x78, 0x3d, 0xc0, 0xdc, 0xa0, 0xd4, 0xc2, 0xd2, 0xd4, 0xe2, 0x12, 0x21,
	0x21, 0x2e, 0x96, 0xbc, 0xc4, 0xdc, 0x54, 0x09, 0x46, 0x05, 0x46, 0x0d, 0xce, 0x20, 0x30, 0x5b,
	0x49, 0x9d, 0x8b, 0x1b, 0xa6, 0xa8, 0x20, 0xa7, 0x52, 0x48, 0x82, 0x8b, 0x3d, 0x37, 0xb5, 0xb8,
	0x38, 0x31, 0x1d, 0xa6, 0x0a, 0xc6, 0x35, 0xf2, 0xe5, 0x62, 0x83, 0x18, 0x2e, 0xe4, 0xcc, 0xc5,
	0x19, 0x9c, 0x58, 0x09, 0xd1, 0x25, 0x24, 0xa5, 0x87, 0x64, 0xa5, 0x1e, 0x8a, 0x7d, 0x52, 0x12,
	0x58, 0xe5, 0x0a, 0x72, 0x2a, 0x95, 0x18, 0x9c, 0x8c, 0xb8, 0x64, 0x32, 0xf3, 0xf5, 0xd2, 0x8b,
	0x0a, 0x92, 0xf5, 0x52, 0x2b, 0x12, 0x73, 0x0b, 0x72, 0x52, 0x8b, 0x91, 0x55, 0x3b, 0x09, 0x40,
	0x94, 0x3b, 0x83, 0x38, 0x01, 0x20, 0xbf, 0x05, 0x30, 0x26, 0xb1, 0x81, 0x3d, 0x69, 0x0c, 0x08,
	0x00, 0x00, 0xff, 0xff, 0xb3, 0x04, 0x03, 0x8e, 0xf4, 0x00, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// HealthClient is the client API for Health service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type HealthClient interface {
	SayHealth(ctx context.Context, in *HealthRequest, opts ...grpc.CallOption) (*HealthReply, error)
}

type healthClient struct {
	cc *grpc.ClientConn
}

func NewHealthClient(cc *grpc.ClientConn) HealthClient {
	return &healthClient{cc}
}

func (c *healthClient) SayHealth(ctx context.Context, in *HealthRequest, opts ...grpc.CallOption) (*HealthReply, error) {
	out := new(HealthReply)
	err := c.cc.Invoke(ctx, "/healthcheck.health/SayHealth", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// HealthServer is the server API for Health service.
type HealthServer interface {
	SayHealth(context.Context, *HealthRequest) (*HealthReply, error)
}

// UnimplementedHealthServer can be embedded to have forward compatible implementations.
type UnimplementedHealthServer struct {
}

func (*UnimplementedHealthServer) SayHealth(ctx context.Context, req *HealthRequest) (*HealthReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SayHealth not implemented")
}

func RegisterHealthServer(s *grpc.Server, srv HealthServer) {
	s.RegisterService(&_Health_serviceDesc, srv)
}

func _Health_SayHealth_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HealthRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(HealthServer).SayHealth(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/healthcheck.health/SayHealth",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(HealthServer).SayHealth(ctx, req.(*HealthRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _Health_serviceDesc = grpc.ServiceDesc{
	ServiceName: "healthcheck.health",
	HandlerType: (*HealthServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SayHealth",
			Handler:    _Health_SayHealth_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "health.proto",
}
