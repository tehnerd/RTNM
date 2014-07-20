package netutils

import (
	"fmt"
	"math/rand"
	"net"
	"rtnm/tlvs"
	"sync/atomic"
	"time"
)

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

/* this is tlv aware instance of the ReadFromTCP for the multinode master deployments.
we do need such awarnes in case
where lots of readers sends msgs to the same chan []byte to protect ourselfs against
the situation where chunk of msgs could be blended inside the channel
*/
func MMReadTLVFromTCP(sock *net.TCPConn, read_chan chan []byte,
	feedback_from_socket chan int, mnum int) {
	msg_buf := make([]byte, 65535)
	tcp_msg := make([]byte, 0)
	var tlv_header tlvs.TLVHeader
	loop := 1
	for loop == 1 {
		bytes, err := sock.Read(msg_buf)
		if err != nil {
			feedback_from_socket <- mnum
			loop = 0
			continue
		}
		tcp_msg = append(tcp_msg, msg_buf[:bytes]...)
		for {
			if len(tcp_msg) < 4 {
				break
			}
			//TODO: add return err in tlvs Decode method
			tlv_header.Decode(tcp_msg[0:4])
			if len(tcp_msg) < int(tlv_header.TLV_length) {
				break
			}
			read_chan <- tcp_msg[:tlv_header.TLV_length]
			tcp_msg = tcp_msg[tlv_header.TLV_length:]
		}
	}
	fmt.Println("exiting read")
}

/*Receive msg from write_chan and send it to tcp socket
Version for the multimaster deploy*/
func MMWriteToTCP(sock *net.TCPConn, write_chan chan []byte,
	feedback_from_socket, feedback_to_socket chan int, mnum int) {
	loop := 1
	for loop == 1 {
		select {
		case msg := <-write_chan:
			_, err := sock.Write(msg)
			if err != nil {
				select {
				case feedback_from_socket <- mnum:
					continue
				case loop = <-feedback_to_socket:
					loop = 0
				}
			}
		case <-feedback_to_socket:
			loop = 0
		}
	}
	fmt.Println("exiting write")
}

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

//reconnecting to remote host for both read and write purpose
func ReconnectTCPRW(ladr, radr net.TCPAddr, msg_buf []byte, write_chan chan []byte,
	read_chan chan []byte, feedback_chan_w, feedback_chan_r chan int, init_msg []byte) {
	loop := 1
	for loop == 1 {
		time.Sleep(time.Duration(20+rand.Intn(15)) * time.Second)
		sock, err := net.DialTCP("tcp", &ladr, &radr)
		if err != nil {
			continue
		}
		//testing health of the new socket. GO sometimes doesnt rise the error when
		// we receive RST from remote side
		_, err = sock.Write(init_msg)
		if err != nil {
			fmt.Println("dead socket")
			sock.Close()
			continue
		}
		loop = 0
		go ReadFromTCP(sock, msg_buf, read_chan, feedback_chan_r)
		go WriteToTCP(sock, write_chan, feedback_chan_w)
	}
	fmt.Println("reconnected to remote host")
}

//reconnecting to remote host for both read and write purpose in multimaster env
func MMReconnectTCPRW(ladr, radr *net.TCPAddr, write_chan chan []byte,
	read_chan chan []byte, feedback_from_socket, feedback_to_socket chan int,
	mnum int, init_msg []byte, mcntr *int32) {
	loop := 1
	for loop == 1 {
		//we do use same src port.w/o tw_reuse reuse of the socket is ~60sec
		time.Sleep(time.Duration(20+rand.Intn(15)) * time.Second)
		sock, err := net.DialTCP("tcp", ladr, radr)
		if err != nil {
			continue
		}
		//testing health of the new socket. GO sometimes doesnt rise the error when
		// we receive RST from remote side
		_, err = sock.Write(init_msg)
		if err != nil {
			fmt.Println("dead socket")
			sock.Close()
			continue
		}
		loop = 2
		atomic.AddInt32(mcntr, 1)
		go MMReadTLVFromTCP(sock, read_chan, feedback_from_socket, mnum)
		go MMWriteToTCP(sock, write_chan, feedback_from_socket, feedback_to_socket, mnum)
	}
	fmt.Println("reconnected to remote host")
}

