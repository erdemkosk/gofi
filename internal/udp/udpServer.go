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

	logs <- "UDP Server created successfully!"

	return &UdpServer{Connection: conn, Address: *udpAddr, IsConnected: true, Logs: logs}, nil
}

func (server *UdpServer) CloseConnection() {
	err := server.Connection.Close()
	if err != nil {
		log.Fatalln("UDP Server cannot be closed!")
	}

	server.IsConnected = false
	server.Logs <- "UDP Server closed successfully!"
}

func (server *UdpServer) Listen(messages chan<- string) error {

	message := []byte("Hey I am server ı know u client.")

	for {
		server.Logs <- "Ready to receive broadcast packets! (Server)"

		// Receiving a message
		recvBuff := make([]byte, 15000)
		_, rmAddr, err := server.Connection.ReadFromUDP(recvBuff)
		if err != nil {
			return err
		}

		server.Logs <- "Discovery packet received from: " + rmAddr.String()
		server.Logs <- "Packet received; data: " + string(recvBuff)

		// Sending the same message back to current client
		server.Connection.WriteToUDP(message, rmAddr)

		messages <- string(recvBuff)

		server.Logs <- "Sent packet to: " + rmAddr.String()
	}
}
