package command

import (
	"encoding/json"
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
	app                *tview.Application
	stopUdpPeerChannel chan bool
	logChannel         chan string
	connectionList     []string
	grid               *tview.Grid
	selectedNodes      map[string]bool
	parentMap          map[*tview.TreeNode]*tview.TreeNode
	listDropDown       *tview.DropDown
)

func (command StartCommand) Execute(cmd *cobra.Command, args []string) {
	stopUdpPeerChannel = make(chan bool)
	logChannel = make(chan string)
	messagesFromUDPClients := make(chan *udp.UdpMessage)

	selectedNodes = make(map[string]bool)
	parentMap = make(map[*tview.TreeNode]*tview.TreeNode)

	app = tview.NewApplication()
	mainFlex, logsBox, serverListDropdown := generateUI()
	listDropDown = serverListDropdown

	go listenForLogs(logChannel, logsBox)

	udpServer, udpClient := udp.CreateUdpPeers(logChannel)

	defer udpServer.CloseConnection()
	defer udpClient.CloseConnection()

	go udpClient.SendBroadcastMessage(stopUdpPeerChannel)

	go udpServer.Listen(stopUdpPeerChannel, messagesFromUDPClients)

	go updateDropdownWithUdpClientMessages(messagesFromUDPClients, serverListDropdown)

	tcpServer, err := tcp.CreateNewTcpServer(logic.GetLocalIP(), config.TCP_PORT, logChannel)

	if err != nil {
		fmt.Println("Error creating TCP server:", err)
		return
	}

	stopTcpServerChannel := make(chan bool)

	go tcpServer.Listen(stopTcpServerChannel)

	if err := app.SetRoot(mainFlex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}

func listenForLogs(logs <-chan string, textView *tview.TextView) {
	for log := range logs {
		textView.SetText(textView.GetText(false) + "\n" + log)
	}
}

func updateDropdownWithUdpClientMessages(messages <-chan *udp.UdpMessage, dropdown *tview.DropDown) {
	for message := range messages {
		stringfyUdpMessage := udp.ConvertUdpMessageToJson(message)

		if message.IP == logic.GetLocalIP() {
			logChannel <- fmt.Sprintf("%s is the current package, ignoring it!", stringfyUdpMessage)
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
			case <-stopUdpPeerChannel:
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

		messageTrim := logic.TrimNullBytes([]byte(option))

		var msg udp.UdpMessage
		err := json.Unmarshal([]byte(messageTrim), &msg)
		if err != nil {
			logChannel <- fmt.Sprintf("Error unmarshaling JSON: %v %s", err, option)
			return
		}

		logChannel <- fmt.Sprintf("ftyfty: %s %d", msg.IP, msg.Port)

		client, err := tcp.CreateNewTcpClient(msg.IP, msg.Port, logChannel)

		messageChannel := make(chan string)

		go func() {
			client.SendMessage("Hello, Server!", messageChannel)
		}()

		if err != nil {
			fmt.Println("Error creating TCP client:", err)
			return
		}

		// Seçilen seçenekle ilgili diğer işlemler burada yapılabilir
	} else {
		logChannel <- "No option selected"
		return
	}

	udp.KillPeers(stopUdpPeerChannel) //stop udp peers

	desktopPath := logic.GetPath("/Desktop")

	pathBox := tview.NewTextView()
	pathBox.SetText(desktopPath).
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetBorder(true).
		SetTitle("Path")

	grid.Clear()
	tree := tview.NewTreeView().
		SetRoot(tview.NewTreeNode(filepath.Base(desktopPath)).SetColor(tcell.ColorLightGray)).
		SetCurrentNode(tview.NewTreeNode(filepath.Base(desktopPath)).SetColor(tcell.ColorDarkSlateBlue))

	tree.SetTitle("Finder").SetBorder(true)
	grid.SetRows(3, 0).
		SetColumns(0).
		AddItem(pathBox, 0, 0, 1, 1, 0, 0, true).
		AddItem(tree, 1, 0, 1, 1, 0, 0, true)

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

			pathBox.SetText(childPath)

			root := tview.NewTreeNode(filepath.Base(childPath)).SetColor(tcell.ColorLightGray)
			tree.SetRoot(root).SetCurrentNode(root)
			addNodes(root, childPath)

			return nil
		} else if event.Key() == tcell.KeyLeft {
			currentNode := tree.GetCurrentNode()
			if currentNode != nil {
				currentPath := pathBox.GetText(true)
				parentPath := filepath.Dir(currentPath)
				if parentPath != currentPath {
					pathBox.SetText(parentPath)

					root := tview.NewTreeNode(filepath.Base(parentPath)).SetColor(tcell.ColorLightGray)
					tree.SetRoot(root).SetCurrentNode(root)
					addNodes(root, parentPath)
				}
			}
			return nil
		}
		return event
	})

	addNodes(tree.GetRoot(), desktopPath)
	app.SetFocus(tree)
}
