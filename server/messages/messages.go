package messages

import (
	"gitlab.com/shar-workflow/shar/common/subj"
)

const (
	StateExecutionComplete      = ".State.Execution.Complete"       // StateExecutionComplete is the execution complete state
	StateProcessExecute         = ".State.Process.Execute"          // StateProcessExecute is the process execute state
	StateProcessComplete        = ".State.Process.Complete"         // StateProcessComplete is the process complete state
	StateProcessTerminated      = ".State.Process.Terminated"       // StateProcessTerminated is the process terminated state
	StateTraversalExecute       = ".State.Traversal.Execute"        //StateTraversalExecute  is the traversal execute state
	StateTraversalComplete      = ".State.Traversal.Complete"       // StateTraversalComplete is the traversal complete state
	StateActivityExecute        = ".State.Activity.Execute"         // StateActivityExecute is the activity execute state
	StateActivityComplete       = ".State.Activity.Complete"        // StateActivityComplete is the activity complete state
	StateActivityAbort          = ".State.Activity.Abort"           // StateActivityAbort is the activity abort state
	StateExecutionExecute       = ".State.Execution.Execute"        // StateExecutionExecute is the execution execute state
	StateExecutionAbort         = ".State.Execution.Abort"          // StateExecutionAbort is the execute abort state
	StateJobExecute             = ".State.Job.Execute."             //StateJobExecute is the job execute state
	StateJobExecuteServiceTask  = ".State.Job.Execute.ServiceTask"  // StateJobExecuteServiceTask is the execute service task state
	StateJobAbortServiceTask    = ".State.Job.Abort.ServiceTask"    // StateJobAbortServiceTask is the abort service task state
	StateJobCompleteServiceTask = ".State.Job.Complete.ServiceTask" // StateJobCompleteServiceTask is the complete service task state
	StateJobAbortGateway        = ".State.Job.Abort.Gateway"        // StateJobAbortGateway is the abort gateway state
	StateJobComplete            = ".State.Job.Complete."            // StateJobComplete is the complete job state
	StateJobExecuteUserTask     = ".State.Job.Execute.UserTask"     // StateJobExecuteUserTask is the execute user task state
	StateJobCompleteUserTask    = ".State.Job.Complete.UserTask"    // StateJobCompleteUserTask is the complete user task state
	StateJobExecuteManualTask   = ".State.Job.Execute.ManualTask"   // StateJobExecuteManualTask is the execute manual task state
	StateJobCompleteManualTask  = ".State.Job.Complete.ManualTask"  // StateJobCompleteManualTask is the complete manual task state
	StateJobExecuteSendMessage  = ".State.Job.Execute.SendMessage"  // StateJobExecuteSendMessage is the execute send message state
	StateJobCompleteSendMessage = ".State.Job.Complete.SendMessage" // StateJobCompleteSendMessage is the complete send message state
	StateLog                    = ".State.Log."                     // StateLog is the log
)
const (
	WorkFlowJobAbortAll               = "WORKFLOW.%s.State.Job.Abort.*"                   // WorkFlowJobAbortAll is the wildcard state message subject for all job abort messages.
	WorkFlowJobCompleteAll            = "WORKFLOW.%s" + StateJobComplete + "*"            // WorkFlowJobCompleteAll is the wildcard state message subject for all job completion messages.
	WorkflowActivityAbort             = "WORKFLOW.%s" + StateActivityAbort                // WorkflowActivityAbort is the state message subject for aborting an activity.
	WorkflowActivityAll               = "WORKFLOW.%s.State.Activity.>"                    // WorkflowActivityAll is the wildcard state message subject for all activity messages.
	WorkflowActivityComplete          = "WORKFLOW.%s" + StateActivityComplete             // WorkflowActivityComplete is the state message subject for completing an activity.
	WorkflowActivityExecute           = "WORKFLOW.%s" + StateActivityExecute              // WorkflowActivityExecute is the state message subject for executing an activity.
	WorkflowCommands                  = "WORKFLOW.%s.Command.>"                           // WorkflowCommands is the wildcard state message subject for all workflow commands.
	WorkflowElementTimedExecute       = "WORKFLOW.%s.Timers.ElementExecute"               // WorkflowElementTimedExecute is the state message subject for a timed element execute operation.
	WorkflowGeneralAbortAll           = "WORKFLOW.%s.State.*.Abort"                       // WorkflowGeneralAbortAll is the wildcard state message subject for all abort messages/.
	WorkflowExecutionAbort            = "WORKFLOW.%s" + StateExecutionAbort               // WorkflowExecutionAbort is the state message subject for an execution instace being aborted.
	WorkflowExecutionAll              = "WORKFLOW.%s.State.Execution.>"                   // WorkflowExecutionAll is the wildcard state message subject for all execution state messages.
	WorkflowExecutionComplete         = "WORKFLOW.%s" + StateExecutionComplete            // WorkflowExecutionComplete is the state message subject for completing an execution instance.
	WorkflowExecutionExecute          = "WORKFLOW.%s" + StateExecutionExecute             // WorkflowExecutionExecute is the state message subject for executing an execution instance.
	ExecutionTerminated               = "WORKFLOW.%s.State.Execution.Terminated"          // ExecutionTerminated is the state message subject for an execution instance terminating.
	WorkflowJobAwaitMessageExecute    = "WORKFLOW.%s" + StateJobExecute + "AwaitMessage"  // WorkflowJobAwaitMessageExecute is the state message subject for awaiting a message.
	WorkflowJobAwaitMessageComplete   = "WORKFLOW.%s.State.Job.Complete.AwaitMessage"     // WorkflowJobAwaitMessageComplete is the state message subject for completing awaiting a message.
	WorkflowJobAwaitMessageAbort      = "WORKFLOW.%s.State.Job.Abort.AwaitMessage"        // WorkflowJobAwaitMessageAbort is the state message subject for aborting awaiting a message.
	WorkflowJobLaunchComplete         = "WORKFLOW.%s.State.Job.Complete.Launch"           // WorkflowJobLaunchComplete is the state message subject for completing a launch subworkflow task.
	WorkflowJobLaunchExecute          = "WORKFLOW.%s" + StateJobExecute + "Launch"        // WorkflowJobLaunchExecute is the state message subject for executing a launch subworkflow task.
	WorkflowJobManualTaskAbort        = "WORKFLOW.%s.State.Job.Abort.ManualTask"          // WorkflowJobManualTaskAbort is the state message subject for sborting a manual task.
	WorkflowJobManualTaskComplete     = "WORKFLOW.%s" + StateJobCompleteManualTask        // WorkflowJobManualTaskComplete is the state message subject for completing a manual task.
	WorkflowJobManualTaskExecute      = "WORKFLOW.%s" + StateJobExecute + "ManualTask"    // WorkflowJobManualTaskExecute is the state message subject for executing a manual task.
	WorkflowJobSendMessageComplete    = "WORKFLOW.%s" + StateJobCompleteSendMessage       // WorkflowJobSendMessageComplete is the state message subject for completing a send message task.
	WorkflowJobSendMessageExecute     = "WORKFLOW.%s" + StateJobExecute + "SendMessage"   // WorkflowJobSendMessageExecute is the state message subject for executing a send workfloe message task.
	WorkflowJobSendMessageExecuteWild = "WORKFLOW.%s" + StateJobExecute + "SendMessage.>" // WorkflowJobSendMessageExecuteWild is the wildcard state message subject for executing a send workfloe message task.
	WorkflowJobServiceTaskAbort       = "WORKFLOW.%s" + StateJobAbortServiceTask          // WorkflowJobServiceTaskAbort is the state message subject for aborting an in progress service task.
	WorkflowJobServiceTaskComplete    = "WORKFLOW.%s" + StateJobCompleteServiceTask       // WorkflowJobServiceTaskComplete is the state message subject for a completed service task,
	WorkflowJobServiceTaskExecute     = "WORKFLOW.%s" + StateJobExecute + "ServiceTask"   // WorkflowJobServiceTaskExecute is the raw state message subject for executing a service task.  An identifier is added to the end to route messages to the clients.
	WorkflowJobServiceTaskExecuteWild = "WORKFLOW.%s" + StateJobExecute + "ServiceTask.>" // WorkflowJobServiceTaskExecuteWild is the wildcard state message subject for all execute service task messages.
	WorkflowJobTimerTaskComplete      = "WORKFLOW.%s.State.Job.Complete.Timer"            // WorkflowJobTimerTaskComplete is the state message subject for completing a timed task.
	WorkflowJobTimerTaskExecute       = "WORKFLOW.%s" + StateJobExecute + "Timer"         // WorkflowJobTimerTaskExecute is the state message subject for executing a timed task.
	WorkflowJobUserTaskAbort          = "WORKFLOW.%s.State.Job.Abort.UserTask"            // WorkflowJobUserTaskAbort is the state message subject for aborting a user task.
	WorkflowJobUserTaskComplete       = "WORKFLOW.%s" + StateJobCompleteUserTask          // WorkflowJobUserTaskComplete is the state message subject for completing a user task.
	WorkflowJobUserTaskExecute        = "WORKFLOW.%s" + StateJobExecute + "UserTask"      // WorkflowJobUserTaskExecute is the state message subject for executing a user task.
	WorkflowJobGatewayTaskComplete    = "WORKFLOW.%s.State.Job.Complete.Gateway"          // WorkflowJobGatewayTaskComplete is the state message subject for completing a gateway task.
	WorkflowJobGatewayTaskExecute     = "WORKFLOW.%s" + StateJobExecute + "Gateway"       // WorkflowJobGatewayTaskExecute is the state message subject for executing a gateway task.
	WorkflowJobGatewayTaskActivate    = "WORKFLOW.%s.State.Job.Activate.Gateway"          // WorkflowJobGatewayTaskActivate is the state message subject for activating a gateway task for creation or re-entry.
	WorkflowJobGatewayTaskReEnter     = "WORKFLOW.%s.State.Job.ReEnter.Gateway"           // WorkflowJobGatewayTaskReEnter is the state message subject for re entering an existing gateway task.
	WorkflowJobGatewayTaskAbort       = "WORKFLOW.%s" + StateJobAbortGateway              // WorkflowJobGatewayTaskAbort is the state message subject for aborting a gateway task.
	WorkflowLog                       = "WORKFLOW.%s" + StateLog                          // WorkflowLog is the state message subject for logging messages to a workflow activity.
	WorkflowLogAll                    = "WORKFLOW.%s" + StateLog + ".*"                   // WorkflowLogAll is the wildcard state message subject for all logging messages.
	WorkflowMessage                   = "WORKFLOW.%s.Message"                             // WorkflowMessage is the state message subject for all workflow messages.
	WorkflowProcessComplete           = "WORKFLOW.%s" + StateProcessComplete              // WorkflowProcessComplete is the state message subject for completing a workfloe process.
	WorkflowProcessExecute            = "WORKFLOW.%s" + StateProcessExecute               // WorkflowProcessExecute is the state message subject for executing a workflow process.
	WorkflowProcessTerminated         = "WORKFLOW.%s" + StateProcessTerminated            // WorkflowProcessTerminated is the state message subject for a workflow process terminating.
	WorkflowProcessCompensate         = "WORKFLOW.%s.State.Process.Compensate"            // WorkflowProcessCompensate is the state message subject for a workflow compensation event.
	WorkflowStateAll                  = "WORKFLOW.%s.State.>"                             // WorkflowStateAll is the wildcard subject for catching all state messages.
	WorkflowTimedExecute              = "WORKFLOW.%s.Timers.WorkflowExecute"              // WorkflowTimedExecute is the state message subject for timed workflow execute operation.
	WorkflowTraversalComplete         = "WORKFLOW.%s" + StateTraversalComplete            // WorkflowTraversalComplete is the state message subject for completing a traversal.
	WorkflowTraversalExecute          = "WORKFLOW.%s" + StateTraversalExecute             // WorkflowTraversalExecute is the state message subject for executing a new traversal.

	WorkflowSystemTaskCreate        = "WORKFLOW.System.Task.Create"        // WorkflowSystemTaskCreate is the task created broadcast message.
	WorkflowSystemTaskUpdate        = "WORKFLOW.System.Task.Update"        // WorkflowSystemTaskUpdate is the task updated broadcast message.
	WorkflowSystemProcessPause      = "WORKFLOW.System.Process.Pause"      // WorkflowSystemProcessPause is the process paused broadcast message.
	WorkflowSystemProcessError      = "WORKFLOW.System.Process.Error"      // WorkflowSystemProcessError is the process error broadcast message.
	WorkflowSystemHistoryArchive    = "WORKFLOW.System.History.Archive"    // WorkflowSystemHistoryArchive is the archive message for history items.
	WorkflowSystemProcessFatalError = "WORKFLOW.System.Process.FatalError" // WorkflowSystemProcessFatalError is the process fatal error broadcast message.
)

