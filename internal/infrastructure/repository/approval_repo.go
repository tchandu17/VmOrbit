package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// ─────────────────────────────────────────────────────────────────────────────
// ApprovalRequestRepo
// ─────────────────────────────────────────────────────────────────────────────

type ApprovalRequestRepo struct{ db *gorm.DB }

func NewApprovalRequestRepo(db *gorm.DB) *ApprovalRequestRepo {
	return &ApprovalRequestRepo{db: db}
}

func (r *ApprovalRequestRepo) Create(ctx context.Context, req *model.ApprovalRequest) error {
	return r.db.WithContext(ctx).Create(req).Error
}

func (r *ApprovalRequestRepo) GetByID(ctx context.Context, id string) (*model.ApprovalRequest, error) {
	var req model.ApprovalRequest
	if err := r.db.WithContext(ctx).
		Preload("Steps", func(db *gorm.DB) *gorm.DB {
			return db.Order("step_order ASC")
		}).
		Preload("History", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		First(&req, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("approval request not found: %w", err)
	}
	return &req, nil
}

func (r *ApprovalRequestRepo) Update(ctx context.Context, req *model.ApprovalRequest) error {
	return r.db.WithContext(ctx).Save(req).Error
}

func (r *ApprovalRequestRepo) List(ctx context.Context, filter port.ApprovalFilter, page port.Page) (*port.PageResult[model.ApprovalRequest], error) {
	var requests []model.ApprovalRequest
	var total int64

	q := r.db.WithContext(ctx).Model(&model.ApprovalRequest{})
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.RequesterID != nil {
		q = q.Where("requester_id = ?", filter.RequesterID)
	}
	if filter.PolicyID != nil {
		q = q.Where("policy_id = ?", filter.PolicyID)
	}
	if filter.Operation != "" {
		q = q.Where("operation = ?", filter.Operation)
	}
	if filter.ResourceType != "" {
		q = q.Where("resource_type = ?", filter.ResourceType)
	}
	if filter.ResourceID != "" {
		q = q.Where("resource_id = ?", filter.ResourceID)
	}
	if filter.Since != nil {
		q = q.Where("created_at >= ?", filter.Since)
	}
	if filter.Until != nil {
		q = q.Where("created_at <= ?", filter.Until)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page.Number - 1) * page.Size
	if err := q.Preload("Steps", func(db *gorm.DB) *gorm.DB {
		return db.Order("step_order ASC")
	}).Order("created_at DESC").Offset(offset).Limit(page.Size).Find(&requests).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / page.Size
	if int(total)%page.Size != 0 {
		totalPages++
	}

	return &port.PageResult[model.ApprovalRequest]{
		Items:      requests,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}

func (r *ApprovalRequestRepo) ListExpired(ctx context.Context, now time.Time) ([]model.ApprovalRequest, error) {
	var requests []model.ApprovalRequest
	if err := r.db.WithContext(ctx).
		Where("status = 'pending' AND expires_at IS NOT NULL AND expires_at <= ?", now).
		Find(&requests).Error; err != nil {
		return nil, err
	}
	return requests, nil
}

func (r *ApprovalRequestRepo) ListPendingForUser(ctx context.Context, userID string, roles []string, page port.Page) (*port.PageResult[model.ApprovalRequest], error) {
	var requests []model.ApprovalRequest
	var total int64

	// Find requests that have a pending step where the user is the approver
	// (either by user ID or by role).
	subQuery := r.db.WithContext(ctx).
		Model(&model.ApprovalStep{}).
		Select("request_id").
		Where("status = 'pending'").
		Where(
			r.db.Where("approver_id = ?", userID).
				Or("approver_role IN ?", append(roles, "")),
		)

	q := r.db.WithContext(ctx).
		Model(&model.ApprovalRequest{}).
		Where("status = 'pending'").
		Where("id IN (?)", subQuery)

	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page.Number - 1) * page.Size
	if err := q.Preload("Steps", func(db *gorm.DB) *gorm.DB {
		return db.Order("step_order ASC")
	}).Order("created_at ASC").Offset(offset).Limit(page.Size).Find(&requests).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / page.Size
	if int(total)%page.Size != 0 {
		totalPages++
	}

	return &port.PageResult[model.ApprovalRequest]{
		Items:      requests,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// ApprovalStepRepo
// ─────────────────────────────────────────────────────────────────────────────

type ApprovalStepRepo struct{ db *gorm.DB }

func NewApprovalStepRepo(db *gorm.DB) *ApprovalStepRepo {
	return &ApprovalStepRepo{db: db}
}

func (r *ApprovalStepRepo) Create(ctx context.Context, s *model.ApprovalStep) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *ApprovalStepRepo) GetByID(ctx context.Context, id string) (*model.ApprovalStep, error) {
	var s model.ApprovalStep
	if err := r.db.WithContext(ctx).First(&s, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("approval step not found: %w", err)
	}
	return &s, nil
}

func (r *ApprovalStepRepo) Update(ctx context.Context, s *model.ApprovalStep) error {
	return r.db.WithContext(ctx).Save(s).Error
}

func (r *ApprovalStepRepo) ListByRequest(ctx context.Context, requestID string) ([]model.ApprovalStep, error) {
	var steps []model.ApprovalStep
	if err := r.db.WithContext(ctx).
		Where("request_id = ?", requestID).
		Order("step_order ASC").
		Find(&steps).Error; err != nil {
		return nil, err
	}
	return steps, nil
}

func (r *ApprovalStepRepo) GetCurrentPendingStep(ctx context.Context, requestID string) (*model.ApprovalStep, error) {
	var s model.ApprovalStep
	if err := r.db.WithContext(ctx).
		Where("request_id = ? AND status = 'pending'", requestID).
		Order("step_order ASC").
		First(&s).Error; err != nil {
		return nil, fmt.Errorf("no pending step found: %w", err)
	}
	return &s, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// ApprovalHistoryRepo
// ─────────────────────────────────────────────────────────────────────────────

type ApprovalHistoryRepo struct{ db *gorm.DB }

func NewApprovalHistoryRepo(db *gorm.DB) *ApprovalHistoryRepo {
	return &ApprovalHistoryRepo{db: db}
}

func (r *ApprovalHistoryRepo) Create(ctx context.Context, h *model.ApprovalHistory) error {
	return r.db.WithContext(ctx).Create(h).Error
}

func (r *ApprovalHistoryRepo) ListByRequest(ctx context.Context, requestID string) ([]model.ApprovalHistory, error) {
	var history []model.ApprovalHistory
	if err := r.db.WithContext(ctx).
		Where("request_id = ?", requestID).
		Order("created_at ASC").
		Find(&history).Error; err != nil {
		return nil, err
	}
	return history, nil
}
