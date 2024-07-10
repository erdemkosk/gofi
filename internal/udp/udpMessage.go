package udp

import (
	"encoding/json"
	"fmt"

	"github.com/erdemkosk/gofi/internal/logic"
)

type UdpMessage struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
	Name string `json:"name"`
}

func ConvertJsonToUdpMessage(message []byte, logs chan<- string) *UdpMessage { //write only channel
	messageTrim := logic.TrimNullBytes([]byte(message))

	var msg UdpMessage
	err := json.Unmarshal([]byte(messageTrim), &msg)
	if err != nil {
		logs <- fmt.Sprintf("Error unmarshaling JSON: %v %s", err, message)
		return nil
	}

	return &msg
}

func ConvertUdpMessageToJson(message *UdpMessage) string {
	jsonBytes, err := json.Marshal(message)
	if err != nil {
		fmt.Println("Error marshalling to JSON:", err)
		return ""
	}

	return string(jsonBytes)
}
