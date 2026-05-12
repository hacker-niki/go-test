package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/nktsenko/org-api/internal/models"
	"github.com/nktsenko/org-api/internal/service"
)

type DepartmentHandler struct {
	svc service.DepartmentService
}

func NewDepartmentHandler(svc service.DepartmentService) *DepartmentHandler {
	return &DepartmentHandler{svc: svc}
}

// POST /departments/
func (h *DepartmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		ParentID *uint  `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	dept, err := h.svc.Create(service.CreateDepartmentInput{
		Name:     body.Name,
		ParentID: body.ParentID,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dept)
}

// GET /departments/{id}
func (h *DepartmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	depth := 1
	if d := r.URL.Query().Get("depth"); d != "" {
		v, err := strconv.Atoi(d)
		if err != nil || v < 0 {
			writeError(w, http.StatusBadRequest, "depth must be a non-negative integer")
			return
		}
		if v > 5 {
			v = 5
		}
		depth = v
	}

	includeEmployees := true
	if ie := r.URL.Query().Get("include_employees"); ie != "" {
		includeEmployees = ie != "false" && ie != "0"
	}

	sortBy := r.URL.Query().Get("sort_by")

	node, err := h.svc.Get(id, depth, includeEmployees, sortBy)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

// PATCH /departments/{id}
func (h *DepartmentHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	// Raw JSON map to distinguish "absent" vs "null" vs value for parent_id
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	input := service.UpdateDepartmentInput{}

	if v, ok := raw["name"]; ok {
		var name string
		if err := json.Unmarshal(v, &name); err != nil {
			writeError(w, http.StatusBadRequest, "invalid name")
			return
		}
		input.Name = &name
	}

	if v, ok := raw["parent_id"]; ok {
		if string(v) == "null" {
			input.ClearParentID = true
		} else {
			var pid uint
			if err := json.Unmarshal(v, &pid); err != nil {
				writeError(w, http.StatusBadRequest, "invalid parent_id")
				return
			}
			input.ParentID = &pid
		}
	}

	dept, err := h.svc.Update(id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dept)
}

// DELETE /departments/{id}?mode=cascade|reassign&reassign_to_department_id=X
func (h *DepartmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	mode := service.DeleteMode(r.URL.Query().Get("mode"))
	if mode == "" {
		writeError(w, http.StatusBadRequest, "mode query param required: cascade or reassign")
		return
	}

	var reassignTo *uint
	if rtStr := r.URL.Query().Get("reassign_to_department_id"); rtStr != "" {
		v, err := strconv.ParseUint(rtStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid reassign_to_department_id")
			return
		}
		u := uint(v)
		reassignTo = &u
	}

	if err := h.svc.Delete(id, mode, reassignTo); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /departments/{id}/employees/
func (h *DepartmentHandler) CreateEmployee(empSvc service.EmployeeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deptID, ok := parseID(w, r)
		if !ok {
			return
		}

		var body struct {
			FullName string      `json:"full_name"`
			Position string      `json:"position"`
			HiredAt  models.Date `json:"hired_at"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}

		emp, err := empSvc.Create(service.CreateEmployeeInput{
			DepartmentID: deptID,
			FullName:     body.FullName,
			Position:     body.Position,
			HiredAt:      body.HiredAt,
		})
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, emp)
	}
}

func parseID(w http.ResponseWriter, r *http.Request) (uint, bool) {
	raw := r.PathValue("id")
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return uint(v), true
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrBadRequest):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
