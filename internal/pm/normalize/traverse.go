package normalize

import "github.com/jacoelho/rq/internal/pm/ast"

// RequestNode contains request data plus folder context from the source tree.
type RequestNode struct {
	Name       string
	FolderPath []string
	Request    ast.Request
	Events     []ast.Event
}

// FullPath returns folder/request path segments.
func (n RequestNode) FullPath() []string {
	path := make([]string, 0, len(n.FolderPath)+1)
	path = append(path, n.FolderPath...)
	path = append(path, n.Name)
	return path
}

// Requests flattens a nested collection into request nodes.
func Requests(collection ast.Collection) []RequestNode {
	var out []RequestNode
	walkItems(collection.Item, nil, collection.Event, &out)
	return out
}

func walkItems(items []ast.Item, folderPath []string, inheritedEvents []ast.Event, out *[]RequestNode) {
	for _, item := range items {
		events := appendEvents(inheritedEvents, item.Event)

		if item.Request != nil {
			node := RequestNode{
				Name:       item.Name,
				FolderPath: append([]string(nil), folderPath...),
				Request:    *item.Request,
				Events:     events,
			}
			*out = append(*out, node)
		}

		if len(item.Item) > 0 {
			nextPath := append(append([]string(nil), folderPath...), item.Name)
			walkItems(item.Item, nextPath, events, out)
		}
	}
}

func appendEvents(parent []ast.Event, current []ast.Event) []ast.Event {
	if len(parent) == 0 && len(current) == 0 {
		return nil
	}

	events := make([]ast.Event, 0, len(parent)+len(current))
	for _, event := range parent {
		events = append(events, cloneEvent(event))
	}
	for _, event := range current {
		events = append(events, cloneEvent(event))
	}

	return events
}

func cloneEvent(event ast.Event) ast.Event {
	cloned := event
	cloned.Script.Exec = append([]string(nil), event.Script.Exec...)
	return cloned
}
