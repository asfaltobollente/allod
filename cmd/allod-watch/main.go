package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/asfaltobollente/allod/internal/version"
	"github.com/asfaltobollente/allod/internal/watch"
	"github.com/spf13/cobra"
)

var (
	cfgPath string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "allod-watch",
		Short: "Allod Watch Sentinel — Monitoraggio remoto esterno e notifiche Telegram per nodi Allod",
	}

	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "watch.yaml", "Percorso del file di configurazione")

	// 1. Run Command
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Avvia il demone di monitoraggio sentinella in primo piano",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := watch.LoadConfig(cfgPath)
			if err != nil {
				log.Fatalf("Errore caricamento configurazione (%s): %v", cfgPath, err)
			}

			fmt.Println("==================================================")
			fmt.Printf("🛡️  Allod Watch Sentinel v%s\n", version.Get())
			fmt.Println("==================================================")

			var tg *watch.TelegramNotifier
			if cfg.Telegram.Enabled && cfg.Telegram.BotToken != "" && cfg.Telegram.ChatID != "" {
				tg = watch.NewTelegramNotifier(cfg.Telegram.BotToken, cfg.Telegram.ChatID, cfg.Telegram.TopicID)
				fmt.Printf("✓ Notifiche Telegram: Abilitate (Chat ID: %s)\n", cfg.Telegram.ChatID)
			} else {
				fmt.Println("ℹ️  Notifiche Telegram: Disabilitate (configura bot_token e chat_id)")
			}

			var weather *watch.WeatherClient
			if cfg.Weather.Enabled {
				weather = watch.NewWeatherClient()
				fmt.Printf("✓ Meteo del giorno: Abilitato (Città: %s)\n", cfg.Weather.City)
			}

			watcher := watch.NewWatcher(cfg.Intervals.DownThresholdDuration(), cfg.Intervals.CheckDuration())

			if tg != nil {
				watcher.OnAlert = func(nodeID, reason string, downtime time.Duration) {
					log.Printf("🚨 Invio allarme Telegram per nodo %s...", nodeID)
					if err := tg.SendAlert(nodeID, reason, downtime); err != nil {
						log.Printf("Errore invio allarme Telegram: %v", err)
					}
				}

				watcher.OnRecover = func(nodeID string, totalDowntime time.Duration) {
					log.Printf("💚 Invio notifica di rientro Telegram per nodo %s...", nodeID)
					if err := tg.SendRecovery(nodeID, totalDowntime); err != nil {
						log.Printf("Errore invio rientro Telegram: %v", err)
					}
				}

				watcher.OnWarning = func(nodeID, title, details string) {
					log.Printf("⚠️  Invio avviso Telegram per nodo %s: %s", nodeID, title)
					if err := tg.SendWarning(nodeID, title, details); err != nil {
						log.Printf("Errore invio avviso Telegram: %v", err)
					}
				}
			}

			for _, n := range cfg.Nodes {
				watcher.AddNode(n.ID, n.URL, n.Token)
				fmt.Printf("✓ Monitoraggio registrato: [%s] -> %s\n", n.ID, n.URL)
			}

			watcher.Start()
			fmt.Printf("✓ Ciclo di controllo attivo (intervallo: %v, soglia allarme: %v)\n",
				cfg.Intervals.CheckDuration(), cfg.Intervals.DownThresholdDuration())

			var digestScheduler *watch.DailyDigestScheduler
			if cfg.Digest.Enabled && tg != nil {
				h, m := cfg.Digest.ParseDigestTime()
				digestScheduler = watch.NewDailyDigestScheduler(h, m, cfg.Weather.City, watcher, tg, weather)
				digestScheduler.Start()
				fmt.Printf("✓ Resoconto mattutino programmato per le %02d:%02d ogni giorno\n", h, m)
			}

			fmt.Println("\nSentinella operativa. Premi Ctrl+C per terminare.")

			// Wait for OS termination signal
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
			<-sigChan

			fmt.Println("\nRicevuto segnale di arresto, chiusura sentinella...")
			watcher.Stop()
			if digestScheduler != nil {
				digestScheduler.Stop()
			}
			fmt.Println("✓ Sentinella arrestata correttamente.")
		},
	}

	// 2. Test Telegram Command
	testTgCmd := &cobra.Command{
		Use:   "test-telegram",
		Short: "Invia un messaggio di prova su Telegram per verificare le credenziali del bot",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := watch.LoadConfig(cfgPath)
			if err != nil {
				log.Fatalf("Errore caricamento configurazione (%s): %v", cfgPath, err)
			}

			if cfg.Telegram.BotToken == "" || cfg.Telegram.ChatID == "" {
				log.Fatal("Errore: bot_token o chat_id mancanti nel file di configurazione o nelle variabili d'ambiente.")
			}

			fmt.Printf("Invio messaggio di prova a Telegram (Chat ID: %s)...\n", cfg.Telegram.ChatID)
			tg := watch.NewTelegramNotifier(cfg.Telegram.BotToken, cfg.Telegram.ChatID, cfg.Telegram.TopicID)

			if err := tg.Test(); err != nil {
				log.Fatalf("✗ Fallito: %v\nAssicurati di aver avviato la chat con il bot premendo 'AVVIA' (/start).", err)
			}

			fmt.Println("✓ Messaggio inviato con successo! Controlla la tua app Telegram.")
		},
	}

	// 3. Test Digest Command
	testDigestCmd := &cobra.Command{
		Use:   "test-digest",
		Short: "Invia immediatamente un resoconto mattutino di prova (con meteo) su Telegram",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := watch.LoadConfig(cfgPath)
			if err != nil {
				log.Fatalf("Errore configurazione: %v", err)
			}

			if cfg.Telegram.BotToken == "" || cfg.Telegram.ChatID == "" {
				log.Fatal("Errore: bot_token o chat_id mancanti.")
			}

			fmt.Println("Generazione del resoconto Allod con meteo di prova...")
			tg := watch.NewTelegramNotifier(cfg.Telegram.BotToken, cfg.Telegram.ChatID, cfg.Telegram.TopicID)
			weather := watch.NewWeatherClient()
			watcher := watch.NewWatcher(cfg.Intervals.DownThresholdDuration(), cfg.Intervals.CheckDuration())

			for _, n := range cfg.Nodes {
				watcher.AddNode(n.ID, n.URL, n.Token)
			}

			h, m := cfg.Digest.ParseDigestTime()
			scheduler := watch.NewDailyDigestScheduler(h, m, cfg.Weather.City, watcher, tg, weather)

			if err := scheduler.TriggerNow(); err != nil {
				log.Fatalf("Errore generazione digest: %v", err)
			}

			fmt.Println("✓ Resoconto inviato con successo su Telegram!")
		},
	}

	// 4. Get Chat ID helper
	getChatIDCmd := &cobra.Command{
		Use:   "get-chat-id",
		Short: "Rileva automaticamente il tuo Chat ID Telegram leggendo i messaggi inviati al bot",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, _ := watch.LoadConfig(cfgPath)
			token := cfg.Telegram.BotToken
			if token == "" {
				log.Fatal("Errore: specifica bot_token in watch.yaml o nella variabile TELEGRAM_BOT_TOKEN.")
			}

			fmt.Println("Interrogazione Telegram per recuperare il tuo Chat ID...")
			fmt.Println("(Se la lista è vuota, apri prima Telegram, cerca il tuo bot e scrivigli qualsiasi cosa o premi /start)")
			fmt.Println()

			chats, err := watch.GetChatIDs(token)
			if err != nil {
				log.Fatalf("Errore: %v", err)
			}

			if len(chats) == 0 {
				fmt.Println("Nessun messaggio recente trovato. Apri il tuo bot su Telegram, premi AVVIA e riprova questo comando!")
				return
			}

			fmt.Println("Chat trovate:")
			for _, c := range chats {
				fmt.Printf("  👉 %s\n", c)
			}
			fmt.Println("\nCopia il numero identificativo (es. 123456789) nel campo 'chat_id' del tuo watch.yaml!")
		},
	}

	// 5. Version
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Stampa la versione di Allod Watch Sentinel",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("allod-watch version %s\n", version.Get())
		},
	}

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(testTgCmd)
	rootCmd.AddCommand(testDigestCmd)
	rootCmd.AddCommand(getChatIDCmd)
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

