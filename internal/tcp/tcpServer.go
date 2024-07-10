package tcp

import (
	"fmt"
	"net"
	"time"
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

func (server *TcpServer) Listen(stop chan bool) error {
	server.Logs <- "--> TCP SERVER Ready to receive connections!"

	for {
		select {
		case <-stop:
			server.Logs <- "--> TCP SERVER Stopping"
			server.CloseConnection()
			return nil
		default:
			server.Connection.SetDeadline(time.Now().Add(5 * time.Second))

			conn, err := server.Connection.AcceptTCP()
			if err != nil {

				netErr, ok := err.(net.Error)
				if ok && netErr.Timeout() {

					continue
				}

				server.Logs <- fmt.Sprintf("--> TCP SERVER Error accepting connection: %v", err)
				continue
			}

			server.Logs <- "--> TCP SERVER Connection accepted from: " + conn.RemoteAddr().String()

			go server.handleConnection(conn)
		}
	}
}

func (server *TcpServer) CloseConnection() {
	server.Logs <- "--> TCP SERVER Closing connection..."

	if server.Connection != nil {
		err := server.Connection.Close()
		if err != nil {
			server.Logs <- fmt.Sprintf("--> TCP SERVER Error closing connection: %v", err)
		}
	}

	server.IsConnected = false
	server.Logs <- "--> TCP SERVER closed successfully!"
}

func (server *TcpServer) handleConnection(conn *net.TCPConn) {
	defer conn.Close()

	recvBuff := make([]byte, 1500)
	_, err := conn.Read(recvBuff)
	if err != nil {
		server.Logs <- fmt.Sprintf("--> TCP SERVER Error reading data: %v", err)
		return
	}

	server.Logs <- "--> TCP SERVER Packet received; data: " + string(recvBuff)

	message := []byte("Response from server")
	_, err = conn.Write(message)
	if err != nil {
		server.Logs <- fmt.Sprintf("--> TCP SERVER Error sending packet: %v", err)
		return
	}

	server.Logs <- "--> TCP SERVER Sent packet to: " + conn.RemoteAddr().String()
}
