package repository

import (
	"github.com/nktsenko/org-api/internal/models"
	"gorm.io/gorm"
)

type DepartmentRepository interface {
	Create(dept *models.Department) error
	GetByID(id uint) (*models.Department, error)
	Update(dept *models.Department) error
	Delete(id uint) error
	NameExistsUnderParent(name string, parentID *uint, excludeID uint) (bool, error)
	GetChildren(parentID uint) ([]models.Department, error)
	GetDescendantIDs(id uint) ([]uint, error)
}

type departmentRepo struct {
	db *gorm.DB
}

func NewDepartmentRepository(db *gorm.DB) DepartmentRepository {
	return &departmentRepo{db: db}
}

func (r *departmentRepo) Create(dept *models.Department) error {
	return r.db.Create(dept).Error
}

func (r *departmentRepo) GetByID(id uint) (*models.Department, error) {
	var dept models.Department
	if err := r.db.First(&dept, id).Error; err != nil {
		return nil, err
	}
	return &dept, nil
}

func (r *departmentRepo) Update(dept *models.Department) error {
	return r.db.Save(dept).Error
}

func (r *departmentRepo) Delete(id uint) error {
	return r.db.Delete(&models.Department{}, id).Error
}

func (r *departmentRepo) NameExistsUnderParent(name string, parentID *uint, excludeID uint) (bool, error) {
	query := r.db.Model(&models.Department{}).Where("name = ? AND id != ?", name, excludeID)
	if parentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", *parentID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *departmentRepo) GetChildren(parentID uint) ([]models.Department, error) {
	var children []models.Department
	if err := r.db.Where("parent_id = ?", parentID).Find(&children).Error; err != nil {
		return nil, err
	}
	return children, nil
}

// GetDescendantIDs returns all descendant IDs via BFS (not including id itself).
func (r *departmentRepo) GetDescendantIDs(id uint) ([]uint, error) {
	var result []uint
	queue := []uint{id}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		var childIDs []uint
		if err := r.db.Model(&models.Department{}).
			Where("parent_id = ?", current).
			Pluck("id", &childIDs).Error; err != nil {
			return nil, err
		}
		result = append(result, childIDs...)
		queue = append(queue, childIDs...)
	}
	return result, nil
}
