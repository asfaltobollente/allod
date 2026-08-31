package main

import (
	"fmt"
	"os"

	"github.com/allod-project/allod/internal/helper"
)

func main() {
	fmt.Println("Avvio allod-helperd (Privileged Root Helper)...")
	
	// Socket temporaneo per il PoC su Windows
	sockPath := "allod-helper.sock" 
	
	srv := helper.Server{
		SocketPath: sockPath,
	}

	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Errore avvio helper: %v\n", err)
		os.Exit(1)
	}
}
