package roles

import (
	"fmt"
	"math/rand"
	"net"
	"rtnm/cfg"
	"rtnm/netutils"
	"rtnm/rtnm_pb"
	"rtnm/timestamps"
	"rtnm/tlvs"
	"rtnm/utils"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"code.google.com/p/goprotobuf/proto"
)

func GenerateInitialHello(cfg_dict *cfg.CfgDict) []byte {
	init_hello := &rtnm_pb.MSGS{
		PReg: &rtnm_pb.ProbeRegister{
			ProbeIp:       proto.String(cfg_dict.Bind_IP.String()),
			ProbeLocation: proto.String(cfg_dict.Location),
		},
	}
	data, _ := proto.Marshal(init_hello)
	data = tlvs.GeneratePBTLV(data)
	return data
}

func GenerateUDPMessage(udpaddr *net.UDPAddr, msg []byte, tos int32) UDPMessage {
	var udpmsg UDPMessage
	udpmsg.UDPAddr = *udpaddr
	udpmsg.Message = msg
	udpmsg.TOS = tos
	return udpmsg
}

//reply for latency test, initiated from retmote side in case we arent running local one
func LatencyTestReply(msg UDPMessage, udp_write_chan chan UDPMessage) {
	timestamp := timestamps.ParseRcvdTimeMsg(msg.Message)
	reply_data := timestamps.GenerateTimeMsg(&timestamp, msg.Time)
	udp_write_chan <- GenerateUDPMessage(&msg.UDPAddr, reply_data, timestamp.TOS)
}

func PeriodicTests(SiteProbes map[string]map[string]ProbeDescrProbe,
	SiteMap map[string]int, udp_write_chan chan UDPMessage,
	latency_test_chan chan UDPMessage, cfg_dict *cfg.CfgDict,
	flag *int32, mutex *sync.RWMutex, report_chan chan TestsReport) {
	//TODO: add feedback chan
	for {
		if len(SiteMap) == 0 && len(SiteProbes) != 0 {
			mutex.Lock()
			for key, _ := range SiteProbes {
				if key != (*cfg_dict).Location {
					SiteMap[key] = 1
				}
			}
			mutex.Unlock()
		}
		time.Sleep(time.Duration(rand.Int31n(15)+10) * time.Second)
		mutex.RLock()
		if len(SiteMap) == 0 {
			mutex.RUnlock()
			continue
		}
		var Site string
		for Site, _ = range SiteMap {
			break
		}
		lock_flag := 1
		if len(SiteProbes[Site]) != 0 {
			probe_n := rand.Int31n(int32(len(SiteProbes[Site])))
			cntr := 0
			for _, value := range SiteProbes[Site] {
				if int32(cntr) == probe_n {
					atomic.AddInt32(flag, 1)
					lock_flag = 0
					mutex.RUnlock()
					mutex.Lock()
					delete(SiteMap, Site)
					mutex.Unlock()
					LatencyTest(value, cfg_dict, udp_write_chan,
						latency_test_chan, flag, report_chan)
					break
				}
				cntr++
			}
		}
		if lock_flag == 1 {
			mutex.RUnlock()
		}
	}
}

//latency test initiated from our side
func LatencyTest(remote_probe ProbeDescrProbe, cfg_dict *cfg.CfgDict,
	udp_write_chan chan UDPMessage, rcvd_msg chan UDPMessage, flag *int32,
	report_chan chan TestsReport) {
	remote_addr := strings.Join([]string{remote_probe.IP.String(),
		strconv.Itoa((*cfg_dict).Port)}, ":")
	udpaddr, err := net.ResolveUDPAddr("udp", remote_addr)
	if err != nil {
		fmt.Println(err)
		panic("cant resolve udp address")
	}
	report := make(map[string]int64)
	for class_name, tos := range timestamps.TOSMAP {
		for pkt_cntr := 0; pkt_cntr < 10; pkt_cntr++ {
			var timestamp timestamps.TimeStamps
			timestamp.TOS = tos
			msg_data := timestamps.GenerateTimeMsg(&timestamp, time.Now())
			udp_write_chan <- GenerateUDPMessage(udpaddr, msg_data, tos)
		}
		for pkt_cntr := 0; pkt_cntr < 10; {
			select {
			case msg := <-rcvd_msg:
				timestamp := timestamps.ParseRcvdTimeMsg(msg.Message)
				//this is msg from remote probe
				if timestamp.T2 == 0 {
					reply_data := timestamps.GenerateTimeMsg(&timestamp, msg.Time)
					udp_write_chan <- GenerateUDPMessage(&msg.UDPAddr, reply_data, timestamp.TOS)
					continue
				}
				//this is msg from previous tests, which came after timeout, so we ignore it
				if timestamp.TOS != tos {
					continue
				}
				RTT := timestamps.CalculateLatency(timestamp, msg.Time)
				if report[class_name] == 0 {
					report[class_name] = RTT
				} else {
					report[class_name] = (report[class_name] + RTT) / 2
				}
				pkt_cntr++
			case <-time.After(1 * time.Second):
				_, exist := report[class_name]
				if !exist {
					report[class_name] = 0
				}
				pkt_cntr = 10
			}
		}
	}
	atomic.AddInt32(flag, -1)
	result := TestsReport{report, remote_probe.Location}
	report_chan <- result
}

