package command

import (
	"fmt"
	"time"

	config "github.com/erdemkosk/gofi/internal"
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

func (command StartCommand) Execute(cmd *cobra.Command, args []string) {
	app = tview.NewApplication()

	logChannel = make(chan string)
	stopToBroadcast = make(chan bool)
	logsBox, mainFlex, dropdown := generateUI()

	// Logları dinleyip UI'yi güncelle
	go listenForLogs(logChannel, logsBox)

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

	messages := make(chan string)

	go server.Listen(messages)

	go listenForMessages(messages, dropdown)

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

func listenForMessages(messages <-chan string, dropdown *tview.DropDown) {
	for message := range messages {
		dropdown.SetOptions([]string{message}, nil)
	}
}

func generateUI() (*tview.TextView, *tview.Flex, *tview.DropDown) {
	// Gauge oluşturma
	gauge := tvxwidgets.NewActivityModeGauge()
	gauge.SetTitle("searching peers")
	gauge.SetPgBgColor(tcell.ColorOrange)
	gauge.SetRect(10, 0, 50, 3)
	gauge.SetBorder(true)

	// Dropdown oluşturma
	dropdown := tview.NewDropDown()
	dropdown.SetLabel("Select an option: ")
	dropdown.SetOptions([]string{"Option 1", "Option 2", "Option 3"}, nil)

	// Buton oluşturma
	button := tview.NewButton("Click me")
	button.SetSelectedFunc(func() {
		stopToBroadcast <- true
	})

	// Sol tarafta grid oluşturma (gauge, dropdown, button)
	leftGrid := tview.NewGrid().
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
	logBox.SetChangedFunc(func() {
		app.Draw()
	})

	// Flex layout oluşturma
	flex := tview.NewFlex().
		AddItem(leftGrid, 0, 1, false). // Sol taraftaki grid, %50 kaplasın
		AddItem(logBox, 0, 1, false)    // Sağ taraftaki log alanı, %50 kaplasın

	// ASCII sanat eseri için TextView oluşturma
	iconBox := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetText(config.AppLogo)

	// Flex layout'un üstüne ekleyerek ana layout'u oluşturma
	mainFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(iconBox, 0, 1, true). // ASCII sanat eseri alanı ekranın tamamına yayılsın
		AddItem(flex, 0, 3, true)     // Diğer alanlar aşağıda yer alsın

	return logBox, mainFlex, dropdown
}
