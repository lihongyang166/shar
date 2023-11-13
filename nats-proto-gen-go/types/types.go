package types

type Param struct {
	Typ string
}

type Service struct {
	Name    string
	Methods []ServiceCall
}

type ServiceCall struct {
	Name     string
	InParam  Param
	OutParam Param
}

type TemplateInput struct {
	Services          []Service
	TypePackage       string
	MessagePrefix     string
	MessageSuffix     string
	PackagePath       string
	OutputPackage     string
	ModuleNamespace   string
	OutputPackageName string
}
