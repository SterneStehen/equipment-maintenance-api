git commit --date="2023-03-19T17:08:59+0300" -m "update jwt dependencies and clean old doc files"
package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueAndVerifyToken(t *testing.T) {
	mgr := NewManager("unit-test-secret-that-is-not-returned", 15*time.Minute)
	raw, expires, err := mgr.Issue(user.User{ID: 42, Role: user.RoleTechnician})
	require.NoError(t, err)
	assert.False(t, strings.Contains(raw, "unit-test-secret"))
	assert.WithinDuration(t, time.Now().Add(15*time.Minute), expires, 2*time.Second)

	p, err := mgr.Verify(raw)
	require.NoError(t, err)
	assert.Equal(t, int64(42), p.UserID)
	assert.Equal(t, user.RoleTechnician, p.Role)
}

func TestVerifyRejectsInvalidTokens(t *testing.T) {
	mgr := NewManager("right-secret", time.Minute)
	other := NewManager("wrong-secret", time.Minute)
	valid, _, err := mgr.Issue(user.User{ID: 2, Role: user.RoleViewer})
	require.NoError(t, err)

	tests := []struct {
		name string
		raw  func() string
	}{
		{name: "garbage", raw: func() string { return "not-a-token" }},
		{name: "wrong signature", raw: func() string {
			raw, _, issueErr := other.Issue(user.User{ID: 2, Role: user.RoleViewer})
			require.NoError(t, issueErr)
			return raw
		}},
		{name: "tampered", raw: func() string { return valid + "x" }},
		{name: "expired", raw: func() string {
			claims := tokenClaims{
				UserID: 2,
				Role:   user.RoleViewer,
				StandardClaims: jwt.StandardClaims{
					ExpiresAt: time.Now().Add(-time.Minute).Unix(),
					Issuer:    issuer,
					Subject:   "2",
				},
			}
			raw, signErr := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("right-secret"))
			require.NoError(t, signErr)
			return raw
		}},
		{name: "missing expiration", raw: func() string {
			claims := tokenClaims{UserID: 2, Role: user.RoleViewer, StandardClaims: jwt.StandardClaims{
				Issuer: issuer, Subject: "2",
			}}
			raw, signErr := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("right-secret"))
			require.NoError(t, signErr)
			return raw
		}},
		{name: "wrong method", raw: func() string {
			claims := tokenClaims{UserID: 2, Role: user.RoleViewer, StandardClaims: jwt.StandardClaims{
				ExpiresAt: time.Now().Add(time.Minute).Unix(), Issuer: issuer, Subject: "2",
			}}
			raw, signErr := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
			require.NoError(t, signErr)
			return raw
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mgr.Verify(tt.raw())
			require.ErrorIs(t, err, ErrInvalidToken)
		})
	}
}
