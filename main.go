package main

import (
	"fmt"
	"time"

	"github.com/erdemkosk/gofi/internal/udp"
)

func main() {
	server, serverErr := udp.CreateNewUdpServer("127.0.0.1", 4444)
	if serverErr != nil {
		fmt.Println("Server error:", serverErr)
		return
	}
	defer server.Connection.Close()

	client, clientErr := udp.CreateNewUdpClient("127.0.0.1", 4444)
	if clientErr != nil {
		fmt.Println("Client error:", clientErr)
		return
	}
	defer client.Connection.Close()

	go func() {
		buffer := make([]byte, 1024)
		for {
			n, addr, err := server.Connection.ReadFromUDP(buffer)
			if err != nil {
				fmt.Println("Error receiving message:", err)
				return
			}
			fmt.Printf("Received message from %s: %s\n", addr, string(buffer[:n]))
		}
	}()

	time.Sleep(1 * time.Second)

	message := []byte("Hello, UDP Server babe!")
	_, err := client.Connection.Write(message)
	if err != nil {
		fmt.Println("Error sending message:", err)
		return
	}
	fmt.Println("Message sent to server")

	time.Sleep(1 * time.Second)
}
