// Code generated by protoc-gen-go.
// source: msgs.proto
// DO NOT EDIT!

/*
Package rtnm_pb is a generated protocol buffer package.

It is generated from these files:
	msgs.proto

It has these top-level messages:
	MSGS
	ProbeRegister
	MasterRegConfirm
	ProbeHello
	AddProbe
	RemoveProbe
	TimeStamps
	Report
	PeerAddProbe
	PeerRemoveProbe
*/
package rtnm_pb

import proto "code.google.com/p/goprotobuf/proto"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = math.Inf

// metamessage; which we could test for submsg
// instead of implementing TLV logick directly
type MSGS struct {
	PReg             *ProbeRegister    `protobuf:"bytes,1,opt" json:"PReg,omitempty"`
	RConf            *MasterRegConfirm `protobuf:"bytes,2,opt" json:"RConf,omitempty"`
	Hello            *ProbeHello       `protobuf:"bytes,3,opt" json:"Hello,omitempty"`
	AProbe           *AddProbe         `protobuf:"bytes,4,opt" json:"AProbe,omitempty"`
	RProbe           *RemoveProbe      `protobuf:"bytes,5,opt" json:"RProbe,omitempty"`
	TStamp           *TimeStamps       `protobuf:"bytes,6,opt" json:"TStamp,omitempty"`
	Rep              *Report           `protobuf:"bytes,7,opt" json:"Rep,omitempty"`
	PAProbe          *PeerAddProbe     `protobuf:"bytes,9,opt" json:"PAProbe,omitempty"`
	PRProbe          *PeerRemoveProbe  `protobuf:"bytes,10,opt" json:"PRProbe,omitempty"`
	XXX_unrecognized []byte            `json:"-"`
}

func (m *MSGS) Reset()         { *m = MSGS{} }
func (m *MSGS) String() string { return proto.CompactTextString(m) }
func (*MSGS) ProtoMessage()    {}

func (m *MSGS) GetPReg() *ProbeRegister {
	if m != nil {
		return m.PReg
	}
	return nil
}

func (m *MSGS) GetRConf() *MasterRegConfirm {
	if m != nil {
		return m.RConf
	}
	return nil
}

func (m *MSGS) GetHello() *ProbeHello {
	if m != nil {
		return m.Hello
	}
	return nil
}

func (m *MSGS) GetAProbe() *AddProbe {
	if m != nil {
		return m.AProbe
	}
	return nil
}

func (m *MSGS) GetRProbe() *RemoveProbe {
	if m != nil {
		return m.RProbe
	}
	return nil
}

func (m *MSGS) GetTStamp() *TimeStamps {
	if m != nil {
		return m.TStamp
	}
	return nil
}

func (m *MSGS) GetRep() *Report {
	if m != nil {
		return m.Rep
	}
	return nil
}

func (m *MSGS) GetPAProbe() *PeerAddProbe {
	if m != nil {
		return m.PAProbe
	}
	return nil
}

func (m *MSGS) GetPRProbe() *PeerRemoveProbe {
	if m != nil {
		return m.PRProbe
	}
	return nil
}

