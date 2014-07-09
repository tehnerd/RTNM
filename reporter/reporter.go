package reporter

import (
	"code.google.com/p/goprotobuf/proto"
	"fmt"
	"rtnm/rtnm_pb"
)

type ReportStruct struct {
	CS1        int64
	CS2        int64
	CS3        int64
	CS4        int64
	CS5        int64
	CS6        int64
	LocalSite  string
	RemoteSite string
}

func CollectReport(report_chan chan []byte) {
	for {
		msg := <-report_chan
		report := &rtnm_pb.MSGS{}
		err := proto.Unmarshal(msg, report)
		if err != nil {
			panic("cant unmarshal")
		}
		fmt.Println(report)
	}
}
