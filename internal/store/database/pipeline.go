// Copyright 2022 Harness Inc. All rights reserved.
// Use of this source code is governed by the Polyform Free Trial License
// that can be found in the LICENSE.md file for this repository.

package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/harness/gitness/internal/store"
	gitness_store "github.com/harness/gitness/store"
	"github.com/harness/gitness/store/database"
	"github.com/harness/gitness/store/database/dbtx"
	"github.com/harness/gitness/types"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

var _ store.PipelineStore = (*pipelineStore)(nil)

const (
	pipelineQueryBase = `
		SELECT` +
		pipelineColumns + `
		FROM pipelines`

	pipelineColumns = `
	pipeline_id
	,pipeline_description
	,pipeline_created_by
	,pipeline_uid
	,pipeline_seq
	,pipeline_repo_id
	,pipeline_default_branch
	,pipeline_config_path
	,pipeline_created
	,pipeline_updated
	,pipeline_version
	`
)

// NewPipelineStore returns a new PipelineStore.
func NewPipelineStore(db *sqlx.DB) *pipelineStore {
	return &pipelineStore{
		db: db,
	}
}

type pipelineStore struct {
	db *sqlx.DB
}

// Find returns a pipeline given a pipeline ID.
func (s *pipelineStore) Find(ctx context.Context, id int64) (*types.Pipeline, error) {
	const findQueryStmt = pipelineQueryBase + `
		WHERE pipeline_id = $1`
	db := dbtx.GetAccessor(ctx, s.db)

	dst := new(types.Pipeline)
	if err := db.GetContext(ctx, dst, findQueryStmt, id); err != nil {
		return nil, database.ProcessSQLErrorf(err, "Failed to find pipeline")
	}
	return dst, nil
}

// FindByUID returns a pipeline for a given repo with a given UID.
func (s *pipelineStore) FindByUID(ctx context.Context, repoID int64, uid string) (*types.Pipeline, error) {
	const findQueryStmt = pipelineQueryBase + `
		WHERE pipeline_repo_id = $1 AND pipeline_uid = $2`
	db := dbtx.GetAccessor(ctx, s.db)

	dst := new(types.Pipeline)
	if err := db.GetContext(ctx, dst, findQueryStmt, repoID, uid); err != nil {
		return nil, database.ProcessSQLErrorf(err, "Failed to find pipeline")
	}
	return dst, nil
}

// Create creates a pipeline.
func (s *pipelineStore) Create(ctx context.Context, pipeline *types.Pipeline) error {
	const pipelineInsertStmt = `
	INSERT INTO pipelines (
		pipeline_description
		,pipeline_uid
		,pipeline_seq
		,pipeline_repo_id
		,pipeline_created_by
		,pipeline_default_branch
		,pipeline_config_path
		,pipeline_created
		,pipeline_updated
		,pipeline_version
	) VALUES (
		:pipeline_description,
		:pipeline_uid,
		:pipeline_seq,
		:pipeline_repo_id,
		:pipeline_created_by,
		:pipeline_default_branch,
		:pipeline_config_path,
		:pipeline_created,
		:pipeline_updated,
		:pipeline_version
	) RETURNING pipeline_id`
	db := dbtx.GetAccessor(ctx, s.db)

	query, arg, err := db.BindNamed(pipelineInsertStmt, pipeline)
	if err != nil {
		return database.ProcessSQLErrorf(err, "Failed to bind pipeline object")
	}

	if err = db.QueryRowContext(ctx, query, arg...).Scan(&pipeline.ID); err != nil {
		return database.ProcessSQLErrorf(err, "Pipeline query failed")
	}

	return nil
}

