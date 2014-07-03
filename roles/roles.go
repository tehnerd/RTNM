package roles
import(
    "net"
    "os"
    "fmt"
    "code.google.com/p/goprotobuf/proto"
    "rtnm/cfg"
    "rtnm/rtnm_pb"
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

func ProbeInitialRegister(sock *net.TCPConn, buffer []byte) error{
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
    if msg.GetPReg() != nil {
        fmt.Println(msg.GetPReg().GetProbeIp())
        fmt.Println(msg.GetPReg().GetProbeLocation())
        return nil
    }
    return nil
}

func ControlProbe(tcp *net.TCPConn){
    msg_buf := make([]byte, 9000)
    defer tcp.Close()
    err:=ProbeInitialRegister(tcp,msg_buf)
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

func ProbeInitialHello(sock *net.TCPConn,context *ProbeContext,
                       cfg_dict *cfg.CfgDict){
    init_hello := &rtnm_pb.MSGS{
                   PReg: &rtnm_pb.ProbeRegister{
                        ProbeIp: proto.String(cfg_dict.Bind_IP.String()),
                        ProbeLocation: proto.String(cfg_dict.Location),
                   },
    }
    data,_ := proto.Marshal(init_hello)
    sock.Write(data)
}

func StartProbe(cfg_dict cfg.CfgDict){
    var masterAddr net.TCPAddr
    var probe_context ProbeContext
    masterAddr.IP = cfg_dict.Master
    masterAddr.Port = cfg_dict.Port
    master_conn,_ := net.DialTCP("tcp",nil,&masterAddr)
    defer master_conn.Close()
    ProbeInitialHello(master_conn,&probe_context,&cfg_dict)
    return
}

func StartMaster(cfg_dict cfg.CfgDict){
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
        go ControlProbe(probe_conn)
    }
}
