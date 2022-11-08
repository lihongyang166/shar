package messages

import "gitlab.com/shar-workflow/shar/common/subj"

const (
	WorkflowStateAll                  = "WORKFLOW.%s.State.>"
	WorkflowJobExecuteAll             = "WORKFLOW.%s.State.Job.Execute.*"
	WorkFlowJobCompleteAll            = "WORKFLOW.%s.State.Job.Complete.*"
	WorkFlowJobAbortAll               = "WORKFLOW.%s.State.Job.Abort.*"
	WorkflowJobServiceTaskExecute     = "WORKFLOW.%s.State.Job.Execute.ServiceTask"
	WorkflowJobServiceTaskExecuteWild = "WORKFLOW.%s.State.Job.Execute.ServiceTask.>"
	WorkflowJobServiceTaskComplete    = "WORKFLOW.%s.State.Job.Complete.ServiceTask"
	WorkflowJobServiceTaskAbort       = "WORKFLOW.%s.State.Job.Abort.ServiceTask"
	WorkflowJobUserTaskExecute        = "WORKFLOW.%s.State.Job.Execute.UserTask"
	WorkflowJobUserTaskComplete       = "WORKFLOW.%s.State.Job.Complete.UserTask"
	WorkflowJobUserTaskAbort          = "WORKFLOW.%s.State.Job.Abort.UserTask"
	WorkflowJobManualTaskExecute      = "WORKFLOW.%s.State.Job.Execute.ManualTask"
	WorkflowJobManualTaskComplete     = "WORKFLOW.%s.State.Job.Complete.ManualTask"
	WorkflowJobManualTaskAbort        = "WORKFLOW.%s.State.Job.Abort.ManualTask"
	WorkflowJobSendMessageExecute     = "WORKFLOW.%s.State.Job.Execute.SendMessage"
	WorkflowJobSendMessageExecuteWild = "WORKFLOW.%s.State.Job.Execute.SendMessage.>"
	WorkflowJobSendMessageComplete    = "WORKFLOW.%s.State.Job.Complete.SendMessage"
	WorkflowJobTimerTaskExecute       = "WORKFLOW.%s.State.Job.Execute.Timer"
	WorkflowJobTimerTaskComplete      = "WORKFLOW.%s.State.Job.Complete.Timer"
	WorkflowJobLaunchExecute          = "WORKFLOW.%s.State.Job.Execute.Launch"
	WorkflowJobLaunchComplete         = "WORKFLOW.%s.State.Job.Complete.Launch"
	WorkflowInstanceExecute           = "WORKFLOW.%s.State.Workflow.Execute"
	WorkflowInstanceComplete          = "WORKFLOW.%s.State.Workflow.Complete"
	WorkflowInstanceTerminated        = "WORKFLOW.%s.State.Workflow.Terminated"
	WorkflowInstanceAbort             = "WORKFLOW.%s.State.Workflow.Abort"
	WorkflowInstanceAll               = "WORKFLOW.%s.State.Workflow.>"
	WorkflowActivityExecute           = "WORKFLOW.%s.State.Activity.Execute"
	WorkflowActivityComplete          = "WORKFLOW.%s.State.Activity.Complete"
	WorkflowActivityAbort             = "WORKFLOW.%s.State.Activity.Abort"
	WorkflowGeneralAbortAll           = "WORKFLOW.%s.State.*.Abort"
	WorkflowActivityAll               = "WORKFLOW.%s.State.Activity.>"
	WorkflowTraversalExecute          = "WORKFLOW.%s.State.Traversal.Execute"
	WorkflowTraversalComplete         = "WORKFLOW.%s.State.Traversal.Complete"
	WorkflowTimedExecute              = "WORKFLOW.%s.Timers.WorkflowExecute"
	WorkflowElementTimedExecute       = "WORKFLOW.%s.Timers.ElementExecute"
	WorkflowLog                       = "WORKFLOW.%s.State.Log"
	WorkflowLogAll                    = "WORKFLOW.%s.State.Log.*"
	WorkflowMessages                  = "WORKFLOW.%s.Message.>"
	WorkflowCommands                  = "WORKFLOW.%s.Command.>"
)

type WorkflowLogLevel string

