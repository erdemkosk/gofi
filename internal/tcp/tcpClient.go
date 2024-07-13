package tcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/erdemkosk/gofi/internal/logic"
)

type TcpClient struct {
	Address     net.TCPAddr
	Connection  *net.TCPConn
	IsConnected bool
	Logs        chan string
	FileQueue   []string
	wg          sync.WaitGroup
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
		FileQueue:   make([]string, 0),
	}

	// Start listening for incoming files
	go client.SendFileToServer(logic.GetPath("/Desktop"))

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

func (client *TcpClient) SendFileToServer(destinationPath string) {
	client.FileQueue = append(client.FileQueue, destinationPath)
	for {
		if len(client.FileQueue) > 0 {
			client.wg.Add(1)
			filePath := client.FileQueue[0]
			client.FileQueue = client.FileQueue[1:]

			go func(path string) {
				defer client.wg.Done()
				err := client.sendFile(path)
				if err != nil {
					client.Logs <- fmt.Sprintf("--> TCP CLIENT Error sending file: %v", err)
				}
			}(filePath)
		}
	}
}

func (client *TcpClient) sendFile(filePath string) error {
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
		return fmt.Errorf("error sending metadata size: %v %s", err, metaDataSize)
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

func (client *TcpClient) EnqueueFile(filePath string) {
	client.FileQueue = append(client.FileQueue, filePath)
}

func (client *TcpClient) WaitUntilFinished() {
	client.wg.Wait()
}
