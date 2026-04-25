package api

import (
	"context"
	"encoding/json"
	"strings"
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
		if field == "dictionary_id" || field == "dictionary_name" || field == "format" {
			t.Fatalf("%s should be optional in schema: %s", field, string(encoded))
		}
	}

}

func TestNormalizeMCPSearchFormatKeepsFormatOptional(t *testing.T) {
	format, err := normalizeMCPSearchFormat(nil)
	if err != nil {
		t.Fatal(err)
	}
	if format != "" {
		t.Fatalf("expected omitted format to keep default output, got %q", format)
	}

	markdown := " markdown "
	format, err = normalizeMCPSearchFormat(&markdown)
	if err != nil {
		t.Fatal(err)
	}
	if format != "markdown" {
		t.Fatalf("expected markdown format, got %q", format)
	}

	jsonFormat := "json"
	format, err = normalizeMCPSearchFormat(&jsonFormat)
	if err != nil {
		t.Fatal(err)
	}
	if format != "" {
		t.Fatalf("expected json format to keep default output, got %q", format)
	}

	invalid := "html"
	if _, err := normalizeMCPSearchFormat(&invalid); err == nil {
		t.Fatal("expected unsupported format to fail")
	}
}

func TestHTMLToMarkdownConvertsDictionaryHTML(t *testing.T) {
	markdown, err := htmlToMarkdown(`<p><strong>Bold</strong> text</p><ul><li>one</li></ul><a href="/search?q=owl">owl</a>`)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"**Bold** text", "- one", "[owl](/search?q=owl)"} {
		if !strings.Contains(markdown, expected) {
			t.Fatalf("expected markdown to contain %q, got:\n%s", expected, markdown)
		}
	}
}

func TestMCPMarkdownSearchResultUsesMarkdownText(t *testing.T) {
	result := mcpMarkdownSearchResult("owl", searchDictionaryOutput{Results: []searchResultInfo{{
		DictionaryID:   1,
		DictionaryName: "Test Dictionary",
		Visibility:     "public",
		Word:           "owl",
		Markdown:       "A **bird**.",
		Score:          1,
		Source:         "exact",
	}}})
	if len(result.Content) != 1 {
		t.Fatalf("expected one text content item, got %d", len(result.Content))
	}
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	for _, expected := range []string{"# Search results for owl", "## owl", "> Dictionary: Test Dictionary", "A **bird**."} {
		if !strings.Contains(textContent.Text, expected) {
			t.Fatalf("expected MCP markdown text to contain %q, got:\n%s", expected, textContent.Text)
		}
	}
}
