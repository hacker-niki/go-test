package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nktsenko/org-api/internal/models"
	"github.com/nktsenko/org-api/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrConflict   = errors.New("conflict")
	ErrBadRequest = errors.New("bad request")
)

type DepartmentNode struct {
	models.Department
	Employees []models.Employee `json:"employees,omitempty"`
	Children  []DepartmentNode  `json:"children,omitempty"`
}

type CreateDepartmentInput struct {
	Name     string
	ParentID *uint
}

type UpdateDepartmentInput struct {
	Name          *string
	ParentID      *uint
	ClearParentID bool // true when parent_id was explicitly set to null
}

type DeleteMode string

const (
	DeleteCascade  DeleteMode = "cascade"
	DeleteReassign DeleteMode = "reassign"
)

type DepartmentService interface {
	Create(input CreateDepartmentInput) (*models.Department, error)
	Get(id uint, depth int, includeEmployees bool, sortBy string) (*DepartmentNode, error)
	Update(id uint, input UpdateDepartmentInput) (*models.Department, error)
	Delete(id uint, mode DeleteMode, reassignTo *uint) error
}

type departmentService struct {
	deptRepo repository.DepartmentRepository
	empRepo  repository.EmployeeRepository
}

func NewDepartmentService(deptRepo repository.DepartmentRepository, empRepo repository.EmployeeRepository) DepartmentService {
	return &departmentService{deptRepo: deptRepo, empRepo: empRepo}
}

func (s *departmentService) Create(input CreateDepartmentInput) (*models.Department, error) {
	name, err := validateName(input.Name)
	if err != nil {
		return nil, err
	}

	if input.ParentID != nil {
		if _, err := s.deptRepo.GetByID(*input.ParentID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("%w: parent department %d", ErrNotFound, *input.ParentID)
			}
			return nil, err
		}
	}

	exists, err := s.deptRepo.NameExistsUnderParent(name, input.ParentID, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("%w: department named %q already exists under this parent", ErrConflict, name)
	}

	dept := &models.Department{
		Name:      name,
		ParentID:  input.ParentID,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.deptRepo.Create(dept); err != nil {
		return nil, err
	}
	return dept, nil
}

func (s *departmentService) Get(id uint, depth int, includeEmployees bool, sortBy string) (*DepartmentNode, error) {
	dept, err := s.deptRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: department %d", ErrNotFound, id)
		}
		return nil, err
	}

	node := &DepartmentNode{Department: *dept}

	if includeEmployees {
		emps, err := s.empRepo.GetByDepartmentID(id, sortBy)
		if err != nil {
			return nil, err
		}
		node.Employees = emps
	}

	if depth > 0 {
		children, err := s.deptRepo.GetChildren(id)
		if err != nil {
			return nil, err
		}
		for _, child := range children {
			childNode, err := s.Get(child.ID, depth-1, includeEmployees, sortBy)
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, *childNode)
		}
	}

	return node, nil
}

func (s *departmentService) Update(id uint, input UpdateDepartmentInput) (*models.Department, error) {
	dept, err := s.deptRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: department %d", ErrNotFound, id)
		}
		return nil, err
	}

	if input.Name != nil {
		name, err := validateName(*input.Name)
		if err != nil {
			return nil, err
		}
		dept.Name = name
	}

	newParentID := dept.ParentID
	if input.ClearParentID {
		newParentID = nil
	} else if input.ParentID != nil {
		newParentID = input.ParentID
	}

	parentChanged := !ptrUintEqual(newParentID, dept.ParentID)
	if parentChanged {
		if err := s.validateParentChange(id, newParentID); err != nil {
			return nil, err
		}
		dept.ParentID = newParentID
	}

	// re-check name uniqueness with possibly new parent
	exists, err := s.deptRepo.NameExistsUnderParent(dept.Name, dept.ParentID, id)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("%w: department named %q already exists under this parent", ErrConflict, dept.Name)
	}

	if err := s.deptRepo.Update(dept); err != nil {
		return nil, err
	}
	return dept, nil
}

func (s *departmentService) validateParentChange(id uint, newParentID *uint) error {
	if newParentID == nil {
		return nil
	}
	if *newParentID == id {
		return fmt.Errorf("%w: department cannot be its own parent", ErrConflict)
	}
	// verify new parent exists
	if _, err := s.deptRepo.GetByID(*newParentID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: parent department %d", ErrNotFound, *newParentID)
		}
		return err
	}
	// check new parent is not a descendant of id (would create cycle)
	descendants, err := s.deptRepo.GetDescendantIDs(id)
	if err != nil {
		return err
	}
	for _, descID := range descendants {
		if descID == *newParentID {
			return fmt.Errorf("%w: moving department %d under its own descendant %d would create a cycle", ErrConflict, id, *newParentID)
		}
	}
	return nil
}

func (s *departmentService) Delete(id uint, mode DeleteMode, reassignTo *uint) error {
	if _, err := s.deptRepo.GetByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: department %d", ErrNotFound, id)
		}
		return err
	}

	switch mode {
	case DeleteCascade:
		// DB CASCADE handles employees and sub-departments
		return s.deptRepo.Delete(id)

	case DeleteReassign:
		if reassignTo == nil {
			return fmt.Errorf("%w: reassign_to_department_id required for reassign mode", ErrBadRequest)
		}
		if *reassignTo == id {
			return fmt.Errorf("%w: cannot reassign employees to the department being deleted", ErrBadRequest)
		}
		if _, err := s.deptRepo.GetByID(*reassignTo); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: target department %d", ErrNotFound, *reassignTo)
			}
			return err
		}
		// collect all IDs in subtree (id + all descendants)
		descendants, err := s.deptRepo.GetDescendantIDs(id)
		if err != nil {
			return err
		}
		allIDs := append([]uint{id}, descendants...)

		if err := s.empRepo.ReassignFromDepartments(allIDs, *reassignTo); err != nil {
			return err
		}
		// DB CASCADE deletes descendant departments + any remaining employees
		return s.deptRepo.Delete(id)

	default:
		return fmt.Errorf("%w: unknown delete mode %q, use cascade or reassign", ErrBadRequest, mode)
	}
}

func ptrUintEqual(a, b *uint) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func validateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%w: name must not be empty", ErrBadRequest)
	}
	if len(name) > 200 {
		return "", fmt.Errorf("%w: name must not exceed 200 characters", ErrBadRequest)
	}
	return name, nil
}
