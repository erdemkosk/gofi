package internal

const (
	UDP_SERVER_BROADCAST_IP = "0.0.0.0"
	UDP_CLIENT_BROADCAST_IP = "255.255.255.255"
	UDP_PORT                = 4444
)

const (
	TCP_SERVER_IP = "0.0.0.0"
	TCP_CLIENT_IP = "127.0.0.1"
	TCP_PORT      = 8888
)

type CommandType int32

const (
	START CommandType = 1
)

const (
	RESET          = "\033[0m"
	RED            = "\033[31m"
	PASTEL_RED     = "\033[91m"
	PASTEL_GREEN   = "\033[92m"
	PASTEL_YELLOW  = "\033[93m"
	PASTEL_BLUE    = "\033[94m"
	PASTEL_MAGENTA = "\033[95m"
	PASTEL_CYAN    = "\033[96m"
	PASTEL_WHITE   = "\033[97m"
	PASTEL_GRAY    = "\033[37m"
	PASTEL_PURPLE  = "\033[35m"
	PASTEL_ORANGE  = "\033[38;5;214m"
)

const (
	AppLogo = `  ___       __ _ 
  / _ \___  / _(_)
 / /_\/ _ \| |_| |
/ /_\\ (_) |  _| |
\____/\___/|_| |_|`
)
