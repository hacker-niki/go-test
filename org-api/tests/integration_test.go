package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/nktsenko/org-api/internal/db"
	"github.com/nktsenko/org-api/internal/handler"
	"github.com/nktsenko/org-api/internal/repository"
	"github.com/nktsenko/org-api/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=UTC",
		getEnvOr("DB_HOST", "localhost"),
		getEnvOr("DB_PORT", "5432"),
		getEnvOr("DB_USER", "postgres"),
		getEnvOr("DB_PASSWORD", "postgres"),
		getEnvOr("DB_NAME", "orgdb"),
	)

	gormDB, err := db.Connect(dsn)
	if err != nil {
		t.Skipf("database unavailable, skipping integration tests: %v", err)
	}

	sqlDB, err := gormDB.DB()
	require.NoError(t, err)

	require.NoError(t, db.RunMigrations(sqlDB))

	// clean state before each test
	sqlDB.Exec("TRUNCATE departments RESTART IDENTITY CASCADE")

	deptRepo := repository.NewDepartmentRepository(gormDB)
	empRepo := repository.NewEmployeeRepository(gormDB)

	deptSvc := service.NewDepartmentService(deptRepo, empRepo)
	empSvc := service.NewEmployeeService(deptRepo, empRepo)

	deptHandler := handler.NewDepartmentHandler(deptSvc)
	empHandler := deptHandler.CreateEmployee(empSvc)

	return handler.NewMux(deptHandler, empHandler)
}

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func postJSON(t *testing.T, srv http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func TestCreateDepartment(t *testing.T) {
	srv := newTestServer(t)

	w := postJSON(t, srv, "/departments/", map[string]any{
		"name": "  Engineering  ",
	})
	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Engineering", resp["name"])
	assert.Nil(t, resp["parent_id"])
}

func TestCreateDepartmentDuplicateNameUnderSameParent(t *testing.T) {
	srv := newTestServer(t)

	postJSON(t, srv, "/departments/", map[string]any{"name": "Backend"})
	w := postJSON(t, srv, "/departments/", map[string]any{"name": "Backend"})
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestCreateEmployee(t *testing.T) {
	srv := newTestServer(t)

	// create department
	w := postJSON(t, srv, "/departments/", map[string]any{"name": "HR"})
	require.Equal(t, http.StatusCreated, w.Code)

	var dept map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &dept))
	deptID := uint(dept["id"].(float64))

	// create employee
	w = postJSON(t, srv, fmt.Sprintf("/departments/%d/employees/", deptID), map[string]any{
		"full_name": "Alice Smith",
		"position":  "HR Manager",
		"hired_at":  "2023-03-15",
	})
	assert.Equal(t, http.StatusCreated, w.Code)

	var emp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &emp))
	assert.Equal(t, "Alice Smith", emp["full_name"])
	assert.Equal(t, "HR Manager", emp["position"])
}

func TestCreateEmployeeInNonExistentDepartment(t *testing.T) {
	srv := newTestServer(t)

	w := postJSON(t, srv, "/departments/9999/employees/", map[string]any{
		"full_name": "Bob",
		"position":  "Dev",
	})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetDepartmentTree(t *testing.T) {
	srv := newTestServer(t)

	// root
	w := postJSON(t, srv, "/departments/", map[string]any{"name": "Root"})
	require.Equal(t, http.StatusCreated, w.Code)
	var root map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &root))
	rootID := uint(root["id"].(float64))

	// child
	w = postJSON(t, srv, "/departments/", map[string]any{
		"name":      "Child",
		"parent_id": rootID,
	})
	require.Equal(t, http.StatusCreated, w.Code)

	// get with depth=1
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/departments/%d?depth=1&include_employees=false", rootID), nil)
	rw := httptest.NewRecorder()
	srv.ServeHTTP(rw, req)
	assert.Equal(t, http.StatusOK, rw.Code)

	var node map[string]any
	require.NoError(t, json.Unmarshal(rw.Body.Bytes(), &node))
	children := node["children"].([]any)
	assert.Len(t, children, 1)
	assert.Equal(t, "Child", children[0].(map[string]any)["name"])
}

