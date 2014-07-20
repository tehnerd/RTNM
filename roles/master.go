package roles

import (
	"fmt"
	"net"
	"os"
	"rtnm/cfg"
	"rtnm/netutils"
	"rtnm/reporter"
	"rtnm/rtnm_pb"
	"rtnm/rtnm_pubsub"
	"rtnm/tlvs"
	"strings"
	"sync"
	"time"

	"code.google.com/p/goprotobuf/proto"
)

//initial syncing of probe's list to new probe
func ProbeInitialSync(write_chan chan []byte, Probes map[string]ProbeDescrMaster,
	LocalProbe *ProbeDescrMaster, mutex *sync.RWMutex, master_ip string) {
	mutex.RLock()
	for ProbeID, ProbeDescr := range Probes {
		if ProbeID != (*LocalProbe).IP.String() {
			msg_pb := &rtnm_pb.MSGS{
				AProbe: &rtnm_pb.AddProbe{
					ProbeIp:       proto.String(ProbeDescr.IP.String()),
					ProbeLocation: proto.String(ProbeDescr.Location),
					MasterIp:      proto.String(master_ip),
				},
			}
			msg, _ := proto.Marshal(msg_pb)
			msg = tlvs.GeneratePBTLV(msg)
			write_chan <- msg
		}
	}
	mutex.RUnlock()
}

func RemoveProbe(broker_pub_chan chan rtnm_pubsub.ProbeInfo,
	broker_unsub_chan chan rtnm_pubsub.PubSubMeta,
	sub_chan rtnm_pubsub.PubSubMeta,
	probe_descr ProbeDescrMaster,
	mutex *sync.RWMutex, Probes map[string]ProbeDescrMaster,
	loop *int, feedback_chan_w chan int) {
	broker_pub_chan <- rtnm_pubsub.ProbeInfo{probe_descr.IP, probe_descr.Location, "Delete"}
	mutex.Lock()
	delete(Probes, probe_descr.IP.String())
	mutex.Unlock()
	broker_unsub_chan <- sub_chan
	*loop = 0
	select {
	case feedback_chan_w <- 1:
	case feedback := <-feedback_chan_w:
		feedback_chan_w <- feedback
	}

}

//Goroutine which controls the Probe
func ControlProbe(sock *net.TCPConn, cfg_dict cfg.CfgDict,
	Probes map[string]ProbeDescrMaster,
	mutex *sync.RWMutex,
	broker_sub_chan chan rtnm_pubsub.PubSubMeta,
	broker_unsub_chan chan rtnm_pubsub.PubSubMeta,
	broker_pub_chan chan rtnm_pubsub.ProbeInfo,
	external_report_chan chan []byte) {
	msg_buf := make([]byte, 65535)
	tcp_msg := make([]byte, 0)
	defer sock.Close()
	var probe_descr ProbeDescrMaster
	var sub_chan rtnm_pubsub.PubSubMeta
	write_chan := make(chan []byte)
	read_chan := make(chan []byte)
	feedback_chan_r := make(chan int)
	feedback_chan_w := make(chan int)
	go netutils.ReadFromTCP(sock, msg_buf, read_chan, feedback_chan_r)
	go netutils.WriteToTCP(sock, write_chan, feedback_chan_w)
	loop := 1
	for loop == 1 {
		select {
		case <-feedback_chan_r:
			RemoveProbe(broker_pub_chan, broker_unsub_chan, sub_chan, probe_descr,
				mutex, Probes, &loop, feedback_chan_w)
		case <-time.After(time.Duration(cfg_dict.KA_interval) * time.Second * 3):
			RemoveProbe(broker_pub_chan, broker_unsub_chan, sub_chan, probe_descr,
				mutex, Probes, &loop, feedback_chan_w)
			fmt.Println("probe timed out")
		case msg_from_probe := <-read_chan:
			tcp_msg = append(tcp_msg, msg_from_probe...)
			if len(tcp_msg) < 4 {
				continue
			}
			for {
				if len(tcp_msg) < 4 {
					break
				}
				var TLV tlvs.TLVHeader
				TLV.Decode(tcp_msg[0:4])
				if len(tcp_msg) < int(TLV.TLV_length) {
					break
				}
				if TLV.TLV_type != 1 && TLV.TLV_subtype != 1 { //1.1 -> protobuf.MSGS
					tcp_msg = tcp_msg[TLV.TLV_length:]
					fmt.Println("unknown tlv")
					continue
				}

				msg := &rtnm_pb.MSGS{}
				err := proto.Unmarshal(tcp_msg[4:TLV.TLV_length], msg)
				if err != nil {
					panic("error during unmarshaling protobuf")
				}
				if msg.GetHello() != nil {
				}
				if msg.GetPReg() != nil {
					probe_descr.IP = net.ParseIP(msg.GetPReg().GetProbeIp())
					probe_descr.Location = msg.GetPReg().GetProbeLocation()
					probe_descr.Preference = 150
					mutex.Lock()
					Probes[msg.GetPReg().GetProbeIp()] = probe_descr
					mutex.Unlock()
					sub_chan.SubscriberID = probe_descr.IP.String()
					sub_chan.SubscriberID = probe_descr.IP.String()
					sub_chan.Chan = make(chan rtnm_pubsub.ProbeInfo)
					broker_sub_chan <- sub_chan
					broker_pub_chan <- rtnm_pubsub.ProbeInfo{probe_descr.IP,
						probe_descr.Location, "Add"}
					reg_confirm := &rtnm_pb.MSGS{
						RConf: &rtnm_pb.MasterRegConfirm{
							ProbeKA:   proto.Uint32(cfg_dict.KA_interval),
							TestsList: proto.String(strings.Join(cfg_dict.Tests, " ")),
						},
					}
					data, _ := proto.Marshal(reg_confirm)
					data = tlvs.GeneratePBTLV(data)
					write_chan <- data
					ProbeInitialSync(write_chan, Probes, &probe_descr,
						mutex, cfg_dict.Bind_IP.String())
				}

				if msg.GetRep() != nil {
					external_report_chan <- tcp_msg[4:TLV.TLV_length]
				}
				tcp_msg = tcp_msg[TLV.TLV_length:]
			}
		case probe_info := <-sub_chan.Chan:
			if probe_info.Action == "Add" {
				msg_pb := &rtnm_pb.MSGS{
					AProbe: &rtnm_pb.AddProbe{
						ProbeIp:       proto.String(probe_info.IP.String()),
						ProbeLocation: proto.String(probe_info.Location),
						MasterIp:      proto.String(cfg_dict.Bind_IP.String()),
					},
				}
				msg, _ := proto.Marshal(msg_pb)
				msg = tlvs.GeneratePBTLV(msg)
				write_chan <- msg
			} else {
				msg_pb := &rtnm_pb.MSGS{
					RProbe: &rtnm_pb.RemoveProbe{
						ProbeIp:       proto.String(probe_info.IP.String()),
						ProbeLocation: proto.String(probe_info.Location),
						MasterIp:      proto.String(cfg_dict.Bind_IP.String()),
					},
				}
				msg, _ := proto.Marshal(msg_pb)
				msg = tlvs.GeneratePBTLV(msg)
				write_chan <- msg
			}

		}
	}
	fmt.Println("probecontrol killed")
}

