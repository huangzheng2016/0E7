package udpcast

import (
	"fmt"
	"net"
	"strings"
	"time"
)

func Udp_sent(server_port string) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("UDPCAST ERROR:", err)
			go Udp_sent(server_port)
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
		message := []byte("0E7" + server_port)
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

	timeout := time.Minute
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
		return "http://" + addr.IP.String() + ":" + message[3:]
	} else {
		return ""
	}
}
