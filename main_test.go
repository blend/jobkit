package jobkit

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/blend/go-sdk/assert"
	"github.com/blend/go-sdk/bufferutil"
	"github.com/blend/go-sdk/cron"
	"github.com/blend/go-sdk/uuid"
	"github.com/blend/go-sdk/web"
)

type jobInvocationOption func(*JobInvocation)

func optJobID(id string) jobInvocationOption {
	return func(ji *JobInvocation) { ji.ID = id }
}

func optJobName(name string) jobInvocationOption {
	return func(ji *JobInvocation) { ji.JobName = name }
}

func optJobStarted(ts time.Time) jobInvocationOption {
	return func(ji *JobInvocation) { ji.Started = ts }
}

func optJobComplete(ts time.Time) jobInvocationOption {
	return func(ji *JobInvocation) { ji.Complete = ts }
}

func optJobElapsed(elapsed time.Duration) jobInvocationOption {
	return func(ji *JobInvocation) { ji.Complete = ji.Started.Add(elapsed) }
}

func optJobStatus(status cron.JobInvocationStatus) jobInvocationOption {
	return func(ji *JobInvocation) { ji.Status = status }
}

func optJobErr(err error) jobInvocationOption {
	return func(ji *JobInvocation) { ji.Err = err }
}

func optJobParameters(params map[string]string) jobInvocationOption {
	return func(ji *JobInvocation) { ji.Parameters = params }
}

func createTestJobInvocation(jobName string, opts ...jobInvocationOption) *JobInvocation {
	output := &bufferutil.Buffer{
		Chunks: []bufferutil.BufferChunk{
			createTestBufferChunk(0),
			createTestBufferChunk(1),
			createTestBufferChunk(2),
			createTestBufferChunk(3),
			createTestBufferChunk(4),
		},
	}
	outputHandlers := new(bufferutil.BufferHandlers)
	output.Handler = outputHandlers.Handle

	jobInvocationOutput := JobInvocationOutput{
		Output:         output,
		OutputHandlers: outputHandlers,
	}
	jobInvocation := &cron.JobInvocation{
		ID:      uuid.V4().String(),
		JobName: jobName,
		Started: time.Now().UTC(),
		Status:  cron.JobInvocationStatusSuccess,
		State:   &jobInvocationOutput,
	}
	jobInvocation.Context = cron.WithJobInvocation(context.Background(), jobInvocation)
	jobInvocation.Context = WithJobInvocationOutput(jobInvocation.Context, &jobInvocationOutput)

	ji := &JobInvocation{
		JobInvocation:       jobInvocation,
		JobInvocationOutput: jobInvocationOutput,
	}
	for _, opt := range opts {
		opt(ji)
	}
	return ji
}

func createTestCompleteJobInvocation(jobName string, elapsed time.Duration, opts ...jobInvocationOption) *JobInvocation {
	ts := time.Now().UTC()
	return createTestJobInvocation(jobName, append([]jobInvocationOption{
		optJobStarted(ts),
		optJobComplete(ts.Add(elapsed)),
		optJobStatus(cron.JobInvocationStatusSuccess),
	}, opts...)...)
}

func createTestFailedJobInvocation(jobName string, elapsed time.Duration, err error, opts ...jobInvocationOption) *JobInvocation {
	ts := time.Now().UTC()
	return createTestJobInvocation(jobName, append([]jobInvocationOption{
		optJobStarted(ts),
		optJobComplete(ts.Add(elapsed)),
		optJobStatus(cron.JobInvocationStatusErrored),
		optJobErr(err),
	}, opts...)...)
}