func TestMoveDepartmentCycleDetection(t *testing.T) {
	srv := newTestServer(t)

	wA := postJSON(t, srv, "/departments/", map[string]any{"name": "A"})
	require.Equal(t, http.StatusCreated, wA.Code)
	var deptA map[string]any
	require.NoError(t, json.Unmarshal(wA.Body.Bytes(), &deptA))
	idA := uint(deptA["id"].(float64))

	wB := postJSON(t, srv, "/departments/", map[string]any{"name": "B", "parent_id": idA})
	require.Equal(t, http.StatusCreated, wB.Code)
	var deptB map[string]any
	require.NoError(t, json.Unmarshal(wB.Body.Bytes(), &deptB))
	idB := uint(deptB["id"].(float64))

	// try to move A under B (B is a child of A → cycle)
	body, _ := json.Marshal(map[string]any{"parent_id": idB})
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/departments/%d", idA), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	srv.ServeHTTP(rw, req)
	assert.Equal(t, http.StatusConflict, rw.Code)
}

func TestSelfParentRejected(t *testing.T) {
	srv := newTestServer(t)

	w := postJSON(t, srv, "/departments/", map[string]any{"name": "Self"})
	require.Equal(t, http.StatusCreated, w.Code)
	var dept map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &dept))
	id := uint(dept["id"].(float64))

	body, _ := json.Marshal(map[string]any{"parent_id": id})
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/departments/%d", id), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	srv.ServeHTTP(rw, req)
	assert.Equal(t, http.StatusConflict, rw.Code)
}

func TestDeleteCascade(t *testing.T) {
	srv := newTestServer(t)

	w := postJSON(t, srv, "/departments/", map[string]any{"name": "ToDelete"})
	require.Equal(t, http.StatusCreated, w.Code)
	var dept map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &dept))
	id := uint(dept["id"].(float64))

	// add employee
	postJSON(t, srv, fmt.Sprintf("/departments/%d/employees/", id), map[string]any{
		"full_name": "Charlie",
		"position":  "Dev",
	})

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/departments/%d?mode=cascade", id), nil)
	rw := httptest.NewRecorder()
	srv.ServeHTTP(rw, req)
	assert.Equal(t, http.StatusNoContent, rw.Code)

	// verify gone
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/departments/%d", id), nil)
	rw = httptest.NewRecorder()
	srv.ServeHTTP(rw, req)
	assert.Equal(t, http.StatusNotFound, rw.Code)
}

func TestDeleteReassign(t *testing.T) {
	srv := newTestServer(t)

	wSrc := postJSON(t, srv, "/departments/", map[string]any{"name": "Source"})
	require.Equal(t, http.StatusCreated, wSrc.Code)
	var src map[string]any
	require.NoError(t, json.Unmarshal(wSrc.Body.Bytes(), &src))
	srcID := uint(src["id"].(float64))

	wDst := postJSON(t, srv, "/departments/", map[string]any{"name": "Target"})
	require.Equal(t, http.StatusCreated, wDst.Code)
	var dst map[string]any
	require.NoError(t, json.Unmarshal(wDst.Body.Bytes(), &dst))
	dstID := uint(dst["id"].(float64))

	postJSON(t, srv, fmt.Sprintf("/departments/%d/employees/", srcID), map[string]any{
		"full_name": "Dave",
		"position":  "QA",
	})

	req := httptest.NewRequest(http.MethodDelete,
		fmt.Sprintf("/departments/%d?mode=reassign&reassign_to_department_id=%d", srcID, dstID), nil)
	rw := httptest.NewRecorder()
	srv.ServeHTTP(rw, req)
	assert.Equal(t, http.StatusNoContent, rw.Code)

	// employee should now be in target department
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/departments/%d?include_employees=true", dstID), nil)
	rw = httptest.NewRecorder()
	srv.ServeHTTP(rw, req)
	assert.Equal(t, http.StatusOK, rw.Code)

	var node map[string]any
	require.NoError(t, json.Unmarshal(rw.Body.Bytes(), &node))
	employees := node["employees"].([]any)
	assert.Len(t, employees, 1)
	assert.Equal(t, "Dave", employees[0].(map[string]any)["full_name"])
}
