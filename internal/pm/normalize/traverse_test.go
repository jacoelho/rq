package normalize

import (
	"reflect"
	"testing"

	"github.com/jacoelho/rq/internal/pm/ast"
)

func TestRequests(t *testing.T) {
	t.Parallel()

	collection := ast.Collection{
		Item: []ast.Item{
			{
				Name: "Folder A",
				Item: []ast.Item{
					{
						Name:    "Req 1",
						Request: &ast.Request{Method: "GET"},
					},
				},
			},
			{
				Name:    "Req 2",
				Request: &ast.Request{Method: "POST"},
			},
		},
	}

	nodes := Requests(collection)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	if nodes[0].Name != "Req 1" {
		t.Fatalf("first node name = %q", nodes[0].Name)
	}
	if !reflect.DeepEqual(nodes[0].FolderPath, []string{"Folder A"}) {
		t.Fatalf("first node folder path = %#v", nodes[0].FolderPath)
	}
	if nodes[1].Name != "Req 2" {
		t.Fatalf("second node name = %q", nodes[1].Name)
	}
}

func TestRequestsInheritsFolderEvents(t *testing.T) {
	t.Parallel()

	folderEvent := ast.Event{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`tests["status"] = responseCode.code === 200;`,
		}},
	}

	collection := ast.Collection{
		Item: []ast.Item{
			{
				Name:  "Folder A",
				Event: []ast.Event{folderEvent},
				Item: []ast.Item{
					{
						Name:    "Req 1",
						Request: &ast.Request{Method: "GET"},
					},
				},
			},
		},
	}

	nodes := Requests(collection)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if !reflect.DeepEqual(nodes[0].Events, []ast.Event{folderEvent}) {
		t.Fatalf("events = %#v", nodes[0].Events)
	}
}

func TestRequestsInheritsCollectionEvents(t *testing.T) {
	t.Parallel()

	collectionEvent := ast.Event{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`tests["collection status"] = responseCode.code === 200;`,
		}},
	}

	collection := ast.Collection{
		Event: []ast.Event{collectionEvent},
		Item: []ast.Item{
			{
				Name:    "Req 1",
				Request: &ast.Request{Method: "GET"},
			},
		},
	}

	nodes := Requests(collection)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if !reflect.DeepEqual(nodes[0].Events, []ast.Event{collectionEvent}) {
		t.Fatalf("events = %#v", nodes[0].Events)
	}
}

func TestRequestsCombinesAncestorAndRequestEventsInOrder(t *testing.T) {
	t.Parallel()

	collectionEvent := ast.Event{
		Listen: "test",
		Script: ast.Script{Exec: []string{`tests["collection"] = true;`}},
	}
	rootEvent := ast.Event{
		Listen: "test",
		Script: ast.Script{Exec: []string{`tests["root"] = true;`}},
	}
	folderEvent := ast.Event{
		Listen: "test",
		Script: ast.Script{Exec: []string{`tests["folder"] = true;`}},
	}
	requestEvent := ast.Event{
		Listen: "test",
		Script: ast.Script{Exec: []string{`tests["request"] = true;`}},
	}

	collection := ast.Collection{
		Event: []ast.Event{collectionEvent},
		Item: []ast.Item{
			{
				Name:  "Root",
				Event: []ast.Event{rootEvent},
				Item: []ast.Item{
					{
						Name:  "Folder A",
						Event: []ast.Event{folderEvent},
						Item: []ast.Item{
							{
								Name:    "Req 1",
								Request: &ast.Request{Method: "GET"},
								Event:   []ast.Event{requestEvent},
							},
						},
					},
					{
						Name: "Folder B",
						Item: []ast.Item{
							{
								Name:    "Req 2",
								Request: &ast.Request{Method: "POST"},
							},
						},
					},
				},
			},
		},
	}

	nodes := Requests(collection)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	byName := make(map[string]RequestNode, len(nodes))
	for _, node := range nodes {
		byName[node.Name] = node
	}

	req1, ok := byName["Req 1"]
	if !ok {
		t.Fatal("Req 1 not found")
	}
	expectedReq1Events := []ast.Event{collectionEvent, rootEvent, folderEvent, requestEvent}
	if !reflect.DeepEqual(req1.Events, expectedReq1Events) {
		t.Fatalf("Req 1 events = %#v", req1.Events)
	}

	req2, ok := byName["Req 2"]
	if !ok {
		t.Fatal("Req 2 not found")
	}
	expectedReq2Events := []ast.Event{collectionEvent, rootEvent}
	if !reflect.DeepEqual(req2.Events, expectedReq2Events) {
		t.Fatalf("Req 2 events = %#v", req2.Events)
	}
}
