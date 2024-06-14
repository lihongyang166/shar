package parser

import (
	"fmt"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/element"
	"gitlab.com/shar-workflow/shar/common/expression"
	"gitlab.com/shar-workflow/shar/common/linter"
	"gitlab.com/shar-workflow/shar/model"
	errors2 "gitlab.com/shar-workflow/shar/server/errors"
	"maps"
	"regexp"
	"strconv"
	"strings"
)

func validModel(workflow *model.Workflow) error {
	// Iterate the processes
	for _, process := range workflow.Process {
		// Check the name
		if err := validName(process.Name); err != nil {
			return fmt.Errorf("invalid process name: %w", err)
		}
		// Iterate through the elements
		for _, ele := range process.Elements {
			if ele.Id == "" {
				return fmt.Errorf("model validation failed: %w", &valError{Err: errors2.ErrMissingID, Context: ele.Name})
			}
			switch ele.Type {
			case element.ServiceTask:
				if err := validServiceTask(ele); err != nil {
					return fmt.Errorf("invalid service task: %w", err)
				}
			case element.Gateway:
				if ele.Gateway.Direction == model.GatewayDirection_convergent && ele.Gateway.ReciprocalId == "" {
					return fmt.Errorf("gateway %s(%s) has no opening gateway: %w", ele.Name, ele.Id, linter.ErrMissingOpeningGateway)
				}
			}
		}
		if err := checkVariables(process); err != nil {
			return fmt.Errorf("invalid variable definition: %w", err)
		}
	}
	for _, i := range workflow.Messages {
		if err := validName(i.Name); err != nil {
			return fmt.Errorf("invalid message name: %w", err)
		}
	}
	return nil
}

type outbound interface {
	GetTarget() string
}

func findElementsReferencingUndefinedVars(eleId string, eles map[string]*model.Element, elesReferencingUndefinedVars map[string]map[string]struct{}, branchOutputVars map[string]map[string]struct{}, branchId string, visited map[string]struct{}) error {
	ele := eles[eleId]

	if ele.Type == "endEvent" {
		return nil
	}

	outputVars := branchOutputVars[branchId]
	if ele.OutputTransform != nil {
		for outputVar := range ele.OutputTransform {
			outputVars[outputVar] = struct{}{}
		}
		branchOutputVars[branchId] = outputVars
	}

	var outbounds []outbound
	if len(ele.Errors) > 0 {
		for _, catchError := range ele.Errors {
			for outputVar := range catchError.OutputTransform {
				outputVars[outputVar] = struct{}{}
			}
			outbounds = append(outbounds, catchError)
		}
	}

	if ele.InputTransform != nil {
		for _, inputVarExpr := range ele.InputTransform {
			if err2 := checkUndefinedVarReference(eleId, inputVarExpr, outputVars, elesReferencingUndefinedVars); err2 != nil {
				return err2
			}
		}
	}

	if ele.Outbound != nil {
		for _, target := range ele.Outbound.Target {
			if target.Conditions != nil {
				for _, conditionExpr := range target.Conditions {
					if err2 := checkUndefinedVarReference(eleId, conditionExpr, outputVars, elesReferencingUndefinedVars); err2 != nil {
						return err2
					}
				}
			}
			outbounds = append(outbounds, target)
		}
	}

	if ele.Type == element.LinkIntermediateThrowEvent {
		var linkIntermediatCatchElement *model.Element
		for _, e := range eles {
			if e.Type == element.LinkIntermediateCatchEvent && ele.Execute == e.Execute {
				linkIntermediatCatchElement = e
			}
		}
		if linkIntermediatCatchElement == nil {
			return fmt.Errorf("failed to find link intermediate catch element for link intermediate throw element")
		}
		outbounds = append(outbounds, &model.Target{Target: linkIntermediatCatchElement.Id})
	}

	var isNewBranch bool
	if len(outbounds) > 1 {
		isNewBranch = true
	}

	for idx, outbound := range outbounds {
		var newBranchId string
		if isNewBranch {
			parentOutputVars := make(map[string]struct{})
			maps.Copy(parentOutputVars, branchOutputVars[branchId])
			newBranchId = branchId + "-" + strconv.Itoa(idx)
			branchOutputVars[newBranchId] = parentOutputVars
		} else {
			newBranchId = branchId
		}
		visited[eleId] = struct{}{}
		if _, alreadyVisited := visited[outbound.GetTarget()]; !alreadyVisited {
			e := findElementsReferencingUndefinedVars(outbound.GetTarget(), eles, elesReferencingUndefinedVars, branchOutputVars, newBranchId, visited)
			if e != nil {
				return e
			}
		}

	}
	return nil
}

