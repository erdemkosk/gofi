package udp

import (
	"fmt"
	"log"
	"net"
)

type UdpClient struct {
	Address    net.UDPAddr
	Connection *net.UDPConn
}

func CreateNewUdpClient(ip string, port int) (*UdpClient, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", ip, port))

	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)

	log.Println("Udp Client connected successfuly")

	return &UdpClient{Connection: conn, Address: *udpAddr}, err
}
