package workorder

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/SterneStehen/equipment-maintenance-api/internal/apperror"
	"github.com/SterneStehen/equipment-maintenance-api/internal/auth"
	"github.com/SterneStehen/equipment-maintenance-api/internal/pagination"
	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/gin-gonic/gin"
)

type svc interface {
	Create(ctx context.Context, actor user.Actor, in CreateInput) (WorkOrder, error)
	ByID(ctx context.Context, actor user.Actor, id int64) (WorkOrder, error)
	List(ctx context.Context, actor user.Actor, f ListFilter) ([]WorkOrder, error)
	Update(ctx context.Context, actor user.Actor, id int64, in UpdateInput) (WorkOrder, error)
	Start(ctx context.Context, actor user.Actor, id int64, note string) (WorkOrder, error)
	Complete(ctx context.Context, actor user.Actor, id int64, note string) (WorkOrder, error)
	Close(ctx context.Context, actor user.Actor, id int64, note string) (WorkOrder, error)
	Cancel(ctx context.Context, actor user.Actor, id int64, note string) (WorkOrder, error)
	AddComment(ctx context.Context, actor user.Actor, id int64, body string) (Comment, error)
	ListComments(ctx context.Context, actor user.Actor, id int64, limit, offset int) ([]Comment, error)
	ListHistory(ctx context.Context, actor user.Actor, id int64, limit, offset int) ([]HistoryEntry, error)
}

type Handler struct {
	svc svc
}

type createReq struct {
	EquipmentID int64    `json:"equipment_id" binding:"required"`
	Title       string   `json:"title" binding:"required"`
	Description string   `json:"description"`
	Priority    Priority `json:"priority"`
	AssignedTo  *int64   `json:"assigned_to"`
}

type updateReq struct {
	Title       string   `json:"title" binding:"required"`
	Description string   `json:"description"`
	Status      Status   `json:"status"`
	Priority    Priority `json:"priority"`
	AssignedTo  *int64   `json:"assigned_to"`
}

type transitionReq struct {
	Note string `json:"note"`
}

type commentReq struct {
	Body string `json:"body" binding:"required"`
}

func NewHandler(svc svc) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Create(c *gin.Context) {
	who, ok := actor(c)
	if !ok {
		return
	}
	var req createReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apperror.Write(c, http.StatusBadRequest, "invalid_request", "Equipment id and title are required")
		return
	}
	wo, err := h.svc.Create(c.Request.Context(), who, CreateInput{
		EquipmentID: req.EquipmentID,
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		AssignedTo:  req.AssignedTo,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"work_order": wo})
}

func (h *Handler) Get(c *gin.Context) {
	who, ok := actor(c)
	if !ok {
		return
	}

	id, ok := idFromPath(c)
	if !ok {
		return
	}
	wo, err := h.svc.ByID(c.Request.Context(), who, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"work_order": wo})
}

func (h *Handler) List(c *gin.Context) {
	who, ok := actor(c)
	if !ok {
		return
	}
	flt, ok := filterFromQuery(c)
	if !ok {
		return
	}
	arr, err := h.svc.List(c.Request.Context(), who, flt)
	if err != nil {
		writeErr(c, err)
		return
	}
	pg := pagination.New(flt.Limit, flt.Offset, len(arr))
	c.JSON(http.StatusOK, gin.H{"work_orders": arr, "limit": pg.Limit, "offset": pg.Offset, "pagination": pg})
}

func (h *Handler) Update(c *gin.Context) {
	who, ok := actor(c)
	if !ok {
		return
	}
	id, ok := idFromPath(c)
	if !ok {
		return
	}
	var req updateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apperror.Write(c, http.StatusBadRequest, "invalid_request", "Title is required")
		return
	}
	wo, err := h.svc.Update(c.Request.Context(), who, id, UpdateInput{
		Title:       req.Title,
		Description: req.Description,
		Status:      req.Status,
		Priority:    req.Priority,
		AssignedTo:  req.AssignedTo,
	})

	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"work_order": wo})
}

func (h *Handler) Start(c *gin.Context) {
	h.transition(c, h.svc.Start)
}

func (h *Handler) Complete(c *gin.Context) {
	h.transition(c, h.svc.Complete)
}

func (h *Handler) Close(c *gin.Context) {
	h.transition(c, h.svc.Close)
}

func (h *Handler) Cancel(c *gin.Context) {
	h.transition(c, h.svc.Cancel)
}

func (h *Handler) AddComment(c *gin.Context) {
	who, ok := actor(c)
	if !ok {
		return
	}
	id, ok := idFromPath(c)
	if !ok {
		return
	}
	var req commentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apperror.Write(c, http.StatusBadRequest, "invalid_request", "Comment body is required")
		return
	}
	comment, err := h.svc.AddComment(c.Request.Context(), who, id, req.Body)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"comment": comment})
}

