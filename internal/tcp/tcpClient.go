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

type TcpClient struct {
	Address     net.TCPAddr
	Connection  *net.TCPConn
	IsConnected bool
	Logs        chan string
	FileQueue   []string
	mutex       sync.Mutex // Mutex for synchronization
}

type FileMetadata struct {
	FileName string `json:"fileName"`
	FileType string `json:"fileType"`
	FileSize int64  `json:"fileSize"`
	FullPath string `json:"fullPath"`
	IsDir    bool   `json:"isDir"`
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

	// Determine the base path for relative path calculation
	basePath := filepath.Dir(destinationPath)

	for {
		client.mutex.Lock()
		if len(client.FileQueue) == 0 {
			client.mutex.Unlock()
			break
		}
		filePath := client.FileQueue[0]
		client.FileQueue = client.FileQueue[1:]
		client.mutex.Unlock()

		// Check if the path is a directory
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			client.Logs <- fmt.Sprintf("--> Error getting file info: %v", err)
			continue
		}

		relativePath, err := filepath.Rel(basePath, filePath)
		if err != nil {
			client.Logs <- fmt.Sprintf("--> Error calculating relative path: %v", err)
			continue
		}

		if fileInfo.IsDir() {
			// Send the directory itself
			err = client.sendDirectory(filePath, relativePath)
			if err != nil {
				client.Logs <- fmt.Sprintf("--> TCP CLIENT Error sending directory %v", err)
			}

			// Recursively add all files and subdirectories to the queue
			err = filepath.Walk(filePath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if path != filePath {
					client.mutex.Lock()
					client.FileQueue = append(client.FileQueue, path)
					client.mutex.Unlock()
				}
				return nil
			})
			if err != nil {
				client.Logs <- fmt.Sprintf("--> TCP CLIENT Error walking directory %v", err)
			}
		} else {
			err = client.sendFile(filePath, relativePath)
			if err != nil {
				client.Logs <- fmt.Sprintf("--> TCP CLIENT Error sending file %v", err)
			}
		}

		time.Sleep(1 * time.Second) // 1 saniye ara
	}

	client.Logs <- "--> All files and directories sent successfully!"
}

func (client *TcpClient) sendDirectory(dirPath string, relativePath string) error {
	client.Logs <- fmt.Sprintf("--> Sending directory: %v", dirPath)

	// Prepare metadata for the directory
	metaData := FileMetadata{
		FileName: filepath.Base(dirPath),
		FileType: "directory",
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
		FullPath: relativePath,
		IsDir:    false,
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
