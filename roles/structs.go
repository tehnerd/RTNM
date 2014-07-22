package roles

import (
	"net"
	"rtnm/cfg"
	"rtnm/tlvs"

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

func ReadFromUDP(udpconn *net.UDPConn, cfg_dict cfg.CfgDict,
	read_chan chan UDPMessage) {
	msg_buf := make([]byte, 9000)
	udp_msg := make([]byte, 0)
	var TLV tlvs.TLVHeader
	for {
		var msg UDPMessage
		bytes, raddr, err := udpconn.ReadFromUDP(msg_buf)
		if err != nil {
			//TODO: add feedback like in tcp
			panic("error while listening for the udp connection")
		}
		udp_msg = append(udp_msg, msg_buf[:bytes]...)
		for {
			if len(udp_msg) < 4 {
				break
			}
			TLV.Decode(udp_msg[:4])
			if TLV.TLV_type != 1 && TLV.TLV_subtype != 1 {
				udp_msg = udp_msg[TLV.TLV_length:]
				continue
			}
			msg.Time = time.Now()
			msg.Message = udp_msg[4:TLV.TLV_length]
			msg.UDPAddr = *raddr
			read_chan <- msg
			udp_msg = udp_msg[TLV.TLV_length:]
		}
		udp_msg = udp_msg[:0]
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
		msg.Message = tlvs.GeneratePBTLV(msg.Message)
		udpconn.WriteToUDP(msg.Message, &msg.UDPAddr)
	}
}

type ProbeContext struct {
	KA_interval uint32
}

func (PC *ProbeContext) setKA(keepalive uint32) { PC.KA_interval = keepalive }
