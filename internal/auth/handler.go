package auth

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/SterneStehen/equipment-maintenance-api/internal/apperror"
	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/gin-gonic/gin"
)

type users interface {
	Register(ctx context.Context, in user.RegisterInput) (user.User, error)
	Authenticate(ctx context.Context, email, password string) (user.User, error)
	ByID(ctx context.Context, id int64) (user.User, error)
	List(ctx context.Context, actor user.Actor) ([]user.User, error)
	Lookup(ctx context.Context, actor user.Actor, id int64) (user.User, error)
	AssignRole(ctx context.Context, actor user.Actor, id int64, role user.Role) (user.User, error)
}

type tokenMaker interface {
	Issue(u user.User) (string, time.Time, error)
}

type Handler struct {
	users  users
	tokens tokenMaker
}

// Role is not part of signup; the server picks it later
type regReq struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	FullName string `json:"full_name" binding:"required"`
}

type loginReq struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type roleReq struct {
	R user.Role `json:"role" binding:"required"`
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

func (h *Handler) ListUsers(c *gin.Context) {
	who, ok := actorFromReq(c)
	if !ok {
		return
	}
	arr, err := h.users.List(c.Request.Context(), who)
	if err != nil {
		userErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": arr})
}

func (h *Handler) GetUser(c *gin.Context) {
	who, ok := actorFromReq(c)
	if !ok {
		return
	}
	id, ok := idFromPath(c)
	if !ok {
		return
	}
	usr, err := h.users.Lookup(c.Request.Context(), who, id)
	if err != nil {
		userErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": usr})
}

func (h *Handler) UpdateUserRole(c *gin.Context) {
	who, ok := actorFromReq(c)
	if !ok {
		return
	}
	id, ok := idFromPath(c)
	if !ok {
		return
	}
	var req roleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apperror.Write(c, http.StatusBadRequest, "invalid_request", "Role is required")
		return
	}

	usr, err := h.users.AssignRole(c.Request.Context(), who, id, req.R)
	if err != nil {
		userErr(c, err)
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
	case errors.Is(err, user.ErrInvalidRole):
		apperror.Write(c, http.StatusBadRequest, "invalid_role", "Role is invalid")
	case errors.Is(err, user.ErrEmailTaken):
		apperror.Write(c, http.StatusConflict, "email_already_registered", "Email address is already registered")
	case errors.Is(err, user.ErrInvalidCredentials):
		apperror.Write(c, http.StatusUnauthorized, "invalid_credentials", "Email or password is incorrect")
	case errors.Is(err, user.ErrNotFound):
		apperror.Write(c, http.StatusNotFound, "not_found", "User was not found")
	case errors.Is(err, user.ErrPermissionDenied):
		apperror.Write(c, http.StatusForbidden, "forbidden", "You do not have permission to do that")
	case errors.Is(err, user.ErrLastAdmin):
		apperror.Write(c, http.StatusConflict, "last_admin", "The last administrator cannot lose admin role")
	default:
		apperror.Write(c, http.StatusInternalServerError, "internal_error", "Unexpected authentication error")
	}
}

func actorFromReq(c *gin.Context) (user.Actor, bool) {
	p, ok := Current(c)
	if !ok {
		apperror.Write(c, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return user.Actor{}, false
	}
	return user.Actor{UserID: p.UserID, Role: p.Role}, true
}

func idFromPath(c *gin.Context) (int64, bool) {
	raw := c.Param("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		apperror.Write(c, http.StatusNotFound, "not_found", "User was not found")
		return 0, false
	}
	return id, true
}
