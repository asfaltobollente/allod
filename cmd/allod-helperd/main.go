package main

import (
	"fmt"
	"os"

	"github.com/allod-project/allod/internal/helper"
)

func main() {
	fmt.Println("Avvio allod-helperd (Privileged Root Helper)...")

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
