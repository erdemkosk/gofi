package udp

import (
	config "github.com/erdemkosk/gofi/internal"
)

func CreateUdpPeers(logChannel chan string) (*UdpServer, *UdpClient) {
	server, serverErr := CreateNewUdpServer(config.UDP_SERVER_BROADCAST_IP, config.UDP_PORT, logChannel)
	if serverErr != nil {
		panic("Cannot create UDP Server! ")
	}

	client, clientErr := CreateNewUdpClient(config.UDP_CLIENT_BROADCAST_IP, config.UDP_PORT, logChannel)
	if clientErr != nil {
		panic("Cannot create UDP Client! ")
	}

	return server, client
}

func KillPeers(stopUdpPeerChannel chan bool) {
	close(stopUdpPeerChannel)
}