func (h *Handler) ListComments(c *gin.Context) {
	who, ok := actor(c)
	if !ok {
		return
	}
	id, ok := idFromPath(c)
	if !ok {
		return
	}
	limit, ok := intQuery(c, "limit", 0)
	if !ok {
		return
	}
	offset, ok := intQuery(c, "offset", 0)
	if !ok {
		return
	}
	arr, err := h.svc.ListComments(c.Request.Context(), who, id, limit, offset)
	if err != nil {
		writeErr(c, err)
		return
	}
	pg := pagination.New(limit, offset, len(arr))
	c.JSON(http.StatusOK, gin.H{"comments": arr, "limit": pg.Limit, "offset": pg.Offset, "pagination": pg})
}

func (h *Handler) ListHistory(c *gin.Context) {
	who, ok := actor(c)
	if !ok {
		return
	}
	id, ok := idFromPath(c)
	if !ok {
		return
	}
	limit, ok := intQuery(c, "limit", 0)
	if !ok {
		return
	}
	offset, ok := intQuery(c, "offset", 0)
	if !ok {
		return
	}
	arr, err := h.svc.ListHistory(c.Request.Context(), who, id, limit, offset)
	if err != nil {
		writeErr(c, err)
		return
	}
	pg := pagination.New(limit, offset, len(arr))
	c.JSON(http.StatusOK, gin.H{"history": arr, "limit": pg.Limit, "offset": pg.Offset, "pagination": pg})
}

func (h *Handler) transition(c *gin.Context, fn func(context.Context, user.Actor, int64, string) (WorkOrder, error)) {
	who, ok := actor(c)
	if !ok {
		return
	}
	id, ok := idFromPath(c)
	if !ok {
		return
	}
	var req transitionReq
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			apperror.Write(c, http.StatusBadRequest, "invalid_request", "Transition note is invalid")
			return
		}
	}
	wo, err := fn(c.Request.Context(), who, id, req.Note)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"work_order": wo})
}

func actor(c *gin.Context) (user.Actor, bool) {
	p, ok := auth.Current(c)
	if !ok {
		apperror.Write(c, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return user.Actor{}, false
	}
	return user.Actor{UserID: p.UserID, Role: p.Role}, true
}

func idFromPath(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		apperror.Write(c, http.StatusNotFound, "not_found", "Work order was not found")
		return 0, false
	}
	return id, true
}

func filterFromQuery(c *gin.Context) (ListFilter, bool) {
	limit, ok := intQuery(c, "limit", 0)
	if !ok {
		return ListFilter{}, false
	}
	offset, ok := intQuery(c, "offset", 0)
	if !ok {
		return ListFilter{}, false
	}
	eq, ok := intQuery(c, "equipment_id", 0)
	if !ok {
		return ListFilter{}, false
	}
	assigned, ok := intQuery(c, "assigned_to", 0)
	if !ok {
		return ListFilter{}, false
	}
	return ListFilter{
		Status:      Status(c.Query("status")),
		Priority:    Priority(c.Query("priority")),
		EquipmentID: int64(eq),
		AssignedTo:  int64(assigned),
		Query:       c.Query("q"),
		Limit:       limit,
		Offset:      offset,
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
		apperror.Write(c, http.StatusForbidden, "forbidden", "You do not have permission to manage work orders")
	case errors.Is(err, ErrInvalidTitle):
		apperror.Write(c, http.StatusBadRequest, "invalid_title", "Work order title is required")
	case errors.Is(err, ErrInvalidStatus):
		apperror.Write(c, http.StatusBadRequest, "invalid_status", "Work order status is invalid")
	case errors.Is(err, ErrInvalidPriority):
		apperror.Write(c, http.StatusBadRequest, "invalid_priority", "Work order priority is invalid")
	case errors.Is(err, ErrInvalidEquipment):
		apperror.Write(c, http.StatusBadRequest, "invalid_equipment", "Equipment id is invalid")
	case errors.Is(err, ErrEquipmentNotFound):
		apperror.Write(c, http.StatusNotFound, "equipment_not_found", "Equipment was not found")
	case errors.Is(err, ErrEquipmentDecommissioned):
		apperror.Write(c, http.StatusConflict, "equipment_decommissioned", "Equipment is decommissioned")
	case errors.Is(err, ErrAssigneeNotFound):
		apperror.Write(c, http.StatusBadRequest, "invalid_assignee", "Assigned technician was not found")
	case errors.Is(err, ErrAssigneeNotTechnician):
		apperror.Write(c, http.StatusBadRequest, "invalid_assignee", "Assignee must be a technician")
	case errors.Is(err, ErrInvalidTransition):
		apperror.Write(c, http.StatusConflict, "invalid_transition", "Work order transition is not allowed")
	case errors.Is(err, ErrTerminalState):
		apperror.Write(c, http.StatusConflict, "terminal_state", "Work order is already closed or canceled")
	case errors.Is(err, ErrTechnicianOwnership):
		apperror.Write(c, http.StatusForbidden, "not_assigned", "Technician is not assigned to this work order")
	case errors.Is(err, ErrInvalidComment):
		apperror.Write(c, http.StatusBadRequest, "invalid_comment", "Comment body is required")
	case errors.Is(err, ErrNotFound):
		apperror.Write(c, http.StatusNotFound, "not_found", "Work order was not found")
	default:
		apperror.Write(c, http.StatusInternalServerError, "internal_error", "Unexpected work order error")
	}
}
