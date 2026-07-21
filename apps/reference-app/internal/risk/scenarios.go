package risk

// Default scenario names used by the POC and SimulationManifest.
const (
	ScenarioFraudApproved = "fraud-approved"
	ScenarioFraudDeclined = "fraud-declined"
)

// MapScenarioStore is an in-memory ScenarioStore.
type MapScenarioStore map[string]Scenario

// Lookup implements ScenarioStore.
func (m MapScenarioStore) Lookup(name string) (Scenario, bool) {
	sc, ok := m[name]
	return sc, ok
}

// DefaultScenarios mirrors simulation/microcks-scenarios/*.yaml.
func DefaultScenarios() MapScenarioStore {
	return MapScenarioStore{
		ScenarioFraudApproved: {Score: 5, Decision: "APPROVE"},
		ScenarioFraudDeclined: {
			Score:    92,
			Decision: "DECLINE",
			Factors:  []string{"VELOCITY_BREACH", "HIGH_AMOUNT"},
		},
	}
}
