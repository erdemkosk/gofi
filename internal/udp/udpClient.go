package udp

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/erdemkosk/gofi/internal"
	"github.com/erdemkosk/gofi/internal/logic"
)

type UdpClient struct {
	Address     net.UDPAddr
	Connection  *net.UDPConn
	IsConnected bool
	Logs        chan string
}

type UdpMessage struct {
	IP   string
	Port int32
	Name string
}

func CreateNewUdpClient(ip string, port int, logs chan string) (*UdpClient, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}

	logs <- "UDP Client connected successfully"

	return &UdpClient{Connection: conn, Address: *udpAddr, IsConnected: true, Logs: logs}, nil
}

func (client *UdpClient) CloseConnection() {
	err := client.Connection.Close()
	if err != nil {
		log.Fatalln("UDP Client cannot be closed!")
	}

	client.IsConnected = false
	log.Println("UDP Client closed successfully!")
}

func (client *UdpClient) SendBroadcastMessage(stop chan bool) {
	client.Logs <- "C : >>>Ready to send broadcast packets! (Client)"

	message := UdpMessage{IP: logic.GetLocalIP(), Port: internal.TCP_PORT, Name: logic.GetHostName()}
	messageBytes, err := json.Marshal(message)
	if err != nil {
		client.Logs <- fmt.Sprintf("Error marshaling message: %v", err)
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_, err := client.Connection.Write(messageBytes)
			client.Logs <- "C : >>>Client sent message (Client)"

			if err != nil {
				log.Println(err)
			}

			// Receive response from server
			buf := make([]byte, 15000)
			amountByte, remAddr, err := client.Connection.ReadFromUDP(buf)

			if err != nil {
				log.Println(err)
			} else {
				client.Logs <- fmt.Sprintf("%d bytes received from %s", amountByte, remAddr.String())
				client.Logs <- ("C : >>>Packet received; data: " + string(buf))
			}

		case <-stop:
			client.Logs <- "C : >>>Stopping broadcast"
			return
		}
	}
}
