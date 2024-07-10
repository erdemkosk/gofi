package udp

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	config "github.com/erdemkosk/gofi/internal"
	"github.com/erdemkosk/gofi/internal/logic"
)

type UdpClient struct {
	Address     net.UDPAddr
	Connection  *net.UDPConn
	IsConnected bool
	Logs        chan string
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

	logs <- "--> UDP CLIENT connected successfully"

	return &UdpClient{Connection: conn, Address: *udpAddr, IsConnected: true, Logs: logs}, nil
}

func (client *UdpClient) CloseConnection() {
	err := client.Connection.Close()
	if err != nil {
		log.Fatalln("--> UDP Client cannot be closed!")
	}

	client.IsConnected = false
	client.Logs <- "--> UDP CLIENT closed successfully!"
}

func (client *UdpClient) SendBroadcastMessage(stop chan bool) {
	client.Logs <- "--> UDP CLIENT ready to send broadcast packets!"

	message := UdpMessage{IP: logic.GetLocalIP(), Port: config.TCP_PORT, Name: logic.GetHostName()}
	messageBytes, err := json.Marshal(message)
	if err != nil {
		client.Logs <- fmt.Sprintf("Error marshaling message: %v", err)
		return
	}

	ticker := time.NewTicker(logic.GenerateRandomTime())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_, err := client.Connection.Write(messageBytes)
			client.Logs <- "--> UDP CLIENT sended broadcast message to everyone who is interested"

			if err != nil {
				client.Logs <- fmt.Sprintf("--> UDP CLIENT Error sending message: %v", err)
			}

			// Receive response from server
			buf := make([]byte, 15000)
			client.Connection.SetReadDeadline(time.Now().Add(5 * time.Second)) // Set a read timeout
			amountByte, remAddr, err := client.Connection.ReadFromUDP(buf)

			if err != nil {
				netErr, ok := err.(net.Error)
				if !ok || !netErr.Timeout() {
					client.Logs <- fmt.Sprintf("--> UDP CLIENT Error receiving response: %v", err)
				}
			} else {
				client.Logs <- fmt.Sprintf("%d bytes received from %s", amountByte, remAddr.String())
				client.Logs <- ("--> UDP CLIENT Packet received; data: " + string(buf))
			}

		case <-stop:
			client.Logs <- "--> UDP CLIENT Stopping"
			ticker.Stop()
			client.CloseConnection()
			return
		}
	}
}
