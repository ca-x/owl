package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"owl/backend/internal/dictionary"
	"owl/backend/pkg/version"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/google/jsonschema-go/jsonschema"
	echo "github.com/labstack/echo/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpTokenRequest struct {
	Token string `json:"token"`
}

type mcpDictionaryInfo struct {
	ID         int    `json:"id" jsonschema:"Dictionary id to pass to search_dictionary."`
	Name       string `json:"name" jsonschema:"Dictionary file/internal name."`
	Title      string `json:"title" jsonschema:"Human-readable dictionary title."`
	Visibility string `json:"visibility" jsonschema:"public or private for this token owner."`
	Entries    int    `json:"entries" jsonschema:"Approximate entry count."`
}

type listDictionariesInput struct{}

type listDictionariesOutput struct {
	Dictionaries []mcpDictionaryInfo `json:"dictionaries" jsonschema:"Public dictionaries plus this token owner's private dictionaries."`
}

type searchDictionaryInput struct {
	DictionaryID   *int    `json:"dictionary_id,omitempty" jsonschema:"Optional dictionary id returned by list_dictionaries. If omitted, all accessible dictionaries are searched."`
	DictionaryName *string `json:"dictionary_name,omitempty" jsonschema:"Optional dictionary name or title returned by list_dictionaries. Used when dictionary_id is omitted."`
	Format         *string `json:"format,omitempty" jsonschema:"Optional text output format. Use markdown to return Markdown text content; omit to keep the default JSON text output."`
	Query          string  `json:"query" jsonschema:"Word, phrase, or headword to look up."`
}

type searchResultInfo struct {
	DictionaryID   int     `json:"dictionary_id"`
	DictionaryName string  `json:"dictionary_name"`
	Visibility     string  `json:"visibility"`
	Word           string  `json:"word"`
	HTML           string  `json:"html"`
	Markdown       string  `json:"markdown,omitempty"`
	Score          float64 `json:"score"`
	Source         string  `json:"source"`
}

type searchDictionaryOutput struct {
	Results []searchResultInfo `json:"results" jsonschema:"Matching entries from the requested dictionary."`
}

func (s *Server) handleGetMCPToken(c *echo.Context) error {
	user := currentUser(c)
	status, err := s.users.GetMCPTokenStatus(c.Request().Context(), user.ID)
	if err != nil {
		return mapEntError(err)
	}
	return c.JSON(http.StatusOK, status)
}

func (s *Server) handleSetMCPToken(c *echo.Context) error {
	user := currentUser(c)
	var req mcpTokenRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	status, err := s.users.SetMCPToken(c.Request().Context(), user.ID, req.Token)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return c.JSON(http.StatusOK, status)
}

func (s *Server) handleGenerateMCPToken(c *echo.Context) error {
	user := currentUser(c)
	status, err := s.users.GenerateMCPToken(c.Request().Context(), user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, status)
}

func (s *Server) handleDeleteMCPToken(c *echo.Context) error {
	user := currentUser(c)
	status, err := s.users.DeleteMCPToken(c.Request().Context(), user.ID)
	if err != nil {
		return mapEntError(err)
	}
	return c.JSON(http.StatusOK, status)
}

func (s *Server) handleMCP(c *echo.Context) error {
	s.mcp.ServeHTTP(c.Response(), c.Request())
	return nil
}

func (s *Server) newMCPHandler() http.Handler {
	return mcp.NewSSEHandler(func(request *http.Request) *mcp.Server {
		token := mcpTokenFromRequest(request)
		claims, err := s.users.AuthenticateMCPToken(request.Context(), token)
		if err != nil {
			return nil
		}
		return s.buildMCPServer(claims.UserID)
	}, nil)
}