//reconnecting to remote host for write only
func ReconnectTCPW(radr net.TCPAddr, write_chan chan []byte, feedback_chan chan int) {
	loop := 1
	for loop == 1 {
		time.Sleep(time.Duration(20+rand.Intn(15)) * time.Second)
		sock, err := net.DialTCP("tcp", nil, &radr)
		if err != nil {
			continue
		}
		//testing health of the new socket. GO sometimes doesnt rise the error when
		// we receive RST from remote side
		_, err = sock.Write([]byte{1})
		if err != nil {
			fmt.Println("dead socket")
			sock.Close()
			continue
		}
		loop = 0
		go WriteToTCP(sock, write_chan, feedback_chan)
	}
	fmt.Println("reporter reconnected")
}

//connecting to multiple remote sites and sent exactly the same msg to all of em
func ConnectionMirrorPool(addresses []string, read_chan chan []byte,
	write_chan chan []byte, feedback_chan1, feedback_chan2 chan int,
	init_msg []byte) {
	if len(addresses) < 2 {
		panic("we need at least ladr and one remote addr")
	}
	var ladr *net.TCPAddr
	var err error
	remote_sites := len(addresses) - 1
	radrs := make([](*net.TCPAddr), remote_sites)
	//actually this is probably not needed at all;coz we already have read_chan
	//but mb add something to it in the future (or remove at all)
	read_chan_ := make(chan []byte)
	write_chans := make([]chan []byte, remote_sites)
	feedback_from_socket := make(chan int)
	feedback_to_sockets := make([]chan int, remote_sites)
	//addresses must be in "ip:port" format"
	if len(addresses[0]) > 0 {
		ladr, err = net.ResolveTCPAddr("tcp", addresses[0])
		if err != nil {
			panic("error in ladr definition")
		}
	} else {
		ladr = nil
	}
	//counter of masters. if == 0 then all masters are dead
	//at the beggining all of em are dead indeed
	mcntr := int32(0)
	for cntr := 0; cntr < remote_sites; cntr++ {
		radrs[cntr], err = net.ResolveTCPAddr("tcp", addresses[1+cntr])
		if err != nil {
			panic("error in remote addr definition")
		}
		write_chans[cntr] = make(chan []byte, 10)
		feedback_to_sockets[cntr] = make(chan int)
		go MMReconnectTCPRW(ladr, radrs[cntr], write_chans[cntr],
			read_chan_, feedback_from_socket, feedback_to_sockets[cntr],
			cntr, init_msg, &mcntr)

	}
	loop := 1
	//STATE: POC, that we could abstract multiple remote master sites in such consturction
	for loop == 1 {
		select {
		case msg_to_write := <-write_chan:
			//TODO: test it. len should protect us against dead master.
			// mb change hardcoded 10?
			if !atomic.CompareAndSwapInt32(&mcntr, 0, 0) {
				for cntr := 0; cntr < remote_sites; cntr++ {
					if len(write_chans[cntr]) > 8 {
						continue
					}
					write_chans[cntr] <- msg_to_write
				}
			}
		case msg_to_read := <-read_chan_:
			read_chan <- msg_to_read
		case feedback := <-feedback_from_socket:
			feedback_to_sockets[feedback] <- feedback
			go MMReconnectTCPRW(ladr, radrs[feedback], write_chans[feedback],
				read_chan_, feedback_from_socket, feedback_to_sockets[feedback],
				feedback, init_msg, &mcntr)
			atomic.AddInt32(&mcntr, -1)
			if atomic.CompareAndSwapInt32(&mcntr, 0, 0) {
				feedback_chan1 <- 1
			}
		}
	}
}
