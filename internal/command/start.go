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
	"github.com/erdemkosk/gofi/internal/udp"
	"github.com/gdamore/tcell/v2"
	"github.com/navidys/tvxwidgets"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
)

type StartCommand struct {
}

var app *tview.Application

var stopUdpPeerChannel chan bool // It will control Udp peers alive status
var logChannel chan string       // It will collect each logs from everywhere

var connectionList []string
var grid *tview.Grid
var desktopPath string
var selectedNodes map[string]bool // Store node paths instead of pointers
var parentMap map[*tview.TreeNode]*tview.TreeNode
var pathBox *tview.TextView

func (command StartCommand) Execute(cmd *cobra.Command, args []string) {
	stopUdpPeerChannel = make(chan bool)
	logChannel = make(chan string)
	messagesFromUDPClients := make(chan string) // ıt will retrive messages from client

	selectedNodes = make(map[string]bool)
	parentMap = make(map[*tview.TreeNode]*tview.TreeNode)

	desktopPath = logic.GetPath("/Desktop")
	app = tview.NewApplication()
	mainFlex, logsBox, serverListDropdown := generateUI()

	go listenForLogs(logChannel, logsBox)

	server, client := udp.CreateUdpPeers(logChannel)

	defer server.CloseConnection()
	defer client.CloseConnection()

	go client.SendBroadcastMessage(stopUdpPeerChannel)

	go server.Listen(stopUdpPeerChannel, messagesFromUDPClients)

	go updateDropdownWithUdpClientMessages(messagesFromUDPClients, serverListDropdown)

	if err := app.SetRoot(mainFlex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}

func listenForLogs(logs <-chan string, textView *tview.TextView) {
	for log := range logs {
		textView.SetText(textView.GetText(false) + "\n" + log)
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

func generateLoadingGauge() *tvxwidgets.ActivityModeGauge {
	gauge := tvxwidgets.NewActivityModeGauge()
	gauge.SetTitle("searching peers")
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
	dropdown.SetLabel("Select an connection: ")
	dropdown.SetOptions([]string{}, nil)

	pathBox = tview.NewTextView()
	pathBox.SetText(desktopPath).
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetBorder(true).
		SetTitle("Path")

	addNodes := func(target *tview.TreeNode, path string) {
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

	button := tview.NewButton("Connect To The Peer")
	button.SetSelectedFunc(func() {
		udp.KillPeers(stopUdpPeerChannel)
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
