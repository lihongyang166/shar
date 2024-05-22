package workflow

// AbortType represents the type of termination being handled by the abort function
type AbortType int

const (
	AbortTypeActivity    = iota // AbortTypeActivity signifies an activity is being aborted
	AbortTypeServiceTask = iota // AbortTypeServiceTask signifies a service task is being aborted
)