func (s *Server) buildMCPServer(userID int) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "owl", Title: "Owl Dictionary", Version: version.GetVersion()}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_dictionaries",
		Title:       "List available dictionaries",
		Description: "List public dictionaries plus private dictionaries owned by the MCP token user.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ listDictionariesInput) (*mcp.CallToolResult, listDictionariesOutput, error) {
		items, err := s.dictionaries.ListAccessible(ctx, userID)
		if err != nil {
			return nil, listDictionariesOutput{}, err
		}
		out := listDictionariesOutput{Dictionaries: make([]mcpDictionaryInfo, 0, len(items))}
		for _, item := range items {
			visibility := "private"
			if item.Public {
				visibility = "public"
			}
			out.Dictionaries = append(out.Dictionaries, mcpDictionaryInfo{
				ID:         item.ID,
				Name:       item.Name,
				Title:      firstNonEmpty(item.Title, item.Name),
				Visibility: visibility,
				Entries:    item.EntryCount,
			})
		}
		return mcpTextResult(out), out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_dictionary",
		Title:       "Search dictionaries",
		Description: "Search by query, optionally narrowed by dictionary_id or dictionary_name from list_dictionaries. Optional format controls only the human-readable MCP text content: omit it or use json for the original JSON text output, or use markdown to convert definitions from HTML to Markdown. If no dictionary is provided, all accessible dictionaries are searched, matching the web search scope.",
		InputSchema: searchDictionaryInputSchema(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input searchDictionaryInput) (*mcp.CallToolResult, searchDictionaryOutput, error) {
		query := strings.TrimSpace(input.Query)
		if query == "" {
			return nil, searchDictionaryOutput{}, fmt.Errorf("query is required")
		}
		format, err := normalizeMCPSearchFormat(input.Format)
		if err != nil {
			return nil, searchDictionaryOutput{}, err
		}
		dictionaryID := 0
		if input.DictionaryID != nil {
			dictionaryID = *input.DictionaryID
		}
		dictionaryName := ""
		if input.DictionaryName != nil {
			dictionaryName = *input.DictionaryName
		}
		dictionaryID, err = s.resolveMCPDictionaryID(ctx, userID, dictionaryID, dictionaryName)
		if err != nil {
			return nil, searchDictionaryOutput{}, err
		}
		results, err := s.dictionaries.Search(ctx, dictionary.SearchParams{UserID: userID, IsAdmin: false, Query: query, DictionaryID: dictionaryID})
		if err != nil {
			return nil, searchDictionaryOutput{}, err
		}
		out := searchDictionaryOutput{Results: make([]searchResultInfo, 0, len(results))}
		markdownRequested := format == "markdown"
		for _, result := range results {
			item := searchResultInfo{
				DictionaryID:   result.DictionaryID,
				DictionaryName: result.DictionaryName,
				Visibility:     result.Visibility,
				Word:           result.Word,
				HTML:           result.HTML,
				Score:          result.Score,
				Source:         result.Source,
			}
			if markdownRequested {
				markdown, err := htmlToMarkdown(result.HTML)
				if err != nil {
					return nil, searchDictionaryOutput{}, fmt.Errorf("convert %q to markdown: %w", result.Word, err)
				}
				item.Markdown = markdown
			}
			out.Results = append(out.Results, item)
		}
		if markdownRequested {
			return mcpMarkdownSearchResult(query, out), out, nil
		}
		return mcpTextResult(out), out, nil
	})

	return server
}

func mcpTokenFromRequest(request *http.Request) string {
	if token := strings.TrimSpace(request.URL.Query().Get("token")); token != "" {
		return token
	}
	auth := strings.TrimSpace(request.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("bearer "):])
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *Server) resolveMCPDictionaryID(ctx context.Context, userID int, dictionaryID int, dictionaryName string) (int, error) {
	if dictionaryID > 0 {
		return dictionaryID, nil
	}
	name := strings.TrimSpace(dictionaryName)
	if name == "" {
		return 0, nil
	}
	items, err := s.dictionaries.ListAccessible(ctx, userID)
	if err != nil {
		return 0, err
	}
	for _, item := range items {
		if strings.EqualFold(item.Name, name) || strings.EqualFold(item.Title, name) || strings.EqualFold(firstNonEmpty(item.Title, item.Name), name) {
			return item.ID, nil
		}
	}
	return 0, fmt.Errorf("dictionary %q is not available to this token", name)
}

func searchDictionaryInputSchema() *jsonschema.Schema {
	schema, err := jsonschema.For[searchDictionaryInput](nil)
	if err != nil {
		panic(fmt.Sprintf("search_dictionary input schema: %v", err))
	}
	formatSchema := schema.Properties["format"]
	if formatSchema == nil {
		panic("search_dictionary input schema: missing format property")
	}
	formatSchema.Description = "Optional output format for the MCP TextContent. Omit this field, pass an empty string, or pass json to keep the default JSON text output. Pass markdown to return readable Markdown text converted from dictionary HTML; structured output still includes result metadata."
	formatSchema.Enum = []any{"json", "markdown"}
	formatSchema.Default = json.RawMessage(`"json"`)
	formatSchema.Examples = []any{"markdown"}
	return schema
}

func normalizeMCPSearchFormat(value *string) (string, error) {
	if value == nil {
		return "", nil
	}
	format := strings.ToLower(strings.TrimSpace(*value))
	switch format {
	case "", "json":
		return "", nil
	case "markdown":
		return "markdown", nil
	default:
		return "", fmt.Errorf("unsupported format %q; omit format or use markdown", *value)
	}
}

func htmlToMarkdown(html string) (string, error) {
	markdown, err := htmltomarkdown.ConvertString(html)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(markdown), nil
}

func mcpMarkdownSearchResult(query string, out searchDictionaryOutput) *mcp.CallToolResult {
	var builder strings.Builder
	if strings.TrimSpace(query) != "" {
		fmt.Fprintf(&builder, "# Search results for %s\n\n", query)
	} else {
		builder.WriteString("# Search results\n\n")
	}
	if len(out.Results) == 0 {
		builder.WriteString("No matching entries found.")
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: builder.String()}}}
	}
	for index, result := range out.Results {
		if index > 0 {
			builder.WriteString("\n\n---\n\n")
		}
		fmt.Fprintf(&builder, "## %s\n\n", result.Word)
		fmt.Fprintf(&builder, "> Dictionary: %s  \n", result.DictionaryName)
		fmt.Fprintf(&builder, "> Visibility: %s  \n", result.Visibility)
		fmt.Fprintf(&builder, "> Source: %s  \n", result.Source)
		fmt.Fprintf(&builder, "> Score: %.4g\n\n", result.Score)
		builder.WriteString(strings.TrimSpace(result.Markdown))
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.TrimSpace(builder.String())}}}
}

func mcpTextResult(value any) *mcp.CallToolResult {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		payload = []byte(fmt.Sprint(value))
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(payload)}}}
}
