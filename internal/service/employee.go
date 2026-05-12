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

type CreateEmployeeInput struct {
	DepartmentID uint
	FullName     string
	Position     string
	HiredAt      models.Date
}

type EmployeeService interface {
	Create(input CreateEmployeeInput) (*models.Employee, error)
}

type employeeService struct {
	deptRepo repository.DepartmentRepository
	empRepo  repository.EmployeeRepository
}

func NewEmployeeService(deptRepo repository.DepartmentRepository, empRepo repository.EmployeeRepository) EmployeeService {
	return &employeeService{deptRepo: deptRepo, empRepo: empRepo}
}

func (s *employeeService) Create(input CreateEmployeeInput) (*models.Employee, error) {
	if _, err := s.deptRepo.GetByID(input.DepartmentID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: department %d", ErrNotFound, input.DepartmentID)
		}
		return nil, err
	}

	fullName := strings.TrimSpace(input.FullName)
	if fullName == "" || len(fullName) > 200 {
		return nil, fmt.Errorf("%w: full_name must be 1..200 characters", ErrBadRequest)
	}

	position := strings.TrimSpace(input.Position)
	if position == "" || len(position) > 200 {
		return nil, fmt.Errorf("%w: position must be 1..200 characters", ErrBadRequest)
	}

	emp := &models.Employee{
		DepartmentID: input.DepartmentID,
		FullName:     fullName,
		Position:     position,
		HiredAt:      input.HiredAt,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.empRepo.Create(emp); err != nil {
		return nil, err
	}
	return emp, nil
}
