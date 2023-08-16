package udpcast

import (
	"fmt"
	"net"
	"strings"
	"time"
)

func Udp_sent(server_tls bool, server_port string) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("UDPCAST ERROR:", err)
			go Udp_sent(server_tls, server_port)
		}
	}()
	for true {
		broadcastIP := net.IPv4(255, 255, 255, 255)
		port := 6102
		conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
			IP:   broadcastIP,
			Port: port,
		})
		if err != nil {
			fmt.Printf("UDPCAST ERROR: %s\n", err.Error())
			return
		}
		defer conn.Close()

		var message []byte
		if server_tls {
			message = []byte("0E7" + "s" + server_port)
		} else {
			message = []byte("0E7" + "n" + server_port)
		}
		_, err = conn.Write(message)
		if err != nil {
			fmt.Printf("UDPCAST ERROR: %s\n", err.Error())
			return
		}
		//fmt.Println("UDPCAST SENT")
		time.Sleep(time.Second)
	}
}
func Udp_receive() string {
	fmt.Println("SERVER NOT FOUND,WAIT FOR UDP CAST")
	port := 6102
	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		Port: port,
	})
	if err != nil {
		fmt.Printf("UDPCAST ERROR: %s\n", err.Error())
		return ""
	}
	defer conn.Close()

	timeout := 120 * time.Second
	conn.SetDeadline(time.Now().Add(timeout))

	buffer := make([]byte, 1024)
	n, addr, err := conn.ReadFromUDP(buffer)
	if err != nil {
		fmt.Printf("UDPCAST ERROR: %s\n", err.Error())
		return ""
	}
	message := string(buffer[:n])
	if strings.HasPrefix(message, "0E7") {
		fmt.Println("SERVER IP REVEIVED")
		if message[3] == 's' {
			return "https://" + addr.IP.String() + ":" + message[4:]
		} else {
			return "http://" + addr.IP.String() + ":" + message[4:]
		}
	} else {
		return ""
	}
}
