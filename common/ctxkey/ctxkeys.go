package ctxkey

type sharContextKey string

// WorkflowInstanceID - the context key for the workflow instance ID
var WorkflowInstanceID = sharContextKey("WORKFLOW_INSTANCE_ID")

// ExecutionID - the context key for the execution ID
var ExecutionID = sharContextKey("EXECUTION_ID")

// ProcessInstanceID - the context key for the process instance ID
var ProcessInstanceID = sharContextKey("PROCESS_INSTANCE_ID")

// TrackingID - the context key for the workflow tracking ID
var TrackingID = sharContextKey("WF_TRACKING_ID")

// SharUser - the context key for the currently authenticated user
var SharUser = sharContextKey("WF_USER")

// APIFunc - the context key for the currently executing API function
var APIFunc = sharContextKey("API_FN")
