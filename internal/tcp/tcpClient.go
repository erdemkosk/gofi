package tcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type FileMetadata struct {
	FileName string `json:"fileName"`
	FileType string `json:"fileType"`
	FileSize int64  `json:"fileSize"`
	IsDir    bool   `json:"isDir"`
	FullPath string `json:"fullPath"`
}

type TcpClient struct {
	Address     net.TCPAddr
	Connection  *net.TCPConn
	IsConnected bool
	Logs        chan string
	FileQueue   []string
	mutex       sync.Mutex // Mutex for synchronization
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
	client.mutex.Lock()
	client.FileQueue = append(client.FileQueue, destinationPath)
	client.Logs <- fmt.Sprintf("--> Queued file or directory: %s", destinationPath)
	client.mutex.Unlock()

	for {
		client.mutex.Lock()
		if len(client.FileQueue) == 0 {
			client.mutex.Unlock()
			break
		}
		filePath := client.FileQueue[0]
		client.FileQueue = client.FileQueue[1:]
		client.mutex.Unlock()

		err := client.sendFileOrDirectory(filePath, "")
		if err != nil {
			client.Logs <- fmt.Sprintf("--> TCP CLIENT Error sending file or directory: %v", err)
		}

		time.Sleep(1 * time.Second) // 1 saniye ara
	}

	client.Logs <- "--> All files and directories sent successfully!"
}

func (client *TcpClient) sendFileOrDirectory(path string, relativePath string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("error getting file information: %v", err)
	}

	if fileInfo.IsDir() {
		return client.sendDirectory(path, relativePath)
	} else {
		return client.sendFile(path, relativePath)
	}
}

func (client *TcpClient) sendDirectory(dirPath string, relativePath string) error {
	client.Logs <- fmt.Sprintf("--> Sending directory: %s", dirPath)

	// Prepare metadata
	metaData := FileMetadata{
		FileName: filepath.Base(dirPath),
		FileType: "",
		FileSize: 0,
		IsDir:    true,
		FullPath: relativePath,
	}

	metaDataBytes, err := json.Marshal(metaData)
	if err != nil {
		return fmt.Errorf("error marshalling metadata: %v", err)
	}

	// Send metadata size
	metaDataSize := fmt.Sprintf("%016d", len(metaDataBytes))
	client.Logs <- fmt.Sprintf("--> Metadata size: %v", metaDataSize)
	_, err = client.Connection.Write([]byte(metaDataSize))
	if err != nil {
		return fmt.Errorf("error sending metadata size: %v %s", err, metaDataSize)
	}

	// Send metadata
	_, err = client.Connection.Write(metaDataBytes)
	if err != nil {
		return fmt.Errorf("error sending metadata: %v", err)
	}

	// Send directory contents
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("error reading directory: %v", err)
	}

	for _, entry := range entries {
		entryPath := filepath.Join(dirPath, entry.Name())
		newRelativePath := filepath.Join(relativePath, filepath.Base(dirPath), entry.Name())
		err := client.sendFileOrDirectory(entryPath, newRelativePath)
		if err != nil {
			return err
		}
	}

	return nil
}

func (client *TcpClient) sendFile(filePath string, relativePath string) error {
	client.Logs <- fmt.Sprintf("--> Sending file: %v", filePath)
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
		IsDir:    false,
		FullPath: relativePath,
	}

	metaDataBytes, err := json.Marshal(metaData)
	if err != nil {
		return fmt.Errorf("error marshalling metadata: %v", err)
	}

	// Send metadata size
	metaDataSize := fmt.Sprintf("%016d", len(metaDataBytes))
	client.Logs <- fmt.Sprintf("--> Metadata size: %v", metaDataSize)
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

	client.Logs <- fmt.Sprintf("--> Sent %d bytes of file data for: %s", totalSent, fileInfo.Name())

	return nil
}
