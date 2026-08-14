package models

import (
	"context"
	"errors"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TWhiteLabelTemplateModel = (*customTWhiteLabelTemplateModel)(nil)

// ErrWhiteLabelTemplatePublishConflict 表示模板或修订状态已被并发修改，不能继续发布。
var ErrWhiteLabelTemplatePublishConflict = errors.New("white-label template publish conflict")

type (
	// TWhiteLabelTemplateModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTWhiteLabelTemplateModel.
	TWhiteLabelTemplateModel interface {
		tWhiteLabelTemplateModel
		PublishRevision(ctx context.Context, tenantID, templateID, revision, draftStatus, publishedStatus, supersededStatus, disabledStatus int64) error
	}

	customTWhiteLabelTemplateModel struct {
		*defaultTWhiteLabelTemplateModel
	}
)

// NewTWhiteLabelTemplateModel returns a model for the database table.
func NewTWhiteLabelTemplateModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TWhiteLabelTemplateModel {
	return &customTWhiteLabelTemplateModel{
		defaultTWhiteLabelTemplateModel: newTWhiteLabelTemplateModel(conn, c, opts...),
	}
}

// PublishRevision 在同一事务中切换当前修订、上一修订和模板状态，并清理相关缓存。
func (m *customTWhiteLabelTemplateModel) PublishRevision(
	ctx context.Context,
	tenantID, templateID, revision, draftStatus, publishedStatus, supersededStatus, disabledStatus int64,
) error {
	type templateState struct {
		ID                int64  `db:"id"`
		TemplateCode      string `db:"template_code"`
		Status            int64  `db:"status"`
		PublishedRevision int64  `db:"published_revision"`
	}
	type revisionState struct {
		ID       int64  `db:"id"`
		Revision int64  `db:"revision"`
		Checksum string `db:"checksum"`
		Status   int64  `db:"status"`
	}

	var template templateState
	var current revisionState
	var previous *revisionState
	err := m.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		if err := session.QueryRowCtx(txCtx, &template, `SELECT id, template_code, status, published_revision
FROM t_white_label_template WHERE id = ? AND tenant_id = ? FOR UPDATE`, templateID, tenantID); err != nil {
			return err
		}
		if template.Status == disabledStatus {
			return ErrWhiteLabelTemplatePublishConflict
		}
		if err := session.QueryRowCtx(txCtx, &current, `SELECT id, revision, checksum, status
FROM t_white_label_template_revision WHERE tenant_id = ? AND template_id = ? AND revision = ? FOR UPDATE`, tenantID, templateID, revision); err != nil {
			return err
		}
		if current.Status != draftStatus {
			return ErrWhiteLabelTemplatePublishConflict
		}
		if template.PublishedRevision > 0 {
			var item revisionState
			if err := session.QueryRowCtx(txCtx, &item, `SELECT id, revision, checksum, status
FROM t_white_label_template_revision WHERE tenant_id = ? AND template_id = ? AND revision = ? FOR UPDATE`, tenantID, templateID, template.PublishedRevision); err != nil {
				return err
			}
			if item.Status != publishedStatus {
				return ErrWhiteLabelTemplatePublishConflict
			}
			previous = &item
			result, err := session.ExecCtx(txCtx, `UPDATE t_white_label_template_revision SET status = ?
WHERE id = ? AND status = ?`, supersededStatus, item.ID, publishedStatus)
			if err != nil {
				return err
			}
			if affected, err := result.RowsAffected(); err != nil || affected != 1 {
				return ErrWhiteLabelTemplatePublishConflict
			}
		}
		result, err := session.ExecCtx(txCtx, `UPDATE t_white_label_template_revision SET status = ?
WHERE id = ? AND status = ?`, publishedStatus, current.ID, draftStatus)
		if err != nil {
			return err
		}
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			return ErrWhiteLabelTemplatePublishConflict
		}
		result, err = session.ExecCtx(txCtx, `UPDATE t_white_label_template
SET status = ?, published_revision = ?, update_time = CURRENT_TIMESTAMP
WHERE id = ? AND tenant_id = ? AND status <> ?`, publishedStatus, revision, templateID, tenantID, disabledStatus)
		if err != nil {
			return err
		}
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			return ErrWhiteLabelTemplatePublishConflict
		}
		return nil
	})
	if err != nil {
		return err
	}

	keys := []string{
		fmt.Sprintf("%s%v", cacheTWhiteLabelTemplateIdPrefix, template.ID),
		fmt.Sprintf("%s%v:%v", cacheTWhiteLabelTemplateTenantIdTemplateCodePrefix, tenantID, template.TemplateCode),
		fmt.Sprintf("%s%v", cacheTWhiteLabelTemplateRevisionIdPrefix, current.ID),
		fmt.Sprintf("%s%v:%v", cacheTWhiteLabelTemplateRevisionTemplateIdChecksumPrefix, templateID, current.Checksum),
		fmt.Sprintf("%s%v:%v", cacheTWhiteLabelTemplateRevisionTemplateIdRevisionPrefix, templateID, current.Revision),
	}
	if previous != nil {
		keys = append(keys,
			fmt.Sprintf("%s%v", cacheTWhiteLabelTemplateRevisionIdPrefix, previous.ID),
			fmt.Sprintf("%s%v:%v", cacheTWhiteLabelTemplateRevisionTemplateIdChecksumPrefix, templateID, previous.Checksum),
			fmt.Sprintf("%s%v:%v", cacheTWhiteLabelTemplateRevisionTemplateIdRevisionPrefix, templateID, previous.Revision),
		)
	}
	return m.DelCacheCtx(ctx, keys...)
}