const (
	WorkflowTelemetryTimer = "WORKFLOW.Message.Telemetry" // WorkflowTelemetryTimer is the message subject for triggering telemetry messages from the server.
)

const (
	WorkflowTelemetryClientCount = "WORKFLOW_TELEMETRY.Client.Count" // WorkflowTelemetryClientCount is the message subject for workflow client count telemetry.
	WorkflowTelemetryLog         = "WORKFLOW_TELEMETRY.Log"          //WorkflowTelemetryLog is the message subject for telemetry logging.
)

// WorkflowLogLevel represents a subject suffix for logging levels
type WorkflowLogLevel string

const (
	LogFatal WorkflowLogLevel = ".Fatal"   // LogFatal is the suffix for a fatal error.
	LogError WorkflowLogLevel = ".Error"   // LogError is the suffix for an error.
	LogWarn  WorkflowLogLevel = ".Warning" // LogWarn is the suffix for a warning.
	LogInfo  WorkflowLogLevel = ".Info"    // LogInfo is the suffix for an information message.
	LogDebug WorkflowLogLevel = ".Debug"   // LogDebug is the suffix for a debug message.
)

// LogLevels provides a way of using an index to select a log level.
var LogLevels = []WorkflowLogLevel{
	LogFatal,
	LogError,
	LogWarn,
	LogInfo,
	LogDebug,
}

