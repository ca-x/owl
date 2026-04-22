package api

import (
	"context"
	"net/http"
	"runtime"
	"strconv"
	"strings"

	"owl/backend/ent"
	"owl/backend/internal/dictionary"
	"owl/backend/internal/models"
	"owl/backend/internal/user"
	"owl/backend/pkg/version"

	echo "github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

const (
	authUserKey  = "auth_user"
	authCookieKey = "owl_token"
)

type Server struct {
	echo        *echo.Echo
	users       *user.Service
	dictionaries *dictionary.Service
	cancel      context.CancelFunc
}

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type toggleDictionaryRequest struct {
	Enabled bool `json:"enabled"`
}

type publicDictionaryRequest struct {
	Public bool `json:"public"`
}

type preferencesRequest struct {
	Language string `json:"language"`
	Theme    string `json:"theme"`
	FontMode string `json:"font_mode"`
}

type authedUser struct {
	ID       int
	Username string
	IsAdmin  bool
}

func New(client *ent.Client, userSvc *user.Service, dictSvc *dictionary.Service, frontendOrigin string) *Server {
	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{AllowOrigins: []string{frontendOrigin}, AllowHeaders: []string{"Authorization", "Content-Type"}, AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodOptions}}))

	s := &Server{echo: e, users: userSvc, dictionaries: dictSvc}
	e.GET("/api/health", s.handleHealth)
	e.GET("/api/public/dictionaries", s.handleListPublicDictionaries)
	e.GET("/api/public/search", s.handlePublicSearch)
	e.GET("/api/public/suggest", s.handlePublicSuggest)
	e.GET("/api/public/dictionaries/:id/resource/*", s.handlePublicDictionaryResource)
	e.POST("/api/auth/register", s.handleRegister)
	e.POST("/api/auth/login", s.handleLogin)
	e.POST("/api/auth/logout", s.handleLogout)

	api := e.Group("/api", s.authMiddleware)
	api.GET("/me", s.handleMe)
	api.GET("/preferences", s.handleGetPreferences)
	api.PUT("/preferences", s.handleUpdatePreferences)
	api.POST("/preferences/font", s.handleUploadFont)
	api.GET("/preferences/font", s.handleGetFont)
	api.GET("/dictionaries", s.handleListDictionaries)
	api.POST("/dictionaries/upload", s.handleUploadDictionary)
	api.DELETE("/dictionaries/:id", s.handleDeleteDictionary)
	api.PATCH("/dictionaries/:id", s.handleToggleDictionary)
	api.PATCH("/dictionaries/:id/public", s.handleSetDictionaryPublic)
	api.POST("/dictionaries/:id/refresh", s.handleRefreshDictionary)
	api.POST("/dictionaries/refresh", s.handleRefreshLibrary)
	api.GET("/dictionaries/:id/resource/*", s.handleDictionaryResource)
	api.GET("/search", s.handleSearch)
	api.GET("/suggest", s.handleSuggest)
	s.registerFrontendRoutes()
	return s
}

func (s *Server) Start(addr string) error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	return echo.StartConfig{
		Address:     addr,
		HideBanner:  true,
		HidePort:    false,
		GracefulTimeout: 10,
	}.Start(ctx, s.echo)
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

func (s *Server) handleHealth(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"status":      "ok",
		"version":     version.GetVersion(),
		"full_version": version.GetFullVersion(),
		"commit":      version.GitCommit,
		"build_time":  version.BuildTime,
		"go_version":  runtime.Version(),
		"os":          runtime.GOOS,
		"arch":        runtime.GOARCH,
	})
}

func (s *Server) handleListPublicDictionaries(c *echo.Context) error {
	items, err := s.dictionaries.ListPublic(c.Request().Context())
	if err != nil {
		return mapEntError(err)
	}
	return c.JSON(http.StatusOK, items)
}

func (s *Server) handlePublicSearch(c *echo.Context) error {
	query := strings.TrimSpace(c.QueryParam("q"))
	if query == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "q is required")
	}
	var dictionaryID int
	if raw := strings.TrimSpace(c.QueryParam("dict")); raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid dict value")
		}
		dictionaryID = id
	}
	results, err := s.dictionaries.Search(c.Request().Context(), dictionary.SearchParams{
		Query:        query,
		DictionaryID: dictionaryID,
		Guest:        true,
	})
	if err != nil {
		return mapEntError(err)
	}
	return c.JSON(http.StatusOK, results)
}

