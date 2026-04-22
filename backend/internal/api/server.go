package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"owl/backend/ent"
	"owl/backend/internal/dictionary"
	"owl/backend/internal/user"

	echo "github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
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

type authedUser struct {
	ID       int
	Username string
	IsAdmin  bool
}

const authUserKey = "auth_user"

func New(client *ent.Client, userSvc *user.Service, dictSvc *dictionary.Service, frontendOrigin string) *Server {
	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{AllowOrigins: []string{frontendOrigin}, AllowHeaders: []string{"Authorization", "Content-Type"}, AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodOptions}}))

	s := &Server{echo: e, users: userSvc, dictionaries: dictSvc}
	e.GET("/api/health", s.handleHealth)
	e.POST("/api/auth/register", s.handleRegister)
	e.POST("/api/auth/login", s.handleLogin)

	api := e.Group("/api", s.authMiddleware)
	api.GET("/me", s.handleMe)
	api.GET("/dictionaries", s.handleListDictionaries)
	api.POST("/dictionaries/upload", s.handleUploadDictionary)
	api.DELETE("/dictionaries/:id", s.handleDeleteDictionary)
	api.PATCH("/dictionaries/:id", s.handleToggleDictionary)
	api.GET("/dictionaries/:id/resource/*", s.handleDictionaryResource)
	api.GET("/search", s.handleSearch)
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
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
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
	return c.JSON(http.StatusOK, resp)
}

func (s *Server) handleMe(c *echo.Context) error {
	user := currentUser(c)
	return c.JSON(http.StatusOK, user)
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

func (s *Server) handleDictionaryResource(c *echo.Context) error {
	user := currentUser(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid dictionary id")
	}
	resourcePath := c.Param("*")
	data, contentType, err := s.dictionaries.OpenResource(c.Request().Context(), id, user.ID, user.IsAdmin, resourcePath)
	if err != nil {
		return mapEntError(err)
	}
	return c.Blob(http.StatusOK, contentType, data)
}

func (s *Server) authMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		authHeader := strings.TrimSpace(c.Request().Header.Get("Authorization"))
		if authHeader == "" {
			return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization header")
		}
		const bearer = "Bearer "
		if !strings.HasPrefix(authHeader, bearer) {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid authorization header")
		}
		claims, err := s.users.ParseToken(strings.TrimSpace(strings.TrimPrefix(authHeader, bearer)))
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
		}
		c.Set(authUserKey, authedUser{ID: claims.UserID, Username: claims.Username, IsAdmin: claims.IsAdmin})
		return next(c)
	}
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
