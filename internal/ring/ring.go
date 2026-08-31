package ring

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

type Dataset struct {
	ID       string `yaml:"id" json:"id"`
	SizeGB   int    `yaml:"size_gb" json:"size_gb"`
	Critical bool   `yaml:"critical" json:"critical"`
}

type Member struct {
	ID       string    `yaml:"id" json:"id"`
	Address  string    `yaml:"address" json:"address"`
	QuotaGB  int       `yaml:"quota_gb" json:"quota_gb"`
	Datasets []Dataset `yaml:"datasets" json:"datasets"`
}

type RingTopology struct {
	Name           string             `yaml:"name" json:"name"`
	TargetReplicas int                `yaml:"target_replicas" json:"target_replicas"`
	Members        map[string]*Member `yaml:"members" json:"members"`
}

type ReplicaAssignment struct {
	DatasetID     string   `json:"dataset_id"`
	OwnerNode     string   `json:"owner_node"`
	SizeGB        int      `json:"size_gb"`
	Critical      bool     `json:"critical"`
	TargetNodes   []string `json:"target_nodes"`
	TargetCount   int      `json:"target_count"`
	RequiredCount int      `json:"required_count"`
	Status        string   `json:"status"`
}

type RemovalImpact struct {
	LostPrimaryDatasets []string       `json:"lost_primary_datasets"`
	DegradedDatasets    []string       `json:"degraded_datasets"`
	RebalanceActions    []string       `json:"rebalance_actions"`
	SurvivingMembers    map[string]int `json:"surviving_members"`
	TotalQuotaRemaining int            `json:"total_quota_remaining"`
	TotalUsedRemaining  int            `json:"total_used_remaining"`
	QuorumHealth        string         `json:"quorum_health"`
}

func NewRingTopology(name string, targetReplicas int) *RingTopology {
	if targetReplicas <= 0 {
		targetReplicas = 2
	}
	return &RingTopology{
		Name:           name,
		TargetReplicas: targetReplicas,
		Members:        make(map[string]*Member),
	}
}

func LoadTopology(path string) (*RingTopology, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var topo RingTopology
	if err := yaml.Unmarshal(data, &topo); err != nil {
		return nil, err
	}
	if topo.TargetReplicas <= 0 {
		topo.TargetReplicas = 2
	}
	return &topo, nil
}

func (r *RingTopology) Save(path string) error {
	data, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (r *RingTopology) AddMember(m *Member) {
	r.Members[m.ID] = m
}

func (r *RingTopology) RemoveMember(id string) {
	delete(r.Members, id)
}

// CalculatePlacement determines replica distribution across the ring
func (r *RingTopology) CalculatePlacement() map[string]ReplicaAssignment {
	placements := make(map[string]ReplicaAssignment)

	numMembers := len(r.Members)
	if numMembers == 0 {
		return placements
	}

	// Capacity tracking per member
	allocatedStorage := make(map[string]int)
	for id := range r.Members {
		allocatedStorage[id] = 0
	}

	// Gather all datasets
	type datasetWithOwner struct {
		dataset Dataset
		ownerID string
	}
	var allDatasets []datasetWithOwner
	for ownerID, member := range r.Members {
		for _, ds := range member.Datasets {
			allDatasets = append(allDatasets, datasetWithOwner{
				dataset: ds,
				ownerID: ownerID,
			})
		}
	}

	// Sort datasets by size descending for best-fit placement
	sort.Slice(allDatasets, func(i, j int) bool {
		return allDatasets[i].dataset.SizeGB > allDatasets[j].dataset.SizeGB
	})

	for _, item := range allDatasets {
		ds := item.dataset
		ownerID := item.ownerID
		key := fmt.Sprintf("%s:%s", ownerID, ds.ID)

		// Required replicas: targetReplicas, capped at available remote nodes
		maxPossibleRemote := numMembers - 1
		reqReplicas := r.TargetReplicas
		if reqReplicas > maxPossibleRemote {
			reqReplicas = maxPossibleRemote
		}
		if reqReplicas < 0 {
			reqReplicas = 0
		}

		// Candidate nodes (anti-affinity: cannot store replica on the owner node)
		type candidate struct {
			id       string
			availCap int
		}
		var candidates []candidate
		for id, m := range r.Members {
			if id != ownerID {
				avail := m.QuotaGB - allocatedStorage[id]
				candidates = append(candidates, candidate{id: id, availCap: avail})
			}
		}

		// Sort candidates by available capacity descending
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].availCap > candidates[j].availCap
		})

		var chosenTargets []string
		for _, c := range candidates {
			if len(chosenTargets) >= reqReplicas {
				break
			}
			if c.availCap >= ds.SizeGB {
				chosenTargets = append(chosenTargets, c.id)
				allocatedStorage[c.id] += ds.SizeGB
			}
		}

		status := "HEALTHY"
		if len(chosenTargets) < reqReplicas || (ds.Critical && len(chosenTargets) < r.TargetReplicas) {
			if len(chosenTargets) == 0 {
				status = "CRITICAL (0 repliche remote)"
			} else {
				status = fmt.Sprintf("DEGRADATO (%d/%d repliche)", len(chosenTargets), r.TargetReplicas)
			}
		} else {
			status = fmt.Sprintf("OK (%d/%d repliche)", len(chosenTargets), r.TargetReplicas)
		}

		placements[key] = ReplicaAssignment{
			DatasetID:     ds.ID,
			OwnerNode:     ownerID,
			SizeGB:        ds.SizeGB,
			Critical:      ds.Critical,
			TargetNodes:   chosenTargets,
			TargetCount:   len(chosenTargets),
			RequiredCount: r.TargetReplicas,
			Status:        status,
		}
	}

	return placements
}