func DebugOutputMaster(Probes map[string]ProbeDescrMaster,
	mutex *sync.RWMutex, cfg_dict cfg.CfgDict) {
	var debugAddr net.TCPAddr
	debugAddr.IP = net.ParseIP("127.0.0.1")
	debugAddr.Port = cfg_dict.DebugPort
	debug_socket, err := net.ListenTCP("tcp", &debugAddr)
	if err != nil {
		panic("cant bind to debug socket")
	}
	for {
		debug_conn, _ := debug_socket.AcceptTCP()
		debug_conn.Write([]byte("debug output\n"))
		mutex.RLock()
		for key, value := range Probes {
			debug_conn.Write([]byte(key))
			debug_conn.Write([]byte(" "))
			debug_conn.Write([]byte(value.Location))
			debug_conn.Write([]byte("\n"))
		}
		mutex.RUnlock()
		debug_conn.Close()
	}
}

//Central hub at master, which runs new goroutine for each probe
func StartMaster(cfg_dict cfg.CfgDict) {
	Probes := make(map[string]ProbeDescrMaster)
	var probes_mutex sync.RWMutex
	var tcpAddr net.TCPAddr
	tcpAddr.IP = cfg_dict.Bind_IP
	tcpAddr.Port = cfg_dict.Port
	sub_chan := make(chan rtnm_pubsub.PubSubMeta)
	unsub_chan := make(chan rtnm_pubsub.PubSubMeta)
	pub_chan := make(chan rtnm_pubsub.ProbeInfo)
	external_report_chan := make(chan []byte, 100)
	if cfg_dict.DebugPort != 0 {
		go DebugOutputMaster(Probes, &probes_mutex, cfg_dict)
	}
	go rtnm_pubsub.StartBroker(sub_chan, unsub_chan, pub_chan)
	go reporter.CollectReportGraphite(external_report_chan, cfg_dict)
	master_socket, err := net.ListenTCP("tcp", &tcpAddr)
	if err != nil {
		fmt.Println("cant open tcp socket")
		os.Exit(1)
	}
	for {
		probe_conn, _ := master_socket.AcceptTCP()
		go ControlProbe(probe_conn, cfg_dict, Probes, &probes_mutex,
			sub_chan, unsub_chan, pub_chan, external_report_chan)
	}
}
