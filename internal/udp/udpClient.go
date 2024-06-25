package udp

import (
	"fmt"
	"log"
	"net"
)

type UdpClient struct {
	Address     net.UDPAddr
	Connection  *net.UDPConn
	IsConnected bool
}

func CreateNewUdpClient(ip string, port int) (*UdpClient, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}

	log.Println("UDP Client connected successfully")

	return &UdpClient{Connection: conn, Address: *udpAddr, IsConnected: true}, nil
}

func (client *UdpClient) CloseConnection() {
	err := client.Connection.Close()
	if err != nil {
		log.Fatalln("UDP Client cannot be closed!")
	}

	client.IsConnected = false
	log.Println("UDP Client closed successfully!")
}

func (client *UdpClient) SendBroadcastMessage() {

	fmt.Println("C : >>>Ready to send broadcast packets! (Client)")

	message := []byte("Hello server I am client")

	_, err := client.Connection.Write(message)
	fmt.Println("C : >>>Client send message (Client)")

	if err != nil {
		log.Println(err)
	}

	// Receive response from server
	buf := make([]byte, 15000)
	amountByte, remAddr, err := client.Connection.ReadFromUDP(buf)

	if err != nil {
		log.Println(err)
	} else {
		fmt.Println(amountByte, "bytes received from", remAddr)
		fmt.Println("C : >>>Packet received; data: " + string(buf))
	}
}
