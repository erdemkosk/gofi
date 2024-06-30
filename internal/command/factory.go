package command

import (
	"fmt"

	config "github.com/erdemkosk/gofi/internal"
)

func CommandFactory(commandType config.CommandType) ICommand {
	if commandType == config.START {
		fmt.Println(commandType)
		return &StartCommand{}
	}

	return nil
}