func (s *Server) handlePublicSuggest(c *echo.Context) error {
	query := strings.TrimSpace(c.QueryParam("q"))
	if query == "" {
		return c.JSON(http.StatusOK, []models.SearchSuggestion{})
	}
	var dictionaryID int
	if raw := strings.TrimSpace(c.QueryParam("dict")); raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid dict value")
		}
		dictionaryID = id
	}
	items, err := s.dictionaries.Suggest(c.Request().Context(), dictionary.SearchParams{
		Query:        query,
		DictionaryID: dictionaryID,
		Guest:        true,
	}, 8)
	if err != nil {
		return mapEntError(err)
	}
	return c.JSON(http.StatusOK, items)
}

func (s *Server) handlePublicDictionaryResource(c *echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid dictionary id")
	}
	resourcePath := c.Param("*")
	data, contentType, err := s.dictionaries.OpenResource(c.Request().Context(), id, 0, false, true, resourcePath)
	if err != nil {
		return mapEntError(err)
	}
	return c.Blob(http.StatusOK, contentType, data)
}

func (s *Server) handleRegister(c *echo.Context) error {
	var req registerRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	resp, err := s.users.Register(c.Request().Context(), req.Username, req.Password)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	s.setAuthCookie(c, resp.Token)
	return c.JSON(http.StatusCreated, resp)
}

func (s *Server) handleLogin(c *echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	resp, err := s.users.Login(c.Request().Context(), req.Username, req.Password)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}
	s.setAuthCookie(c, resp.Token)
	return c.JSON(http.StatusOK, resp)
}

func (s *Server) handleLogout(c *echo.Context) error {
	s.clearAuthCookie(c)
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) handleMe(c *echo.Context) error {
	user := currentUser(c)
	return c.JSON(http.StatusOK, user)
}

func (s *Server) handleGetPreferences(c *echo.Context) error {
	user := currentUser(c)
	prefs, err := s.users.GetPreferences(c.Request().Context(), user.ID)
	if err != nil {
		return mapEntError(err)
	}
	return c.JSON(http.StatusOK, prefs)
}

func (s *Server) handleUpdatePreferences(c *echo.Context) error {
	user := currentUser(c)
	var req preferencesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	prefs, err := s.users.UpdatePreferences(c.Request().Context(), user.ID, models.UserPreferences{
		Language: req.Language,
		Theme:    req.Theme,
		FontMode: req.FontMode,
	})
	if err != nil {
		return mapEntError(err)
	}
	return c.JSON(http.StatusOK, prefs)
}

func (s *Server) handleUploadFont(c *echo.Context) error {
	user := currentUser(c)
	fontFile, err := c.FormFile("font")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "font file is required")
	}
	prefs, err := s.users.UploadFont(c.Request().Context(), user.ID, fontFile)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return c.JSON(http.StatusOK, prefs)
}

func (s *Server) handleGetFont(c *echo.Context) error {
	user := currentUser(c)
	data, contentType, err := s.users.LoadFont(c.Request().Context(), user.ID)
	if err != nil {
		return mapEntError(err)
	}
	return c.Blob(http.StatusOK, contentType, data)
}

func (s *Server) handleListDictionaries(c *echo.Context) error {
	user := currentUser(c)
	items, err := s.dictionaries.List(c.Request().Context(), user.ID, user.IsAdmin)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}

func (s *Server) handleUploadDictionary(c *echo.Context) error {
	user := currentUser(c)
	mdxFile, err := c.FormFile("mdx")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "mdx file is required")
	}
	form, err := c.MultipartForm()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid multipart form")
	}
	mddFiles := form.File["mdd"]
	item, err := s.dictionaries.Upload(c.Request().Context(), user.ID, mdxFile, mddFiles)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return c.JSON(http.StatusCreated, item)
}

