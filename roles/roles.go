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
	TimeSkew int64
}

type UDPMessage struct {
	UDPAddr net.UDPAddr
	Message []byte
	Time    time.Time
	TOS     int
}

type TimeStamps struct {
	T1 int64
	T2 int64
	T3 int64
	T4 int64
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
		if msg.TOS != 0 {
			fd, _ := udpconn.File()
			syscall.SetsockoptInt(int(fd.Fd()), syscall.IPPROTO_IP, syscall.IP_TOS, msg.TOS)
			fd.Close()
		}
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
	go ReadFromTCP(sock, msg_buf, read_chan, feedback_chan_r)
	go WriteToTCP(sock, write_chan, feedback_chan_w)
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
			fmt.Println(*msg)
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

//Fucntion to calculates timeskew between probes. if it's initial msg(from local to remote)
//then we ignores ReceiveTime variable
func TimeSkewCalculation(remote_probe ProbeDescrProbe, cfg_dict *cfg.CfgDict,
	udp_write_chan chan UDPMessage, TimeStamp *TimeStamps,
	ReceiveTime time.Time) {
	remote_addr := strings.Join([]string{remote_probe.IP.String(),
		strconv.Itoa((*cfg_dict).Port)}, ":")
	udpaddr, err := net.ResolveUDPAddr("udp", remote_addr)
	if err != nil {
		fmt.Println(err)
		panic("cant resolve udp address")
	}
	if (*TimeStamp).T1 == 0 {
		(*TimeStamp).T1 = time.Now().UnixNano()
	} else if (*TimeStamp).T2 == 0 {
		(*TimeStamp).T2 = ReceiveTime.UnixNano()
		(*TimeStamp).T3 = time.Now().UnixNano()
	}
	timestamp_msg := &rtnm_pb.MSGS{
		TStamp: &rtnm_pb.TimeStamps{
			T1: proto.Int64((*TimeStamp).T1),
			T2: proto.Int64((*TimeStamp).T2),
			T3: proto.Int64((*TimeStamp).T3),
		},
	}
	timestamp_data, err := proto.Marshal(timestamp_msg)
	if err != nil {
		panic("cant marshal time pb")
	}
	var msg UDPMessage
	msg.UDPAddr = *udpaddr
	msg.Message = timestamp_data
	msg.TOS = 192 //CS6
	udp_write_chan <- msg
}

//Main probe's logic's implementation
func StartProbe(cfg_dict cfg.CfgDict) {
	var masterAddr net.TCPAddr
	msg_buf := make([]byte, 9000)
	udp_msg_buf := make([]byte, 9000)
	var probe_context ProbeContext
	Probes := make(map[string]ProbeDescrProbe)
	masterAddr.IP = cfg_dict.Master
	masterAddr.Port = cfg_dict.Port
	master_conn, _ := net.DialTCP("tcp", nil, &masterAddr)
	udp_lladr := strings.Join([]string{cfg_dict.Bind_IP.String(),
		strconv.Itoa(cfg_dict.Port)}, ":")
	udpaddr, err := net.ResolveUDPAddr("udp", udp_lladr)
	if err != nil {
		fmt.Println(err)
		panic("cant resolve udp address")
	}
	udpconn, err := net.ListenUDP("udp", udpaddr)
	if err != nil {
		panic("cant bind udp to local address")
	}
	defer master_conn.Close()
	defer udpconn.Close()
	ProbeInitialHello(master_conn, &probe_context, &cfg_dict, msg_buf)
	fmt.Println(probe_context)
	write_chan := make(chan []byte)
	read_chan := make(chan []byte)
	udp_write_chan := make(chan UDPMessage)
	udp_read_chan := make(chan UDPMessage)
	feedback_chan_r := make(chan int)
	feedback_chan_w := make(chan int)
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
	loop := 1
	fmt.Println(Probes)
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
					msg.GetAProbe().GetProbeLocation(), 0}
				_, exist := Probes[NewProbe.IP.String()]
				if !exist {
					Probes[NewProbe.IP.String()] = NewProbe
					var TimeStamp TimeStamps
					go TimeSkewCalculation(NewProbe, &cfg_dict, udp_write_chan,
						&TimeStamp, time.Now())
				}
			}
			if msg.GetRProbe() != nil {
				//TODO: add logic in case of two masters
				delete(Probes, msg.GetRProbe().GetProbeIp())
			}
		case msg_from_probe := <-udp_read_chan:
			fmt.Println(msg_from_probe)
		case <-time.After(time.Duration(probe_context.KA_interval+
			(probe_context.KA_interval/10*(rand.Uint32()%3))) * time.Second):
			write_chan <- hello_data
		case <-feedback_chan_r:
			feedback_chan_w <- 1
			loop = 0
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
