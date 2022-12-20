package errors

import (
	"errors"
)

var (
	ErrClosing                        = errors.New("SHAR server is shutting down")                                                               // ErrClosing si-gnifies SHAR server is shutting down.
	ErrWorkflowInstanceNotFound       = errors.New("workflow instance not found")                                                                // ErrWorkflowInstanceNotFound - workflow instance not found.
	ErrWorkflowNotFound               = errors.New("workflow not found")                                                                         // ErrWorkflowNotFound - workflow not found.
	ErrWorkflowVersionNotFound        = errors.New("workflow version not found")                                                                 // ErrWorkflowVersionNotFound - workflow version not found.
	ErrElementNotFound                = errors.New("element not found")                                                                          // ErrElementNotFound - element not found.
	ErrStateNotFound                  = errors.New("state not found")                                                                            // ErrStateNotFound - state not found.
	ErrJobNotFound                    = errors.New("job not found")                                                                              // ErrJobNotFound - job not found.
	ErrMissingCorrelation             = errors.New("missing correlation key")                                                                    // ErrMissingCorrelation - missing correllation key.
	ErrFatalBadDuration               = &ErrWorkflowFatal{Err: errors.New("timer embargo value could not be evaluated to an int or a duration")} // ErrFatalBadDuration - the timer embargo value could not be evaluated to an int or a duration
	ErrWorkflowErrorNotFound          = errors.New("workflow error number not found")                                                            // ErrWorkflowErrorNotFound - the workflow error thrown is not recognised.
	ErrUndefinedVariable              = errors.New("undefined variable")                                                                         // ErrUndefinedVariable - a variable was referred to in the workflow that hasn't been declared.
	ErrMissingErrorCode               = errors.New("missing error code")                                                                         // ErrMissingErrorCode - no workflow error code was provided.
	ErrBadlyQuotedIdentifier          = errors.New("identifier not correctly quoted")                                                            // ErrBadlyQuotedIdentifier - an identifier was not correctgly formatted.
	ErrMessageSenderAlreadyRegistered = errors.New("message sender already registered")                                                          // ErrMessageSenderAlreadyRegistered - a message sender with this signature was already registered.
	ErrServiceTaskAlreadyRegistered   = errors.New("service task already registered")                                                            // ErrServiceTaskAlreadyRegistered - a service task with this signature was already registered.
	ErrFailedToParseISO8601           = errors.New("could not parse Duration string")                                                            // ErrFailedToParseISO8601 - string cound not be parsed into an ISO 8601 duration.
	ErrBadTimerEventDefinition        = errors.New("found timerEventDefinition, but it had no time or Duration specified")                       // ErrBadTimerEventDefinition - no timer defined of a timer event.
	ErrBadTimeCycle                   = errors.New("time cycle was not in the correct format")                                                   // ErrBadTimeCycle - time cycle was not specified correctly.
	ErrCorrelationFail                = errors.New("could not find correlation message")                                                         // ErrCorrelationFail - could not correlate the workflow message.
	ErrMissingID                      = errors.New("missing id")                                                                                 // ErrMissingID - could not validate the id in the model
	ErrMissingServiceTaskDefinition   = errors.New("missing service task definition")                                                            // ErrMissingServiceTaskDefinition - could not find the definition for the service task.
	ErrExtractingVar                  = errors.New("could not extract var")                                                                      // ErrExtractingVar - failod to extract client variable.
	ErrExpectedVar                    = errors.New("workflow expects variable")                                                                  // ErrExpectedVar - a variable was expected by workflow that was not provided.
	ErrCancelFailed                   = errors.New("failed to cancel workflow instance")                                                         // ErrCancelFailed - an attempt to cancel a workflow instance failed.
	ErrInvalidState                   = errors.New("invalid cancellation state")                                                                 // ErrInvalidState - an attempt was made to p[erform an action witrh an invalid cancellation state.
	ErrApiAuthZFail                   = errors.New("failed to authorize API call")                                                               // ErrApiAuthZFail - an attempt was made to call an API that failed an authorization check.
	ErrApiAuthNFail                   = errors.New("failed to authenticate API call")                                                            // ErrApiAuthNFail - an attempt was made to call an API that failed an authentication check.
)

const TraceLevel = -41   // TraceLevel specifies a custom level for trace logging.
const VerboseLevel = -51 // VerboseLevel specifies a custom level vor verbose logging.

// ErrWorkflowFatal signifys that the workflow must terniate
type ErrWorkflowFatal struct {
	Err error
}

// Error returns the string version of the ErrWorkflowFatal error
func (e ErrWorkflowFatal) Error() string {
	return e.Err.Error()
}

// IsWorkflowFatal is a quick test to check whether the error contains ErrWorkflowFatal
func IsWorkflowFatal(err error) bool {
	var wff *ErrWorkflowFatal
	return errors.As(err, &wff)
}
