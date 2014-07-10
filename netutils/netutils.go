package netutils

import (
	"fmt"
	"math/rand"
	"net"
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

//not yet implemented
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
