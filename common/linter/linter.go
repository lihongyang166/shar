package linter

import (
	errors2 "errors"
	"fmt"
	"gitlab.com/shar-workflow/shar/common/element"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/errors"
)

// MessageType describes a class of linter message.
type MessageType uint8

const (
	MessageTypeError   MessageType = iota // MessageTypeError - an error linter message.
	MessageTypeWarning                    // MessageTypeWarning - a warning linter message.
	MessageTypeInfo                       // MessageTypeInfo -
)

var (
	ErrMissingEndEvent       = errors2.New("missing end event")       // ErrMissingEndEvent indicates that a procewss terminates without an end event.
	ErrMissingOpeningGateway = errors2.New("missing opening gateway") // ErrMissingOpeningGateway indicates that a gateway is being closed and no corresponding open exists.
)

// Message - a linter finding
type Message struct {
	Type MessageType
	Text string
}

// Lint - executes the linter versus a workflow
func Lint(wf *model.Workflow, warningsAsErrors bool) ([]Message, error) {
	var err error
	mn := 0
	if warningsAsErrors {
		mn = 1
	}
	m := make([]Message, 0)
	// process level rules
	for _, p := range wf.Process {
		linkEventRules(&m, p)
		err := startDirectedScan(p, &directedTraversalInstruction{
			EndOfProcess: func(elem *model.Element) error {
				switch elem.Type {
				case element.EndEvent, element.CompensateEndEvent:
				default:
					return fmt.Errorf("last event '%s' is %s not a recognized end event: %w", elem.Name, elem.Type, ErrMissingEndEvent)
				}
				return nil
			},
		})
		if err != nil {
			return nil, fmt.Errorf("a directed scan of the process revealed errors: %w", err)
		}
	}
	for _, x := range m {
		if int(x.Type) <= mn {
			err = fmt.Errorf("linter returned errors: %w", errors.ErrLint)
		}
	}
	return m, err
}

func linkEventRules(msgs *[]Message, process *model.Process) {

	catch := make(map[string]string)
	throw := make(map[string]string)
	for _, e := range process.Elements {
		if e.Type == element.LinkIntermediateCatchEvent {
			if x, ok := catch[e.Execute]; !ok {
				catch[e.Execute] = e.Id
			} else {
				*msgs = append(*msgs, Message{Type: MessageTypeError, Text: fmt.Sprintf("Duplicate link catch: %s in %s", x, process.Id)})
			}
		}
		if e.Type == element.LinkIntermediateThrowEvent {
			throw[e.Execute] = e.Id
		}
	}
	for k, v := range throw {
		if _, ok := catch[k]; !ok {
			*msgs = append(*msgs, Message{Type: MessageTypeError, Text: fmt.Sprintf("No link catch for throw %s in %s in %s", k, v, process.Id)})
		}
	}
	for k, v := range catch {
		if _, ok := throw[k]; !ok {
			*msgs = append(*msgs, Message{Type: MessageTypeWarning, Text: fmt.Sprintf("No link throw for catch %s in %s in %s", k, v, process.Id)})
		}
	}
}
