package timestamps

import (
	"code.google.com/p/goprotobuf/proto"
	"rtnm/rtnm_pb"
	"time"
)

type TimeStamps struct {
	TOS int32
	T1  int64
	T2  int64
	T3  int64
}

var TOSMAP = map[string]int32{
	"CS0": 0,
	"CS1": 32,
	"CS2": 64,
	"CS3": 96,
	"CS4": 128,
	"CS5": 160,
	"CS6": 192,
}

//generating pb from TimeStamps
func GenerateTimeMsg(TimeStamp *TimeStamps, ReceiveTime time.Time) []byte {
	if (*TimeStamp).T1 == 0 {
		(*TimeStamp).T1 = time.Now().UnixNano()
	} else if (*TimeStamp).T2 == 0 {
		(*TimeStamp).T2 = ReceiveTime.UnixNano()
		(*TimeStamp).T3 = time.Now().UnixNano()
	}
	timestamp_msg := &rtnm_pb.MSGS{
		TStamp: &rtnm_pb.TimeStamps{
			T1:  proto.Int64((*TimeStamp).T1),
			T2:  proto.Int64((*TimeStamp).T2),
			T3:  proto.Int64((*TimeStamp).T3),
			TOS: proto.Int32((*TimeStamp).TOS),
		},
	}
	timestamp_data, err := proto.Marshal(timestamp_msg)
	if err != nil {
		panic("cant marshal time pb")
	}
	return timestamp_data
}

//generating TimeStamps from protobuf; we do
//already know that his msg could be Unmarshaled so we dont test for err
func ParseRcvdTimeMsg(data []byte) TimeStamps {
	msg := &rtnm_pb.MSGS{}
	proto.Unmarshal(data, msg)
	var timestamp TimeStamps
	timestamp.T1 = msg.GetTStamp().GetT1()
	timestamp.T2 = msg.GetTStamp().GetT1()
	timestamp.T3 = msg.GetTStamp().GetT1()
	timestamp.TOS = msg.GetTStamp().GetTOS()
	return timestamp
}

func CalculateLatency(TimeStamp *TimeStamps, ReceiveTime time.Time) int64 {
	return (ReceiveTime.UnixNano() - (TimeStamp.T3 - TimeStamp.T2) - TimeStamp.T1)
}
