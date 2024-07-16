package keys

// ContextKey is the wrapper for using context keys
type ContextKey string

const (
	// ElementName is the key of the currently executing workflow element.
	ElementName = "el_name"
	// ElementID is the key for the workflow element ID.
	ElementID = "el_id"
	// ElementType is the key for the BPMN name for the element.
	ElementType = "el_type"
	// ProcessInstanceID is the key for the unique identifier for the executing workflow instance.
	ProcessInstanceID = "pi_id"
	// WorkflowID is the key for the originating versioned workflow that started the instance.
	WorkflowID = "wf_id"
	// ActivityID is the key for the name of the activity ID associated with an executing Job.
	ActivityID = "job_activity_id"
	// JobType is the key for the type of executing job.
	JobType = "job_type"
	// JobID is the key for the executing job ID.
	JobID = "job_id"
	// ParentProcessInstanceID is the key for the parent process instance if this is a spawned process
	ParentProcessInstanceID = "parent_pi_id"
	// WorkflowName is the key for the originator workflow name.
	WorkflowName = "wf_name"
	// ParentInstanceElementID is the key for the element in the parent instance that launched a sub workflow instance.
	ParentInstanceElementID = "parent_el_id"
	// Execute is a key that is used for the execution command of a Job or workflow activity.
	Execute = "wf_ex"
	// Condition is a key for a business rule to evaluate.
	Condition = "el_cond"
	// State is a key for the description of the current execution state of a workflow instance.
	State = "el_state"
	// TrackingID is a key for the unique ID for the executing state.
	TrackingID = "tr_id"
	// ParentTrackingID is a key for the parent of the current executing state.
	ParentTrackingID = "parent_tr_id"
	// ExecutionID is a key to correlate a process and any spawned sub process with each other
	ExecutionID = "exec_id"
	// ProcessId is a key for the unique id of a process
	ProcessId = "p_id"
)
