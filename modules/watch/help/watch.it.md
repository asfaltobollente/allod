# Modulo Watchdog (Monitoraggio Risorse e Processi)

Il modulo `watch` fornisce il monitoraggio continuo in background delle risorse di sistema, dello stato dei container e dell'integrità dello storage.

## Invarianti Monitorate

* **Riserva di RAM**: Assicura che l'utilizzo della memoria rimanga entro le soglie sicure, preservando i 2.0 GB riservati al sistema operativo.
* **Invarianti di Storage**: Controlla lo spazio libero sul pool Btrfs e avvisa prima che l'esaurimento dello storage causi problemi al copy-on-write.
* **Vitalità dei Processi**: Verifica che i servizi utente Quadlet e il demone helper siano attivi e rispondenti.
