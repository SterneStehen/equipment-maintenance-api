package equipment

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/SterneStehen/equipment-maintenance-api/internal/apperror"
	"github.com/SterneStehen/equipment-maintenance-api/internal/auth"
	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/gin-gonic/gin"
)

type svc interface {
	Create(ctx context.Context, actor user.Actor, in CreateInput) (Equipment, error)
	ByID(ctx context.Context, actor user.Actor, id int64) (Equipment, error)
	List(ctx context.Context, actor user.Actor, f ListFilter) ([]Equipment, error)
	Update(ctx context.Context, actor user.Actor, id int64, in UpdateInput) (Equipment, error)
	Decommission(ctx context.Context, actor user.Actor, id int64) (Equipment, error)
}

type Handler struct {
	svc svc
}

type createReq struct {
	SerialNumber string `json:"serial_number" binding:"required"`
	Name         string `json:"name" binding:"required"`
	Model        string `json:"model"`
	Location     string `json:"location"`
	Notes        string `json:"notes"`
}

type updateReq struct {
	Name     string `json:"name" binding:"required"`
	Model    string `json:"model"`
	Location string `json:"location"`
	Status   Status `json:"status"`
	Notes    string `json:"notes"`
}

func NewHandler(svc svc) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Create(c *gin.Context) {
	who, ok := who(c)
	if !ok {
		return
	}

	var req createReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apperror.Write(c, http.StatusBadRequest, "invalid_request", "Serial number and name are required")
		return
	}

	x, err := h.svc.Create(c.Request.Context(), who, CreateInput{
		SerialNumber: req.SerialNumber,
		Name:         req.Name,
		Model:        req.Model,
		Location:     req.Location,
		Notes:        req.Notes,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"equipment": x})
}

func (h *Handler) List(c *gin.Context) {
	who, ok := who(c)
	if !ok {
		return
	}

	flt, ok := filterFromQuery(c)
	if !ok {
		return
	}
	stuff, err := h.svc.List(c.Request.Context(), who, flt)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"equipment": stuff, "limit": flt.Limit, "offset": flt.Offset})
}

func (h *Handler) Get(c *gin.Context) {
	who, ok := who(c)
	if !ok {
		return
	}
	id, ok := eqID(c)
	if !ok {
		return
	}
	x, err := h.svc.ByID(c.Request.Context(), who, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"equipment": x})
}

func (h *Handler) Update(c *gin.Context) {
	who, ok := who(c)
	if !ok {
		return
	}
	id, ok := eqID(c)
	if !ok {
		return
	}

	var req updateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apperror.Write(c, http.StatusBadRequest, "invalid_request", "Name is required")
		return
	}
	x, err := h.svc.Update(c.Request.Context(), who, id, UpdateInput{
		Name:     req.Name,
		Model:    req.Model,
		Location: req.Location,
		Status:   req.Status,
		Notes:    req.Notes,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"equipment": x})
}

func (h *Handler) Decommission(c *gin.Context) {
	who, ok := who(c)
	if !ok {
		return
	}
	id, ok := eqID(c)
	if !ok {
		return
	}
	x, err := h.svc.Decommission(c.Request.Context(), who, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"equipment": x})
}

func (h *Handler) Delete(c *gin.Context) {
	apperror.Write(c, http.StatusMethodNotAllowed, "not_supported", "Equipment can only be decommissioned")
}

func who(c *gin.Context) (user.Actor, bool) {
	p, ok := auth.Current(c)
	if !ok {
		apperror.Write(c, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return user.Actor{}, false
	}
	return user.Actor{UserID: p.UserID, Role: p.Role}, true
}

func eqID(c *gin.Context) (int64, bool) {
	raw := c.Param("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		apperror.Write(c, http.StatusNotFound, "not_found", "Equipment was not found")
		return 0, false
	}
	return id, true
}

func filterFromQuery(c *gin.Context) (ListFilter, bool) {
	lim, ok := intQuery(c, "limit", 0)
	if !ok {
		return ListFilter{}, false
	}
	off, ok := intQuery(c, "offset", 0)
	if !ok {
		return ListFilter{}, false
	}

	return ListFilter{
		Status: Status(c.Query("status")),
		Query:  c.Query("q"),
		Limit:  lim,
		Offset: off,
	}, true
}

func intQuery(c *gin.Context, key string, def int) (int, bool) {
	raw := c.Query(key)
	if raw == "" {
		return def, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		apperror.Write(c, http.StatusBadRequest, "invalid_request", key+" must be a number")
		return 0, false
	}
	return n, true
}

func writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrPermissionDenied):
		apperror.Write(c, http.StatusForbidden, "forbidden", "You do not have permission to manage equipment")
	case errors.Is(err, ErrInvalidSerial):
		apperror.Write(c, http.StatusBadRequest, "invalid_serial_number", "Serial number is invalid")
	case errors.Is(err, ErrInvalidName):
		apperror.Write(c, http.StatusBadRequest, "invalid_name", "Equipment name is required")
	case errors.Is(err, ErrInvalidStatus):
		apperror.Write(c, http.StatusBadRequest, "invalid_status", "Equipment status is invalid")
	case errors.Is(err, ErrSerialTaken):
		apperror.Write(c, http.StatusConflict, "serial_number_exists", "Equipment serial number already exists")
	case errors.Is(err, ErrAlreadyRetired):
		apperror.Write(c, http.StatusConflict, "already_decommissioned", "Equipment is already decommissioned")
	case errors.Is(err, ErrNotFound):
		apperror.Write(c, http.StatusNotFound, "not_found", "Equipment was not found")
	default:
		apperror.Write(c, http.StatusInternalServerError, "internal_error", "Unexpected equipment error")
	}
}
