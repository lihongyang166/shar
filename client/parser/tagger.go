package parser

import (
	errors2 "errors"
	"fmt"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/element"
	"gitlab.com/shar-workflow/shar/model"
)

func tagWorkflow(wf *model.Workflow) error {
	for _, process := range wf.Process {
		els := make(map[string]*model.Element)
		eleIdsReferences := getElementIdReferences(process)
		common.IndexProcessElements(process.Elements, els)
		if err := tagGateways(process, els, eleIdsReferences); err != nil {
			return fmt.Errorf("an error occurred tagging the process gateways: %w", err)
		}
	}
	return nil
}

func tagGateways(process *model.Process, els map[string]*model.Element, eleIdReferences map[string][]string) error {
	for _, el := range process.Elements {
		if el.Type == element.Gateway {
			var numIn, numOut int
			if ins, ok := eleIdReferences[el.Id]; ok {
				numIn = len(ins)
			}
			if el.Outbound != nil {
				numOut = len(el.Outbound.Target)
			}
			if numIn == 1 && numOut > 1 {
				el.Gateway.Direction = model.GatewayDirection_divergent
			} else if numIn > 1 && numOut == 1 {
				el.Gateway.Direction = model.GatewayDirection_convergent
			} else {
				// Bad Gateway... 503 :-)
				return fmt.Errorf("cannot discern gateway type due to ambiguous inputs and outputs: %w", errors2.New("unsupported gateway type"))
			}
		}
	}
	for _, el := range process.Elements {
		if el.Type == element.Gateway && el.Gateway.Direction == model.GatewayDirection_convergent {
			if err := tagReciprocal(el, eleIdReferences, els); err != nil {
				return fmt.Errorf("find reciprocal gateway: %w", err)
			}
		}
	}

	return nil
}

func getElementIdReferences(process *model.Process) map[string][]string {
	eleIdReferencedBy := make(map[string][]string)
	for _, el := range process.Elements {
		if el.Outbound != nil {
			for _, c := range el.Outbound.Target {
				elementId := c.Target
				if _, ok := eleIdReferencedBy[elementId]; !ok {
					eleIdReferencedBy[elementId] = make([]string, 0)
				}
				eleIdReferencedBy[elementId] = append(eleIdReferencedBy[elementId], el.Id)
			}
		}
	}
	return eleIdReferencedBy
}

func tagReciprocal(gw *model.Element, eleIdReferences map[string][]string, eleIndex map[string]*model.Element) error {
	stack := 0
	result := make(map[string]struct{})
	recurseRecip(gw, gw, eleIdReferences, result, stack, eleIndex)
	// This has paths which lead to no gateway
	if _, ok := result["[<<null>>]"]; ok {
		// This is a gateway without a reciprocal, so set fixed expectations on the gateway
		// so treat this as a solitary gateway
		if expectations, ok := eleIdReferences[gw.Id]; ok {
			gw.Gateway.FixedExpectations = expectations
		}
		return nil
	}
	if len(result) > 1 {
		// This is a gateway with multiple inbound gateways
		// so treat this as a solitary gateway
		if expectations, ok := eleIdReferences[gw.Id]; ok {
			gw.Gateway.FixedExpectations = expectations
		}
		return nil
	}
	// set the reciprocal
	var only *model.Element
	for i := range result {
		only = eleIndex[i]
		break
	}
	gw.Gateway.ReciprocalId = only.Id
	only.Gateway.ReciprocalId = gw.Id
	return nil
}

func recurseRecip(gw *model.Element, el *model.Element, eleIdReferences map[string][]string, result map[string]struct{}, stack int, eleIndex map[string]*model.Element) {
	if gw != el && el.Type == element.Gateway && gw.Gateway.Type == el.Gateway.Type {
		if el.Gateway.Direction == model.GatewayDirection_convergent {
			stack++
		}
		if el.Gateway.Direction == model.GatewayDirection_divergent {
			if stack == 0 {
				result[el.Id] = struct{}{}
				return
			}
			stack--
		}
	}
	elementReferences, ok := eleIdReferences[el.Id]
	if !ok {
		result["[<<null>>]"] = struct{}{}
		return
	}
	for _, next := range elementReferences {
		recurseRecip(gw, eleIndex[next], eleIdReferences, result, stack, eleIndex)
	}
}
