package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"owl/backend/ent"
	entfont "owl/backend/ent/font"
	entuser "owl/backend/ent/user"
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
	exists, err := s.client.User.Query().Where(entuser.UsernameEQ(username)).Exist(ctx)
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
	u, err := s.client.User.Create().SetUsername(username).SetDisplayName(username).SetPasswordHash(string(hash)).Save(ctx)
	if err != nil {
		return nil, err
	}
	return s.buildAuthResponse(u)
}

func (s *Service) Login(ctx context.Context, username, password string) (*models.AuthResponse, error) {
	u, err := s.client.User.Query().Where(entuser.UsernameEQ(strings.TrimSpace(username))).Only(ctx)
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
	u, err := s.client.User.Get(context.Background(), claims.UserID)
	if err == nil {
		return s.userSummaryFromUser(u)
	}
	return models.UserSummary{ID: claims.UserID, Username: claims.Username, DisplayName: claims.Username, IsAdmin: claims.IsAdmin}
}

func (s *Service) EnsureAdmin(ctx context.Context, username, password string) error {
	exists, err := s.client.User.Query().Where(entuser.UsernameEQ(username)).Exist(ctx)
	if err != nil || exists {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.client.User.Create().SetUsername(username).SetDisplayName(username).SetPasswordHash(string(hash)).SetIsAdmin(true).Save(ctx)
	return err
}

func (s *Service) GetPreferences(ctx context.Context, userID int) (*models.UserPreferences, error) {
	u, err := s.client.User.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.buildPreferences(ctx, u), nil
}

func (s *Service) GetUserSummary(ctx context.Context, userID int) (*models.UserSummary, error) {
	u, err := s.client.User.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	summary := s.userSummaryFromUser(u)
	return &summary, nil
}

func (s *Service) UpdatePreferences(ctx context.Context, userID int, prefs models.UserPreferences) (*models.UserPreferences, error) {
	fontMode := normalizeFontMode(prefs.FontMode)
	update := s.client.User.UpdateOneID(userID).
		SetLanguage(normalizeLanguage(prefs.Language)).
		SetTheme(normalizeTheme(prefs.Theme)).
		SetFontMode(fontMode).
		SetRecentSearchLimit(normalizeRecentSearchLimit(prefs.RecentSearchLimit))
	if displayName := strings.TrimSpace(prefs.DisplayName); displayName != "" {
		update = update.SetDisplayName(displayName)
	}
	if fontMode == "custom" && strings.TrimSpace(prefs.CustomFontName) != "" {
		fontEntity, err := s.client.Font.Query().Where(entfont.NameEQ(filepath.Base(prefs.CustomFontName))).Only(ctx)
		if err == nil {
			update = update.SetSelectedFontID(fontEntity.ID)
		}
	} else if fontMode != "custom" {
		update = update.ClearSelectedFont()
	}
	updated, err := update.Save(ctx)
	if err != nil {
		return nil, err
	}
	return s.buildPreferences(ctx, updated), nil
}

func (s *Service) UploadAvatar(ctx context.Context, userID int, avatarFile *multipart.FileHeader) (*models.UserPreferences, error) {
	if avatarFile == nil {
		return nil, fmt.Errorf("avatar file is required")
	}
	userDir := filepath.Join(s.dataDir, "avatars", fmt.Sprintf("user-%d", userID))
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return nil, err
	}
	filename := filepath.Base(avatarFile.Filename)
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
	default:
		return nil, fmt.Errorf("unsupported avatar type")
	}
	src, err := avatarFile.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()
	avatarPath := filepath.Join(userDir, filename)
	dst, err := os.Create(avatarPath)
	if err != nil {
		return nil, err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return nil, err
	}
	updated, err := s.client.User.UpdateOneID(userID).
		SetAvatarName(filename).
		SetAvatarPath(avatarPath).
		SetAvatarMime(detectAvatarContentType(avatarPath)).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return s.buildPreferences(ctx, updated), nil
}

