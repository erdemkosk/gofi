package command

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	config "github.com/erdemkosk/gofi/internal"
	"github.com/erdemkosk/gofi/internal/logic"
	"github.com/erdemkosk/gofi/internal/tcp"
	"github.com/erdemkosk/gofi/internal/udp"
	"github.com/gdamore/tcell/v2"
	"github.com/navidys/tvxwidgets"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
)

type StartCommand struct{}

var (
	app                            *tview.Application
	stopUnusedPeersChannel         chan bool //UDP Client , UDP Server and TCP server acting together. If anyone who is interested to connect after broadcast we dont need 3 of them!
	stopUnusedTcpServerChannel     chan bool //UDP Client , UDP Server and TCP server acting together. If anyone who is interested to connect after broadcast we dont need 3 of them!
	clientConnectedTpServerChannel chan bool
	logChannel                     chan string
	connectionList                 []string
	grid                           *tview.Grid
	selectedNodes                  map[string]bool
	parentMap                      map[*tview.TreeNode]*tview.TreeNode
	listDropDown                   *tview.DropDown
	currentPath                    string
	tcpClient                      *tcp.TcpClient
	tcpServer                      *tcp.TcpServer
)

func (command StartCommand) Execute(cmd *cobra.Command, args []string) {
	stopUnusedPeersChannel = make(chan bool)
	stopUnusedTcpServerChannel = make(chan bool)
	logChannel = make(chan string)
	messagesFromUDPClients := make(chan *udp.UdpMessage)
	clientConnectedTpServerChannel = make(chan bool)

	selectedNodes = make(map[string]bool)
	parentMap = make(map[*tview.TreeNode]*tview.TreeNode)

	app = tview.NewApplication()
	mainFlex, logsBox, serverListDropdown := generateUI()
	listDropDown = serverListDropdown

	go listenForLogs(logChannel, logsBox)
	go listenForTcpConnection()

	udpServer, udpClient := udp.CreateUdpPeers(logChannel)
	tcpServer, _ = tcp.CreateNewTcpServer(logic.GetLocalIP(), config.TCP_PORT, logChannel)

	defer udpServer.CloseConnection()
	defer udpClient.CloseConnection()
	defer tcpServer.CloseConnection()

	go udpClient.SendBroadcastMessage(stopUnusedPeersChannel)

	go udpServer.Listen(stopUnusedPeersChannel, messagesFromUDPClients)

	go tcpServer.Listen(stopUnusedTcpServerChannel, clientConnectedTpServerChannel) // ıf the button click (if we are tcp client we dont need this server too! we will be client not server) If anyone connected we will know and change uı

	go updateDropdownWithUdpClientMessages(messagesFromUDPClients, serverListDropdown)

	if err := app.SetRoot(mainFlex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}

func listenForLogs(logs <-chan string, textView *tview.TextView) {
	for log := range logs {
		textView.SetText(textView.GetText(false) + "\n" + log)
		textView.ScrollToEnd()
	}
}

func listenForTcpConnection() {
	for msg := range clientConnectedTpServerChannel {
		// Update the TextView with the received log message
		if msg {
			changeUiState()
		}
	}
}

func updateDropdownWithUdpClientMessages(messages <-chan *udp.UdpMessage, dropdown *tview.DropDown) {
	for message := range messages {
		stringfyUdpMessage := udp.ConvertUdpMessageToJson(message)

		if message.IP == logic.GetLocalIP() {
			logChannel <- fmt.Sprintf("--> %s is the current computer, so ignoring it!", stringfyUdpMessage)
			continue
		}

		if !logic.Contains(connectionList, stringfyUdpMessage) {
			connectionList = append(connectionList, stringfyUdpMessage)
			dropdown.SetOptions(connectionList, nil)

			if len(connectionList) > 0 {
				dropdown.SetCurrentOption(0)
			}
		}
	}
}

func generateLoadingGauge() *tvxwidgets.ActivityModeGauge {
	gauge := tvxwidgets.NewActivityModeGauge()
	gauge.SetTitle("Searching peers")
	gauge.SetPgBgColor(tcell.ColorOrange)
	gauge.SetRect(10, 0, 50, 3)
	gauge.SetBorder(true)

	update := func() {
		tick := time.NewTicker(50 * time.Millisecond)
		for {
			select {
			case <-tick.C:
				gauge.Pulse()
				app.Draw()
			case <-stopUnusedPeersChannel:
				tick.Stop()
				return
			}
		}
	}
	go update()

	return gauge
}

func generateUI() (*tview.Flex, *tview.TextView, *tview.DropDown) {
	dropdown := tview.NewDropDown()
	dropdown.SetLabel("Select a connection: ")
	dropdown.SetOptions([]string{}, nil)

	button := tview.NewButton("Connect to the Peer")
	button.SetSelectedFunc(connectButtonHandler)

	grid = tview.NewGrid().
		SetRows(3, 3, 3).
		SetColumns(0).
		SetBorders(true).
		AddItem(generateLoadingGauge(), 0, 0, 1, 1, 0, 0, true).
		AddItem(dropdown, 1, 0, 1, 1, 0, 0, true).
		AddItem(button, 2, 0, 1, 1, 0, 0, true)

	logBox := tview.NewTextView()
	logBox.SetBorder(true)
	logBox.SetTitle("Logs")
	logBox.SetTextAlign(tview.AlignLeft)
	logBox.SetDynamicColors(true)
	logBox.SetScrollable(true)
	logBox.SetChangedFunc(func() {
		app.Draw()
	})

	flex := tview.NewFlex().
		AddItem(grid, 0, 1, false).
		AddItem(logBox, 0, 1, false)

	iconBox := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetText(config.AppLogo)

	mainFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(iconBox, 0, 1, true).
		AddItem(flex, 0, 3, true)

	return mainFlex, logBox, dropdown
}

func addNodes(target *tview.TreeNode, path string) {
	target.ClearChildren()
	target.SetText("Loading...")

	go func() {
		files, err := logic.ReadDir(path)
		if err != nil {
			log.Printf("Cannot read directory: %v", err)
			return
		}

		app.QueueUpdateDraw(func() {
			target.ClearChildren()
			for _, file := range files {
				node := tview.NewTreeNode(file.Name())
				node.SetReference(filepath.Join(path, file.Name()))

				nodePath := filepath.Join(path, file.Name())
				if selectedNodes[nodePath] {
					node.SetColor(tcell.ColorYellow)
				} else {
					node.SetColor(tcell.ColorLightGray)
				}

				if file.IsDir() {
					node.SetSelectable(true).
						SetExpanded(false)
					node.SetText(file.Name() + "/")
				} else {
					node.SetSelectable(true)
				}

				target.AddChild(node)
				parentMap[node] = target
			}

			target.SetText(path)
		})
	}()
}

func connectButtonHandler() {
	index, option := listDropDown.GetCurrentOption()
	if index != -1 {

		msg := udp.ConvertJsonToUdpMessage([]byte(option), logChannel)
		var err error

		tcpClient, err = tcp.CreateNewTcpClient(msg.IP, msg.Port, logChannel)

		if err != nil {
			fmt.Println("Error creating TCP client:", err)
			return
		}

	} else {
		logChannel <- "--> No peer selected!"
		return
	}

	// In here we are client
	close(stopUnusedTcpServerChannel)

	changeUiState()
}

func changeUiState() {
	close(stopUnusedPeersChannel)

	desktopPath := logic.GetPath("/Desktop")

	currentPath = desktopPath

	grid.Clear()
	tree := tview.NewTreeView().
		SetRoot(tview.NewTreeNode(filepath.Base(desktopPath)).SetColor(tcell.ColorLightGray)).
		SetCurrentNode(tview.NewTreeNode(filepath.Base(desktopPath)).SetColor(tcell.ColorDarkSlateBlue))

	tree.SetTitle("Finder").SetBorder(true)
	grid.SetRows(0).
		SetColumns(0, 0).
		AddItem(tree, 0, 0, 1, 1, 0, 0, true).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(tview.NewTextView().SetTitle("Received Data").SetBorder(true), 0, 1, false).
			AddItem(tview.NewTextView().SetTitle("Sent Data").SetBorder(true), 0, 1, false), 0, 1, 1, 1, 0, 0, true)

	tree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune && event.Rune() == ' ' {
			node := tree.GetCurrentNode()
			if len(node.GetChildren()) == 0 {
				addNodes(node, node.GetReference().(string))
			}
			node.SetExpanded(!node.IsExpanded())
			return nil
		} else if event.Key() == tcell.KeyEnter {
			node := tree.GetCurrentNode()
			nodePath := node.GetReference().(string)
			if selectedNodes[nodePath] {
				node.SetColor(tcell.ColorLightGray)
				delete(selectedNodes, nodePath)
			} else {
				node.SetColor(tcell.ColorYellow)
				selectedNodes[nodePath] = true
			}
			return nil
		} else if event.Key() == tcell.KeyRight {
			node := tree.GetCurrentNode()

			if node == nil {
				logChannel <- "Error: Current node is nil"
				return nil
			}

			childPath, ok := node.GetReference().(string)
			if !ok {
				logChannel <- "Error: Failed to cast node reference to string"
				return nil
			}

			fileInfo, err := os.Stat(childPath)
			if err != nil {
				logChannel <- fmt.Sprintf("Error getting file info: %v", err)
				return nil
			}

			if !fileInfo.IsDir() {
				return nil
			}

			currentPath = childPath
			root := tview.NewTreeNode(filepath.Base(childPath)).SetColor(tcell.ColorLightGray)
			tree.SetRoot(root).SetCurrentNode(root)
			addNodes(root, childPath)

			return nil
		} else if event.Key() == tcell.KeyLeft {
			currentNode := tree.GetCurrentNode()
			if currentNode != nil {
				parentPath := filepath.Dir(currentPath)
				if parentPath != currentPath {
					currentPath = parentPath

					root := tview.NewTreeNode(filepath.Base(parentPath)).SetColor(tcell.ColorLightGray)
					tree.SetRoot(root).SetCurrentNode(root)
					addNodes(root, parentPath)
				}
			}
			return nil
		} else if event.Key() == tcell.KeyEsc {
			go SendSelectedFiles()
			return nil
		}

		return event
	})

	addNodes(tree.GetRoot(), desktopPath)
	app.SetFocus(tree)
}

func SendSelectedFiles() {
	for filePath := range selectedNodes {
		fileName := filepath.Base(filePath)
		// Dosya gönderme işlemi yapılabilir
		logChannel <- fmt.Sprintf("Sending file: %s", fileName)
		// İstemci tarafından seçilen dosyaları gönder
		// Burada dosyaların TCP istemcisine gönderilmesi için gerekli işlemler yapılabilir
		// Örneğin:
		if tcpClient != nil {
			err := tcpClient.SendFileToServer(filePath)
			if err != nil {
				logChannel <- fmt.Sprintf("Error sending file %s: %v", fileName, err)
			}
		} else if tcpServer != nil {
			err := tcpServer.SendFileToClient(filePath)
			if err != nil {
				logChannel <- fmt.Sprintf("Error sending file %s: %v", fileName, err)
			}
		}
	}
}
