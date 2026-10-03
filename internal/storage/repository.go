package storage

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"homedashboard/internal/domain"
)

type Repository struct{ DB *gorm.DB }

func recordError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return err
}

func (r *Repository) NeedsSetup(ctx context.Context) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).Model(&UserModel{}).Count(&count).Error
	return count == 0, err
}

func (r *Repository) FindUser(ctx context.Context, field, value string) (UserModel, error) {
	var user UserModel
	if field != "id" && field != "username" {
		return user, domain.ErrNotFound
	}
	err := r.DB.WithContext(ctx).Where(field+" = ?", value).First(&user).Error
	return user, recordError(err)
}

func (r *Repository) Setup(ctx context.Context, username, hash string) (domain.User, error) {
	user := UserModel{ID: uuid.NewString(), Singleton: 1, Username: username, PasswordHash: hash}
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&UserModel{}).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return domain.ErrAlreadySetup
		}
		if err := tx.Create(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return domain.ErrAlreadySetup
			}
			return err
		}
		dashboard := DashboardModel{ID: uuid.NewString(), Name: "내 홈서버", Revision: 1}
		if err := tx.Create(&dashboard).Error; err != nil {
			return err
		}
		return tx.Create(&GroupModel{ID: uuid.NewString(), DashboardID: dashboard.ID, Name: "링크", Position: 0, Revision: 1}).Error
	})
	return domain.User{ID: user.ID, Username: user.Username}, err
}

func (r *Repository) Dashboards(ctx context.Context) ([]domain.Dashboard, error) {
	var rows []DashboardModel
	if err := r.DB.WithContext(ctx).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Dashboard, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.Dashboard{ID: row.ID, Name: row.Name, Revision: row.Revision, Groups: []domain.Group{}})
	}
	return result, nil
}

func (r *Repository) Dashboard(ctx context.Context, id string) (domain.Dashboard, error) {
	var result domain.Dashboard
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row DashboardModel
		if err := tx.First(&row, "id = ?", id).Error; err != nil {
			return recordError(err)
		}
		result = domain.Dashboard{ID: row.ID, Name: row.Name, Revision: row.Revision, Groups: []domain.Group{}}
		var groups []GroupModel
		if err := tx.Order("position, id").Find(&groups, "dashboard_id = ?", id).Error; err != nil {
			return err
		}
		for _, group := range groups {
			var services []ServiceModel
			if err := tx.Order("position, id").Find(&services, "group_id = ?", group.ID).Error; err != nil {
				return err
			}
			item := toGroup(group)
			for _, service := range services {
				item.Services = append(item.Services, toService(service))
			}
			result.Groups = append(result.Groups, item)
		}
		return nil
	})
	return result, err
}

func (r *Repository) Service(ctx context.Context, id string) (domain.Service, error) {
	var row ServiceModel
	err := r.DB.WithContext(ctx).First(&row, "id = ?", id).Error
	return toService(row), recordError(err)
}

func nextPosition(tx *gorm.DB, model any, condition string, id string) (int, error) {
	var position int
	err := tx.Model(model).Where(condition, id).Select("COALESCE(MAX(position), -1) + 1").Scan(&position).Error
	return position, err
}

func bumpDashboard(tx *gorm.DB, id string) error {
	return tx.Model(&DashboardModel{}).Where("id = ?", id).UpdateColumn("revision", gorm.Expr("revision + 1")).Error
}

func (r *Repository) CreateService(ctx context.Context, input domain.ServiceInput) (domain.Service, error) {
	row := serviceModel(input)
	row.ID, row.Revision = uuid.NewString(), 1
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var group GroupModel
		if err := tx.First(&group, "id = ?", input.GroupID).Error; err != nil {
			return recordError(err)
		}
		position, err := nextPosition(tx, &ServiceModel{}, "group_id = ?", group.ID)
		if err != nil {
			return err
		}
		row.Position = position
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return bumpDashboard(tx, group.DashboardID)
	})
	return toService(row), err
}