func (s *Service) LoadAvatar(ctx context.Context, userID int) ([]byte, string, error) {
	u, err := s.client.User.Get(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(u.AvatarPath) == "" {
		return nil, "", fmt.Errorf("avatar not found")
	}
	data, err := os.ReadFile(u.AvatarPath)
	if err != nil {
		return nil, "", err
	}
	return data, firstNonEmpty(strings.TrimSpace(u.AvatarMime), detectAvatarContentType(u.AvatarPath)), nil
}

func (s *Service) UploadFont(ctx context.Context, userID int, fontFile *multipart.FileHeader) (*models.UserPreferences, error) {
	if fontFile == nil {
		return nil, fmt.Errorf("font file is required")
	}
	fontDir := s.sharedFontDir()
	if err := os.MkdirAll(fontDir, 0o755); err != nil {
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
	fontPath := filepath.Join(fontDir, filename)
	dst, err := os.Create(fontPath)
	if err != nil {
		return nil, err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return nil, err
	}
	family := sanitizeFontFamily(strings.TrimSuffix(filename, ext))
	fontEntity, err := s.client.Font.Query().Where(entfont.NameEQ(filename)).Only(ctx)
	if err == nil {
		fontEntity, err = s.client.Font.UpdateOneID(fontEntity.ID).
			SetFamily(family).
			SetPath(fontPath).
			SetMime(detectFontContentType(fontPath)).
			Save(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		fontEntity, err = s.client.Font.Create().
			SetName(filename).
			SetFamily(family).
			SetPath(fontPath).
			SetMime(detectFontContentType(fontPath)).
			Save(ctx)
		if err != nil {
			return nil, err
		}
	}
	updated, err := s.client.User.UpdateOneID(userID).
		SetFontMode("custom").
		SetSelectedFontID(fontEntity.ID).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return s.buildPreferences(ctx, updated), nil
}

func (s *Service) DeleteFont(ctx context.Context, userID int, fontName string) (*models.UserPreferences, error) {
	target := filepath.Base(strings.TrimSpace(fontName))
	if target == "" {
		return nil, fmt.Errorf("font name is required")
	}
	fontEntity, err := s.client.Font.Query().Where(entfont.NameEQ(target)).Only(ctx)
	if err != nil {
		return nil, err
	}
	_ = os.Remove(fontEntity.Path)
	if _, err := s.client.User.Update().
		Where(entuser.HasSelectedFontWith(entfont.IDEQ(fontEntity.ID))).
		SetFontMode("sans").
		ClearSelectedFont().
		Save(ctx); err != nil {
		return nil, err
	}
	if err := s.client.Font.DeleteOneID(fontEntity.ID).Exec(ctx); err != nil {
		return nil, err
	}
	u, err := s.client.User.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.buildPreferences(ctx, u), nil
}

func (s *Service) LoadFont(ctx context.Context, userID int) ([]byte, string, error) {
	u, err := s.client.User.Get(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	selected, err := s.selectedFont(ctx, u)
	if err != nil || selected == nil {
		return nil, "", fmt.Errorf("custom font not found")
	}
	data, err := os.ReadFile(selected.Path)
	if err != nil {
		return nil, "", err
	}
	return data, detectFontContentType(selected.Path), nil
}

func (s *Service) selectedFont(ctx context.Context, u *ent.User) (*ent.Font, error) {
	if u == nil {
		return nil, fmt.Errorf("user is nil")
	}
	if u.Edges.SelectedFont != nil {
		return u.Edges.SelectedFont, nil
	}
	selected, err := u.QuerySelectedFont().Only(ctx)
	if err != nil {
		return nil, err
	}
	return selected, nil
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
	case "paper", "blue", "green", "retro", "ibm", "nokia", "gameboy", "blackberry", "nintendo", "dark", "mono", "system":
		return value
	case "light", "sepia":
		return "paper"
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

func normalizeRecentSearchLimit(value int) int {
	if value < 0 {
		return 0
	}
	if value > 20 {
		return 20
	}
	return value
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

func (s *Service) GetMCPTokenStatus(ctx context.Context, userID int) (*models.MCPTokenStatus, error) {
	u, err := s.client.User.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &models.MCPTokenStatus{Configured: strings.TrimSpace(u.McpTokenHash) != "", Hint: u.McpTokenHint}, nil
}

func (s *Service) SetMCPToken(ctx context.Context, userID int, token string) (*models.MCPTokenStatus, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("mcp token is required")
	}
	if len(token) < 16 {
		return nil, fmt.Errorf("mcp token must be at least 16 characters")
	}
	hash := hashMCPToken(token)
	exists, err := s.client.User.Query().Where(entuser.McpTokenHashEQ(hash), entuser.IDNEQ(userID)).Exist(ctx)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("mcp token is already in use")
	}
	hint := tokenHint(token)
	if _, err := s.client.User.UpdateOneID(userID).SetMcpTokenHash(hash).SetMcpTokenHint(hint).Save(ctx); err != nil {
		return nil, err
	}
	return &models.MCPTokenStatus{Configured: true, Hint: hint, Token: token}, nil
}

func (s *Service) GenerateMCPToken(ctx context.Context, userID int) (*models.MCPTokenStatus, error) {
	for range 3 {
		token, err := randomMCPToken()
		if err != nil {
			return nil, err
		}
		status, err := s.SetMCPToken(ctx, userID, token)
		if err == nil {
			return status, nil
		}
		if !strings.Contains(err.Error(), "already in use") {
			return nil, err
		}
	}
	return nil, fmt.Errorf("failed to generate a unique mcp token")
}

func (s *Service) DeleteMCPToken(ctx context.Context, userID int) (*models.MCPTokenStatus, error) {
	if _, err := s.client.User.UpdateOneID(userID).SetMcpTokenHash("").SetMcpTokenHint("").Save(ctx); err != nil {
		return nil, err
	}
	return &models.MCPTokenStatus{Configured: false}, nil
}

func (s *Service) AuthenticateMCPToken(ctx context.Context, token string) (*Claims, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("mcp token is required")
	}
	hash := hashMCPToken(token)
	u, err := s.client.User.Query().Where(entuser.McpTokenHashEQ(hash)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("invalid mcp token")
	}
	if subtle.ConstantTimeCompare([]byte(hash), []byte(u.McpTokenHash)) != 1 {
		return nil, fmt.Errorf("invalid mcp token")
	}
	return &Claims{UserID: u.ID, Username: u.Username, IsAdmin: u.IsAdmin}, nil
}

func randomMCPToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "owl_mcp_" + base64.RawURLEncoding.EncodeToString(bytes), nil
}

func hashMCPToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func tokenHint(token string) string {
	token = strings.TrimSpace(token)
	if len(token) <= 8 {
		return token
	}
	return token[:4] + "…" + token[len(token)-4:]
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
	return &models.AuthResponse{Token: tokenString, User: s.userSummaryFromUser(u)}, nil
}

func (s *Service) userSummaryFromUser(u *ent.User) models.UserSummary {
	summary := models.UserSummary{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: firstNonEmpty(strings.TrimSpace(u.DisplayName), u.Username),
		IsAdmin:     u.IsAdmin,
	}
	if strings.TrimSpace(u.AvatarPath) != "" {
		summary.AvatarURL = "/api/preferences/avatar"
	}
	return summary
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func detectAvatarContentType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return http.DetectContentType([]byte(path))
	}
}

func (s *Service) sharedFontDir() string {
	return filepath.Join(s.dataDir, "shared-fonts")
}

func (s *Service) ListSharedFonts(ctx context.Context) []models.SharedFont {
	fonts, err := s.client.Font.Query().Order(entfont.ByName()).All(ctx)
	if err != nil {
		return []models.SharedFont{}
	}
	out := make([]models.SharedFont, 0, len(fonts))
	for _, font := range fonts {
		out = append(out, sharedFontModel(font))
	}
	return out
}

func (s *Service) LoadSharedFont(ctx context.Context, fontName string) ([]byte, string, error) {
	target := filepath.Base(strings.TrimSpace(fontName))
	if target == "" {
		return nil, "", fmt.Errorf("font name is required")
	}
	fontEntity, err := s.client.Font.Query().Where(entfont.NameEQ(target)).Only(ctx)
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(fontEntity.Path)
	if err != nil {
		return nil, "", err
	}
	return data, firstNonEmpty(strings.TrimSpace(fontEntity.Mime), detectFontContentType(fontEntity.Path)), nil
}

func sharedFontModel(font *ent.Font) models.SharedFont {
	return models.SharedFont{
		Name:   font.Name,
		Family: font.Family,
		URL:    "/api/public/fonts/" + url.PathEscape(font.Name),
	}
}

func (s *Service) buildPreferences(ctx context.Context, u *ent.User) *models.UserPreferences {
	prefs := &models.UserPreferences{
		Language:          normalizeLanguage(u.Language),
		Theme:             normalizeTheme(u.Theme),
		FontMode:          normalizeFontMode(u.FontMode),
		DisplayName:       firstNonEmpty(strings.TrimSpace(u.DisplayName), u.Username),
		RecentSearchLimit: normalizeRecentSearchLimit(u.RecentSearchLimit),
		AvailableFonts:    s.ListSharedFonts(ctx),
	}
	if selected, err := s.selectedFont(ctx, u); err == nil && selected != nil {
		prefs.CustomFontName = selected.Name
		prefs.CustomFontFamily = selected.Family
		prefs.CustomFontURL = sharedFontModel(selected).URL
	}
	if strings.TrimSpace(u.AvatarPath) != "" {
		prefs.AvatarURL = "/api/preferences/avatar"
	}
	return prefs
}
