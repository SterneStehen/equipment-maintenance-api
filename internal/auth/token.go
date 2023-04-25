package auth

import (
	"errors"
	"strconv"
	"time"

	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/golang-jwt/jwt/v4"
)

const issuer = "equipment-maintenance-api"

var ErrInvalidToken = errors.New("invalid access token")

type Principal struct {
	UserID int64
	Role   user.Role
}

type tokenClaims struct {
	UserID int64     `json:"uid"`
	Role   user.Role `json:"role"`
	jwt.StandardClaims
}

type Manager struct {
	secret []byte
	ttl    time.Duration
}

func NewManager(secret string, ttl time.Duration) *Manager {
	return &Manager{secret: []byte(secret), ttl: ttl}
}

func (m *Manager) Issue(u user.User) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(m.ttl)
	c := tokenClaims{
		UserID: u.ID,
		Role:   u.Role,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: exp.Unix(),
			IssuedAt:  now.Unix(),
			NotBefore: now.Unix(),
			Issuer:    issuer,
			Subject:   strconv.FormatInt(u.ID, 10),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	raw, err := tok.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return raw, exp, nil
}

func (m *Manager) Verify(raw string) (Principal, error) {
	c := &tokenClaims{}
	// We only make HS256 tokens, so accepting anything else would just invite trouble
	p := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	tok, err := p.ParseWithClaims(raw, c, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil || !tok.Valid || c.ExpiresAt == 0 || c.Issuer != issuer || c.UserID < 1 || !c.Role.Valid() {
		return Principal{}, ErrInvalidToken
	}
	if c.Subject != strconv.FormatInt(c.UserID, 10) {
		return Principal{}, ErrInvalidToken
	}
	return Principal{UserID: c.UserID, Role: c.Role}, nil
}
