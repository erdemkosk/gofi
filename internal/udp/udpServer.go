package udp

import (
	"fmt"
	"log"
	"net"
)

type UdpServer struct {
	Address    net.UDPAddr
	Connection *net.UDPConn
}

func CreateNewUdpServer(ip string, port int) (*UdpServer, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", ip, port))

	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", udpAddr)

	log.Println("Udp Server created successfuly")

	return &UdpServer{Connection: conn, Address: *udpAddr}, err
}
