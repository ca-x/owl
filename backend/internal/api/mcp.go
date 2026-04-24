package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"owl/backend/internal/dictionary"
	"owl/backend/pkg/version"

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
	DictionaryID int    `json:"dictionary_id" jsonschema:"Dictionary id returned by list_dictionaries."`
	Query        string `json:"query" jsonschema:"Word, phrase, or headword to look up."`
}

type searchResultInfo struct {
	DictionaryID   int     `json:"dictionary_id"`
	DictionaryName string  `json:"dictionary_name"`
	Visibility     string  `json:"visibility"`
	Word           string  `json:"word"`
	HTML           string  `json:"html"`
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
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_dictionary",
		Title:       "Search a dictionary",
		Description: "Search a specified dictionary by id. The id must come from list_dictionaries and must be public or owned by the MCP token user.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input searchDictionaryInput) (*mcp.CallToolResult, searchDictionaryOutput, error) {
		query := strings.TrimSpace(input.Query)
		if input.DictionaryID <= 0 {
			return nil, searchDictionaryOutput{}, fmt.Errorf("dictionary_id is required")
		}
		if query == "" {
			return nil, searchDictionaryOutput{}, fmt.Errorf("query is required")
		}
		results, err := s.dictionaries.Search(ctx, dictionary.SearchParams{UserID: userID, IsAdmin: false, Query: query, DictionaryID: input.DictionaryID})
		if err != nil {
			return nil, searchDictionaryOutput{}, err
		}
		out := searchDictionaryOutput{Results: make([]searchResultInfo, 0, len(results))}
		for _, result := range results {
			out.Results = append(out.Results, searchResultInfo{
				DictionaryID:   result.DictionaryID,
				DictionaryName: result.DictionaryName,
				Visibility:     result.Visibility,
				Word:           result.Word,
				HTML:           result.HTML,
				Score:          result.Score,
				Source:         result.Source,
			})
		}
		return nil, out, nil
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
