package jobkit

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/blend/go-sdk/bufferutil"
	"github.com/blend/go-sdk/cron"
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
					id uuid not null primary key,
					job_name varchar(255) not null,
					started timestamp not null,
					complete timestamp,
					status varchar(64) not null,
					parameters json,
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

// JobInvocation returns the row as a job invocation.
// It does stuff like wires up the output handlers etc. so you can use it transparently
// with the rest of the management server actions.
func (ji jobInvocationRow) JobInvocation() *JobInvocation {
	output := bufferutil.NewBuffer([]byte(ji.Output))
	outputHandlers := new(bufferutil.BufferHandlers)
	output.Handler = outputHandlers.Handle

	jio := &JobInvocation{
		JobInvocation: &cron.JobInvocation{
			ID:         ji.ID.String(),
			JobName:    ji.JobName,
			Started:    ji.Started,
			Complete:   ji.Complete,
			Parameters: ji.Parameters,
			Status:     cron.JobInvocationStatus(ji.Status),
		},
		JobInvocationOutput: JobInvocationOutput{
			Output:         output,
			OutputHandlers: outputHandlers,
		},
	}
	if ji.Err != "" {
		jio.Err = fmt.Errorf(ji.Err)
	}
	return jio
}

// Add adds a result.
func (h *HistoryPostgres) Add(ctx context.Context, ji *JobInvocation) error {
	obj := jobInvocationRow{
		ID:         uuid.MustParse(ji.ID),
		JobName:    ji.JobName,
		Started:    ji.Started,
		Complete:   ji.Complete,
		Status:     string(ji.Status),
		Parameters: ji.Parameters,
		Output:     ji.Output.String(),
	}
	if ji.Err != nil {
		obj.Err = fmt.Sprintf("%+v", ji.Err)
	}
	return h.Conn.Invoke(
		db.OptContext(ctx),
		db.OptTx(h.Tx),
	).Create(obj)
}

// Get gets all results for a given job.
func (h *HistoryPostgres) Get(ctx context.Context, jobName string) (output []*JobInvocation, err error) {
	var invocations []jobInvocationRow
	err = h.Conn.Invoke(
		db.OptContext(ctx),
		db.OptTx(h.Tx),
	).Query(
		fmt.Sprintf("select %s from %s where job_name = $1", db.ColumnNamesCSV(jobInvocationRow{}), jobInvocationRow{}.TableName()),
		jobName,
	).OutMany(&invocations)
	if err != nil {
		return
	}

	output = make([]*JobInvocation, len(invocations))
	for index := range invocations {
		output[index] = invocations[index].JobInvocation()
	}
	return
}

// GetByID gets a specific result.
func (h *HistoryPostgres) GetByID(ctx context.Context, jobName, invocationID string) (*JobInvocation, error) {
	var output jobInvocationRow
	_, err := h.Conn.Invoke(
		db.OptContext(ctx),
		db.OptTx(h.Tx),
	).Query(
		fmt.Sprintf("select %s from %s where job_name = $1 and id = $2", db.ColumnNamesCSV(jobInvocationRow{}), jobInvocationRow{}.TableName()),
		jobName,
		invocationID,
	).Out(&output)
	return output.JobInvocation(), err
}

// Cull culls history.
func (h *HistoryPostgres) Cull(ctx context.Context, jobName string, maxCount int, maxAge time.Duration) (err error) {
	opts := []db.InvocationOption{db.OptContext(ctx), db.OptTx(h.Tx)}

	if maxCount > 0 && maxAge > 0 {
		_, err = h.Conn.Invoke(opts...).Exec(`
		WITH ranked_history AS ( SELECT id, job_name, ROW_NUMBER() OVER (PARTITION BY job_name ORDER BY complete DESC) as history_rank FROM job_invocations WHERE job_name = $1)
		DELETE FROM job_invocations ji USING ranked_history rh
		WHERE
			ji.id = rh.id
			AND ji.job_name = rh.job_name
			AND ji.job_name = $1
			AND ( rh.history_rank > $2 OR ji.complete < $3 )`, jobName, maxCount, time.Now().UTC().Add(-maxAge))
	} else if maxCount > 0 {
		_, err = h.Conn.Invoke(opts...).Exec(`
		WITH ranked_history AS ( SELECT id, job_name, ROW_NUMBER() OVER (PARTITION BY job_name ORDER BY complete DESC) as history_rank FROM job_invocations WHERE job_name = $1 )
		DELETE FROM job_invocations ji USING ranked_history rh
		WHERE
			ji.id = rh.id
			AND ji.job_name = rh.job_name
			AND ji.job_name = $1
			AND rh.history_rank > $2`, jobName, maxCount)
	} else if maxAge > 0 {
		_, err = h.Conn.Invoke(opts...).Exec(`
		DELETE FROM job_invocations WHERE job_name = $1 AND complete < $2
		`, jobName, time.Now().UTC().Add(-maxAge))
	}
	return
}