// Update updates a pipeline.
func (s *pipelineStore) Update(ctx context.Context, p *types.Pipeline) error {
	const pipelineUpdateStmt = `
	UPDATE pipelines
	SET
		pipeline_description = :pipeline_description,
		pipeline_uid = :pipeline_uid,
		pipeline_seq = :pipeline_seq,
		pipeline_default_branch = :pipeline_default_branch,
		pipeline_config_path = :pipeline_config_path,
		pipeline_updated = :pipeline_updated,
		pipeline_version = :pipeline_version
	WHERE pipeline_id = :pipeline_id AND pipeline_version = :pipeline_version - 1`
	updatedAt := time.Now()
	pipeline := *p

	pipeline.Version++
	pipeline.Updated = updatedAt.UnixMilli()

	db := dbtx.GetAccessor(ctx, s.db)

	query, arg, err := db.BindNamed(pipelineUpdateStmt, pipeline)
	if err != nil {
		return database.ProcessSQLErrorf(err, "Failed to bind pipeline object")
	}

	result, err := db.ExecContext(ctx, query, arg...)
	if err != nil {
		return database.ProcessSQLErrorf(err, "Failed to update pipeline")
	}

	count, err := result.RowsAffected()
	if err != nil {
		return database.ProcessSQLErrorf(err, "Failed to get number of updated rows")
	}

	if count == 0 {
		return gitness_store.ErrVersionConflict
	}

	p.Updated = pipeline.Updated
	p.Version = pipeline.Version
	return nil
}

// List lists all the pipelines for a repository.
func (s *pipelineStore) List(
	ctx context.Context,
	repoID int64,
	filter types.ListQueryFilter,
) ([]*types.Pipeline, error) {
	stmt := database.Builder.
		Select(pipelineColumns).
		From("pipelines").
		Where("pipeline_repo_id = ?", fmt.Sprint(repoID))

	if filter.Query != "" {
		stmt = stmt.Where("LOWER(pipeline_uid) LIKE ?", fmt.Sprintf("%%%s%%", strings.ToLower(filter.Query)))
	}

	stmt = stmt.Limit(database.Limit(filter.Size))
	stmt = stmt.Offset(database.Offset(filter.Page, filter.Size))

	sql, args, err := stmt.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to convert query to sql")
	}

	db := dbtx.GetAccessor(ctx, s.db)

	dst := []*types.Pipeline{}
	if err = db.SelectContext(ctx, &dst, sql, args...); err != nil {
		return nil, database.ProcessSQLErrorf(err, "Failed executing custom list query")
	}

	return dst, nil
}

// ListLatest lists all the pipelines under a repository with information
// about the latest build if available.
func (s *pipelineStore) ListLatest(
	ctx context.Context,
	repoID int64,
	filter types.ListQueryFilter,
) ([]*types.Pipeline, error) {
	const pipelineExecutionColumns = pipelineColumns + `
	,executions.execution_id
	,executions.execution_pipeline_id
	,execution_repo_id
	,execution_trigger
	,execution_number
	,execution_status
	,execution_error
	,execution_link
	,execution_message
	,execution_after
	,execution_timestamp
	,execution_title
	,execution_author
	,execution_author_name
	,execution_author_email
	,execution_author_avatar
	,execution_source
	,execution_target
	,execution_source_repo
	,execution_started
	,execution_finished
	,execution_created
	,execution_updated
	`
	// Create a subquery to get max execution IDs for each unique execution pipeline ID.
	subquery := database.Builder.
		Select("execution_pipeline_id, execution_id, MAX(execution_number)").
		From("executions").
		Where("execution_repo_id = ?").
		GroupBy("execution_pipeline_id")

	// Convert the subquery to SQL.
	subquerySQL, _, err := subquery.ToSql()
	if err != nil {
		return nil, err
	}

	// Left join the previous table with executions and pipelines table.
	stmt := database.Builder.
		Select(pipelineExecutionColumns).
		From("pipelines").
		LeftJoin("("+subquerySQL+") AS max_executions ON pipelines.pipeline_id = max_executions.execution_pipeline_id").
		LeftJoin("executions ON executions.execution_id = max_executions.execution_id").
		Where("pipeline_repo_id = ?", fmt.Sprint(repoID))

	if filter.Query != "" {
		stmt = stmt.Where("LOWER(pipeline_uid) LIKE ?", fmt.Sprintf("%%%s%%", strings.ToLower(filter.Query)))
	}
	stmt = stmt.Limit(database.Limit(filter.Size))
	stmt = stmt.Offset(database.Offset(filter.Page, filter.Size))

	sql, args, err := stmt.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to convert query to sql")
	}

	db := dbtx.GetAccessor(ctx, s.db)

	dst := []*pipelineExecutionJoin{}
	if err = db.SelectContext(ctx, &dst, sql, args...); err != nil {
		return nil, database.ProcessSQLErrorf(err, "Failed executing custom list query")
	}

	return convert(dst), nil
}

