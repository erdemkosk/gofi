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
			server.Connection.SetDeadline(time.Now().Add(10 * time.Second))

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
	defer server.currentConnection.Close()

	for {
		// Metadata size buffer reading
		sizeBuffer := make([]byte, 16)
		_, err := io.ReadFull(server.currentConnection, sizeBuffer)
		if err != nil {
			if err == io.EOF {
				server.Logs <- "--> TCP SERVER Connection closed by client"
				break
			}
			server.Logs <- fmt.Sprintf("--> TCP SERVER Error reading metadata size: %v", err)
			return
		}
		metadataSize, err := strconv.ParseInt(strings.TrimSpace(string(sizeBuffer)), 10, 64)
		if err != nil {
			server.Logs <- fmt.Sprintf("--> TCP SERVER Error converting metadata size: %v", err)
			return
		}

		// Metadata buffer reading
		metaDataBuffer := make([]byte, metadataSize)
		_, err = io.ReadFull(server.currentConnection, metaDataBuffer)
		if err != nil {
			server.Logs <- fmt.Sprintf("--> TCP SERVER Error reading metadata: %v", err)
			return
		}

		// Metadata unmarshalling
		var fileMetaData FileMetadata
		err = json.Unmarshal(metaDataBuffer, &fileMetaData)
		if err != nil {
			server.Logs <- fmt.Sprintf("--> TCP SERVER Error unmarshalling metadata: %v", err)
			return
		}

		// Determine destination path
		var destinationPath string
		if fileMetaData.FullPath == "" {
			destinationPath = filepath.Join(logic.GetPath("/Desktop"), fileMetaData.FileName)
		} else {
			destinationPath = filepath.Join(logic.GetPath("/Desktop"), fileMetaData.FullPath)
		}

		server.Logs <- fmt.Sprintf("--> DESTINATION: %v", destinationPath)

		if fileMetaData.IsDir {
			server.Logs <- fmt.Sprintf("--> TCP SERVER Received directory: %v", fileMetaData.FileName)
			// Create directory if it doesn't exist
			err := os.MkdirAll(destinationPath, os.ModePerm)
			if err != nil {
				server.Logs <- fmt.Sprintf("--> TCP SERVER Error creating directory: %v", err)
				return
			}

			// Send ACK to client
			_, err = server.currentConnection.Write([]byte("ACK"))
			if err != nil {
				server.Logs <- fmt.Sprintf("--> TCP SERVER Error sending ACK: %v", err)
				return
			}

			continue
		}

		// For files, create parent directories if they don't exist
		parentDir := filepath.Dir(destinationPath)
		err = os.MkdirAll(parentDir, os.ModePerm)
		if err != nil {
			server.Logs <- fmt.Sprintf("--> TCP SERVER Error creating parent directory: %v", err)
			return
		}

		// Create file
		file, err := os.Create(destinationPath)
		if err != nil {
			server.Logs <- fmt.Sprintf("--> TCP SERVER Error creating file: %v", err)
			return
		}

		// Read file data
		receivedBytes := int64(0)
		buffer := make([]byte, 1024)
		for receivedBytes < fileMetaData.FileSize {
			n, err := server.currentConnection.Read(buffer)
			if err != nil {
				server.Logs <- fmt.Sprintf("--> TCP SERVER Error reading file data: %v", err)
				file.Close()
				return
			}

			_, err = file.Write(buffer[:n])
			if err != nil {
				server.Logs <- fmt.Sprintf("--> TCP SERVER Error writing to file: %v", err)
				file.Close()
				return
			}

			receivedBytes += int64(n)
		}

		file.Close()
		server.Logs <- fmt.Sprintf("--> TCP SERVER File received and saved: %s", destinationPath)

		// Send ACK to client
		_, err = server.currentConnection.Write([]byte("ACK"))
		if err != nil {
			server.Logs <- fmt.Sprintf("--> TCP SERVER Error sending ACK: %v", err)
			return
		}
	}
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
