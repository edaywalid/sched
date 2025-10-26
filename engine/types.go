package engine

type WorkflowTask struct {
	WorkflowID string
	Name       string
	Input      any
}

type Signal struct {
	Name  string
	Input []byte
}
