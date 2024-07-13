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

	"github.com/erdemkosk/gofi/internal/logic"
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

	client := &TcpClient{
		Connection:  conn,
		Address:     *tcpAddr,
		IsConnected: true,
		Logs:        logs,
	}

	// Start listening for incoming files

	go client.ListenForFiles(logic.GetPath("/Desktop"))

	return client, nil
}

func (client *TcpClient) CloseConnection() {
	err := client.Connection.Close()
	if err != nil {
		fmt.Println("--> TCP CLIENT cannot be closed!")
	}

	client.IsConnected = false
	client.Logs <- "--> TCP CLIENT closed successfully!"
}

func (client *TcpClient) ListenForFiles(destinationPath string) {
	for {
		err := client.ReceiveFile(destinationPath)
		if err != nil {
			client.Logs <- fmt.Sprintf("--> TCP CLIENT Error receiving file: %v", err)
		}
	}
}

func (client *TcpClient) ReceiveFile(destinationPath string) error {
	// Read metadata size
	metaDataSizeBuff := make([]byte, 16)
	_, err := client.Connection.Read(metaDataSizeBuff)
	if err != nil {
		return fmt.Errorf("error reading metadata size: %v", err)
	}

	metaDataSize, err := strconv.Atoi(strings.TrimSpace(string(metaDataSizeBuff)))
	if err != nil {
		return fmt.Errorf("error converting metadata size: %v", err)
	}

	// Read metadata
	metaDataBuff := make([]byte, metaDataSize)
	_, err = client.Connection.Read(metaDataBuff)
	if err != nil {
		return fmt.Errorf("error reading metadata: %v", err)
	}

	var metaData FileMetadata
	err = json.Unmarshal(metaDataBuff, &metaData)
	if err != nil {
		return fmt.Errorf("error unmarshalling metadata: %v", err)
	}

	client.Logs <- fmt.Sprintf("--> TCP CLIENT Received file metadata: %+v", metaData)

	// Prepare destination file
	filePath := filepath.Join(destinationPath, metaData.FileName)
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()

	// Receive file data
	recvBuffer := make([]byte, 1024)
	totalReceived := 0
	for {
		n, err := client.Connection.Read(recvBuffer)
		if err != nil {
			if err == io.EOF {
				client.Logs <- "--> TCP CLIENT File receiving completed"
				break
			}
			return fmt.Errorf("error reading file data: %v", err)
		}

		_, err = file.Write(recvBuffer[:n])
		if err != nil {
			return fmt.Errorf("error writing file data: %v", err)
		}

		totalReceived += n
	}

	client.Logs <- fmt.Sprintf("--> TCP CLIENT Received %d bytes of file data", totalReceived)

	return nil
}

func (client *TcpClient) SendFileToServer(filePath string) error {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	// Get file information
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("error getting file information: %v", err)
	}

	// Prepare metadata
	metaData := FileMetadata{
		FileName: fileInfo.Name(),
		FileType: filepath.Ext(fileInfo.Name()),
		FileSize: fileInfo.Size(),
	}

	metaDataBytes, err := json.Marshal(metaData)
	if err != nil {
		return fmt.Errorf("error marshalling metadata: %v", err)
	}

	// Send metadata size
	metaDataSize := fmt.Sprintf("%016d", len(metaDataBytes))
	_, err = client.Connection.Write([]byte(metaDataSize))
	if err != nil {
		return fmt.Errorf("error sending metadata size: %v", err)
	}

	// Send metadata
	_, err = client.Connection.Write(metaDataBytes)
	if err != nil {
		return fmt.Errorf("error sending metadata: %v", err)
	}

	// Send file data
	sendBuffer := make([]byte, 1024)
	totalSent := 0
	for {
		n, err := file.Read(sendBuffer)
		if err != nil && err != io.EOF {
			return fmt.Errorf("error reading file: %v", err)
		}

		if n == 0 {
			break
		}

		_, err = client.Connection.Write(sendBuffer[:n])
		if err != nil {
			return fmt.Errorf("error sending file data: %v", err)
		}

		totalSent += n
	}

	client.Logs <- fmt.Sprintf("--> TCP CLIENT Sent %d bytes of file data", totalSent)

	return nil
}