const (
	LogFatal WorkflowLogLevel = ".Fatal"
	LogError WorkflowLogLevel = ".Error"
	LogWarn  WorkflowLogLevel = ".Warning"
	LogInfo  WorkflowLogLevel = ".Info"
	LogDebug WorkflowLogLevel = ".Debug"
)

var LogLevels = []WorkflowLogLevel{
	LogFatal,
	LogError,
	LogWarn,
	LogInfo,
	LogDebug,
}

var AllMessages = []string{
	subj.NS(WorkflowInstanceAll, "*"),
	subj.NS(WorkFlowJobCompleteAll, "*"),
	subj.NS(WorkFlowJobAbortAll, "*"),
	subj.NS(WorkflowJobServiceTaskExecuteWild, "*"),
	subj.NS(WorkflowJobSendMessageExecuteWild, "*"),
	subj.NS(WorkflowJobUserTaskExecute, "*"),
	subj.NS(WorkflowJobManualTaskExecute, "*"),
	subj.NS(WorkflowJobTimerTaskExecute, "*"),
	subj.NS(WorkflowJobLaunchExecute, "*"),
	subj.NS(WorkflowActivityExecute, "*"),
	subj.NS(WorkflowActivityComplete, "*"),
	subj.NS(WorkflowActivityAbort, "*"),
	subj.NS(WorkflowTraversalExecute, "*"),
	subj.NS(WorkflowTraversalComplete, "*"),
	subj.NS(WorkflowMessages, "*"),
	subj.NS(WorkflowTimedExecute, "*"),
	subj.NS(WorkflowElementTimedExecute, "*"),
	subj.NS(WorkflowCommands, "*"),
	subj.NS(WorkflowLogAll, "*"),
	//subj.NS(WorkflowAbortAll, "*"),
	APIAll,
}

var WorkflowMessageFormat = "WORKFLOW.%s.Message.%s.%s"

const (
	APIAll                       = "Workflow.Api.*"
	APIStoreWorkflow             = "WORKFLOW.Api.StoreWorkflow"
	APILaunchWorkflow            = "WORKFLOW.Api.LaunchWorkflow"
	APIListWorkflows             = "WORKFLOW.Api.ListWorkflows"
	APIListWorkflowInstance      = "WORKFLOW.Api.ListWorkflowInstance"
	APIGetWorkflowStatus         = "WORKFLOW.Api.GetWorkflowInstanceStatus"
	APICancelWorkflowInstance    = "WORKFLOW.Api.CancelWorkflowInstance"
	APISendMessage               = "WORKFLOW.Api.SendMessage"
	APICompleteManualTask        = "WORKFLOW.Api.CompleteManualTask"
	APICompleteServiceTask       = "WORKFLOW.Api.CompleteServiceTask"
	APICompleteUserTask          = "WORKFLOW.Api.CompleteUserTask"
	APICompleteSendMessageTask   = "WORKFLOW.Api.CompleteSendMessageTask"
	APIListUserTaskIDs           = "WORKFLOW.Api.ListUserTaskIDs"
	APIGetUserTask               = "WORKFLOW.Api.GetUserTask"
	APIHandleWorkflowError       = "WORKFLOW.Api.HandleWorkflowError"
	APIGetServerInstanceStats    = "WORKFLOW.Api.GetServerInstanceStats"
	APIGetServiceTaskRoutingID   = "WORKFLOW.Api.GetServiceTaskRoutingID"
	APIGetMessageSenderRoutingID = "WORKFLOW.Api.GetMessageSenderRoutingID"
)

var (
	KvMessageSubs  = "WORKFLOW_MSGSUBS"
	KvMessageSub   = "WORKFLOW_MSGSUB"
	KvJob          = "WORKFLOW_JOB"
	KvVersion      = "WORKFLOW_VERSION"
	KvDefinition   = "WORKFLOW_DEF"
	KvTracking     = "WORKFLOW_TRACKING"
	KvInstance     = "WORKFLOW_INSTANCE"
	KvMessageName  = "WORKFLOW_MSGNAME"
	KvMessageID    = "WORKFLOW_MSGID"
	KvUserTask     = "WORKFLOW_USERTASK"
	KvOwnerName    = "WORKFLOW_OWNERNAME"
	KvOwnerID      = "WORKFLOW_OWNERID"
	KvClientTaskID = "WORKFLOW_CLIENTTASK"
	KvWfName       = "WORKFLOW_NAME"
	KvVarState     = "WORKFLOW_VARSTATE"
)