func checkUndefinedVarReference(eleId string, expr string, outputVars map[string]struct{}, elesReferencingUndefinedVars map[string]map[string]struct{}) error {
	vars, err := expression.GetVariables(expr)
	if err != nil {
		return fmt.Errorf("invalid input variable expression: %w", err)
	}
	for vr := range vars {
		if _, exists := outputVars[vr]; !exists {
			undefinedVars, exists := elesReferencingUndefinedVars[eleId]
			if !exists {
				undefinedVars = make(map[string]struct{})
			}
			undefinedVars[vr] = struct{}{}
			elesReferencingUndefinedVars[eleId] = undefinedVars
		}
	}
	return nil
}

func checkVariables(process *model.Process) error {
	elesById := make(map[string]*model.Element)
	common.IndexProcessElements(process.Elements, elesById)
	startElementIds := make([]string, 0)
	startElementIds = findElementIdsWithType(startElementIds, element.StartEvent, process.Elements)

	branchOutputVars := make(map[string]map[string]struct{})
	elementsReferencingUndefinedVars := make(map[string]map[string]struct{})
	for idx, startElementId := range startElementIds {
		branchId := strconv.Itoa(idx)
		branchOutputVars[branchId] = make(map[string]struct{})
		alreadyVisited := make(map[string]struct{})
		err := findElementsReferencingUndefinedVars(startElementId, elesById, elementsReferencingUndefinedVars, branchOutputVars, branchId, alreadyVisited)
		if err != nil {
			return fmt.Errorf("error when findElementsReferencingUndefinedVars: %w", err)
		}
	}

	if len(elementsReferencingUndefinedVars) > 0 {
		errMessage := buildErrorMessage(elementsReferencingUndefinedVars)
		return fmt.Errorf("elements referencing potentially undefined variables: %s, %w", errMessage, errors2.ErrUndefinedVariable)
	}

	return nil
}

func buildErrorMessage(elementsReferencingUndefinedVars map[string]map[string]struct{}) string {
	errElements := make([]string, len(elementsReferencingUndefinedVars))
	for eleId, varNames := range elementsReferencingUndefinedVars {
		eleVarNames := make([]string, 0, len(varNames))
		for varName := range varNames {
			eleVarNames = append(eleVarNames, varName)
		}
		errElement := eleId + ":[" + strings.Join(eleVarNames, ",") + "]"
		errElements = append(errElements, errElement)
	}
	errMessage := strings.Join(errElements, "; ")
	return errMessage
}

type valError struct {
	Err     error
	Context string
}

func (e valError) Error() string {
	return fmt.Sprintf("%s: %s\n", e.Err.Error(), e.Context)
}

//goland:noinspection GoUnnecessarilyExportedIdentifiers
func (e valError) Unwrap() error {
	return e.Err
}

func validServiceTask(j *model.Element) error {
	if j.Execute == "" {
		return fmt.Errorf("service task validation failed: %w", &valError{Err: errors2.ErrMissingServiceTaskDefinition, Context: j.Id})
	}
	return nil
}

var validKeyRe = regexp.MustCompile(`\A[-/_=\.a-zA-Z0-9]+\z`)

// is a NATS compatible name
func validName(name string) error {
	if len(name) == 0 || name[0] == '.' || name[len(name)-1] == '.' {
		return fmt.Errorf("'%s' contains invalid characters when used with SHAR", name)
	}
	if !validKeyRe.MatchString(name) {
		return fmt.Errorf("'%s' contains invalid characters when used with SHAR", name)
	}
	return nil
}
