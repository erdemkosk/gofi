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
}

func CreateNewUdpServer(ip string, port int) (*UdpServer, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}

	log.Println("UDP Server created successfully!")

	return &UdpServer{Connection: conn, Address: *udpAddr, IsConnected: true}, nil
}

func (server *UdpServer) CloseConnection() {
	err := server.Connection.Close()
	if err != nil {
		log.Fatalln("UDP Server cannot be closed!")
	}

	server.IsConnected = false
	log.Println("UDP Server closed successfully!")
}

func (server *UdpServer) Listen() error {
	message := []byte("Hey I am server ı know u client.")

	for {
		fmt.Println("S : >>>Ready to receive broadcast packets! (Server)")

		// Receiving a message
		recvBuff := make([]byte, 15000)
		_, rmAddr, err := server.Connection.ReadFromUDP(recvBuff)

		if err != nil {
			panic(err)
		}

		fmt.Println("S : >>>Discovery packet received from: " + rmAddr.String())
		fmt.Println("S : >>>Packet received; data: " + string(recvBuff))

		// Sending the same message back to current client
		server.Connection.WriteToUDP(message, rmAddr)

		fmt.Println("S : >>>Sent packet to: " + rmAddr.String())
	}

}
