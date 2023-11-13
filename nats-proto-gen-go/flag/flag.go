package flag

const (
	GenerateExamples = "generate-examples"
	// ModuleNamespace is the flag name for the local module namespace appended to imports.
	ModuleNamespace = "module-namespace"
	// OutputPackage is the flag name for the output package location.
	OutputPackage = "output-package"
	// MessagePrefix is the flag name for the NATS message prefix.
	MessagePrefix = "message-prefix"
)

// Set is the set of flags associated with the CLI.
type Set struct {
	GenerateExamples bool
	ModuleNamespace  string
	OutputPackage    string
	MessagePrefix    string
}

// Value contains the values of the SHAR CLI flags.
var Value Set
