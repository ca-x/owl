package user

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"owl/backend/ent"
	"owl/backend/ent/user"
	"owl/backend/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	client    *ent.Client
	jwtSecret []byte
	dataDir   string
}

type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
	jwt.RegisteredClaims
}

func NewService(client *ent.Client, jwtSecret string, dataDir string) *Service {
	return &Service{client: client, jwtSecret: []byte(jwtSecret), dataDir: dataDir}
}

func (s *Service) Register(ctx context.Context, username, password string) (*models.AuthResponse, error) {
	username = strings.TrimSpace(username)
	if username == "" || strings.TrimSpace(password) == "" {
		return nil, fmt.Errorf("username and password are required")
	}
	exists, err := s.client.User.Query().Where(user.UsernameEQ(username)).Exist(ctx)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("username already exists")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u, err := s.client.User.Create().SetUsername(username).SetPasswordHash(string(hash)).Save(ctx)
	if err != nil {
		return nil, err
	}
	return s.buildAuthResponse(u)
}

func (s *Service) Login(ctx context.Context, username, password string) (*models.AuthResponse, error) {
	u, err := s.client.User.Query().Where(user.UsernameEQ(strings.TrimSpace(username))).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	return s.buildAuthResponse(u)
}

func (s *Service) ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func (s *Service) UserSummaryFromClaims(claims *Claims) models.UserSummary {
	return models.UserSummary{ID: claims.UserID, Username: claims.Username, IsAdmin: claims.IsAdmin}
}

func (s *Service) EnsureAdmin(ctx context.Context, username, password string) error {
	exists, err := s.client.User.Query().Where(user.UsernameEQ(username)).Exist(ctx)
	if err != nil || exists {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.client.User.Create().SetUsername(username).SetPasswordHash(string(hash)).SetIsAdmin(true).Save(ctx)
	return err
}

func (s *Service) GetPreferences(ctx context.Context, userID int) (*models.UserPreferences, error) {
	u, err := s.client.User.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.preferencesFromUser(u), nil
}

func (s *Service) UpdatePreferences(ctx context.Context, userID int, prefs models.UserPreferences) (*models.UserPreferences, error) {
	updated, err := s.client.User.UpdateOneID(userID).
		SetLanguage(normalizeLanguage(prefs.Language)).
		SetTheme(normalizeTheme(prefs.Theme)).
		SetFontMode(normalizeFontMode(prefs.FontMode)).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return s.preferencesFromUser(updated), nil
}

func (s *Service) UploadFont(ctx context.Context, userID int, fontFile *multipart.FileHeader) (*models.UserPreferences, error) {
	if fontFile == nil {
		return nil, fmt.Errorf("font file is required")
	}
	userDir := filepath.Join(s.dataDir, "fonts", fmt.Sprintf("user-%d", userID))
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return nil, err
	}
	filename := filepath.Base(fontFile.Filename)
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".ttf", ".otf", ".woff", ".woff2":
	default:
		return nil, fmt.Errorf("unsupported font type")
	}
	src, err := fontFile.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	fontPath := filepath.Join(userDir, filename)
	dst, err := os.Create(fontPath)
	if err != nil {
		return nil, err
	}
	defer dst.Close()
	if _, err := dst.ReadFrom(src); err != nil {
		return nil, err
	}

	fontFamily := sanitizeFontFamily(strings.TrimSuffix(filename, ext))
	updated, err := s.client.User.UpdateOneID(userID).
		SetFontMode("custom").
		SetCustomFontName(filename).
		SetCustomFontPath(fontPath).
		SetCustomFontFamily(fontFamily).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return s.preferencesFromUser(updated), nil
}

func (s *Service) LoadFont(ctx context.Context, userID int) ([]byte, string, error) {
	u, err := s.client.User.Get(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(u.CustomFontPath) == "" {
		return nil, "", fmt.Errorf("custom font not found")
	}
	data, err := os.ReadFile(u.CustomFontPath)
	if err != nil {
		return nil, "", err
	}
	return data, detectFontContentType(u.CustomFontPath), nil
}

func (s *Service) preferencesFromUser(u *ent.User) *models.UserPreferences {
	prefs := &models.UserPreferences{
		Language:         normalizeLanguage(u.Language),
		Theme:            normalizeTheme(u.Theme),
		FontMode:         normalizeFontMode(u.FontMode),
		CustomFontName:   u.CustomFontName,
		CustomFontFamily: u.CustomFontFamily,
	}
	if strings.TrimSpace(u.CustomFontPath) != "" {
		prefs.CustomFontURL = "/api/preferences/font"
	}
	return prefs
}

func normalizeLanguage(value string) string {
	switch strings.TrimSpace(value) {
	case "en":
		return "en"
	default:
		return "zh-CN"
	}
}

func normalizeTheme(value string) string {
	switch strings.TrimSpace(value) {
	case "light", "dark", "sepia", "system":
		return value
	default:
		return "system"
	}
}

func normalizeFontMode(value string) string {
	switch strings.TrimSpace(value) {
	case "sans", "serif", "mono", "custom":
		return value
	default:
		return "sans"
	}
}

func sanitizeFontFamily(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	if value == "" {
		return "Owl Custom Font"
	}
	return value
}

func detectFontContentType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".woff2":
		return "font/woff2"
	case ".woff":
		return "font/woff"
	case ".otf":
		return "font/otf"
	case ".ttf":
		return "font/ttf"
	default:
		return http.DetectContentType([]byte(path))
	}
}

func (s *Service) buildAuthResponse(u *ent.User) (*models.AuthResponse, error) {
	now := time.Now()
	claims := Claims{
		UserID:   u.ID,
		Username: u.Username,
		IsAdmin:  u.IsAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   fmt.Sprintf("user:%d", u.ID),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return nil, err
	}
	return &models.AuthResponse{Token: tokenString, User: models.UserSummary{ID: u.ID, Username: u.Username, IsAdmin: u.IsAdmin}}, nil
}
