package company

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"go-code-runner/internal/models"
	rep "go-code-runner/internal/repository/company"
	"golang.org/x/crypto/bcrypt"
)

type service struct {
	repo rep.Repository
}

func New(repo rep.Repository) Service { return &service{repo: repo} }

func hashPwd(p string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(b), err
}

func checkPwd(hash, p string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(p)) == nil
}

func (s *service) Register(ctx context.Context, name, email, password string) (*models.Company, error) {
	h, err := hashPwd(password)
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, &models.Company{Name: name, Email: email, PasswordHash: h})
}

const jwtSecret = "your-secret-key-here"

func generateJWT(companyID int) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"company_id": companyID,
		"exp":        time.Now().Add(time.Hour * 24).Unix(),
	})

	return token.SignedString([]byte(jwtSecret))
}

func (s *service) Login(ctx context.Context, email, password string) (*models.Company, string, error) {
	c, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return nil, "", err
	}
	if !checkPwd(c.PasswordHash, password) {
		return nil, "", errors.New("invalid credentials")
	}

	token, err := generateJWT(c.ID)
	if err != nil {
		return nil, "", err
	}

	return c, token, nil
}

func (s *service) GenerateClientID(ctx context.Context, companyID int) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	clientID := hex.EncodeToString(b)
	if err := s.repo.UpdateClientID(ctx, companyID, clientID); err != nil {
		return "", err
	}
	return clientID, nil
}

func (s *service) GenerateAPIKey(ctx context.Context, companyID int) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	key := hex.EncodeToString(b)
	if err := s.repo.UpdateAPIKey(ctx, companyID, key); err != nil {
		return "", err
	}
	return key, nil
}
