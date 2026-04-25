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
		Required   []string `json:"required"`
		Properties map[string]struct {
			Description string   `json:"description"`
			Enum        []string `json:"enum"`
			Default     string   `json:"default"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(encoded, &schema); err != nil {
		t.Fatal(err)
	}
	for _, field := range schema.Required {
		if field == "dictionary_id" || field == "dictionary_name" || field == "format" {
			t.Fatalf("%s should be optional in schema: %s", field, string(encoded))
		}
	}
	formatProperty, ok := schema.Properties["format"]
	if !ok {
		t.Fatalf("format property missing from schema: %s", string(encoded))
	}
	for _, expected := range []string{"Optional output format", "json", "markdown", "instead of html", "reducing token usage"} {
		if !strings.Contains(formatProperty.Description, expected) {
			t.Fatalf("format description should mention %q, got %q", expected, formatProperty.Description)
		}
	}
	if formatProperty.Default != "json" {
		t.Fatalf("expected format default json, got %q", formatProperty.Default)
	}
	if strings.Join(formatProperty.Enum, ",") != "json,markdown" {
		t.Fatalf("expected format enum json,markdown; got %#v", formatProperty.Enum)
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

func TestMCPSearchResultOmitsUnusedDefinitionFormat(t *testing.T) {
	jsonPayload, err := json.Marshal(searchDictionaryOutput{Results: []searchResultInfo{{
		DictionaryID:   1,
		DictionaryName: "Test Dictionary",
		Visibility:     "public",
		Word:           "owl",
		HTML:           "<p>HTML only</p>",
		Score:          1,
		Source:         "exact",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	jsonText := string(jsonPayload)
	if !strings.Contains(jsonText, `"html"`) {
		t.Fatalf("expected default JSON payload to include html, got %s", jsonText)
	}
	if strings.Contains(jsonText, `"markdown"`) {
		t.Fatalf("expected default JSON payload to omit markdown, got %s", jsonText)
	}

	markdownPayload, err := json.Marshal(searchDictionaryOutput{Results: []searchResultInfo{{
		DictionaryID:   1,
		DictionaryName: "Test Dictionary",
		Visibility:     "public",
		Word:           "owl",
		Markdown:       "Markdown only",
		Score:          1,
		Source:         "exact",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	markdownText := string(markdownPayload)
	if !strings.Contains(markdownText, `"markdown"`) {
		t.Fatalf("expected markdown payload to include markdown, got %s", markdownText)
	}
	if strings.Contains(markdownText, `"html"`) {
		t.Fatalf("expected markdown payload to omit html, got %s", markdownText)
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
