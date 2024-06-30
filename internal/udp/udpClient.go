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
	if client.Connection != nil {
		err := client.Connection.Close()
		if err != nil {
			log.Println("UDP Client cannot be closed:", err)
		}
		client.IsConnected = false
		log.Println("UDP Client closed successfully!")
	}
}

func (client *UdpClient) SendBroadcastMessage(stop chan bool, logs chan<- string) {
	logs <- "C : >>>Ready to send broadcast packets! (Client)"

	message := UdpMessage{IP: logic.GetLocalIP(), Port: internal.TCP_PORT, Name: logic.GetHostName()}
	messageBytes, err := json.Marshal(message)
	if err != nil {
		logs <- fmt.Sprintf("Error marshaling message: %v", err)
		return
	}

	ticker := time.NewTicker(5 * time.Second)

	for {
		select {
		case <-ticker.C:
			_, err := client.Connection.Write(messageBytes)
			logs <- "C : >>>Client sent message (Client)"

			if err != nil {
				logs <- fmt.Sprintf("Error sending message: %v", err)
			}

			// Receive response from server
			buf := make([]byte, 15000)
			client.Connection.SetReadDeadline(time.Now().Add(1 * time.Second)) // Set a read timeout
			amountByte, remAddr, err := client.Connection.ReadFromUDP(buf)

			if err != nil {
				netErr, ok := err.(net.Error)
				if !ok || !netErr.Timeout() {
					logs <- fmt.Sprintf("Error receiving response: %v", err)
				}
			} else {
				logs <- fmt.Sprintf("%d bytes received from %s", amountByte, remAddr.String())
				logs <- ("C : >>>Packet received; data: " + string(buf))
			}

		case <-stop:
			logs <- "C : >>>Stopping broadcast"
			ticker.Stop()
			//client.CloseConnection()
			return
		}
	}
}
