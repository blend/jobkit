package jobkit

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/blend/go-sdk/db"
	"github.com/blend/go-sdk/db/migration"
	"github.com/blend/go-sdk/uuid"
)

var (
	_ HistoryProvider = (*HistoryPostgres)(nil)
)

// HistoryPostgres implements a sqlite history provider.
type HistoryPostgres struct {
	Conn *db.Connection
	Tx   *sql.Tx
}

// Initialize creates the schema for the provider.
func (h *HistoryPostgres) Initialize(ctx context.Context) error {
	return migration.NewWithGroups(
		migration.NewGroupWithAction(
			migration.TableNotExists("job_invocations"),
			migration.Statements(
				`create table job_invocations (
					id uuid not null,
					job_name varchar(255) not null,
					started timestamp not null,
					complete timestamp,
					status varchar(64) not null,
					parameters jsonb,
					err text,
					output text
				)`,
			),
			migration.OptGroupTx(h.Tx),
		),
	).Apply(ctx, h.Conn)
}

type jobInvocationRow struct {
	ID         uuid.UUID         `db:"id,pk"`
	JobName    string            `db:"job_name"`
	Started    time.Time         `db:"started"`
	Complete   time.Time         `db:"complete"`
	Status     string            `db:"status"`
	Parameters map[string]string `db:"parameters,json"`
	Err        string            `db:"err"`
	Output     string            `db:"output"`
}

func (ji jobInvocationRow) TableName() string { return "job_invocations" }

// Add adds a result.
func (h *HistoryPostgres) Add(ctx context.Context, ji *JobInvocation) error {
	return h.Conn.Invoke(db.OptContext(ctx)).Create(jobInvocationRow{
		ID:         uuid.MustParse(ji.ID),
		JobName:    ji.JobName,
		Started:    ji.Started,
		Complete:   ji.Complete,
		Status:     string(ji.Status),
		Parameters: ji.Parameters,
		Err:        fmt.Sprintf("%+v", ji.Err),
		Output:     ji.Output.String(),
	})
}

// Get gets all results for a given job.
func (h *HistoryPostgres) Get(ctx context.Context, jobName string) ([]*JobInvocation, error) {
	var output []*JobInvocation
	err := h.Conn.Invoke(db.OptContext(ctx)).Query(
		fmt.Sprintf("select %s from %s where job_name = $1", db.ColumnNamesCSV(jobInvocationRow{}), jobInvocationRow{}.TableName()),
		jobName,
	).OutMany(&output)
	return output, err
}

// GetByID gets a specific result.
func (h *HistoryPostgres) GetByID(ctx context.Context, jobName, invocationID string) (*JobInvocation, error) {
	var output JobInvocation
	err := h.Conn.Invoke(db.OptContext(ctx)).Query(
		fmt.Sprintf("select %s from %s where job_name = $1 and invocation_id = $2", db.ColumnNamesCSV(jobInvocationRow{}), jobInvocationRow{}.TableName()),
		jobName,
		invocationID,
	).OutMany(&output)
	return &output, err
}

// Cull culls history.
func (h *HistoryPostgres) Cull(ctx context.Context, jobName string, maxCount int, maxAge time.Duration) (err error) {
	if maxCount > 0 && maxAge > 0 {
		_, err = h.Conn.Invoke(db.OptContext(ctx)).Exec(`
		WITH ranked_history AS ( SELECT id, job_name, ROW_NUMBER() OVER (partition by job_name, order by completed asc) as history_rank )
		DELETE FROM job_invocations ji INNER JOIN ranked_history rh ON ji.id = rh.id AND ji.job_name = rh.job_name
		WHERE ji.job_name = $1 AND ( rh.hisory_rank < $2 OR ji.completed < $3 )`, jobName, maxCount, time.Now().UTC().Add(-maxAge))
	} else if maxCount > 0 {
		_, err = h.Conn.Invoke(db.OptContext(ctx)).Exec(`
		WITH ranked_history AS ( SELECT id, job_name, ROW_NUMBER() OVER (partition by job_name, order by completed asc) as history_rank )
		DELETE FROM job_invocations ji INNER JOIN ranked_history rh ON ji.id = rh.id AND ji.job_name = rh.job_name
		WHERE job_name = $1 AND hisory_rank < $2`, jobName, maxCount)
	} else if maxAge > 0 {
		_, err = h.Conn.Invoke(db.OptContext(ctx)).Exec(`
		DELETE FROM job_invocations WHERE job_name = $1 AND completed < $1
		`, jobName, time.Now().UTC().Add(-maxAge))
	}
	return
}
