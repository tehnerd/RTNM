package roles

import (
	"code.google.com/p/goprotobuf/proto"
	"fmt"
	"math/rand"
	"net"
	"os"
	"rtnm/cfg"
	"rtnm/netutils"
	"rtnm/reporter"
	"rtnm/rtnm_pb"
	"rtnm/rtnm_pubsub"
	"rtnm/timestamps"
	"rtnm/tlvs"
	"strconv"
	"strings"
	"sync"
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

//Receive initial info from probe, like: location, address etc
func ProbeInitialRegister(sock *net.TCPConn, msg_buf []byte,
	cfg_dict cfg.CfgDict,
	Probes map[string]ProbeDescrMaster,
	mutex *sync.RWMutex) (ProbeDescrMaster, error) {
	bytes, err := sock.Read(msg_buf)
	var ProbeDescr ProbeDescrMaster
	if err != nil {
		return ProbeDescr, err
	}
	msg := &rtnm_pb.MSGS{}
	//fastfix. will redo latter
	if len(msg_buf) < 4 {
		panic("not TLVed msg")
	}
	tcp_msg := msg_buf[:bytes]
	for {
		if len(tcp_msg) < 4 {
			break
		}
		var TLV tlvs.TLVHeader
		TLV.Decode(tcp_msg[0:4])
		if len(tcp_msg) < int(TLV.TLV_length) ||
			(TLV.TLV_type != 1 && TLV.TLV_subtype != 1) {
			panic("initial msg tlv len error")
		}
		err = proto.Unmarshal(tcp_msg[4:TLV.TLV_length], msg)
		if err != nil {
			fmt.Println("error during initial unmarshal")
			return ProbeDescr, err
		}
		if msg.GetPReg() != nil {
			mutex.Lock()
			//TODO: checks for err, coz we could deadlock ourself
			ProbeDescr.IP = net.ParseIP(msg.GetPReg().GetProbeIp())
			ProbeDescr.Location = msg.GetPReg().GetProbeLocation()
			ProbeDescr.Preference = 150
			Probes[msg.GetPReg().GetProbeIp()] = ProbeDescr
			mutex.Unlock()
		}
		tcp_msg = tcp_msg[TLV.TLV_length:]
	}
	reg_confirm := &rtnm_pb.MSGS{
		RConf: &rtnm_pb.MasterRegConfirm{
			ProbeKA:   proto.Uint32(cfg_dict.KA_interval),
			TestsList: proto.String(strings.Join(cfg_dict.Tests, " ")),
		},
	}
	data, _ := proto.Marshal(reg_confirm)
	data = tlvs.GeneratePBTLV(data)
	sock.Write(data)
	return ProbeDescr, nil
}

//initial syncing of probe's list to new probe
func ProbeInitialSync(write_chan chan []byte, Probes map[string]ProbeDescrMaster,
	LocalProbe *ProbeDescrMaster, mutex *sync.RWMutex) {
	mutex.RLock()
	defer mutex.RUnlock()
	for ProbeID, ProbeDescr := range Probes {
		if ProbeID != (*LocalProbe).IP.String() {
			msg_pb := &rtnm_pb.MSGS{
				AProbe: &rtnm_pb.AddProbe{
					ProbeIp:       proto.String(ProbeDescr.IP.String()),
					ProbeLocation: proto.String(ProbeDescr.Location),
				},
			}
			msg, _ := proto.Marshal(msg_pb)
			msg = tlvs.GeneratePBTLV(msg)
			write_chan <- msg
		}
	}
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
	feedback_chan_w <- 1
}

//Goroutine which controls the Probe
func ControlProbe(sock *net.TCPConn, cfg_dict cfg.CfgDict,
	Probes map[string]ProbeDescrMaster,
	mutex *sync.RWMutex,
	broker_sub_chan chan rtnm_pubsub.PubSubMeta,
	broker_unsub_chan chan rtnm_pubsub.PubSubMeta,
	broker_pub_chan chan rtnm_pubsub.ProbeInfo,
	external_report_chan chan []byte) {
	msg_buf := make([]byte, 9000)
	tcp_msg := make([]byte, 0)
	defer sock.Close()
	probe_descr, err := ProbeInitialRegister(sock, msg_buf, cfg_dict, Probes, mutex)
	if err != nil {
		return
	}
	var sub_chan rtnm_pubsub.PubSubMeta
	sub_chan.SubscriberID = probe_descr.IP.String()
	sub_chan.Chan = make(chan rtnm_pubsub.ProbeInfo)
	broker_sub_chan <- sub_chan
	broker_pub_chan <- rtnm_pubsub.ProbeInfo{probe_descr.IP, probe_descr.Location, "Add"}
	write_chan := make(chan []byte)
	read_chan := make(chan []byte)
	feedback_chan_r := make(chan int)
	feedback_chan_w := make(chan int)
	go netutils.ReadFromTCP(sock, msg_buf, read_chan, feedback_chan_r)
	go netutils.WriteToTCP(sock, write_chan, feedback_chan_w)
	ProbeInitialSync(write_chan, Probes, &probe_descr, mutex)
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

type ProbeContext struct {
	KA_interval uint32
}

func (PC *ProbeContext) setKA(keepalive uint32) { PC.KA_interval = keepalive }

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
	flag *int, mutex *sync.RWMutex, report_chan chan TestsReport) {
	//TODO: add feedback chan
	for {
		if len(SiteMap) == 0 && len(SiteProbes) != 0 {
			mutex.Lock()
			for key, _ := range SiteProbes {
				SiteMap[key] = 1
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
					*flag = 1
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
	udp_write_chan chan UDPMessage, rcvd_msg chan UDPMessage, flag *int,
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
	*flag = 0
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
	var ladr, masterAddr net.TCPAddr
	msg_buf := make([]byte, 9000)
	tcp_msg := make([]byte, 0)
	udp_msg_buf := make([]byte, 9000)
	var probe_context ProbeContext
	SiteProbes := make(map[string]map[string]ProbeDescrProbe)
	SiteMap := make(map[string]int)
	masterAddr.IP = cfg_dict.Master
	masterAddr.Port = cfg_dict.Port
	ladr.IP = cfg_dict.Bind_IP
	master_conn, _ := net.DialTCP("tcp", &ladr, &masterAddr)
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
	defer master_conn.Close()
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
	go netutils.ReadFromTCP(master_conn, msg_buf, read_chan, feedback_chan_r)
	go netutils.WriteToTCP(master_conn, write_chan, feedback_chan_w)
	go ReadFromUDP(udpconn, cfg_dict, udp_msg_buf, udp_read_chan)
	go WriteToUDP(udpconn, cfg_dict, udp_write_chan)
	hello_msg := &rtnm_pb.MSGS{
		Hello: &rtnm_pb.ProbeHello{
			Hello: proto.String("PING"),
		},
	}
	rand.Seed(time.Now().Unix())
	hello_data, _ := proto.Marshal(hello_msg)
	hello_data = tlvs.GeneratePBTLV(hello_data)
	write_chan <- GenerateInitialHello(&cfg_dict)
	write_chan <- hello_data
	go send_keepalive(hello_data, write_chan, keepalive_chan)
	loop := 1
	test_running := 0
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
						msg.GetAProbe().GetProbeLocation()}
					mutex.RLock()
					_, site_exist := SiteProbes[NewProbe.Location]
					mutex.RUnlock()
					if !site_exist {
						mutex.Lock()
						SiteProbes[NewProbe.Location] = make(map[string]ProbeDescrProbe)
						SiteProbes[NewProbe.Location][NewProbe.IP.String()] = NewProbe
						SiteMap[NewProbe.Location] = 0
						mutex.Unlock()
					} else {
						mutex.RLock()
						_, probe_exist := SiteProbes[NewProbe.Location][NewProbe.IP.String()]
						mutex.RUnlock()
						if !probe_exist {
							mutex.Lock()
							SiteProbes[NewProbe.Location][NewProbe.IP.String()] = NewProbe
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
					mutex.Lock()
					delete(SiteProbes[RProbe_Location], RProbe_IP)
					if len(SiteProbes[RProbe_Location]) == 0 {
						delete(SiteProbes, RProbe_Location)
						delete(SiteMap, RProbe_Location)
					}
					mutex.Unlock()
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
				if test_running != 0 {
					latency_test_chan <- msg_from_probe
				} else {
					go LatencyTestReply(msg_from_probe, udp_write_chan)
				}
			}
		case <-feedback_chan_r:
			feedback_chan_w <- 1
			mutex.Lock()
			for key, _ := range SiteProbes {
				delete(SiteProbes, key)
			}
			for key, _ := range SiteMap {
				delete(SiteMap, key)
			}
			go netutils.ReconnectTCPRW(ladr, masterAddr, msg_buf, write_chan,
				read_chan, feedback_chan_w, feedback_chan_r,
				GenerateInitialHello(&cfg_dict))
		case report := <-report_chan:
			data := GenerateReport(&report, &cfg_dict)
			write_chan <- data
		}
	}
	return
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