// msg, which probe sends to master during startup
type ProbeRegister struct {
	ProbeIp          *string `protobuf:"bytes,1,opt" json:"ProbeIp,omitempty"`
	ProbeLocation    *string `protobuf:"bytes,2,opt" json:"ProbeLocation,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *ProbeRegister) Reset()         { *m = ProbeRegister{} }
func (m *ProbeRegister) String() string { return proto.CompactTextString(m) }
func (*ProbeRegister) ProtoMessage()    {}

func (m *ProbeRegister) GetProbeIp() string {
	if m != nil && m.ProbeIp != nil {
		return *m.ProbeIp
	}
	return ""
}

func (m *ProbeRegister) GetProbeLocation() string {
	if m != nil && m.ProbeLocation != nil {
		return *m.ProbeLocation
	}
	return ""
}

// msg from master to probe during startup
type MasterRegConfirm struct {
	ProbeKA          *uint32 `protobuf:"varint,1,opt" json:"ProbeKA,omitempty"`
	TestsList        *string `protobuf:"bytes,2,opt" json:"TestsList,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *MasterRegConfirm) Reset()         { *m = MasterRegConfirm{} }
func (m *MasterRegConfirm) String() string { return proto.CompactTextString(m) }
func (*MasterRegConfirm) ProtoMessage()    {}

func (m *MasterRegConfirm) GetProbeKA() uint32 {
	if m != nil && m.ProbeKA != nil {
		return *m.ProbeKA
	}
	return 0
}

func (m *MasterRegConfirm) GetTestsList() string {
	if m != nil && m.TestsList != nil {
		return *m.TestsList
	}
	return ""
}

// keepalive msg
type ProbeHello struct {
	Hello            *string `protobuf:"bytes,1,opt" json:"Hello,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *ProbeHello) Reset()         { *m = ProbeHello{} }
func (m *ProbeHello) String() string { return proto.CompactTextString(m) }
func (*ProbeHello) ProtoMessage()    {}

func (m *ProbeHello) GetHello() string {
	if m != nil && m.Hello != nil {
		return *m.Hello
	}
	return ""
}

// msg from master to probe, master send it when
// new probe registers
type AddProbe struct {
	ProbeIp          *string `protobuf:"bytes,1,opt" json:"ProbeIp,omitempty"`
	ProbeLocation    *string `protobuf:"bytes,2,opt" json:"ProbeLocation,omitempty"`
	MasterIp         *string `protobuf:"bytes,3,opt" json:"MasterIp,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *AddProbe) Reset()         { *m = AddProbe{} }
func (m *AddProbe) String() string { return proto.CompactTextString(m) }
func (*AddProbe) ProtoMessage()    {}

func (m *AddProbe) GetProbeIp() string {
	if m != nil && m.ProbeIp != nil {
		return *m.ProbeIp
	}
	return ""
}

func (m *AddProbe) GetProbeLocation() string {
	if m != nil && m.ProbeLocation != nil {
		return *m.ProbeLocation
	}
	return ""
}

func (m *AddProbe) GetMasterIp() string {
	if m != nil && m.MasterIp != nil {
		return *m.MasterIp
	}
	return ""
}

// msg from master to probe, master send it when
// some probe timed out
type RemoveProbe struct {
	ProbeIp          *string `protobuf:"bytes,1,opt" json:"ProbeIp,omitempty"`
	ProbeLocation    *string `protobuf:"bytes,2,opt" json:"ProbeLocation,omitempty"`
	MasterIp         *string `protobuf:"bytes,3,opt" json:"MasterIp,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *RemoveProbe) Reset()         { *m = RemoveProbe{} }
func (m *RemoveProbe) String() string { return proto.CompactTextString(m) }
func (*RemoveProbe) ProtoMessage()    {}

func (m *RemoveProbe) GetProbeIp() string {
	if m != nil && m.ProbeIp != nil {
		return *m.ProbeIp
	}
	return ""
}

func (m *RemoveProbe) GetProbeLocation() string {
	if m != nil && m.ProbeLocation != nil {
		return *m.ProbeLocation
	}
	return ""
}

func (m *RemoveProbe) GetMasterIp() string {
	if m != nil && m.MasterIp != nil {
		return *m.MasterIp
	}
	return ""
}

// msg from probe to probe. we are using it to calculate timeskew
// between the nodes
type TimeStamps struct {
	T1               *int64 `protobuf:"varint,1,opt" json:"T1,omitempty"`
	T2               *int64 `protobuf:"varint,2,opt" json:"T2,omitempty"`
	T3               *int64 `protobuf:"varint,3,opt" json:"T3,omitempty"`
	TOS              *int32 `protobuf:"varint,4,opt" json:"TOS,omitempty"`
	XXX_unrecognized []byte `json:"-"`
}

func (m *TimeStamps) Reset()         { *m = TimeStamps{} }
func (m *TimeStamps) String() string { return proto.CompactTextString(m) }
func (*TimeStamps) ProtoMessage()    {}

func (m *TimeStamps) GetT1() int64 {
	if m != nil && m.T1 != nil {
		return *m.T1
	}
	return 0
}

func (m *TimeStamps) GetT2() int64 {
	if m != nil && m.T2 != nil {
		return *m.T2
	}
	return 0
}

func (m *TimeStamps) GetT3() int64 {
	if m != nil && m.T3 != nil {
		return *m.T3
	}
	return 0
}

func (m *TimeStamps) GetTOS() int32 {
	if m != nil && m.TOS != nil {
		return *m.TOS
	}
	return 0
}

// latency tests report
type Report struct {
	CS1              *int64  `protobuf:"varint,1,opt" json:"CS1,omitempty"`
	CS2              *int64  `protobuf:"varint,2,opt" json:"CS2,omitempty"`
	CS3              *int64  `protobuf:"varint,3,opt" json:"CS3,omitempty"`
	CS4              *int64  `protobuf:"varint,4,opt" json:"CS4,omitempty"`
	CS5              *int64  `protobuf:"varint,5,opt" json:"CS5,omitempty"`
	CS6              *int64  `protobuf:"varint,6,opt" json:"CS6,omitempty"`
	LocalSite        *string `protobuf:"bytes,7,opt" json:"LocalSite,omitempty"`
	RemoteSite       *string `protobuf:"bytes,8,opt" json:"RemoteSite,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *Report) Reset()         { *m = Report{} }
func (m *Report) String() string { return proto.CompactTextString(m) }
func (*Report) ProtoMessage()    {}

func (m *Report) GetCS1() int64 {
	if m != nil && m.CS1 != nil {
		return *m.CS1
	}
	return 0
}

func (m *Report) GetCS2() int64 {
	if m != nil && m.CS2 != nil {
		return *m.CS2
	}
	return 0
}

func (m *Report) GetCS3() int64 {
	if m != nil && m.CS3 != nil {
		return *m.CS3
	}
	return 0
}

func (m *Report) GetCS4() int64 {
	if m != nil && m.CS4 != nil {
		return *m.CS4
	}
	return 0
}

func (m *Report) GetCS5() int64 {
	if m != nil && m.CS5 != nil {
		return *m.CS5
	}
	return 0
}

func (m *Report) GetCS6() int64 {
	if m != nil && m.CS6 != nil {
		return *m.CS6
	}
	return 0
}

func (m *Report) GetLocalSite() string {
	if m != nil && m.LocalSite != nil {
		return *m.LocalSite
	}
	return ""
}

func (m *Report) GetRemoteSite() string {
	if m != nil && m.RemoteSite != nil {
		return *m.RemoteSite
	}
	return ""
}

// msg from peer(master) to peer, peer send it when
// new probe registers
type PeerAddProbe struct {
	ProbeIp          *string `protobuf:"bytes,1,opt" json:"ProbeIp,omitempty"`
	ProbeLocation    *string `protobuf:"bytes,2,opt" json:"ProbeLocation,omitempty"`
	OriginMaster     *string `protobuf:"bytes,3,opt" json:"OriginMaster,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *PeerAddProbe) Reset()         { *m = PeerAddProbe{} }
func (m *PeerAddProbe) String() string { return proto.CompactTextString(m) }
func (*PeerAddProbe) ProtoMessage()    {}

func (m *PeerAddProbe) GetProbeIp() string {
	if m != nil && m.ProbeIp != nil {
		return *m.ProbeIp
	}
	return ""
}

func (m *PeerAddProbe) GetProbeLocation() string {
	if m != nil && m.ProbeLocation != nil {
		return *m.ProbeLocation
	}
	return ""
}

func (m *PeerAddProbe) GetOriginMaster() string {
	if m != nil && m.OriginMaster != nil {
		return *m.OriginMaster
	}
	return ""
}

// msg from peer(master) to peer, peer send it when
// some probe timed out
type PeerRemoveProbe struct {
	ProbeIp          *string `protobuf:"bytes,1,opt" json:"ProbeIp,omitempty"`
	ProbeLocation    *string `protobuf:"bytes,2,opt" json:"ProbeLocation,omitempty"`
	OriginMaster     *string `protobuf:"bytes,3,opt" json:"OriginMaster,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *PeerRemoveProbe) Reset()         { *m = PeerRemoveProbe{} }
func (m *PeerRemoveProbe) String() string { return proto.CompactTextString(m) }
func (*PeerRemoveProbe) ProtoMessage()    {}

func (m *PeerRemoveProbe) GetProbeIp() string {
	if m != nil && m.ProbeIp != nil {
		return *m.ProbeIp
	}
	return ""
}

func (m *PeerRemoveProbe) GetProbeLocation() string {
	if m != nil && m.ProbeLocation != nil {
		return *m.ProbeLocation
	}
	return ""
}

func (m *PeerRemoveProbe) GetOriginMaster() string {
	if m != nil && m.OriginMaster != nil {
		return *m.OriginMaster
	}
	return ""
}

func init() {
}
