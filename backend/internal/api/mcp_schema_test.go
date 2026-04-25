package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPSearchDictionaryOptionalDictionaryInput(t *testing.T) {
	ctx := context.Background()
	server := (&Server{}).buildMCPServer(1)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	tools, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var searchTool *mcp.Tool
	for _, tool := range tools.Tools {
		if tool.Name == "search_dictionary" {
			searchTool = tool
			break
		}
	}
	if searchTool == nil {
		t.Fatal("search_dictionary tool not found")
	}
	encoded, err := json.Marshal(searchTool.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(encoded, &schema); err != nil {
		t.Fatal(err)
	}
	for _, field := range schema.Required {
		if field == "dictionary_id" || field == "dictionary_name" {
			t.Fatalf("%s should be optional in schema: %s", field, string(encoded))
		}
	}

}
