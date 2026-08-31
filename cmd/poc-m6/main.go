package main

import (
	"fmt"
	"strings"

	"github.com/allod-project/allod/internal/ring"
)

func main() {
	fmt.Println("=== Inizio Test M6: Il Gruppo a Tre Nodi & Topologia Ring ===")

	// 1. Fase 1: Gruppo iniziale a 2 nodi (Luca & Marco)
	fmt.Println("\n--- FASE 1: Gruppo a 2 Nodi (Luca & Marco) ---")
	topo := ring.NewRingTopology("allod-ring-demo", 2)

	luca := &ring.Member{
		ID:      "allod-luca",
		Address: "100.64.0.1",
		QuotaGB: 500,
		Datasets: []ring.Dataset{
			{ID: "photos", SizeGB: 40, Critical: true},
			{ID: "documents", SizeGB: 10, Critical: true},
		},
	}
	marco := &ring.Member{
		ID:      "allod-marco",
		Address: "100.64.0.2",
		QuotaGB: 500,
		Datasets: []ring.Dataset{
			{ID: "media", SizeGB: 50, Critical: true},
		},
	}

	topo.AddMember(luca)
	topo.AddMember(marco)

	placements2 := topo.CalculatePlacement()
	fmt.Printf("Membri: %d | Target Repliche: %d\n", len(topo.Members), topo.TargetReplicas)
	for key, p := range placements2 {
		fmt.Printf("  • Dataset %-22s -> Repliche su: %-15s (Stato: %s)\n",
			key, fmt.Sprintf("[%s]", strings.Join(p.TargetNodes, ", ")), p.Status)
	}

	// 2. Fase 2: Ingresso del 3° nodo (Sara) - Zero Config per Luca e Marco
	fmt.Println("\n--- FASE 2: Ingresso Zero-Config del 3° Nodo (Sara) ---")
	sara := &ring.Member{
		ID:      "allod-sara",
		Address: "100.64.0.3",
		QuotaGB: 500,
		Datasets: []ring.Dataset{
			{ID: "design-work", SizeGB: 35, Critical: true},
		},
	}

	// Sara joins the group ring definition
	topo.AddMember(sara)

	placements3 := topo.CalculatePlacement()
	fmt.Printf("Membri: %d | Target Repliche: %d\n", len(topo.Members), topo.TargetReplicas)

	allHave2Replicas := true
	for key, p := range placements3 {
		fmt.Printf("  • Dataset %-22s (%2d GB) -> Repliche su: %-25s | Stato: %s\n",
			key, p.SizeGB, fmt.Sprintf("[%s]", strings.Join(p.TargetNodes, ", ")), p.Status)

		if p.Critical && p.TargetCount < 2 {
			allHave2Replicas = false
		}
	}

	if allHave2Replicas {
		fmt.Println("\n✅ SUCCESSO: Regola delle 2 repliche verificata! Ogni dataset critico è salvato su 2 nodi remoti indipendenti.")
	} else {
		fmt.Println("\n❌ ERRORE: Uno o più dataset critici non hanno 2 repliche.")
	}

	// 3. Fase 3: Simulazione Rimozione / Guasto di Marco
	fmt.Println("\n--- FASE 3: Simulazione Uscita/Guasto di un Membro (allod-marco) ---")
	impact, err := topo.SimulateRemoval("allod-marco")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Stato Quorum: %s\n", impact.QuorumHealth)
	fmt.Println("Dataset Primari persi sul nodo rimosso:")
	for _, ds := range impact.LostPrimaryDatasets {
		fmt.Printf("  • %s\n", ds)
	}

	fmt.Println("Dataset degradati che necessitano riallocazione:")
	for _, ds := range impact.DegradedDatasets {
		fmt.Printf("  ⚠️ %s\n", ds)
	}

	fmt.Println("Piano di Riallocazione sui nodi superstiti:")
	for _, act := range impact.RebalanceActions {
		fmt.Printf("  -> %s\n", act)
	}

	fmt.Printf("Capacità Residua Superstiti: %d GB usati su %d GB totali\n",
		impact.TotalUsedRemaining, impact.TotalQuotaRemaining)

	fmt.Println("\n=== Test M6 Completato con Successo! ===")
}
