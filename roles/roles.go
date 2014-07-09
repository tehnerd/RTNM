package roles

import (
	"code.google.com/p/goprotobuf/proto"
	"fmt"
	"math/rand"
	"net"
	"os"
	"rtnm/cfg"
	"rtnm/rtnm_pb"
	"rtnm/rtnm_pubsub"
	"rtnm/timestamps"
    "rtnm/reporter"
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

//Receive msg from tcp socket and send it as a []byte to read_chan
func ReadFromTCP(sock *net.TCPConn, msg_buf []byte, read_chan chan []byte,
	feedback_chan chan int) {
	loop := 1
	for loop == 1 {
		bytes, err := sock.Read(msg_buf)
		if err != nil {
			feedback_chan <- 1
			loop = 0
			continue
		}
		read_chan <- msg_buf[:bytes]
	}
	fmt.Println("exiting read")
}

//Receive msg from write_chan and send it to tcp socket
func WriteToTCP(sock *net.TCPConn, write_chan chan []byte,
	feedback_chan chan int) {
	loop := 1
	for loop == 1 {
		select {
		case msg := <-write_chan:
			_, err := sock.Write(msg)
			if err != nil {
				feedback_chan <- 1
				continue
			}
		case <-feedback_chan:
			loop = 0
		}
	}
	fmt.Println("exiting write")
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
	err = proto.Unmarshal(msg_buf[:bytes], msg)
	if err != nil {
		fmt.Println("error during unmarshal")
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
	reg_confirm := &rtnm_pb.MSGS{
		RConf: &rtnm_pb.MasterRegConfirm{
			ProbeKA:   proto.Uint32(cfg_dict.KA_interval),
			TestsList: proto.String(strings.Join(cfg_dict.Tests, " ")),
		},
	}
	data, _ := proto.Marshal(reg_confirm)
	sock.Write(data)
	//hack to handle tcp offloading
	sock.Read(msg_buf)
	return ProbeDescr, nil
}

//initial syncing of probe's list to new probe
func ProbeInitialSync(write_chan chan []byte, Probes map[string]ProbeDescrMaster,
	LocalProbe *ProbeDescrMaster, mutex *sync.RWMutex) {
	mutex.RLock()
	defer mutex.RUnlock()
	for ProbeID, ProbeDescr := range Probes {
		if ProbeID != (*LocalProbe).IP.String() {
			fmt.Println(ProbeDescr)
			msg_pb := &rtnm_pb.MSGS{
				AProbe: &rtnm_pb.AddProbe{
					ProbeIp:       proto.String(ProbeDescr.IP.String()),
					ProbeLocation: proto.String(ProbeDescr.Location),
				},
			}
			msg, _ := proto.Marshal(msg_pb)
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
	broker_pub_chan chan rtnm_pubsub.ProbeInfo) {
	sock.SetNoDelay(true)
	msg_buf := make([]byte, 9000)
	defer sock.Close()
	probe_descr, err := ProbeInitialRegister(sock, msg_buf, cfg_dict, Probes, mutex)
	fmt.Println(Probes)
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
    external_report_chan := make(chan []byte, 100)
	go ReadFromTCP(sock, msg_buf, read_chan, feedback_chan_r)
	go WriteToTCP(sock, write_chan, feedback_chan_w)
    go reporter.CollectReport(external_report_chan)
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
			msg := &rtnm_pb.MSGS{}
			err := proto.Unmarshal(msg_from_probe, msg)
			if err != nil {
				panic("error during unmarshaling protobuf")
			}
			if msg.GetHello() != nil {
				fmt.Println(msg)
				continue
			}
			if msg.GetRep() != nil {
                external_report_chan <- msg_from_probe
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
				write_chan <- msg
			} else {
				msg_pb := &rtnm_pb.MSGS{
					RProbe: &rtnm_pb.RemoveProbe{
						ProbeIp: proto.String(probe_info.IP.String()),
					},
				}
				msg, _ := proto.Marshal(msg_pb)
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

//Inital hello/registration msg to Master, sends Probe location etc
func ProbeInitialHello(sock *net.TCPConn, context *ProbeContext,
	cfg_dict *cfg.CfgDict, msg_buf []byte) {
	init_hello := &rtnm_pb.MSGS{
		PReg: &rtnm_pb.ProbeRegister{
			ProbeIp:       proto.String(cfg_dict.Bind_IP.String()),
			ProbeLocation: proto.String(cfg_dict.Location),
		},
	}
	data, _ := proto.Marshal(init_hello)
	sock.Write(data)
	bytes, err := sock.Read(msg_buf)
	if err != nil {
		return
	}
	msg := &rtnm_pb.MSGS{}
	err = proto.Unmarshal(msg_buf[:bytes], msg)
	if err != nil {
		fmt.Println("error during unmarshal")
		return
	}
	if msg.GetRConf() != nil {
		context.setKA(msg.GetRConf().GetProbeKA())
	}
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

func PeriodicTests(Probes map[string]ProbeDescrProbe, udp_write_chan chan UDPMessage,
	latency_test_chan chan UDPMessage, cfg_dict *cfg.CfgDict,
	flag *int, mutex *sync.RWMutex, report_chan chan TestsReport) {
	//TODO: add feedback chan
	for {
		time.Sleep(time.Duration(rand.Int31n(15)+10) * time.Second)
		if len(Probes) != 0 {
			//TODO: more sophisticated way to select remote probes
			//this one is for tests only
			probe_n := rand.Int31n(int32(len(Probes)))
			cntr := 0
			for _, value := range Probes {
				if int32(cntr) == probe_n {
					mutex.RUnlock()
					*flag = 1
					LatencyTest(value, cfg_dict, udp_write_chan,
						latency_test_chan, flag, report_chan)
					break
				}
				cntr++
			}
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
	return data
}

//TODO: feedback chan. right now could hang forever in case
//of dead master
func send_keepalive(keepalive []byte, write_chan chan []byte,
	ka_interval uint32) {
	loop := 1
	for loop == 1 {
		time.Sleep(time.Duration(ka_interval) * time.Second)
		write_chan <- keepalive
	}
}

//Main probe's logic's implementation
func StartProbe(cfg_dict cfg.CfgDict) {
	var lladr, masterAddr net.TCPAddr
	msg_buf := make([]byte, 9000)
	udp_msg_buf := make([]byte, 9000)
	var probe_context ProbeContext
	Probes := make(map[string]ProbeDescrProbe)
	masterAddr.IP = cfg_dict.Master
	masterAddr.Port = cfg_dict.Port
	lladr.IP = cfg_dict.Bind_IP
	master_conn, _ := net.DialTCP("tcp", &lladr, &masterAddr)
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
	ProbeInitialHello(master_conn, &probe_context, &cfg_dict, msg_buf)
	write_chan := make(chan []byte)
	read_chan := make(chan []byte)
	udp_write_chan := make(chan UDPMessage)
	udp_read_chan := make(chan UDPMessage)
	//not sure that this(buffered chan) is good idea, will test and decide after
	latency_test_chan := make(chan UDPMessage, 100)
	report_chan := make(chan TestsReport)
	feedback_chan_r := make(chan int)
	feedback_chan_w := make(chan int)
	var mutex sync.RWMutex
	go ReadFromTCP(master_conn, msg_buf, read_chan, feedback_chan_r)
	go WriteToTCP(master_conn, write_chan, feedback_chan_w)
	go ReadFromUDP(udpconn, cfg_dict, udp_msg_buf, udp_read_chan)
	go WriteToUDP(udpconn, cfg_dict, udp_write_chan)
	hello_msg := &rtnm_pb.MSGS{
		Hello: &rtnm_pb.ProbeHello{
			Hello: proto.String("PING"),
		},
	}
	rand.Seed(time.Now().Unix())
	hello_data, _ := proto.Marshal(hello_msg)
	write_chan <- hello_data
	go send_keepalive(hello_data, write_chan, probe_context.KA_interval)
	loop := 1
	test_running := 0
	go PeriodicTests(Probes, udp_write_chan, latency_test_chan, &cfg_dict,
		&test_running, &mutex, report_chan)
	for loop == 1 {
		select {
		case msg_from_master := <-read_chan:
			msg := &rtnm_pb.MSGS{}
			err := proto.Unmarshal(msg_from_master, msg)
			if err != nil {
				fmt.Println("error during unmarshal")
				return
			}
			fmt.Println(msg)
			if msg.GetAProbe() != nil {
				NewProbe := ProbeDescrProbe{net.ParseIP(msg.GetAProbe().GetProbeIp()),
					msg.GetAProbe().GetProbeLocation()}
				mutex.RLock()
				_, exist := Probes[NewProbe.IP.String()]
				mutex.RUnlock()
				if !exist {
					mutex.Lock()
					Probes[NewProbe.IP.String()] = NewProbe
					mutex.Unlock()
				}
			}
			if msg.GetRProbe() != nil {
				//TODO: add logic in case of two masters
				mutex.Lock()
				delete(Probes, msg.GetRProbe().GetProbeIp())
				mutex.Unlock()
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
			loop = 0
		case report := <-report_chan:
			data := GenerateReport(&report, &cfg_dict)
			write_chan <- data
		}
	}
	return
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
	go rtnm_pubsub.StartBroker(sub_chan, unsub_chan, pub_chan)
	master_socket, err := net.ListenTCP("tcp", &tcpAddr)
	if err != nil {
		fmt.Println("cant open tcp socket")
		os.Exit(1)
	}
	for {
		probe_conn, _ := master_socket.AcceptTCP()
		go ControlProbe(probe_conn, cfg_dict, Probes, &probes_mutex,
			sub_chan, unsub_chan, pub_chan)
	}
}
