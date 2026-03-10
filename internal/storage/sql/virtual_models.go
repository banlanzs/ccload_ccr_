package sql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"ccLoad+ccr/internal/model"
)

// ==================== Virtual Model CRUD ====================

// ListVirtualModels 获取所有虚拟模型列表（含关联规则数量）
func (s *SQLStore) ListVirtualModels(ctx context.Context) ([]*model.VirtualModel, error) {
	query := `
		SELECT vm.id, vm.name, vm.alias, vm.description, vm.enabled, vm.default_fallback, vm.created_at, vm.updated_at,
		       COALESCE(ma.assoc_count, 0) AS associations_count
		FROM virtual_models vm
		LEFT JOIN (
		    SELECT virtual_model_id, COUNT(*) AS assoc_count
		    FROM model_associations
		    GROUP BY virtual_model_id
		) ma ON vm.id = ma.virtual_model_id
		ORDER BY vm.id ASC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var models []*model.VirtualModel
	for rows.Next() {
		vm := &model.VirtualModel{}
		var createdAt, updatedAt int64
		if err := rows.Scan(&vm.ID, &vm.Name, &vm.Alias, &vm.Description, &vm.Enabled,
			&vm.DefaultFallback, &createdAt, &updatedAt, &vm.AssociationsCount); err != nil {
			return nil, err
		}
		vm.CreatedAt = model.JSONTime{Time: unixToTime(createdAt)}
		vm.UpdatedAt = model.JSONTime{Time: unixToTime(updatedAt)}
		models = append(models, vm)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return models, nil
}

// GetVirtualModel 根据ID获取虚拟模型
func (s *SQLStore) GetVirtualModel(ctx context.Context, id int64) (*model.VirtualModel, error) {
	query := `
		SELECT id, name, alias, description, enabled, default_fallback, created_at, updated_at
		FROM virtual_models
		WHERE id = ?
	`
	vm := &model.VirtualModel{}
	var createdAt, updatedAt int64
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&vm.ID, &vm.Name, &vm.Alias, &vm.Description, &vm.Enabled,
		&vm.DefaultFallback, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("virtual model not found")
		}
		return nil, err
	}
	vm.CreatedAt = model.JSONTime{Time: unixToTime(createdAt)}
	vm.UpdatedAt = model.JSONTime{Time: unixToTime(updatedAt)}
	return vm, nil
}

// GetVirtualModelByName 根据名称获取虚拟模型
func (s *SQLStore) GetVirtualModelByName(ctx context.Context, name string) (*model.VirtualModel, error) {
	query := `
		SELECT id, name, alias, description, enabled, default_fallback, created_at, updated_at
		FROM virtual_models
		WHERE name = ?
	`
	vm := &model.VirtualModel{}
	var createdAt, updatedAt int64
	err := s.db.QueryRowContext(ctx, query, name).Scan(
		&vm.ID, &vm.Name, &vm.Alias, &vm.Description, &vm.Enabled,
		&vm.DefaultFallback, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("virtual model not found")
		}
		return nil, err
	}
	vm.CreatedAt = model.JSONTime{Time: unixToTime(createdAt)}
	vm.UpdatedAt = model.JSONTime{Time: unixToTime(updatedAt)}
	return vm, nil
}

// CreateVirtualModel 创建虚拟模型
func (s *SQLStore) CreateVirtualModel(ctx context.Context, vm *model.VirtualModel) (*model.VirtualModel, error) {
	nowUnix := timeToUnix(time.Now())

	query := `
		INSERT INTO virtual_models(name, alias, description, enabled, default_fallback, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?)
	`
	result, err := s.db.ExecContext(ctx, query,
		vm.Name, vm.Alias, vm.Description, boolToInt(vm.Enabled), vm.DefaultFallback,
		nowUnix, nowUnix,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "Duplicate") {
			return nil, fmt.Errorf("virtual model name already exists: %s", vm.Name)
		}
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get last insert id: %w", err)
	}

	return s.GetVirtualModel(ctx, id)
}

// UpdateVirtualModel 更新虚拟模型
func (s *SQLStore) UpdateVirtualModel(ctx context.Context, id int64, vm *model.VirtualModel) error {
	// 确认目标存在
	if _, err := s.GetVirtualModel(ctx, id); err != nil {
		return err
	}

	updatedAtUnix := timeToUnix(time.Now())

	query := `
		UPDATE virtual_models
		SET name=?, alias=?, description=?, enabled=?, default_fallback=?, updated_at=?
		WHERE id=?
	`
	_, err := s.db.ExecContext(ctx, query,
		vm.Name, vm.Alias, vm.Description, boolToInt(vm.Enabled), vm.DefaultFallback,
		updatedAtUnix, id,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "Duplicate") {
			return fmt.Errorf("virtual model name already exists: %s", vm.Name)
		}
		return err
	}
	return nil
}

// DeleteVirtualModel 删除虚拟模型
func (s *SQLStore) DeleteVirtualModel(ctx context.Context, id int64) error {
	// 检查记录是否存在（幂等性）
	if _, err := s.GetVirtualModel(ctx, id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil // 记录不存在，直接返回
		}
		return err
	}

	query := `DELETE FROM virtual_models WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

// ==================== Model Association CRUD ====================

// ListModelAssociations 获取指定虚拟模型的关联规则列表
func (s *SQLStore) ListModelAssociations(ctx context.Context, virtualModelID int64) ([]*model.ModelAssociation, error) {
	query := `
		SELECT id, virtual_model_id, channel_id, channel_tags, match_type, pattern, priority, enabled, created_at, updated_at
		FROM model_associations
		WHERE virtual_model_id = ?
		ORDER BY priority DESC, id ASC
	`
	rows, err := s.db.QueryContext(ctx, query, virtualModelID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var associations []*model.ModelAssociation
	for rows.Next() {
		ma := &model.ModelAssociation{}
		var createdAt, updatedAt int64
		var matchType string
		if err := rows.Scan(&ma.ID, &ma.VirtualModelID, &ma.ChannelID, &ma.ChannelTags, &matchType, &ma.Pattern,
			&ma.Priority, &ma.Enabled, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		ma.MatchType = model.MatchType(matchType)
		ma.CreatedAt = model.JSONTime{Time: unixToTime(createdAt)}
		ma.UpdatedAt = model.JSONTime{Time: unixToTime(updatedAt)}
		associations = append(associations, ma)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return associations, nil
}

// ListAllModelAssociations 获取所有关联规则列表
func (s *SQLStore) ListAllModelAssociations(ctx context.Context) ([]*model.ModelAssociation, error) {
	query := `
		SELECT id, virtual_model_id, channel_id, channel_tags, match_type, pattern, priority, enabled, created_at, updated_at
		FROM model_associations
		ORDER BY priority DESC, id ASC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var associations []*model.ModelAssociation
	for rows.Next() {
		ma := &model.ModelAssociation{}
		var createdAt, updatedAt int64
		var matchType string
		if err := rows.Scan(&ma.ID, &ma.VirtualModelID, &ma.ChannelID, &ma.ChannelTags, &matchType, &ma.Pattern,
			&ma.Priority, &ma.Enabled, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		ma.MatchType = model.MatchType(matchType)
		ma.CreatedAt = model.JSONTime{Time: unixToTime(createdAt)}
		ma.UpdatedAt = model.JSONTime{Time: unixToTime(updatedAt)}
		associations = append(associations, ma)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return associations, nil
}

// GetModelAssociation 根据ID获取关联规则
func (s *SQLStore) GetModelAssociation(ctx context.Context, id int64) (*model.ModelAssociation, error) {
	query := `
		SELECT id, virtual_model_id, channel_id, channel_tags, match_type, pattern, priority, enabled, created_at, updated_at
		FROM model_associations
		WHERE id = ?
	`
	ma := &model.ModelAssociation{}
	var createdAt, updatedAt int64
	var matchType string
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&ma.ID, &ma.VirtualModelID, &ma.ChannelID, &ma.ChannelTags, &matchType, &ma.Pattern,
		&ma.Priority, &ma.Enabled, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("model association not found")
		}
		return nil, err
	}
	ma.MatchType = model.MatchType(matchType)
	ma.CreatedAt = model.JSONTime{Time: unixToTime(createdAt)}
	ma.UpdatedAt = model.JSONTime{Time: unixToTime(updatedAt)}
	return ma, nil
}

// CreateModelAssociation 创建关联规则
func (s *SQLStore) CreateModelAssociation(ctx context.Context, ma *model.ModelAssociation) (*model.ModelAssociation, error) {
	nowUnix := timeToUnix(time.Now())

	query := `
		INSERT INTO model_associations(virtual_model_id, channel_id, channel_tags, match_type, pattern, priority, enabled, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := s.db.ExecContext(ctx, query,
		ma.VirtualModelID, ma.ChannelID, ma.ChannelTags, string(ma.MatchType), ma.Pattern,
		ma.Priority, boolToInt(ma.Enabled), nowUnix, nowUnix,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "Duplicate") {
			return nil, fmt.Errorf("model association already exists")
		}
		if strings.Contains(err.Error(), "FOREIGN KEY") || strings.Contains(err.Error(), "foreign key") {
			return nil, fmt.Errorf("invalid virtual_model_id or channel_id")
		}
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get last insert id: %w", err)
	}

	return s.GetModelAssociation(ctx, id)
}

// UpdateModelAssociation 更新关联规则
func (s *SQLStore) UpdateModelAssociation(ctx context.Context, id int64, ma *model.ModelAssociation) error {
	// 确认目标存在
	if _, err := s.GetModelAssociation(ctx, id); err != nil {
		return err
	}

	updatedAtUnix := timeToUnix(time.Now())

	query := `
		UPDATE model_associations
		SET virtual_model_id=?, channel_id=?, channel_tags=?, match_type=?, pattern=?, priority=?, enabled=?, updated_at=?
		WHERE id=?
	`
	_, err := s.db.ExecContext(ctx, query,
		ma.VirtualModelID, ma.ChannelID, ma.ChannelTags, string(ma.MatchType), ma.Pattern,
		ma.Priority, boolToInt(ma.Enabled), updatedAtUnix, id,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "Duplicate") {
			return fmt.Errorf("model association already exists")
		}
		if strings.Contains(err.Error(), "FOREIGN KEY") || strings.Contains(err.Error(), "foreign key") {
			return fmt.Errorf("invalid virtual_model_id or channel_id")
		}
		return err
	}
	return nil
}

// DeleteModelAssociation 删除关联规则
func (s *SQLStore) DeleteModelAssociation(ctx context.Context, id int64) error {
	// 检查记录是否存在（幂等性）
	if _, err := s.GetModelAssociation(ctx, id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil // 记录不存在，直接返回
		}
		return err
	}

	query := `DELETE FROM model_associations WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

// ListModelAssociationsWithDetails 获取带详情的关联规则列表
func (s *SQLStore) ListModelAssociationsWithDetails(ctx context.Context, virtualModelID int64) ([]*model.ModelAssociationWithDetails, error) {
	query := `
		SELECT ma.id, ma.virtual_model_id, ma.channel_id, ma.channel_tags, ma.match_type, ma.pattern, ma.priority, ma.enabled,
		       ma.created_at, ma.updated_at, vm.name AS virtual_model_name, c.name AS channel_name
		FROM model_associations ma
		LEFT JOIN virtual_models vm ON ma.virtual_model_id = vm.id
		LEFT JOIN channels c ON ma.channel_id = c.id
		WHERE ma.virtual_model_id = ?
		ORDER BY ma.priority DESC, ma.id ASC
	`
	rows, err := s.db.QueryContext(ctx, query, virtualModelID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var associations []*model.ModelAssociationWithDetails
	for rows.Next() {
		mad := &model.ModelAssociationWithDetails{}
		var createdAt, updatedAt int64
		var matchType string
		if err := rows.Scan(&mad.ID, &mad.VirtualModelID, &mad.ChannelID, &mad.ChannelTags, &matchType, &mad.Pattern,
			&mad.Priority, &mad.Enabled, &createdAt, &updatedAt,
			&mad.VirtualModelName, &mad.ChannelName); err != nil {
			return nil, err
		}
		mad.MatchType = model.MatchType(matchType)
		mad.CreatedAt = model.JSONTime{Time: unixToTime(createdAt)}
		mad.UpdatedAt = model.JSONTime{Time: unixToTime(updatedAt)}
		associations = append(associations, mad)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return associations, nil
}
