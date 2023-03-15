package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/SterneStehen/equipment-maintenance-api/internal/apperror"
	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/gin-gonic/gin"
)

type users interface {
	Register(ctx context.Context, in user.RegisterInput) (user.User, error)
	Authenticate(ctx context.Context, email, password string) (user.User, error)
	ByID(ctx context.Context, id int64) (user.User, error)
}

type tokenMaker interface {
	Issue(u user.User) (string, time.Time, error)
}

type Handler struct {
	users  users
	tokens tokenMaker
}

// No role here on purpose. Registration decides it on the server
type regReq struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	FullName string `json:"full_name" binding:"required"`
}

type loginReq struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func NewHandler(users users, tokens tokenMaker) *Handler {
	return &Handler{users: users, tokens: tokens}
}

func (h *Handler) Register(c *gin.Context) {
	var req regReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apperror.Write(c, http.StatusBadRequest, "invalid_request", "A valid email, password, and full name are required")
		return
	}

	usr, err := h.users.Register(c.Request.Context(), user.RegisterInput{
		Email: req.Email, Password: req.Password, FullName: req.FullName,
	})
	if err != nil {
		userErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user": usr})
}

func (h *Handler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apperror.Write(c, http.StatusBadRequest, "invalid_request", "Email and password are required")
		return
	}

	usr, err := h.users.Authenticate(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		userErr(c, err)
		return
	}

	token, expires, err := h.tokens.Issue(usr)
	if err != nil {
		apperror.Write(c, http.StatusInternalServerError, "internal_error", "Could not issue access token")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_at":   expires,
		"user":         usr,
	})
}

func (h *Handler) Me(c *gin.Context) {
	p, ok := Current(c)
	if !ok {
		apperror.Write(c, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return
	}
	usr, err := h.users.ByID(c.Request.Context(), p.UserID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			apperror.Write(c, http.StatusUnauthorized, "unauthorized", "Authenticated user no longer exists")
			return
		}
		apperror.Write(c, http.StatusInternalServerError, "internal_error", "Could not load current user")
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": usr})
}

func userErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, user.ErrInvalidEmail):
		apperror.Write(c, http.StatusBadRequest, "invalid_email", "Email address is invalid")
	case errors.Is(err, user.ErrInvalidPassword):
		apperror.Write(c, http.StatusBadRequest, "invalid_password", "Password must contain 8 to 72 bytes")
	case errors.Is(err, user.ErrInvalidName):
		apperror.Write(c, http.StatusBadRequest, "invalid_full_name", "Full name is required")
	case errors.Is(err, user.ErrEmailTaken):
		apperror.Write(c, http.StatusConflict, "email_already_registered", "Email address is already registered")
	case errors.Is(err, user.ErrInvalidCredentials):
		apperror.Write(c, http.StatusUnauthorized, "invalid_credentials", "Email or password is incorrect")
	default:
		apperror.Write(c, http.StatusInternalServerError, "internal_error", "Unexpected authentication error")
	}
}