func createTestJobManager() *cron.JobManager {
	test0inner := cron.NewJob(cron.OptJobName("test0"))
	test1inner := cron.NewJob(cron.OptJobName("test1"))
	test2inner := cron.NewJob(cron.OptJobName("test2 job.foo"))

	test0History := new(HistoryMemory)
	test0History.Initialize(context.TODO())
	test0History.AddMany(context.TODO(),
		createTestCompleteJobInvocation("test0", 200*time.Millisecond),
		createTestCompleteJobInvocation("test0", 250*time.Millisecond),
		createTestFailedJobInvocation("test0", 5*time.Second, fmt.Errorf("this is only a test %s", uuid.V4().String())),
	)

	test1History := new(HistoryMemory)
	test1History.Initialize(context.TODO())
	test1History.AddMany(context.TODO(),
		createTestCompleteJobInvocation("test1", 200*time.Millisecond),
		createTestCompleteJobInvocation("test1", 250*time.Millisecond),
		createTestCompleteJobInvocation("test1", 300*time.Millisecond),
		createTestCompleteJobInvocation("test1", 350*time.Millisecond),
	)

	test2History := new(HistoryMemory)
	test2History.Initialize(context.TODO())
	test2History.AddMany(context.TODO(),
		createTestCompleteJobInvocation("test2 job.foo", 200*time.Millisecond),
		createTestCompleteJobInvocation("test2 job.foo", 250*time.Millisecond),
		createTestCompleteJobInvocation("test2 job.foo", 300*time.Millisecond),
		createTestCompleteJobInvocation("test2 job.foo", 350*time.Millisecond),
	)

	test0 := MustNewJob(test0inner, OptJobHistory(test0History))
	test1 := MustNewJob(test1inner, OptJobHistory(test1History))
	test2 := MustNewJob(test2inner, OptJobHistory(test2History))

	jm := cron.New()
	jm.LoadJobs(test0, test1, test2)
	jm.Jobs["test0"].Current = createTestCompleteJobInvocation("test0", 200*time.Millisecond).JobInvocation
	return jm
}

func createTestManagementServer() (*cron.JobManager, *web.App) {
	jm := createTestJobManager()
	return jm, NewServer(jm, Config{})
}

func firstJobScheduler(jm *cron.JobManager) *cron.JobScheduler {
	sorted := sortedJobs(jm)
	if len(sorted) > 0 {
		return sorted[0]
	}
	return nil
}

func firstInvocation(jm *cron.JobManager) *JobInvocation {
	for _, js := range jm.Jobs {
		job, ok := js.Job.(*Job)
		if !ok {
			return nil
		}
		history, ok := job.HistoryProvider.(*HistoryMemory)
		if !ok {
			return nil
		}

		jis := history.History[js.Name()]
		if len(jis) == 0 {
			return nil
		}
		return jis[0]
	}
	return nil
}

func firstCompleteInvocation(jm *cron.JobManager) *JobInvocation {
	for _, js := range jm.Jobs {
		if js.Current != nil {
			continue
		}
		job, ok := js.Job.(*Job)
		if !ok {
			return nil
		}
		history, ok := job.HistoryProvider.(*HistoryMemory)
		if !ok {
			return nil
		}

		jis := history.History[js.Name()]
		if len(jis) == 0 {
			return nil
		}
		return jis[0]
	}
	return nil
}

func createTimestamp(adjustBy time.Duration) time.Time {
	return time.Date(2019, 10, 01, 12, 11, 10, 9, time.UTC).Add(adjustBy)
}

// sortedJobs returns the list of jobs ordered by job name.
func sortedJobs(jm *cron.JobManager) []*cron.JobScheduler {
	var output []*cron.JobScheduler
	for _, js := range jm.Jobs {
		output = append(output, js)
	}
	sort.Sort(cron.JobSchedulersByJobNameAsc(output))
	return output
}

func createTestBufferChunk(index int) bufferutil.BufferChunk {
	return bufferutil.BufferChunk{
		Timestamp: createTimestamp(time.Duration(index) * time.Second),
		Data:      []byte(uuid.V4().String()),
	}
}

func TestMain(m *testing.M) {
	assert.Main(m)
}