func (r *Repository) UpdateService(ctx context.Context, id string, expected int64, input domain.ServiceInput) (domain.Service, error) {
	var row ServiceModel
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&row, "id = ?", id).Error; err != nil {
			return recordError(err)
		}
		if row.Revision != expected {
			return domain.ErrConflict
		}
		var oldGroup, newGroup GroupModel
		if err := tx.First(&oldGroup, "id = ?", row.GroupID).Error; err != nil {
			return recordError(err)
		}
		if err := tx.First(&newGroup, "id = ?", input.GroupID).Error; err != nil {
			return recordError(err)
		}
		if oldGroup.DashboardID != newGroup.DashboardID {
			return &domain.ValidationError{Message: "같은 대시보드의 그룹을 선택해주세요."}
		}
		position := row.Position
		if oldGroup.ID != newGroup.ID {
			var err error
			position, err = nextPosition(tx, &ServiceModel{}, "group_id = ?", newGroup.ID)
			if err != nil {
				return err
			}
			if err := bumpDashboard(tx, oldGroup.DashboardID); err != nil {
				return err
			}
		}
		result := tx.Model(&row).Where("revision = ?", expected).Updates(map[string]any{
			"group_id": input.GroupID, "name": input.Name, "url": input.URL, "icon_key": input.IconKey,
			"icon_mode": input.IconMode, "icon_url": input.IconURL, "icon_asset_id": input.IconAssetID, "web_check_enabled": input.WebCheckEnabled == nil || *input.WebCheckEnabled,
			"description": input.Description, "hidden": input.Hidden, "position": position, "revision": gorm.Expr("revision + 1"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		return tx.First(&row, "id = ?", id).Error
	})
	return toService(row), err
}

func (r *Repository) DeleteService(ctx context.Context, id string, expected int64) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row ServiceModel
		if err := tx.First(&row, "id = ?", id).Error; err != nil {
			return recordError(err)
		}
		if row.Revision != expected {
			return domain.ErrConflict
		}
		var group GroupModel
		if err := tx.First(&group, "id = ?", row.GroupID).Error; err != nil {
			return recordError(err)
		}
		result := tx.Where("id = ? AND revision = ?", id, expected).Delete(&ServiceModel{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		return bumpDashboard(tx, group.DashboardID)
	})
}

func (r *Repository) CreateGroup(ctx context.Context, dashboardID, name string) (domain.Group, error) {
	row := GroupModel{ID: uuid.NewString(), DashboardID: dashboardID, Name: name, Revision: 1}
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var dashboard DashboardModel
		if err := tx.First(&dashboard, "id = ?", dashboardID).Error; err != nil {
			return recordError(err)
		}
		position, err := nextPosition(tx, &GroupModel{}, "dashboard_id = ?", dashboardID)
		if err != nil {
			return err
		}
		row.Position = position
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return bumpDashboard(tx, dashboardID)
	})
	return toGroup(row), err
}

func (r *Repository) UpdateGroup(ctx context.Context, id, name string, expected int64) (domain.Group, error) {
	var row GroupModel
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&row, "id = ?", id).Error; err != nil {
			return recordError(err)
		}
		if row.Revision != expected {
			return domain.ErrConflict
		}
		result := tx.Model(&row).Where("revision = ?", expected).Updates(map[string]any{"name": name, "revision": gorm.Expr("revision + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		return tx.First(&row, "id = ?", id).Error
	})
	return toGroup(row), err
}

func (r *Repository) DeleteGroup(ctx context.Context, id string, expected int64) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row GroupModel
		if err := tx.First(&row, "id = ?", id).Error; err != nil {
			return recordError(err)
		}
		if row.Revision != expected {
			return domain.ErrConflict
		}
		var count int64
		if err := tx.Model(&ServiceModel{}).Where("group_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return domain.ErrGroupNotEmpty
		}
		if err := tx.Where("id = ? AND revision = ?", id, expected).Delete(&GroupModel{}).Error; err != nil {
			return err
		}
		return bumpDashboard(tx, row.DashboardID)
	})
}

