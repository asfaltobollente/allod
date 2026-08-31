package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/allod-project/allod/internal/watch"
)

func main() {
	fmt.Println("=== Inizio Test M5: Sorveglianza Mesh (Watchdog) ===")
	
	// Create a mock server that simulates the friend's node
	mockServerAddr := "127.0.0.1:40005"
	serverMux := http.NewServeMux()
	serverMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mockServer := &http.Server{Addr: mockServerAddr, Handler: serverMux}
	
	// Start the mock server in the background
	go func() {
		mockServer.ListenAndServe()
	}()
	
	// Wait a moment for the server to start
	time.Sleep(500 * time.Millisecond)
	
	// Initialize the watcher with a 3-second threshold for the PoC (instead of 48 hours)
	// and check every 1 second
	threshold := 3 * time.Second
	interval := 1 * time.Second
	fmt.Printf("Configurazione Watchdog: soglia allarme = %v, intervallo controlli = %v\n", threshold, interval)
	
	w := watch.NewWatcher(threshold, interval)
	w.AddPeer("nodo-marco", mockServerAddr)
	w.Start()
	
	fmt.Println("\nFase 1: Il nodo di Marco è ONLINE. Il watchdog registra i battiti...")
	time.Sleep(4 * time.Second) // Let it ping a few times successfully
	
	fmt.Println("\nFase 2: Il nodo di Marco va OFFLINE. (Simulazione rottura o ransomware che stacca la rete)...")
	mockServer.Close() // Shut down the server
	
	fmt.Println("In attesa che scatti la soglia di tolleranza...")
	
	// Wait long enough for the threshold to pass and the alert to fire
	time.Sleep(5 * time.Second)
	
	w.Stop()
	fmt.Println("\n=== Test Completato ===")
}
