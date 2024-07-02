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

var logChannel chan string
var app *tview.Application
var stopToBroadcast chan bool // It will control udp client broadcast message
var connectionList []string
var grid *tview.Grid
var desktopPath string
var selectedNodes map[*tview.TreeNode]bool

func (command StartCommand) Execute(cmd *cobra.Command, args []string) {
	desktopPath = GetEnvolveHomePath()
	app = tview.NewApplication()

	logChannel = make(chan string)
	stopToBroadcast = make(chan bool)
	messagesFromClients := make(chan string)
	selectedNodes = make(map[*tview.TreeNode]bool)

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

	// Flex layout'u uygulamaya kök olarak ayarla
	if err := app.SetRoot(mainFlex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}

func listenForLogs(logs <-chan string, textView *tview.TextView) {
	for log := range logs {
		time.Sleep(1 * time.Second)
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

func ReadDir(path string, excludeNames []string) ([]os.FileInfo, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var filteredFiles []os.DirEntry
	for _, file := range files {
		if !contains(excludeNames, file.Name()) {
			filteredFiles = append(filteredFiles, file)
		}
	}

	var fileInfos []os.FileInfo
	for _, file := range filteredFiles {
		info, err := file.Info()
		if err != nil {
			return nil, err
		}
		fileInfos = append(fileInfos, info)
	}

	return fileInfos, nil
}

func generateUI() (*tview.Flex, *tview.TextView, *tview.DropDown) {
	// Gauge oluşturma
	gauge := tvxwidgets.NewActivityModeGauge()
	gauge.SetTitle("searching peers")
	gauge.SetPgBgColor(tcell.ColorOrange)
	gauge.SetRect(10, 0, 50, 3)
	gauge.SetBorder(true)

	// Dropdown oluşturma
	dropdown := tview.NewDropDown()
	dropdown.SetLabel("Select an connection: ")
	dropdown.SetOptions([]string{}, nil)

	// Buton oluşturma
	button := tview.NewButton("Click me")
	button.SetSelectedFunc(func() {
		stopToBroadcast <- true
		grid.Clear()
		tree := tview.NewTreeView().
			SetRoot(tview.NewTreeNode(desktopPath).SetColor(tcell.ColorLightGray)).
			SetCurrentNode(tview.NewTreeNode(desktopPath).SetColor(tcell.ColorDarkSlateBlue))

		tree.SetTitle("Envs").SetBorder(true)
		// Soldaki grid'i temizle ve tree'yi tam olarak ekle
		grid.SetRows(0). // Tek bir satır ayarla, böylece tamamını kaplar
					SetColumns(0).                        // Tek bir sütun ayarla, böylece tamamını kaplar
					AddItem(tree, 0, 0, 1, 1, 0, 0, true) // Tree'yi tamamını kaplayacak şekilde ekle

		tree.SetSelectedFunc(func(node *tview.TreeNode) {
			if selectedNodes[node] {
				node.SetColor(tcell.ColorLightGray) // Eğer düğüm zaten seçiliyse rengini eski haline döndür
				delete(selectedNodes, node)         // Düğümü seçili düğümler listesinden çıkar
			} else {
				node.SetColor(tcell.ColorYellow) // Eğer düğüm seçili değilse rengini değiştir
				selectedNodes[node] = true       // Düğümü seçili düğümler listesine ekle
			}
		})

		addNodes := func(target *tview.TreeNode, path string) {
			files, err := ReadDir(path, []string{".DS_Store"})
			if err != nil {
				log.Fatalf("Cannot read directory: %v", err)
			}

			for _, file := range files {
				node := tview.NewTreeNode(file.Name()).
					SetReference(filepath.Join(path, file.Name()))
				if file.IsDir() {
					node.SetColor(tcell.ColorLightGray)
					node.SetSelectable(true)
					node.SetExpanded(false)
				} else {
					node.SetColor(tcell.ColorLightGray)
					node.SetSelectable(true)
				}
				target.AddChild(node)
			}
		}

		addNodes(tree.GetRoot(), desktopPath)
	})

	// Sol tarafta grid oluşturma (gauge, dropdown, button)
	grid = tview.NewGrid().
		SetRows(3, 3, 3). // Üç satır ayarla
		SetColumns(0).    // Tek bir sütun ayarla
		SetBorders(true).
		AddItem(gauge, 0, 0, 1, 1, 0, 0, true).    // Gauge en üstte
		AddItem(dropdown, 1, 0, 1, 1, 0, 0, true). // Dropdown ortada
		AddItem(button, 2, 0, 1, 1, 0, 0, true)    // Buton en altta

	// Sağ tarafta log alanı oluşturma
	logBox := tview.NewTextView()
	logBox.SetBorder(true)
	logBox.SetTitle("Logs")
	logBox.SetTextAlign(tview.AlignLeft)
	logBox.SetDynamicColors(true)
	logBox.SetScrollable(true)
	logBox.SetChangedFunc(func() {
		app.Draw()
	})

	// Flex layout oluşturma
	flex := tview.NewFlex().
		AddItem(grid, 0, 1, false).  // Sol taraftaki grid, %50 kaplasın
		AddItem(logBox, 0, 1, false) // Sağ taraftaki log alanı, %50 kaplasın

	// ASCII sanat eseri için TextView oluşturma
	iconBox := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetText(config.AppLogo)

	// Flex layout'un üstüne ekleyerek ana layout'u oluşturma
	mainFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(iconBox, 0, 1, true). // ASCII sanat eseri alanı ekranın tamamına yayılsın
		AddItem(flex, 0, 3, true)     // Diğer alanlar aşağıda yer alsın

	return mainFlex, logBox, dropdown
}