// SimulateRemoval simulates what happens if a member is removed from the ring
func (r *RingTopology) SimulateRemoval(memberID string) (*RemovalImpact, error) {
	removedMember, exists := r.Members[memberID]
	if !exists {
		return nil, fmt.Errorf("membro '%s' non trovato nel ring", memberID)
	}

	currentPlacements := r.CalculatePlacement()

	// Build temporary topology without the member
	tempRing := NewRingTopology(r.Name, r.TargetReplicas)
	for id, m := range r.Members {
		if id != memberID {
			// Deep copy member
			datasetsCopy := make([]Dataset, len(m.Datasets))
			copy(datasetsCopy, m.Datasets)
			tempRing.AddMember(&Member{
				ID:       m.ID,
				Address:  m.Address,
				QuotaGB:  m.QuotaGB,
				Datasets: datasetsCopy,
			})
		}
	}

	newPlacements := tempRing.CalculatePlacement()

	impact := &RemovalImpact{
		SurvivingMembers: make(map[string]int),
	}

	// 1. Lost primary datasets (datasets owned by the removed member)
	for _, ds := range removedMember.Datasets {
		impact.LostPrimaryDatasets = append(impact.LostPrimaryDatasets, fmt.Sprintf("%s (di %s, %d GB)", ds.ID, memberID, ds.SizeGB))
	}

	// 2. Degraded datasets (datasets that lost a replica previously on memberID)
	for _, curr := range currentPlacements {
		if curr.OwnerNode == memberID {
			continue // Handled in lost primary
		}
		hasTarget := false
		for _, target := range curr.TargetNodes {
			if target == memberID {
				hasTarget = true
				break
			}
		}
		if hasTarget {
			impact.DegradedDatasets = append(impact.DegradedDatasets, fmt.Sprintf("%s:%s (%d GB)", curr.OwnerNode, curr.DatasetID, curr.SizeGB))
		}
	}

	// 3. Rebalance actions
	for _, newPlace := range newPlacements {
		if newPlace.TargetCount > 0 {
			impact.RebalanceActions = append(impact.RebalanceActions,
				fmt.Sprintf("Dataset %s:%s -> Repliche riassegnate a: %v", newPlace.OwnerNode, newPlace.DatasetID, newPlace.TargetNodes))
		} else {
			impact.RebalanceActions = append(impact.RebalanceActions,
				fmt.Sprintf("⚠️ Dataset %s:%s -> Nessun nodo disponibile per ospitare la replica!", newPlace.OwnerNode, newPlace.DatasetID))
		}
	}

	// 4. Remaining capacity
	for _, m := range tempRing.Members {
		impact.TotalQuotaRemaining += m.QuotaGB
	}
	for _, p := range newPlacements {
		for _, t := range p.TargetNodes {
			impact.SurvivingMembers[t] += p.SizeGB
			impact.TotalUsedRemaining += p.SizeGB
		}
	}

	// 5. Quorum Health
	survivorCount := len(tempRing.Members)
	if survivorCount >= 2 {
		impact.QuorumHealth = fmt.Sprintf("QUORUM PRESERVATO (%d nodi sopravvissuti, federazione attiva)", survivorCount)
	} else {
		impact.QuorumHealth = "QUORUM PERSO (Meno di 2 nodi rimasti nel ring, backup federato disattivato)"
	}

	return impact, nil
}
