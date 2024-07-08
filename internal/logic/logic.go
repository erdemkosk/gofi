package logic

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"time"
)

func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func GetHostName() string {
	name, err := os.Hostname()
	if err != nil {
		fmt.Println("Error:", err)
	}

	return name
}

func Contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

func IsJSON(s string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

func TrimNullBytes(b []byte) []byte {
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] != 0 {
			return b[:i+1]
		}
	}
	return nil
}

func GetPath(path string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, path)
}

func ReadDir(path string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var filteredEntries []os.DirEntry
	for _, entry := range entries {
		filteredEntries = append(filteredEntries, entry)
	}

	return filteredEntries, nil
}

func GenerateRandomTime() time.Duration {
	minInterval := 5
	maxInterval := 10
	randomInterval := rand.Intn(maxInterval-minInterval+1) + minInterval

	return time.Duration(randomInterval) * time.Second
}