func (r *Repository) SaveLayout(ctx context.Context, id string, expected int64, layout domain.Layout) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var dashboard DashboardModel
		if err := tx.First(&dashboard, "id = ?", id).Error; err != nil {
			return recordError(err)
		}
		if dashboard.Revision != expected {
			return domain.ErrConflict
		}
		var groups []GroupModel
		if err := tx.Find(&groups, "dashboard_id = ?", id).Error; err != nil {
			return err
		}
		groupSet := map[string]GroupModel{}
		for _, group := range groups {
			groupSet[group.ID] = group
		}
		var rows []ServiceModel
		if len(groups) > 0 {
			ids := make([]string, 0, len(groups))
			for _, group := range groups {
				ids = append(ids, group.ID)
			}
			if err := tx.Where("group_id IN ?", ids).Find(&rows).Error; err != nil {
				return err
			}
		}
		serviceSet := map[string]ServiceModel{}
		for _, row := range rows {
			serviceSet[row.ID] = row
		}
		invalid := func() error {
			return &domain.ValidationError{Message: "모든 그룹과 숨긴 카드를 포함해 각 항목을 한 번씩 보내주세요."}
		}
		for i, item := range layout.Groups {
			group, ok := groupSet[item.GroupID]
			if !ok {
				return invalid()
			}
			delete(groupSet, item.GroupID)
			if group.Position != i {
				if err := tx.Model(&GroupModel{}).Where("id = ?", group.ID).Updates(map[string]any{"position": i, "revision": gorm.Expr("revision + 1")}).Error; err != nil {
					return err
				}
			}
			for j, serviceID := range item.ServiceIDs {
				row, ok := serviceSet[serviceID]
				if !ok {
					return invalid()
				}
				delete(serviceSet, serviceID)
				if row.GroupID != item.GroupID || row.Position != j {
					if err := tx.Model(&ServiceModel{}).Where("id = ?", serviceID).Updates(map[string]any{"group_id": item.GroupID, "position": j, "revision": gorm.Expr("revision + 1")}).Error; err != nil {
						return err
					}
				}
			}
		}
		if len(groupSet) > 0 || len(serviceSet) > 0 {
			return invalid()
		}
		return bumpDashboard(tx, id)
	})
}

func toService(row ServiceModel) domain.Service {
	return domain.Service{ID: row.ID, GroupID: row.GroupID, Name: row.Name, URL: row.URL, IconKey: row.IconKey, IconMode: row.IconMode, IconURL: row.IconURL, IconAssetID: row.IconAssetID, WebCheckEnabled: row.WebCheckEnabled, Description: row.Description, Hidden: row.Hidden, Position: row.Position, Revision: row.Revision, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func (r *Repository) MonitorLinks(ctx context.Context) ([]domain.Service, error) {
	var rows []ServiceModel
	if err := r.DB.WithContext(ctx).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	links := make([]domain.Service, 0, len(rows))
	for _, row := range rows {
		links = append(links, toService(row))
	}
	return links, nil
}

func serviceModel(input domain.ServiceInput) ServiceModel {
	return ServiceModel{GroupID: input.GroupID, Name: input.Name, URL: input.URL, IconKey: input.IconKey, IconMode: input.IconMode, IconURL: input.IconURL, IconAssetID: input.IconAssetID, WebCheckEnabled: input.WebCheckEnabled == nil || *input.WebCheckEnabled, Description: input.Description, Hidden: input.Hidden}
}
func toGroup(row GroupModel) domain.Group {
	return domain.Group{ID: row.ID, DashboardID: row.DashboardID, Name: row.Name, Position: row.Position, Revision: row.Revision, Services: []domain.Service{}}
}
