package main

import (
	"fmt"
	"time"

	"github.com/erdemkosk/gofi/internal/udp"
)

func main() {
	server, serverErr := udp.CreateNewUdpServer("0.0.0.0", 4444)
	if serverErr != nil {
		fmt.Println("Server error:", serverErr)
		return
	}
	defer server.CloseConnection()

	client, clientErr := udp.CreateNewUdpClient("127.0.0.1", 4444)
	if clientErr != nil {
		fmt.Println("Client error:", clientErr)
		return
	}
	defer client.CloseConnection()

	go func() {
		client.SendBroadcastMessage()
	}()

	time.Sleep(1 * time.Second)

	server.Listen()

	time.Sleep(10 * time.Second)
}
