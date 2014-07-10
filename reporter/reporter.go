package reporter

import (
	"code.google.com/p/goprotobuf/proto"
	"net"
	"rtnm/cfg"
	"rtnm/netutils"
	"rtnm/rtnm_pb"
	"strconv"
	"strings"
	"time"
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

func CollectReportGraphite(report_chan chan []byte, cfg_dict cfg.CfgDict) {
	write_chan := make(chan []byte)
	feedback_chan := make(chan int)
	var reporterAddr net.TCPAddr
	fields := strings.Split(cfg_dict.Reporter, ":")
	reporterAddr.IP = net.ParseIP(fields[0])
	reporterAddr.Port, _ = strconv.Atoi(fields[1])
	sock, err := net.DialTCP("tcp", nil, &reporterAddr)
	if err != nil {
		panic("cant connect to graphite server")
	}
	go netutils.WriteToTCP(sock, write_chan, feedback_chan)
	for {
		msg := <-report_chan
		report_msg := &rtnm_pb.MSGS{}
		proto.Unmarshal(msg, report_msg)
		var report ReportStruct
		report.CS1 = report_msg.GetRep().GetCS1()
		report.CS2 = report_msg.GetRep().GetCS2()
		report.CS3 = report_msg.GetRep().GetCS3()
		report.CS4 = report_msg.GetRep().GetCS4()
		report.CS5 = report_msg.GetRep().GetCS5()
		report.CS6 = report_msg.GetRep().GetCS6()
		report.LocalSite = report_msg.GetRep().GetLocalSite()
		report.RemoteSite = report_msg.GetRep().GetRemoteSite()
		key := strings.Join([]string{report.LocalSite, report.RemoteSite}, "-")
		key = strings.Join([]string{"stats.RTT", key}, ".")
		time := strconv.FormatInt(time.Now().Unix(), 10)
		write_chan <- []byte(strings.Join([]string{strings.Join([]string{key, "CS1"}, "."), strconv.FormatInt(report.CS1, 10), time, "\n"}, " "))
		write_chan <- []byte(strings.Join([]string{strings.Join([]string{key, "CS2"}, "."), strconv.FormatInt(report.CS2, 10), time, "\n"}, " "))
		write_chan <- []byte(strings.Join([]string{strings.Join([]string{key, "CS3"}, "."), strconv.FormatInt(report.CS3, 10), time, "\n"}, " "))
		write_chan <- []byte(strings.Join([]string{strings.Join([]string{key, "CS4"}, "."), strconv.FormatInt(report.CS4, 10), time, "\n"}, " "))
		write_chan <- []byte(strings.Join([]string{strings.Join([]string{key, "CS5"}, "."), strconv.FormatInt(report.CS5, 10), time, "\n"}, " "))
		write_chan <- []byte(strings.Join([]string{strings.Join([]string{key, "CS6"}, "."), strconv.FormatInt(report.CS6, 10), time, "\n"}, " "))
	}
}