// AllMessages provides the list of subscriptions for the WORKFLOW stream.
var AllMessages = []string{
	//subj.NS(WorkflowAbortAll, "*"),
	subj.NS(WorkflowSystemTaskCreate, "*"),
	subj.NS(WorkflowSystemTaskUpdate, "*"),
	subj.NS(WorkflowSystemProcessPause, "*"),
	subj.NS(WorkflowSystemProcessError, "*"),

	subj.NS(WorkFlowJobAbortAll, "*"),
	subj.NS(WorkFlowJobCompleteAll, "*"),
	subj.NS(WorkflowActivityAbort, "*"),
	subj.NS(WorkflowActivityComplete, "*"),
	subj.NS(WorkflowActivityExecute, "*"),
	subj.NS(WorkflowCommands, "*"),
	subj.NS(WorkflowElementTimedExecute, "*"),
	subj.NS(WorkflowExecutionAll, "*"),
	subj.NS(WorkflowJobAwaitMessageExecute, "*"),
	subj.NS(WorkflowJobLaunchExecute, "*"),
	subj.NS(WorkflowJobManualTaskExecute, "*"),
	subj.NS(WorkflowJobSendMessageExecuteWild, "*"),
	subj.NS(WorkflowJobServiceTaskExecuteWild, "*"),
	subj.NS(WorkflowJobGatewayTaskExecute, "*"),
	subj.NS(WorkflowJobTimerTaskExecute, "*"),
	subj.NS(WorkflowJobUserTaskExecute, "*"),
	subj.NS(WorkflowLogAll, "*"),
	subj.NS(WorkflowMessage, "*"),
	subj.NS(WorkflowProcessComplete, "*"),
	subj.NS(WorkflowProcessExecute, "*"),
	subj.NS(WorkflowProcessTerminated, "*"),
	subj.NS(WorkflowProcessCompensate, "*"),
	subj.NS(WorkflowTimedExecute, "*"),
	subj.NS(WorkflowTraversalComplete, "*"),
	subj.NS(WorkflowTraversalExecute, "*"),
	subj.NS(WorkflowJobGatewayTaskActivate, "*"),
	subj.NS(WorkflowJobGatewayTaskReEnter, "*"),
	WorkflowTelemetryTimer,
	"WORKFLOW.*.Compensate.*",
	"$JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.WORKFLOW.>", // Dead letter functionality
}

