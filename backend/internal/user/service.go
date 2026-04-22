package user

import (
	"context"
	"fmt"
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
}

type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
	jwt.RegisteredClaims
}

func NewService(client *ent.Client, jwtSecret string) *Service {
	return &Service{client: client, jwtSecret: []byte(jwtSecret)}
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
