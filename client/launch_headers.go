package client

import "gitlab.com/shar-workflow/shar/model"

// LaunchHeaders represents a map of string headers that can be attached to a process when it is launched.
// The keys are the header names and the values are the header values.
type LaunchHeaders map[string]string

// LaunchParams represents the parameters for launching a new process within a workflow/BPMN definition.
type LaunchParams struct {
	LaunchHeaders
	ProcessID string     // Process ID to start.
	Vars      model.Vars // Variables to launch the process with
}

// LoadWorkflowParams represents the parameters for loading a workflow. It includes a `WithLaunchHeaders` field
// that represents the headers to attach to a process when it is launched, and a `Name` field that specifies the name
// of the workflow to load.
type LoadWorkflowParams struct {
	LaunchHeaders
	Name         string // Name of the workflow to load.
	WorkflowBPMN []byte // WorkflowBPMN - byte array containing a valid BPMN XML workflow.
}
