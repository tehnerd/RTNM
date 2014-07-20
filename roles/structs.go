package roles

import (
	"net"
	"rtnm/cfg"

	"syscall"
	"time"
)

type ProbeDescrMaster struct {
	IP         net.IP
	Location   string
	Preference int
}

type ProbeDescrProbe struct {
	IP       net.IP
	Location string
	MasterIp []string
}

func (pd *ProbeDescrProbe) AppendMaster(master string) {
	pd.MasterIp = append(pd.MasterIp, master)
}

type UDPMessage struct {
	UDPAddr net.UDPAddr
	Message []byte
	Time    time.Time
	TOS     int32
}

type TestsReport struct {
	Report   map[string]int64
	PeerSite string
}

func ReadFromUDP(udpconn *net.UDPConn, cfg_dict cfg.CfgDict, msg_buf []byte,
	read_chan chan UDPMessage) {
	for {
		var msg UDPMessage
		bytes, raddr, err := udpconn.ReadFromUDP(msg_buf)
		if err != nil {
			//TODO: add feedback like in tcp
			panic("error while listening for the udp connection")
		}
		msg.Time = time.Now()
		msg.Message = msg_buf[:bytes]
		msg.UDPAddr = *raddr
		read_chan <- msg
	}
}

func WriteToUDP(udpconn *net.UDPConn, cfg_dict cfg.CfgDict,
	write_chan chan UDPMessage) {
	for {
		msg := <-write_chan
		fd, _ := udpconn.File()
		syscall.SetsockoptInt(int(fd.Fd()), syscall.IPPROTO_IP, syscall.IP_TOS,
			int(msg.TOS))
		fd.Close()
		udpconn.WriteToUDP(msg.Message, &msg.UDPAddr)
	}
}

type ProbeContext struct {
	KA_interval uint32
}

func (PC *ProbeContext) setKA(keepalive uint32) { PC.KA_interval = keepalive }
