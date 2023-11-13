package commands

import (
	_ "embed"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/yoheimuta/go-protoparser"
	"github.com/yoheimuta/go-protoparser/parser"
	"gitlab.com/shar-workflow/nats-proto-gen-go/flag"
	"gitlab.com/shar-workflow/nats-proto-gen-go/types"
	"os"
	"path"
	"reflect"
	"strings"
	"text/template"
)

//go:embed templates/services.gotmpl
var servicesTemplate string

//go:embed templates/servers.gotmpl
var serversTemplate string

//go:embed templates/server-example.gotmpl
var serverExampleTemplate string

//go:embed  templates/clients.gotmpl
var clientTemplate string

//go:embed  templates/client-example.gotmpl
var clientExampleTemplate string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "nats-proto-gen-go",
	Short: "nats-proto-gen-go command line application",
	Long:  `Generates NATS request/reply service definitions from a proto file containing RPC server`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: generate,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {

	},
	Args: cobra.MatchAll(cobra.ExactArgs(1)),
}

func init() {
	RootCmd.PersistentFlags().StringVar(&flag.Value.ModuleNamespace, flag.ModuleNamespace, "protogen", "sets the local module namespace for imports")
	RootCmd.PersistentFlags().StringVar(&flag.Value.OutputPackage, flag.OutputPackage, "protogen", "sets the output package name")
	RootCmd.PersistentFlags().BoolVar(&flag.Value.GenerateExamples, flag.GenerateExamples, true, "enables/disables generating example code")
	RootCmd.PersistentFlags().StringVar(&flag.Value.MessagePrefix, flag.MessagePrefix, "", "sets the prefix for NATS messages")
}

// Execute adds all child commands to the root command and sets flag appropriately.
// This is called by main.main(). It only needs to happen once to the RootCmd.
func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func generate(cmd *cobra.Command, args []string) {
	if err := cmd.ValidateArgs(args); err != nil {
		panic(err)
	}
	file := args[0]
	rdr, err := os.Open(file)
	if err != nil {
		panic(fmt.Errorf("failed to open %s, err %v\n", file, err))
	}
	defer rdr.Close()

	protoModel, err := protoparser.Parse(rdr)
	if err != nil {
		panic(fmt.Errorf("failed to parse, err %v\n", err))
	}
	services, packagePath := parseProtoBody(protoModel.ProtoBody)

	packagePathSplit := strings.Split(packagePath, "/")
	outputPackagePathSplit := strings.Split(flag.Value.OutputPackage, "/")
	input := types.TemplateInput{
		TypePackage:       strings.TrimSuffix(packagePathSplit[len(packagePathSplit)-1], "\""),
		PackagePath:       packagePath,
		Services:          services,
		MessagePrefix:     flag.Value.MessagePrefix,
		OutputPackage:     flag.Value.OutputPackage,
		OutputPackageName: outputPackagePathSplit[len(outputPackagePathSplit)-1],
		ModuleNamespace:   path.Join(flag.Value.ModuleNamespace, flag.Value.OutputPackage),
	}

	if err := os.MkdirAll(flag.Value.OutputPackage, 0777); err != nil {
		panic(err)
	}

	if err := applyTemplate(input, servicesTemplate, flag.Value.OutputPackage+"/services.go"); err != nil {
		panic(err)
	}
	if err := applyTemplate(input, serversTemplate, flag.Value.OutputPackage+"/servers.go"); err != nil {
		panic(err)
	}
	if err := renderServices(input, clientTemplate, services, func(service types.Service) (string, string) {
		return flag.Value.OutputPackage, strings.ToLower(service.Name) + "-client.go"
	}); err != nil {
		panic(err)
	}

	if flag.Value.GenerateExamples {
		examplesDir := "nats-proto-examples"
		if err := os.MkdirAll(examplesDir, 0777); err != nil {
			panic(err)
		}

		if err := renderServices(input, clientExampleTemplate, services, func(service types.Service) (string, string) {
			return path.Join(examplesDir, strings.ToLower(service.Name)+"-"+"client"), "example-" + strings.ToLower(service.Name) + "-client.go"
		}); err != nil {
			panic(err)
		}

		if err := renderServices(input, servicesTemplate, services, func(service types.Service) (string, string) {
			return path.Join(examplesDir, strings.ToLower(service.Name)+"-"+"server"), "example-" + strings.ToLower(service.Name) + "-server.go"
		}); err != nil {
			panic(err)
		}
	}
}

func renderServices(input types.TemplateInput, template string, services []types.Service, dirFn func(service types.Service) (string, string)) error {
	for i := range input.Services {
		inSvr := input
		inSvr.Services = []types.Service{services[i]}
		dir, file := dirFn(services[i])
		if err := os.MkdirAll(dir, 0777); err != nil {
			return err
		}
		if err := applyTemplate(inSvr, template, path.Join(dir, file)); err != nil {
			return err
		}
	}
	return nil
}

func applyTemplate(input types.TemplateInput, fileTemplate string, output string) error {
	tmpl, err := template.New("services").Parse(fileTemplate)
	if err != nil {
		return err
	}
	fil, err := os.Create(output)
	if err != nil {
		return err
	}
	defer fil.Close()
	if err := tmpl.Execute(fil, input); err != nil {
		return err
	}
	return nil
}

func parseProtoBody(body []parser.Visitee) ([]types.Service, string) {
	var goPackage string
	sc := make([]types.Service, 0)
	for _, elem := range body {
		switch body := elem.(type) {
		case *parser.Option:
			if body.OptionName == "go_package" {
				goPackage = body.Constant
			}
		case *parser.Service:
			svc := types.Service{
				Name: body.ServiceName,
			}
			sc = append(sc, parseService(body, &svc))
		}
	}
	return sc, goPackage
}

func parseService(body *parser.Service, service *types.Service) types.Service {
	for _, elem := range body.ServiceBody {
		switch el := elem.(type) {
		case *parser.RPC:
			sc := types.ServiceCall{
				Name: el.RPCName,
			}
			sc = parseServiceCall(el, sc)
			if el.RPCRequest.IsStream || el.RPCResponse.IsStream {
				panic("parser does not support streams")
			}
			service.Methods = append(service.Methods, sc)
		default:
			fmt.Println(reflect.TypeOf(el).Name())
		}
	}
	return *service //parseServiceCall(body.ServiceBody, sc)
}

func parseServiceCall(body *parser.RPC, sc types.ServiceCall) types.ServiceCall {
	sc.Name = body.RPCName
	sc.InParam = types.Param{Typ: body.RPCRequest.MessageType}
	sc.OutParam = types.Param{Typ: body.RPCResponse.MessageType}
	return sc
}
