package tcp

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
)

type FileMetadata struct {
	FileName string
	FileType string
	FileSize int64
}

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

func (client *TcpClient) SendFile(filePath string) {
	client.Logs <- "--> TCP CLIENT Initiating file transfer"

	// Dosya meta verilerini hazırla
	metaData, err := prepareFileMetadata(filePath)
	if err != nil {
		client.Logs <- fmt.Sprintf("--> TCP CLIENT Error preparing file metadata: %v", err)
		return
	}

	// Meta verilerini sunucuya gönder
	metaDataBytes, err := json.Marshal(metaData)
	if err != nil {
		client.Logs <- fmt.Sprintf("--> TCP CLIENT Error encoding metadata: %v", err)
		return
	}

	_, err = client.Connection.Write(metaDataBytes)
	if err != nil {
		client.Logs <- fmt.Sprintf("--> TCP CLIENT Error sending metadata: %v", err)
		return
	}

	// Dosyayı sunucuya gönder
	err = client.sendFileData(filePath)
	if err != nil {
		client.Logs <- fmt.Sprintf("--> TCP CLIENT Error sending file: %v", err)
		return
	}

	client.Logs <- "--> TCP CLIENT File sent successfully"
}

func (client *TcpClient) sendFileData(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	sendBuff := make([]byte, 4096)
	for {
		n, err := file.Read(sendBuff)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading file: %v", err)
		}

		_, err = client.Connection.Write(sendBuff[:n])
		if err != nil {
			return fmt.Errorf("error sending file: %v", err)
		}
	}

	return nil
}

func prepareFileMetadata(filePath string) (FileMetadata, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return FileMetadata{}, fmt.Errorf("error getting file info: %v", err)
	}

	fileName := fileInfo.Name()
	fileType := "unknown" // Dilerseniz dosya tipi için bir kontrol eklenebilir.
	fileSize := fileInfo.Size()

	metaData := FileMetadata{
		FileName: fileName,
		FileType: fileType,
		FileSize: fileSize,
	}

	return metaData, nil
}
