package command

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	config "github.com/erdemkosk/gofi/internal"
	"github.com/erdemkosk/gofi/internal/logic"
	"github.com/erdemkosk/gofi/internal/udp"
	"github.com/gdamore/tcell/v2"
	"github.com/navidys/tvxwidgets"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
)

type StartCommand struct {
}

var logChannel chan string
var app *tview.Application
var stopToBroadcast chan bool
var connectionList []string
var grid *tview.Grid
var desktopPath string
var selectedNodes map[string]bool // Store node paths instead of pointers
var parentMap map[*tview.TreeNode]*tview.TreeNode
var pathBox *tview.TextView

func (command StartCommand) Execute(cmd *cobra.Command, args []string) {
	desktopPath = GetEnvolveHomePath()
	app = tview.NewApplication()

	logChannel = make(chan string)
	stopToBroadcast = make(chan bool)
	messagesFromClients := make(chan string)
	selectedNodes = make(map[string]bool)
	parentMap = make(map[*tview.TreeNode]*tview.TreeNode)

	mainFlex, logsBox, serverListDropdown := generateUI()

	go listenForLogs(logChannel, logsBox)
	go updateDropdownWithUdpClientMessages(messagesFromClients, serverListDropdown)

	server, serverErr := udp.CreateNewUdpServer(config.UDP_SERVER_BROADCAST_IP, config.UDP_PORT, logChannel)
	if serverErr != nil {
		fmt.Println("Server error:", serverErr)
		return
	}
	defer server.CloseConnection()

	client, clientErr := udp.CreateNewUdpClient(config.UDP_CLIENT_BROADCAST_IP, config.UDP_PORT, logChannel)
	if clientErr != nil {
		fmt.Println("Client error:", clientErr)
		return
	}
	defer client.CloseConnection()

	go client.SendBroadcastMessage(stopToBroadcast, logChannel)

	go server.Listen(messagesFromClients)

	if err := app.SetRoot(mainFlex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}

func listenForLogs(logs <-chan string, textView *tview.TextView) {
	for log := range logs {
		currentText := textView.GetText(false)
		textView.SetText(currentText + "\n" + log)
	}
}

func updateDropdownWithUdpClientMessages(messages <-chan string, dropdown *tview.DropDown) {
	for message := range messages {
		messageTrim := logic.TrimNullBytes([]byte(message))

		var msg udp.UdpMessage
		err := json.Unmarshal([]byte(messageTrim), &msg)
		if err != nil {
			logChannel <- fmt.Sprintf("Error unmarshaling JSON: %v %s", err, message)
			continue
		}

		if msg.IP == logic.GetLocalIP() {
			logChannel <- fmt.Sprintf("%s is current package so ignore it !", message)
			continue
		}

		if !logic.Contains(connectionList, message) {
			connectionList = append(connectionList, message)
			dropdown.SetOptions(connectionList, nil)
		}
	}
}

func GetEnvolveHomePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "/Desktop")
}

func contains(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

func ReadDir(path string, excludeNames []string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var filteredEntries []os.DirEntry
	for _, entry := range entries {
		if !contains(excludeNames, entry.Name()) {
			filteredEntries = append(filteredEntries, entry)
		}
	}

	return filteredEntries, nil
}

func generateUI() (*tview.Flex, *tview.TextView, *tview.DropDown) {
	gauge := tvxwidgets.NewActivityModeGauge()
	gauge.SetTitle("searching peers")
	gauge.SetPgBgColor(tcell.ColorOrange)
	gauge.SetRect(10, 0, 50, 3)
	gauge.SetBorder(true)

	dropdown := tview.NewDropDown()
	dropdown.SetLabel("Select an connection: ")
	dropdown.SetOptions([]string{}, nil)

	pathBox = tview.NewTextView()
	pathBox.SetText(desktopPath).
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetBorder(true).
		SetTitle("Current Path")

	var addNodes func(target *tview.TreeNode, path string)
	addNodes = func(target *tview.TreeNode, path string) {
		target.ClearChildren()
		target.SetText("Loading...")

		go func() {
			files, err := ReadDir(path, []string{".DS_Store"})
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

	button := tview.NewButton("Connect")
	button.SetSelectedFunc(func() {
		stopToBroadcast <- true
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
	})

	grid = tview.NewGrid().
		SetRows(3, 3, 3).
		SetColumns(0).
		SetBorders(true).
		AddItem(gauge, 0, 0, 1, 1, 0, 0, true).
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
