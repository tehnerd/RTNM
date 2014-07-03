package roles
import(
    "net"
    "os"
    "fmt"
    "code.google.com/p/goprotobuf/proto"
    "rtnm/cfg"
    "rtnm/rtnm_pb"
    "sync"
    "strings"
)

type ProbeDescrMaster struct {
    IP net.IP
    Location string
    Preference int
}

type ProbeDescrProbe struct {
    IP net.IP
    Location string
    TimeSkew int64
}

func ProbeInitialRegister(sock *net.TCPConn, buffer []byte,
                          cfg_dict cfg.CfgDict,
                          Probes map[string]ProbeDescrMaster,
                          mutex *sync.RWMutex) error{
    bytes,err := sock.Read(buffer)
    if err != nil{
        return err
    }
    msg := &rtnm_pb.MSGS{}
    err = proto.Unmarshal(buffer[:bytes],msg)
    if err != nil{
        fmt.Println("error during unmarshal")
        return err
    }
    var ProbeDescr ProbeDescrMaster
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
            ProbeKA: proto.Uint32(cfg_dict.KA_interval),
            TestsList: proto.String(strings.Join(cfg_dict.Tests," ")),
        },
    }
    data,_ := proto.Marshal(reg_confirm)
    sock.Write(data)
    return nil
}

func ControlProbe(tcp *net.TCPConn,cfg_dict cfg.CfgDict,
                  Probes map[string]ProbeDescrMaster,
                  mutex *sync.RWMutex){
    msg_buf := make([]byte, 9000)
    defer tcp.Close()
    err:=ProbeInitialRegister(tcp, msg_buf, cfg_dict, Probes, mutex)
    fmt.Println(Probes)
    if err != nil {
        return
    }
    for {
        bytes,err := tcp.Read(msg_buf)
        if err != nil {
            return
        }
        fmt.Println(bytes)
    }
}

type ProbeContext struct{
    KA_interval uint32
}
func (PC *ProbeContext)setKA(keepalive uint32){PC.KA_interval = keepalive}

func ProbeInitialHello(sock *net.TCPConn,context *ProbeContext,
                       cfg_dict *cfg.CfgDict, buffer []byte){
    init_hello := &rtnm_pb.MSGS{
                   PReg: &rtnm_pb.ProbeRegister{
                        ProbeIp: proto.String(cfg_dict.Bind_IP.String()),
                        ProbeLocation: proto.String(cfg_dict.Location),
                   },
    }
    data,_ := proto.Marshal(init_hello)
    sock.Write(data)
    bytes,err := sock.Read(buffer)
    if err != nil {
        return
    }
    msg := &rtnm_pb.MSGS{}
    err = proto.Unmarshal(buffer[:bytes],msg)
    if err != nil{
        fmt.Println("error during unmarshal")
        return 
    }
    if msg.GetRConf() != nil{
        context.setKA(msg.GetRConf().GetProbeKA())
    }
}

func StartProbe(cfg_dict cfg.CfgDict){
    var masterAddr net.TCPAddr
    msg_buffer := make([]byte,9000)
    var probe_context ProbeContext
    masterAddr.IP = cfg_dict.Master
    masterAddr.Port = cfg_dict.Port
    master_conn,_ := net.DialTCP("tcp",nil,&masterAddr)
    defer master_conn.Close()
    ProbeInitialHello(master_conn,&probe_context,&cfg_dict, msg_buffer)
    fmt.Println(probe_context)
    return
}

func StartMaster(cfg_dict cfg.CfgDict){
    Probes := make(map[string]ProbeDescrMaster)
    var probes_mutex sync.RWMutex
    var tcpAddr net.TCPAddr
    tcpAddr.IP = cfg_dict.Bind_IP
    tcpAddr.Port = cfg_dict.Port
    master_socket,err := net.ListenTCP("tcp",&tcpAddr)
    if err != nil {
        fmt.Println("cant open tcp socket")
        os.Exit(1)
    }
    for {
        probe_conn,_ := master_socket.AcceptTCP()
        go ControlProbe(probe_conn, cfg_dict, Probes, &probes_mutex)
    }
}