// WorkflowMessageFormat provides the template for sending workflow messages.
var WorkflowMessageFormat = "WORKFLOW.%s.Message.%s"

const (
	APIAll                            = "WORKFLOW.Api.*"                              // APIAll is all API message subjects.
	APIStoreWorkflow                  = "WORKFLOW.Api.StoreWorkflow"                  // APIStoreWorkflow is the store Workflow API subject.
	APILaunchProcess                  = "WORKFLOW.Api.LaunchProcess"                  // APILaunchProcess is the launch process API subject.
	APIListWorkflows                  = "WORKFLOW.Api.ListWorkflows"                  // APIListWorkflows is the list workflows API subject.
	APIListExecution                  = "WORKFLOW.Api.ListExecution"                  // APIListExecution is the list workflow instances API subject.
	APIListExecutionProcesses         = "WORKFLOW.Api.ListExecutionProcesses"         // APIListExecutionProcesses is the get processes of a running workflow instance API subject.
	APICancelExecution                = "WORKFLOW.Api.CancelExecution"                // APICancelExecution is the cancel an execution API subject.
	APISendMessage                    = "WORKFLOW.Api.SendMessage"                    // APISendMessage is the send workflow message API subject.
	APICancelProcessInstance          = "WORKFLOW.API.CancelProcessInstance"          // APICancelProcessInstance is the cancel process instance API message subject.
	APICompleteManualTask             = "WORKFLOW.Api.CompleteManualTask"             // APICompleteManualTask is the complete manual task API subject.
	APICompleteServiceTask            = "WORKFLOW.Api.CompleteServiceTask"            // APICompleteServiceTask is the complete service task API subject.
	APICompleteUserTask               = "WORKFLOW.Api.CompleteUserTask"               // APICompleteUserTask is the complete user task API subject.
	APICompleteSendMessageTask        = "WORKFLOW.Api.CompleteSendMessageTask"        // APICompleteSendMessageTask is the complete send message task API subject.
	APIDeprecateServiceTask           = "WORKFLOW.Api.DeprecateServiceTask"           // APIDeprecateServiceTask is the deprecate service task API subject.
	APIListExecutableProcess          = "WORKFLOW.Api.APIListExecutableProcess"       // APIListExecutableProcess is the list executable process API subject.
	APIListUserTaskIDs                = "WORKFLOW.Api.ListUserTaskIDs"                // APIListUserTaskIDs is the list user task IDs API subject.
	APIGetUserTask                    = "WORKFLOW.Api.GetUserTask"                    // APIGetUserTask is the get user task API subject.
	APIGetTaskSpecVersions            = "WORKFLOW.Api.GetTaskSpecVersions"            // APIGetTaskSpecVersions is the get task versions API subject.
	APIHandleWorkflowError            = "WORKFLOW.Api.HandleWorkflowError"            // APIHandleWorkflowError is the handle workflow error API subject.
	APIHandleWorkflowFatalError       = "WORKFLOW.Api.HandleWorkflowFatalError"       // APIHandleWorkflowFatalError is the handle workflow fatal error API subject.
	APIRegisterTask                   = "Workflow.Api.RegisterTask"                   // APIRegisterTask registers a task with SHAR and returns the id.  If the task already exists then the ID is returned of the existing task.
	APIGetProcessInstanceStatus       = "WORKFLOW.Api.GetProcessInstanceStatus"       // APIGetProcessInstanceStatus is the get process instance status API subject.
	APIGetTaskSpec                    = "WORKFLOW.Api.GetTaskSpec"                    // APIGetTaskSpec is the get task spec API message subject.
	APIGetWorkflowVersions            = "WORKFLOW.Api.GetWorkflowVersions"            // APIGetWorkflowVersions is the get workflow versions API message subject.
	APIGetWorkflow                    = "WORKFLOW.Api.GetWorkflow"                    // APIGetWorkflow is the get workflow API message subject.
	APIGetProcessHistory              = "WORKFLOW.Api.GetProcessHistory"              // APIGetProcessHistory is the get process history API message subject.
	APIGetVersionInfo                 = "WORKFLOW.API.GetVersionInfo"                 // APIGetVersionInfo is the get server version information API message subject.
	APIGetTaskSpecUsage               = "WORKFLOW.Api.GetTaskSpecUsage"               // APIGetTaskSpecUsage is the get task spec usage API message subject.
	APIListTaskSpecUIDs               = "WORKFLOW.Api.ListTaskSpecUIDs"               // APIListTaskSpecUIDs is the list task spec UIDs API message subject.
	APIHeartbeat                      = "WORKFLOW.Api.Heartbeat"                      // APIHeartbeat // is the heartbeat API message subject.
	APILog                            = "WORKFLOW.Api.Log"                            // APILog // is the client logging message subject.
	APIGetJob                         = "WORKFLOW.Api.GetJob"                         // APIGetJob is the get job API subject.
	APIResolveWorkflow                = "WORKFLOW.Api.ResolveWorkflow"                // APIResolveWorkflow is the resolve workflow API subject.
	APIGetCompensationInputVariables  = "WORKFLOW.Api.GetCompensationInputVariables"  // APIGetCompensationInputVariables is the get compensation input variables message subject.
	APIGetCompensationOutputVariables = "WORKFLOW.Api.GetCompensationOutputVariables" // APIGetCompensationOutputVariables is the get compensation output variables message subject.
)

