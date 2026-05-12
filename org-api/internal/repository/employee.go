package repository

import (
	"github.com/nktsenko/org-api/internal/models"
	"gorm.io/gorm"
)

type EmployeeRepository interface {
	Create(emp *models.Employee) error
	GetByDepartmentID(deptID uint, sortBy string) ([]models.Employee, error)
	ReassignFromDepartments(fromIDs []uint, toDeptID uint) error
}

type employeeRepo struct {
	db *gorm.DB
}

func NewEmployeeRepository(db *gorm.DB) EmployeeRepository {
	return &employeeRepo{db: db}
}

func (r *employeeRepo) Create(emp *models.Employee) error {
	return r.db.Create(emp).Error
}

func (r *employeeRepo) GetByDepartmentID(deptID uint, sortBy string) ([]models.Employee, error) {
	var employees []models.Employee
	order := "created_at ASC"
	if sortBy == "full_name" {
		order = "full_name ASC"
	}
	if err := r.db.Where("department_id = ?", deptID).Order(order).Find(&employees).Error; err != nil {
		return nil, err
	}
	return employees, nil
}

func (r *employeeRepo) ReassignFromDepartments(fromIDs []uint, toDeptID uint) error {
	if len(fromIDs) == 0 {
		return nil
	}
	return r.db.Model(&models.Employee{}).
		Where("department_id IN ?", fromIDs).
		Update("department_id", toDeptID).Error
}