// UpdateOptLock updates the pipeline using the optimistic locking mechanism.
func (s *pipelineStore) UpdateOptLock(ctx context.Context,
	pipeline *types.Pipeline,
	mutateFn func(pipeline *types.Pipeline) error) (*types.Pipeline, error) {
	for {
		dup := *pipeline

		err := mutateFn(&dup)
		if err != nil {
			return nil, err
		}

		err = s.Update(ctx, &dup)
		if err == nil {
			return &dup, nil
		}
		if !errors.Is(err, gitness_store.ErrVersionConflict) {
			return nil, err
		}

		pipeline, err = s.Find(ctx, pipeline.ID)
		if err != nil {
			return nil, err
		}
	}
}

// Count of pipelines under a repo.
func (s *pipelineStore) Count(ctx context.Context, repoID int64, filter types.ListQueryFilter) (int64, error) {
	stmt := database.Builder.
		Select("count(*)").
		From("pipelines").
		Where("pipeline_repo_id = ?", repoID)

	if filter.Query != "" {
		stmt = stmt.Where("LOWER(pipeline_uid) LIKE ?", fmt.Sprintf("%%%s%%", strings.ToLower(filter.Query)))
	}

	sql, args, err := stmt.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "Failed to convert query to sql")
	}

	db := dbtx.GetAccessor(ctx, s.db)

	var count int64
	err = db.QueryRowContext(ctx, sql, args...).Scan(&count)
	if err != nil {
		return 0, database.ProcessSQLErrorf(err, "Failed executing count query")
	}
	return count, nil
}

// Delete deletes a pipeline given a pipeline ID.
func (s *pipelineStore) Delete(ctx context.Context, id int64) error {
	const pipelineDeleteStmt = `
		DELETE FROM pipelines
		WHERE pipeline_id = $1`

	db := dbtx.GetAccessor(ctx, s.db)

	if _, err := db.ExecContext(ctx, pipelineDeleteStmt, id); err != nil {
		return database.ProcessSQLErrorf(err, "Could not delete pipeline")
	}

	return nil
}

// DeleteByUID deletes a pipeline with a given UID under a given repo.
func (s *pipelineStore) DeleteByUID(ctx context.Context, repoID int64, uid string) error {
	const pipelineDeleteStmt = `
	DELETE FROM pipelines
	WHERE pipeline_repo_id = $1 AND pipeline_uid = $2`

	db := dbtx.GetAccessor(ctx, s.db)

	if _, err := db.ExecContext(ctx, pipelineDeleteStmt, repoID, uid); err != nil {
		return database.ProcessSQLErrorf(err, "Could not delete pipeline")
	}

	return nil
}

// Increment increments the pipeline sequence number. It will keep retrying in case
// of optimistic lock errors.
func (s *pipelineStore) IncrementSeqNum(ctx context.Context, pipeline *types.Pipeline) (*types.Pipeline, error) {
	for {
		var err error
		pipeline.Seq++
		err = s.Update(ctx, pipeline)
		if err == nil {
			return pipeline, nil
		} else if !errors.Is(err, gitness_store.ErrVersionConflict) {
			return pipeline, errors.Wrap(err, "could not increment pipeline sequence number")
		}
		pipeline, err = s.Find(ctx, pipeline.ID)
		if err != nil {
			return nil, errors.Wrap(err, "could not increment pipeline sequence number")
		}
	}
}