var (
	KvJob              = "WORKFLOW_JOB"             // KvJob is the name of the key value store that holds workflow jobs.
	KvVersion          = "WORKFLOW_VERSION"         // KvVersion is the name of the key value store that holds an ordered list of workflow version IDs for a given workflow
	KvDefinition       = "WORKFLOW_DEF"             // KvDefinition is the name of the key value store that holds the state machine definition for workflows
	KvTracking         = "WORKFLOW_TRACKING"        // KvTracking is the name of the key value store that holds the state of a workflow task.
	KvInstance         = "WORKFLOW_INSTANCE"        // KvInstance is the name of the key value store that holds workflow instance information.
	KvExecution        = "WORKFLOW_EXECUTION"       // KvExecution is the name of the key value store that holds execution information.
	KvUserTask         = "WORKFLOW_USERTASK"        // KvUserTask is the name of the key value store that holds active user tasks.
	KvOwnerName        = "WORKFLOW_OWNERNAME"       // KvOwnerName is the name of the key value store that holds owner names for owner IDs
	KvOwnerID          = "WORKFLOW_OWNERID"         // KvOwnerID is the name of the key value store that holds owner IDs for owner names.
	KvClientTaskID     = "WORKFLOW_CLIENTTASK"      // KvClientTaskID is the name of the key value store that holds the unique ID used by clients to subscribe to service task messages.
	KvWfName           = "WORKFLOW_NAME"            // KvWfName is the name of the key value store that holds workflow IDs for workflow names.
	KvProcessInstance  = "WORKFLOW_PROCESS"         // KvProcessInstance is the name of the key value store holding process instances.
	KvGateway          = "WORKFLOW_GATEWAY"         // KvGateway is the name of the key value store holding gateway instances.
	KvHistory          = "WORKFLOW_HISTORY"         // KvHistory is the name of the key value store holding process histories.
	KvLock             = "WORKFLOW_GENLCK"          // KvLock is the name of the key value store holding locks.
	KvMessageTypes     = "WORKFLOW_MSGTYPES"        // KvMessageTypes is the name of the key value store containing known message types.
	KvTaskSpecVersions = "WORKFLOW_TSPECVER"        // KvTaskSpecVersions is the name of the key value store holding task specification versions.
	KvTaskSpec         = "WORKFLOW_TSKSPEC"         // KvTaskSpec is the name of the key value store holding task specification.
	KvProcess          = "WORKFLOW_PROCESS_MAPPING" // KvProcess is the name of the key value store mapping process names to workflow names.
	KvMessages         = "WORKFLOW_MESSAGES"        // KvMessages is the name of the key value store containing messages.
	KvClients          = "WORKFLOW_CLIENTS"         // KvClients is the name of the key value store containing connected clients.
)
