package tcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/erdemkosk/gofi/internal/logic"
)

type TcpServer struct {
	Address           net.TCPAddr
	Connection        *net.TCPListener
	IsConnected       bool
	Logs              chan string
	currentConnection *net.TCPConn
}

type FileMetadata struct {
	FileName string
	FileType string
	FileSize int64
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

			server.currentConnection = conn

			go server.handleConnection()
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

func (server *TcpServer) handleConnection() {

	// Metadata boyutunu okuma
	metaDataSizeBuff := make([]byte, 16)
	_, err := server.currentConnection.Read(metaDataSizeBuff)
	if err != nil {
		server.Logs <- fmt.Sprintf("--> TCP SERVER Error reading metadata size: %v", err)
		return
	}

	metaDataSize, err := strconv.Atoi(strings.TrimSpace(string(metaDataSizeBuff)))
	if err != nil {
		server.Logs <- fmt.Sprintf("--> TCP SERVER Error converting metadata size: %v", err)
		return
	}

	// Metadata'yı okuma
	metaDataBuff := make([]byte, metaDataSize)
	_, err = server.currentConnection.Read(metaDataBuff)
	if err != nil {
		server.Logs <- fmt.Sprintf("--> TCP SERVER Error reading metadata: %v", err)
		return
	}

	var metaData FileMetadata
	err = json.Unmarshal(metaDataBuff, &metaData)
	if err != nil {
		server.Logs <- fmt.Sprintf("--> TCP SERVER Error unmarshalling metadata: %v", err)
		return
	}

	server.Logs <- fmt.Sprintf("--> TCP SERVER Received file metadata: %+v", metaData)

	server.Logs <- fmt.Sprintf("--> !!!!!!! %s", metaData.FileName)
	server.Logs <- fmt.Sprintf("--> !!!!!!! %s", filepath.Join("/Desktop", metaData.FileName))

	// Dosya yazma işlemi
	filePath := filepath.Join(logic.GetPath("/Desktop"), metaData.FileName) // Örneğin, "/Desktop" klasörüne kaydedilecek
	file, err := os.Create(filePath)
	if err != nil {
		server.Logs <- fmt.Sprintf("--> TCP SERVER Error creating file: %v", err)
		return
	}
	defer file.Close()

	recvBuff := make([]byte, 1024)
	totalReceived := 0
	for {
		n, err := server.currentConnection.Read(recvBuff)
		if err != nil {
			if err == io.EOF {
				server.Logs <- "--> TCP SERVER File receiving completed"
				break
			}
			server.Logs <- fmt.Sprintf("--> TCP SERVER Error reading file data: %v", err)
			return
		}

		_, err = file.Write(recvBuff[:n])
		if err != nil {
			server.Logs <- fmt.Sprintf("--> TCP SERVER Error writing file data: %v", err)
			return
		}

		totalReceived += n
	}

	server.Logs <- fmt.Sprintf("--> TCP SERVER Received %d bytes of file data", totalReceived)
}

func (server *TcpServer) SendFileToClient(filePath string) error {
	// Open the file to send
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	// Get file info for metadata
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("error getting file info: %v", err)
	}

	// Prepare file metadata
	metaData := FileMetadata{
		FileName: fileInfo.Name(),
		FileType: filepath.Ext(filePath),
		FileSize: fileInfo.Size(),
	}

	// Resolve client address (assuming we have a known client address in the server)
	clientAddr := server.Address.IP.String() + ":" + strconv.Itoa(server.Address.Port)

	// Connect to client
	conn, err := net.Dial("tcp", clientAddr)
	if err != nil {
		return fmt.Errorf("error connecting to client: %v", err)
	}
	defer conn.Close()

	// Marshal metadata to JSON
	metaDataJSON, err := json.Marshal(metaData)
	if err != nil {
		return fmt.Errorf("error marshalling metadata to JSON: %v", err)
	}

	// Send metadata size
	metaDataSize := len(metaDataJSON)
	metaDataSizeStr := fmt.Sprintf("%16d", metaDataSize)
	_, err = conn.Write([]byte(metaDataSizeStr))
	if err != nil {
		return fmt.Errorf("error sending metadata size: %v", err)
	}

	// Send metadata
	_, err = conn.Write(metaDataJSON)
	if err != nil {
		return fmt.Errorf("error sending metadata: %v", err)
	}

	// Send file data
	sendBuffer := make([]byte, 1024)
	for {
		bytesRead, err := file.Read(sendBuffer)
		if err != nil && err != io.EOF {
			return fmt.Errorf("error reading file: %v", err)
		}

		if bytesRead == 0 {
			break
		}

		_, err = conn.Write(sendBuffer[:bytesRead])
		if err != nil {
			return fmt.Errorf("error sending file data: %v", err)
		}
	}

	server.Logs <- fmt.Sprintf("--> TCP SERVER File sent to %s: %s", clientAddr, metaData.FileName)

	return nil
}