func GenerateReport(report *TestsReport, cfg_dict *cfg.CfgDict) []byte {
	msg := &rtnm_pb.MSGS{
		Rep: &rtnm_pb.Report{
			//hardcode of classes but shouldnt be an issue
			CS1:        proto.Int64((*report).Report["CS1"]),
			CS2:        proto.Int64((*report).Report["CS2"]),
			CS3:        proto.Int64((*report).Report["CS3"]),
			CS4:        proto.Int64((*report).Report["CS4"]),
			CS5:        proto.Int64((*report).Report["CS5"]),
			CS6:        proto.Int64((*report).Report["CS6"]),
			LocalSite:  proto.String((*cfg_dict).Location),
			RemoteSite: proto.String((*report).PeerSite),
		},
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		panic("we cant generate report")
	}
	data = tlvs.GeneratePBTLV(data)
	return data
}

//TODO: feedback chan. right now could hang forever in case
//of dead master
func send_keepalive(keepalive []byte, write_chan chan []byte,
	keepalive_chan chan int) {
	loop := 1
	KA := 300
	for loop == 1 {
		select {
		case <-time.After(time.Duration(KA) * time.Second):
			write_chan <- keepalive
		case KA = <-keepalive_chan:
		}
	}
}

func DebugOutputProbe(SiteProbes map[string]map[string]ProbeDescrProbe,
	mutex *sync.RWMutex, cfg_dict cfg.CfgDict) {
	var debugAddr net.TCPAddr
	debugAddr.IP = net.ParseIP("127.0.0.1")
	debugAddr.Port = cfg_dict.DebugPortProbe
	debug_socket, err := net.ListenTCP("tcp", &debugAddr)
	if err != nil {
		panic("cant bind to debug socket")
	}
	for {
		debug_conn, _ := debug_socket.AcceptTCP()
		debug_conn.Write([]byte("debug output\n"))
		mutex.RLock()
		for site, value := range SiteProbes {
			debug_conn.Write([]byte(site))
			debug_conn.Write([]byte(":\n"))
			for probe_ip, _ := range value {
				debug_conn.Write([]byte(probe_ip))
				debug_conn.Write([]byte("\n"))
			}
		}
		mutex.RUnlock()
		debug_conn.Close()
	}
}