func (s *Server) handleDeleteDictionary(c *echo.Context) error {
	user := currentUser(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid dictionary id")
	}
	if err := s.dictionaries.Delete(c.Request().Context(), id, user.ID, user.IsAdmin); err != nil {
		return mapEntError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) handleToggleDictionary(c *echo.Context) error {
	user := currentUser(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid dictionary id")
	}
	var req toggleDictionaryRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	item, err := s.dictionaries.Toggle(c.Request().Context(), id, req.Enabled, user.ID, user.IsAdmin)
	if err != nil {
		return mapEntError(err)
	}
	return c.JSON(http.StatusOK, item)
}

func (s *Server) handleSetDictionaryPublic(c *echo.Context) error {
	user := currentUser(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid dictionary id")
	}
	var req publicDictionaryRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	item, err := s.dictionaries.SetPublic(c.Request().Context(), id, req.Public, user.ID, user.IsAdmin)
	if err != nil {
		return mapEntError(err)
	}
	return c.JSON(http.StatusOK, item)
}

func (s *Server) handleRefreshDictionary(c *echo.Context) error {
	user := currentUser(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid dictionary id")
	}
	report, err := s.dictionaries.Refresh(c.Request().Context(), id, user.ID, user.IsAdmin)
	if err != nil {
		return mapEntError(err)
	}
	return c.JSON(http.StatusOK, report)
}

func (s *Server) handleRefreshLibrary(c *echo.Context) error {
	user := currentUser(c)
	report, err := s.dictionaries.RefreshLibrary(c.Request().Context(), user.ID, user.IsAdmin)
	if err != nil {
		return mapEntError(err)
	}
	return c.JSON(http.StatusOK, report)
}

func (s *Server) handleSearch(c *echo.Context) error {
	user := currentUser(c)
	query := strings.TrimSpace(c.QueryParam("q"))
	if query == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "q is required")
	}
	var dictionaryID int
	if raw := strings.TrimSpace(c.QueryParam("dict")); raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid dict value")
		}
		dictionaryID = id
	}
	results, err := s.dictionaries.Search(c.Request().Context(), dictionary.SearchParams{UserID: user.ID, IsAdmin: user.IsAdmin, Query: query, DictionaryID: dictionaryID})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, results)
}

func (s *Server) handleSuggest(c *echo.Context) error {
	user := currentUser(c)
	query := strings.TrimSpace(c.QueryParam("q"))
	if query == "" {
		return c.JSON(http.StatusOK, []models.SearchSuggestion{})
	}
	var dictionaryID int
	if raw := strings.TrimSpace(c.QueryParam("dict")); raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid dict value")
		}
		dictionaryID = id
	}
	items, err := s.dictionaries.Suggest(c.Request().Context(), dictionary.SearchParams{
		UserID:       user.ID,
		IsAdmin:      user.IsAdmin,
		Query:        query,
		DictionaryID: dictionaryID,
	}, 8)
	if err != nil {
		return mapEntError(err)
	}
	return c.JSON(http.StatusOK, items)
}

func (s *Server) handleDictionaryResource(c *echo.Context) error {
	user := currentUser(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid dictionary id")
	}
	resourcePath := c.Param("*")
	data, contentType, err := s.dictionaries.OpenResource(c.Request().Context(), id, user.ID, user.IsAdmin, false, resourcePath)
	if err != nil {
		return mapEntError(err)
	}
	return c.Blob(http.StatusOK, contentType, data)
}

func (s *Server) authMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		authHeader := strings.TrimSpace(c.Request().Header.Get("Authorization"))
		tokenString := ""
		if authHeader != "" {
			const bearer = "Bearer "
			if !strings.HasPrefix(authHeader, bearer) {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid authorization header")
			}
			tokenString = strings.TrimSpace(strings.TrimPrefix(authHeader, bearer))
		} else if cookie, err := c.Cookie(authCookieKey); err == nil {
			tokenString = strings.TrimSpace(cookie.Value)
		}
		if tokenString == "" {
			return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization header")
		}
		claims, err := s.users.ParseToken(tokenString)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
		}
		c.Set(authUserKey, authedUser{ID: claims.UserID, Username: claims.Username, IsAdmin: claims.IsAdmin})
		return next(c)
	}
}

func (s *Server) setAuthCookie(c *echo.Context, token string) {
	c.SetCookie(&http.Cookie{
		Name:     authCookieKey,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearAuthCookie(c *echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     authCookieKey,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func currentUser(c *echo.Context) authedUser {
	value := c.Get(authUserKey)
	user, ok := value.(authedUser)
	if !ok {
		panic("auth user missing in context")
	}
	return user
}

func mapEntError(err error) error {
	if err == nil {
		return nil
	}
	if ent.IsNotFound(err) || strings.Contains(err.Error(), "resource not found") {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	return echo.NewHTTPError(http.StatusBadRequest, err.Error())
}
