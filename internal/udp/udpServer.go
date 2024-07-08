package udp

import (
	"fmt"
	"log"
	"net"
)

type UdpServer struct {
	Address     net.UDPAddr
	Connection  *net.UDPConn
	IsConnected bool
	Logs        chan string
}

func CreateNewUdpServer(ip string, port int, logs chan string) (*UdpServer, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}

	logs <- "--> UDP SERVER created successfully!"

	return &UdpServer{Connection: conn, Address: *udpAddr, IsConnected: true, Logs: logs}, nil
}

func (server *UdpServer) CloseConnection() {
	err := server.Connection.Close()
	if err != nil {
		log.Fatalln("--> UDP SERVER cannot be closed!")
	}

	server.IsConnected = false
	server.Logs <- "--> UDP SERVER closed successfully!"
}

func (server *UdpServer) Listen(stop chan bool, messages chan<- string) error {
	message := []byte("Hey I am server ı know u client.")

	for {
		server.Logs <- "--> UDP SERVER Ready to receive broadcast packets!"

		recvBuff := make([]byte, 15000)
		_, rmAddr, err := server.Connection.ReadFromUDP(recvBuff)
		if err != nil {
			return err
		}

		server.Logs <- "--> UDP SERVER Discovery packet received from: " + rmAddr.String()
		server.Logs <- "--> UDP SERVER Packet received; data: " + string(recvBuff)

		// Sending the same message back to current client
		server.Connection.WriteToUDP(message, rmAddr)

		messages <- string(recvBuff)

		server.Logs <- "--> UDP SERVER Sent packet to: " + rmAddr.String()

		select {
		case <-stop:
			server.Logs <- "--> UDP SERVER Stopping"
			server.CloseConnection()
			return nil
		}
	}
}
