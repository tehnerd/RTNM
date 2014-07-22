package cfg

import (
	"bufio"
	"net"
	"os"
	"strconv"
	"strings"
)

type CfgDict struct {
	CnC            bool
	Master         net.IP
	Port           int
	Peer           net.IP
	Location       string
	Reporter       string
	Bind_IP        net.IP
	KA_interval    uint32
	Tests          []string
	DebugPort      int
	DebugPortProbe int
	Masters        []string
	Peers          []string
	ProbePort      int
}

func ReadConfig() CfgDict {
	fd, err := os.Open(os.Args[1])
	if err != nil {
		os.Exit(1)
	}
	defer fd.Close()
	var cfg_dict CfgDict
	cfg_dict.CnC = false
	var ladr string
	var masters_ip []string
	cfg_reader := bufio.NewReader(fd)
	line, err := cfg_reader.ReadString('\n')
	for err == nil {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			switch fields[0] {
			case "port:":
				port, _ := strconv.Atoi(fields[1])
				cfg_dict.Port = port
			case "probe_port:":
				port, _ := strconv.Atoi(fields[1])
				cfg_dict.ProbePort = port
			case "cnc:":
				if fields[1] == "true" {
					cfg_dict.CnC = true
				}
			case "ka_interval:":
				ka, _ := strconv.Atoi(fields[1])
				cfg_dict.KA_interval = uint32(ka)
			case "tests:":
				for cntr := 1; cntr < len(fields); cntr++ {
					cfg_dict.Tests = append(cfg_dict.Tests, fields[cntr])
				}
			case "bind_ip:":
				ladr = fields[1]
				cfg_dict.Bind_IP = net.ParseIP(fields[1])
			case "location:":
				cfg_dict.Location = fields[1]
			case "master:":
				masters_ip = append(masters_ip, fields[1])
				cfg_dict.Master = net.ParseIP(fields[1])
			case "peer:":
				cfg_dict.Peer = net.ParseIP(fields[1])
			case "reporter:":
				cfg_dict.Reporter = fields[1]
			case "debug_port:":
				port, _ := strconv.Atoi(fields[1])
				cfg_dict.DebugPort = port
			case "debug_port_probe:":
				port, _ := strconv.Atoi(fields[1])
				cfg_dict.DebugPortProbe = port

			default:
			}
		}
		line, err = cfg_reader.ReadString('\n')
	}
	if cfg_dict.ProbePort == 0 {
		ladr = strings.Join([]string{ladr, strconv.Itoa(cfg_dict.Port)}, ":")
	} else {
		ladr = strings.Join([]string{ladr, strconv.Itoa(cfg_dict.ProbePort)}, ":")
	}
	cfg_dict.Masters = append(cfg_dict.Masters, ladr)
	if len(masters_ip) > 0 {
		for cntr := 0; cntr < len(masters_ip); cntr++ {
			madr := strings.Join([]string{masters_ip[cntr],
				strconv.Itoa(cfg_dict.Port)}, ":")
			cfg_dict.Masters = append(cfg_dict.Masters, madr)
		}
	}
	return cfg_dict
}
