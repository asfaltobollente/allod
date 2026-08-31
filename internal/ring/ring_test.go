package ring

import (
	"testing"
)

func TestRingTwoNodesPlacement(t *testing.T) {
	topo := NewRingTopology("test-ring-2", 2)
	topo.AddMember(&Member{
		ID:      "node-a",
		QuotaGB: 500,
		Datasets: []Dataset{
			{ID: "data1", SizeGB: 50, Critical: true},
		},
	})
	topo.AddMember(&Member{
		ID:      "node-b",
		QuotaGB: 500,
		Datasets: []Dataset{
			{ID: "data2", SizeGB: 30, Critical: true},
		},
	})

	placements := topo.CalculatePlacement()
	if len(placements) != 2 {
		t.Fatalf("expected 2 dataset placements, got %d", len(placements))
	}

	pA := placements["node-a:data1"]
	if len(pA.TargetNodes) != 1 || pA.TargetNodes[0] != "node-b" {
		t.Errorf("expected node-a:data1 placed on node-b, got %v", pA.TargetNodes)
	}

	pB := placements["node-b:data2"]
	if len(pB.TargetNodes) != 1 || pB.TargetNodes[0] != "node-a" {
		t.Errorf("expected node-b:data2 placed on node-a, got %v", pB.TargetNodes)
	}
}

func TestRingThreeNodesTwoReplicas(t *testing.T) {
	topo := NewRingTopology("test-ring-3", 2)
	topo.AddMember(&Member{
		ID:      "node-a",
		QuotaGB: 500,
		Datasets: []Dataset{
			{ID: "data1", SizeGB: 40, Critical: true},
		},
	})
	topo.AddMember(&Member{
		ID:      "node-b",
		QuotaGB: 500,
		Datasets: []Dataset{
			{ID: "data2", SizeGB: 50, Critical: true},
		},
	})
	topo.AddMember(&Member{
		ID:      "node-c",
		QuotaGB: 500,
		Datasets: []Dataset{
			{ID: "data3", SizeGB: 20, Critical: true},
		},
	})

	placements := topo.CalculatePlacement()
	for key, p := range placements {
		if p.TargetCount != 2 {
			t.Errorf("expected 2 replicas for dataset %s, got %d (targets: %v)", key, p.TargetCount, p.TargetNodes)
		}
		// Verify anti-affinity: owner node is never in target nodes
		for _, target := range p.TargetNodes {
			if target == p.OwnerNode {
				t.Errorf("anti-affinity violation: dataset %s has target on owner %s", key, target)
			}
		}
	}
}

func TestSimulateRemoval(t *testing.T) {
	topo := NewRingTopology("test-ring-sim", 2)
	topo.AddMember(&Member{
		ID:      "node-a",
		QuotaGB: 500,
		Datasets: []Dataset{
			{ID: "data1", SizeGB: 40, Critical: true},
		},
	})
	topo.AddMember(&Member{
		ID:      "node-b",
		QuotaGB: 500,
		Datasets: []Dataset{
			{ID: "data2", SizeGB: 50, Critical: true},
		},
	})
	topo.AddMember(&Member{
		ID:      "node-c",
		QuotaGB: 500,
		Datasets: []Dataset{
			{ID: "data3", SizeGB: 20, Critical: true},
		},
	})

	impact, err := topo.SimulateRemoval("node-b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(impact.LostPrimaryDatasets) != 1 {
		t.Errorf("expected 1 lost primary dataset, got %d", len(impact.LostPrimaryDatasets))
	}
	if len(impact.DegradedDatasets) != 2 {
		t.Errorf("expected 2 degraded datasets losing replica on node-b, got %d", len(impact.DegradedDatasets))
	}
}
