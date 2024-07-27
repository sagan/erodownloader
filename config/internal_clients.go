package config

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/sagan/erodownloader/constants"
	"github.com/sagan/erodownloader/util"
)

// Internal local clients.
var InternalClients = []*ClientConfig{
	{
		Name:     constants.LOCAL_CLIENT,
		Type:     "aria2",
		Url:      "http://localhost:6800/jsonrpc",
		SavePath: "", // Default to ~/Downloads
		Internal: true,
		Local:    true,
	},
}

func init() {
	for _, client := range InternalClients {
		internalClientsConfigMap[client.Name] = client
		if client.SavePath == "" {
			client.SavePath = filepath.Join(util.Unwrap(os.UserHomeDir()), "Downloads")
		}
		if runtime.GOOS == "windows" {
			client.Windows = true
		}
	}
}
