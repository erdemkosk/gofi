package tcp

import (
	"fmt"
	"log"
	"net"
)

type TcpClient struct {
	Address     net.TCPAddr
	Connection  *net.TCPConn
	IsConnected bool
	Logs        chan string
}

func CreateNewTcpClient(ip string, port int, logs chan string) (*TcpClient, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return nil, err
	}

	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return nil, err
	}

	logs <- "--> TCP CLIENT connected successfully!"

	return &TcpClient{Connection: conn, Address: *tcpAddr, IsConnected: true, Logs: logs}, nil
}

func (client *TcpClient) CloseConnection() {
	err := client.Connection.Close()
	if err != nil {
		log.Fatalln("--> TCP CLIENT cannot be closed!")
	}

	client.IsConnected = false
	client.Logs <- "--> TCP CLIENT closed successfully!"
}

func (client *TcpClient) SendMessage(message string, response chan<- string) {
	client.Logs <- "--> TCP CLIENT Sending message"

	_, err := client.Connection.Write([]byte(message))
	if err != nil {
		client.Logs <- fmt.Sprintf("--> TCP CLIENT Error sending message: %v", err)
		return
	}

	client.Logs <- "--> TCP CLIENT Message sent"

	recvBuff := make([]byte, 1500)
	_, err = client.Connection.Read(recvBuff)
	if err != nil {
		client.Logs <- fmt.Sprintf("--> TCP CLIENT Error receiving message: %v", err)
		return
	}

	client.Logs <- "--> TCP CLIENT Response received; data: " + string(recvBuff)
	response <- string(recvBuff)
}
