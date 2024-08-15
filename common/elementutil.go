package common

import "gitlab.com/shar-workflow/shar/model"

// ElementTable indexes an entire process for quick ID lookups
func ElementTable(process *model.Workflow) map[string]*model.Element {
	el := make(map[string]*model.Element)
	for _, i := range process.Process {
		IndexProcessElements(i.Elements, el)
	}
	return el
}

// IndexProcessElements is the recursive part of the index
func IndexProcessElements(elements []*model.Element, el map[string]*model.Element) {
	for _, i := range elements {
		el[i.Id] = i
		if i.Process != nil {
			IndexProcessElements(i.Process.Elements, el)
		}
	}
}

// SeekElement locates an element from a workflow by ID
func SeekElement(process *model.Workflow, id string) *model.Element {
	for _, pr := range process.Process {
		for _, i := range pr.Elements {
			if i.Id == id {
				return i
			}
		}
	}
	return nil
}
