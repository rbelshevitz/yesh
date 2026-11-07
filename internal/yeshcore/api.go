package yeshcore

// Plan parses source and renders the same non-executing plan used by yesh.
// Keeping this boundary in the shared package prevents companion tools from
// reimplementing the parser or inventing a second plan format.
func Plan(source, filename string) (string, error) {
	items, err := parse(source, filename)
	if err != nil {
		return "", err
	}
	return renderPlan(items, filename)
}

type ServiceResourceState = serviceResourceState
type ServiceResourceStateSample = serviceResourceStateSample

const ServiceResourceStateSchema = serviceResourceStateSchema

func DefaultServiceResourceStateDir(unit string) (string, error) {
	return defaultServiceResourceStateDir(unit)
}

func ServiceResourceJournalPath(statePath string) string {
	return serviceResourceJournalPath(statePath)
}

func ReadServiceResourceJournal(path string) ([]ServiceResourceState, error) {
	return readServiceResourceJournal(path)
}

func WriteServiceResourceState(path string, state ServiceResourceState) error {
	_, err := writeServiceResourceState(&runtime{values: map[string]value{}}, path, state)
	return err
}

func SystemctlArgv(manager string, args ...string) []string {
	return systemctlArgv(manager, args...)
}
