package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/allod-project/allod/internal/helper"
)

func main() {
	fmt.Println("Avvio allod-helperd (Privileged Root Helper)...")

	// Ensure system administrative paths are in PATH
	curPath := os.Getenv("PATH")
	standardPaths := "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	if curPath == "" {
		os.Setenv("PATH", standardPaths)
	} else if !strings.Contains(curPath, "/usr/sbin") {
		os.Setenv("PATH", standardPaths+":"+curPath)
	}

	sockPath := "allod-helper.sock"
	if os.Geteuid() == 0 {
		if err := os.MkdirAll("/run/allod", 0755); err == nil {
			sockPath = "/run/allod/helper.sock"
		}
	}

	srv := helper.Server{
		SocketPath: sockPath,
	}

	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Errore avvio helper: %v\n", err)
		os.Exit(1)
	}
}
