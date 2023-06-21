package maintenance

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
	List(ctx context.Context, actor user.Actor, f ListFilter) ([]Record, error)
}

type Handler struct {
	svc svc
}

func NewHandler(svc svc) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) List(c *gin.Context) {
	who, ok := actor(c)
	if !ok {
		return
	}
	f, ok := filter(c)
	if !ok {
		return
	}
	arr, err := h.svc.List(c.Request.Context(), who, f)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"maintenance_records": arr, "limit": f.Limit, "offset": f.Offset})
}

func actor(c *gin.Context) (user.Actor, bool) {
	p, ok := auth.Current(c)
	if !ok {
		apperror.Write(c, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return user.Actor{}, false
	}
	return user.Actor{UserID: p.UserID, Role: p.Role}, true
}

func filter(c *gin.Context) (ListFilter, bool) {
	workOrderID, ok := intQ(c, "work_order_id")
	if !ok {
		return ListFilter{}, false
	}
	equipmentID, ok := intQ(c, "equipment_id")
	if !ok {
		return ListFilter{}, false
	}
	performedBy, ok := intQ(c, "performed_by")
	if !ok {
		return ListFilter{}, false
	}
	limit, ok := intQ(c, "limit")
	if !ok {
		return ListFilter{}, false
	}
	offset, ok := intQ(c, "offset")
	if !ok {
		return ListFilter{}, false
	}
	return ListFilter{
		WorkOrderID: int64(workOrderID),
		EquipmentID: int64(equipmentID),
		PerformedBy: int64(performedBy),
		Limit:       limit,
		Offset:      offset,
	}, true
}

func intQ(c *gin.Context, key string) (int, bool) {
	raw := c.Query(key)
	if raw == "" {
		return 0, true
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
		apperror.Write(c, http.StatusForbidden, "forbidden", "You do not have permission to read maintenance records")
	case errors.Is(err, ErrInvalidFilter):
		apperror.Write(c, http.StatusBadRequest, "invalid_filter", "Maintenance record filter is invalid")
	default:
		apperror.Write(c, http.StatusInternalServerError, "internal_error", "Unexpected maintenance error")
	}
}
