package tcp

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

type TcpServer struct {
	Address     net.TCPAddr
	Connection  *net.TCPListener
	IsConnected bool
	Logs        chan string
	currentConn *net.TCPConn // Bu yeni bir alan, mevcut bağlantıyı saklamak için
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

func (server *TcpServer) Listen(stop chan bool, connectionEstablished chan<- bool) error {
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

			if connectionEstablished != nil {
				connectionEstablished <- true
			}

			server.currentConn = conn // Mevcut bağlantıyı kaydet

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

func (server *TcpServer) ReceiveFile() {
	fileName, fileSize, err := server.receiveMetaData(server.currentConn)
	if err != nil {
		server.Logs <- fmt.Sprintf("--> TCP SERVER Error receiving meta data: %v", err)
		return
	}

	file, err := os.Create(fileName)
	if err != nil {
		server.Logs <- fmt.Sprintf("--> TCP SERVER Error creating file: %v", err)
		return
	}
	defer file.Close()

	recvBuff := make([]byte, 4096)
	var receivedBytes int64

	for receivedBytes < fileSize {
		n, err := server.currentConn.Read(recvBuff)
		if err != nil {
			if err == io.EOF {
				server.Logs <- "--> TCP SERVER File transfer complete"
				break
			}
			server.Logs <- fmt.Sprintf("--> TCP SERVER Error reading file: %v", err)
			return
		}

		_, err = file.Write(recvBuff[:n])
		if err != nil {
			server.Logs <- fmt.Sprintf("--> TCP SERVER Error writing file: %v", err)
			return
		}

		receivedBytes += int64(n)
	}

	server.Logs <- fmt.Sprintf("--> TCP SERVER File %s received successfully", fileName)
}

func (server *TcpServer) SendFileToClient(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		server.Logs <- fmt.Sprintf("--> TCP SERVER Error opening file: %v", err)
		return err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		server.Logs <- fmt.Sprintf("--> TCP SERVER Error getting file info: %v", err)
		return err
	}

	conn := server.currentConn // Mevcut bağlantıyı al
	if conn == nil {
		return fmt.Errorf("no active connection")
	}

	err = server.sendMetaData(conn, fileInfo.Name(), fileInfo.Size())
	if err != nil {
		server.Logs <- fmt.Sprintf("--> TCP SERVER Error sending meta data: %v", err)
		return err
	}

	sendBuff := make([]byte, 4096)
	for {
		n, err := file.Read(sendBuff)
		if err != nil {
			if err == io.EOF {
				server.Logs <- "--> TCP SERVER File transfer complete"
				break
			}
			server.Logs <- fmt.Sprintf("--> TCP SERVER Error reading file: %v", err)
			return err
		}

		_, err = conn.Write(sendBuff[:n])
		if err != nil {
			server.Logs <- fmt.Sprintf("--> TCP SERVER Error sending file: %v", err)
			return err
		}
	}

	server.Logs <- fmt.Sprintf("--> TCP SERVER File %s sent successfully", filePath)
	return nil
}

func (server *TcpServer) receiveMetaData(conn *net.TCPConn) (string, int64, error) {
	var fileNameLength uint32
	err := binary.Read(conn, binary.LittleEndian, &fileNameLength)
	if err != nil {
		return "", 0, err
	}

	fileNameBuff := make([]byte, fileNameLength)
	_, err = conn.Read(fileNameBuff)
	if err != nil {
		return "", 0, err
	}
	fileName := string(fileNameBuff)

	var fileSize int64
	err = binary.Read(conn, binary.LittleEndian, &fileSize)
	if err != nil {
		return "", 0, err
	}

	return fileName, fileSize, nil
}

func (server *TcpServer) sendMetaData(conn *net.TCPConn, fileName string, fileSize int64) error {
	fileNameLength := uint32(len(fileName))

	err := binary.Write(conn, binary.LittleEndian, fileNameLength)
	if err != nil {
		return err
	}

	_, err = conn.Write([]byte(fileName))
	if err != nil {
		return err
	}

	err = binary.Write(conn, binary.LittleEndian, fileSize)
	if err != nil {
		return err
	}

	return nil
}
