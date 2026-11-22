package api

import (
	"io/fs"
	"net/http"
	"strings"

	"owl/backend/web"

	echo "github.com/labstack/echo/v5"
)

func (s *Server) registerFrontendRoutes() {
	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		panic(err)
	}

	spa := s.handleSPA(distFS)
	s.echo.GET("/", spa)
	// The desktop web view supplies its session on the first navigation.
	// The existing authentication middleware sets the usual HttpOnly cookie.
	s.echo.GET("/workspace", func(c *echo.Context) error {
		c.Response().Header().Set("Cache-Control", "private, no-store")
		if c.Request().Header.Get("Authorization") != "" {
			return s.authMiddleware(spa)(c)
		}
		return spa(c)
	})
	s.echo.GET("/*", spa)
}

func (s *Server) handleSPA(distFS fs.FS) echo.HandlerFunc {
	fileServer := http.FileServer(http.FS(distFS))
	return func(c *echo.Context) error {
		path := c.Request().URL.Path
		if strings.HasPrefix(path, "/api/") {
			return echo.NewHTTPError(http.StatusNotFound, "not found")
		}
		trimmed := strings.TrimPrefix(path, "/")
		if trimmed == "" {
			return c.FileFS("index.html", distFS)
		}
		if _, err := fs.Stat(distFS, trimmed); err == nil {
			fileServer.ServeHTTP(c.Response(), c.Request())
			return nil
		}
		return c.FileFS("index.html", distFS)
	}
}
