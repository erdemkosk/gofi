package tcp

import (
	"fmt"
	"log"
	"net"
)

type TcpServer struct {
	Address     net.TCPAddr
	Connection  *net.TCPListener
	IsConnected bool
	Logs        chan string
}

func CreateNewTcpServer(ip string, port int, logs chan string) (*TcpServer, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, err
	}

	logs <- "--> TCP SERVER created successfully!"

	return &TcpServer{Connection: conn, Address: *tcpAddr, IsConnected: true, Logs: logs}, nil
}

func (server *TcpServer) CloseConnection() {
	err := server.Connection.Close()
	if err != nil {
		log.Fatalln("--> TCP SERVER cannot be closed!")
	}

	server.IsConnected = false
	server.Logs <- "--> TCP SERVER closed successfully!"
}

func (server *TcpServer) Listen(stop chan bool) error {
	server.Logs <- "--> TCP SERVER Ready to receive connections!"

	messages := make(chan string)

	for {
		conn, err := server.Connection.AcceptTCP()
		if err != nil {
			server.Logs <- fmt.Sprintf("--> TCP SERVER Error accepting connection: %v", err)
			return err
		}

		server.Logs <- "--> TCP SERVER Connection accepted from: " + conn.RemoteAddr().String()

		go server.handleConnection(conn, messages)

		select {
		case <-stop:
			server.Logs <- "--> TCP SERVER Stopping"
			server.CloseConnection()
			return nil
		default:
		}
	}
}

func (server *TcpServer) handleConnection(conn *net.TCPConn, messages chan<- string) {
	defer conn.Close()

	recvBuff := make([]byte, 1500)
	_, err := conn.Read(recvBuff)
	if err != nil {
		server.Logs <- fmt.Sprintf("--> TCP SERVER Error reading data: %v", err)
		return
	}

	server.Logs <- "--> TCP SERVER Packet received; data: " + string(recvBuff)

	message := []byte("Gofi")
	_, err = conn.Write(message)
	if err != nil {
		server.Logs <- fmt.Sprintf("--> TCP SERVER Error sending packet: %v", err)
		return
	}

	messages <- string(recvBuff)
	server.Logs <- "--> TCP SERVER Sent packet to: " + conn.RemoteAddr().String()
}