//Main probe's logic's implementation
func StartProbe(cfg_dict cfg.CfgDict) {
	tcp_msg := make([]byte, 0)
	var probe_context ProbeContext
	SiteProbes := make(map[string]map[string]ProbeDescrProbe)
	SiteMap := make(map[string]int)
	udp_lladr := strings.Join([]string{cfg_dict.Bind_IP.String(),
		strconv.Itoa(cfg_dict.Port)}, ":")
	udpaddr, err := net.ResolveUDPAddr("udp", udp_lladr)
	if err != nil {
		panic("cant resolve udp address")
	}
	udpconn, err := net.ListenUDP("udp", udpaddr)
	if err != nil {
		panic("cant bind udp to local address")
	}
	defer udpconn.Close()
	write_chan := make(chan []byte)
	read_chan := make(chan []byte)
	udp_write_chan := make(chan UDPMessage)
	udp_read_chan := make(chan UDPMessage)
	//not sure that this(buffered chan) is good idea, will test and decide after
	latency_test_chan := make(chan UDPMessage, 100)
	report_chan := make(chan TestsReport)
	feedback_chan_r := make(chan int)
	feedback_chan_w := make(chan int)
	keepalive_chan := make(chan int)
	var mutex sync.RWMutex
	if cfg_dict.DebugPortProbe != 0 {
		go DebugOutputProbe(SiteProbes, &mutex, cfg_dict)
	}
	go netutils.ConnectionMirrorPool(cfg_dict.Masters, read_chan, write_chan,
		feedback_chan_r, feedback_chan_w, GenerateInitialHello(&cfg_dict))
	go ReadFromUDP(udpconn, cfg_dict, udp_read_chan)
	go WriteToUDP(udpconn, cfg_dict, udp_write_chan)
	hello_msg := &rtnm_pb.MSGS{
		Hello: &rtnm_pb.ProbeHello{
			Hello: proto.String("PING"),
		},
	}
	rand.Seed(time.Now().Unix())
	hello_data, _ := proto.Marshal(hello_msg)
	hello_data = tlvs.GeneratePBTLV(hello_data)
	go send_keepalive(hello_data, write_chan, keepalive_chan)
	loop := 1
	test_running := int32(0)
	go PeriodicTests(SiteProbes, SiteMap, udp_write_chan, latency_test_chan,
		&cfg_dict, &test_running, &mutex, report_chan)
	for loop == 1 {
		select {
		case msg_from_master := <-read_chan:
			tcp_msg = append(tcp_msg, msg_from_master...)
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
					continue
				}
				msg := &rtnm_pb.MSGS{}
				err := proto.Unmarshal(tcp_msg[4:TLV.TLV_length], msg)
				if err != nil {
					fmt.Println(err)
					panic("pb unmarshling failed")
				}
				if msg.GetAProbe() != nil {
					NewProbe := ProbeDescrProbe{net.ParseIP(msg.GetAProbe().GetProbeIp()),
						msg.GetAProbe().GetProbeLocation(),
						[]string{msg.GetAProbe().GetMasterIp()}}
					loc := NewProbe.Location
					ip := NewProbe.IP.String()
					mutex.RLock()
					_, site_exist := SiteProbes[loc]
					mutex.RUnlock()
					if !site_exist {
						mutex.Lock()
						SiteProbes[loc] = make(map[string]ProbeDescrProbe)
						SiteProbes[loc][ip] = NewProbe
						if NewProbe.Location != cfg_dict.Location {
							SiteMap[loc] = 1
						}
						mutex.Unlock()
					} else {
						mutex.RLock()
						_, probe_exist := SiteProbes[loc][ip]
						mutex.RUnlock()
						if !probe_exist {
							mutex.Lock()
							SiteProbes[loc][ip] = NewProbe
							mutex.Unlock()
						} else if !utils.StringInSlice(SiteProbes[loc][ip].MasterIp,
							NewProbe.MasterIp[0]) {
							mutex.Lock()
							probe := SiteProbes[loc][ip]
							(&probe).AppendMaster(NewProbe.MasterIp[0])
							SiteProbes[loc][ip] = probe
							mutex.Unlock()
						}
					}
				}
				if msg.GetRConf() != nil {
					probe_context.setKA(msg.GetRConf().GetProbeKA())
					keepalive_chan <- int(probe_context.KA_interval)
				}
				if msg.GetRProbe() != nil {
					//TODO: add logic in case of two masters
					RProbe_IP := msg.GetRProbe().GetProbeIp()
					RProbe_Location := msg.GetRProbe().GetProbeLocation()
					RProbe_MasterIp := msg.GetRProbe().GetMasterIp()
					mutex.RLock()
					probe, exist := SiteProbes[RProbe_Location][RProbe_IP]
					mutex.RUnlock()
					if exist {
						if utils.StringInSlice(probe.MasterIp, RProbe_MasterIp) {
							if len(probe.MasterIp) == 1 {
								mutex.Lock()
								delete(SiteProbes[RProbe_Location], RProbe_IP)
								if len(SiteProbes[RProbe_Location]) == 0 {
									delete(SiteProbes, RProbe_Location)
									delete(SiteMap, RProbe_Location)
								}
								mutex.Unlock()
							} else {
								probe.MasterIp = utils.SliceWOString(probe.MasterIp,
									RProbe_MasterIp)
								mutex.Lock()
								SiteProbes[RProbe_Location][RProbe_IP] = probe
								mutex.Unlock()
							}
						}
					}
				}
				tcp_msg = tcp_msg[TLV.TLV_length:]
			}
		case msg_from_probe := <-udp_read_chan:
			msg := &rtnm_pb.MSGS{}
			err := proto.Unmarshal(msg_from_probe.Message, msg)
			if err != nil {
				fmt.Println("error during unmarshal")
				fmt.Println(err)
				continue
			}
			if msg.GetTStamp() != nil {
				if atomic.CompareAndSwapInt32(&test_running, 1, 1) {
					latency_test_chan <- msg_from_probe
				} else {
					go LatencyTestReply(msg_from_probe, udp_write_chan)
				}
			}
		case <-feedback_chan_r:
			mutex.Lock()
			for key, _ := range SiteProbes {
				delete(SiteProbes, key)
			}
			for key, _ := range SiteMap {
				delete(SiteMap, key)
			}
			mutex.Unlock()
		case report := <-report_chan:
			data := GenerateReport(&report, &cfg_dict)
			write_chan <- data
		}
	}
	return
}
