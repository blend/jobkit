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
		Data:      []byte(uuid.V4()),
	}
}

func createTestCompleteJobInvocation(jobName string, elapsed time.Duration) *JobInvocation {
	ts := time.Now().UTC()
	output := &JobInvocationOutput{
		Output: &bufferutil.Buffer{
			Chunks: []bufferutil.BufferChunk{
				createTestBufferChunk(0),
				createTestBufferChunk(1),
				createTestBufferChunk(2),
				createTestBufferChunk(3),
				createTestBufferChunk(4),
			},
		},
	}

	ji := &cron.JobInvocation{
		ID:       uuid.V4().String(),
		JobName:  jobName,
		Started:  ts,
		Complete: ts.Add(elapsed),
		Status:   cron.JobInvocationStatusSuccess,
		State:    output,
	}
	ji.Context = cron.WithJobInvocation(context.Background(), ji)
	ji.Context = WithJobInvocationOutput(ji.Context, output)

	return &JobInvocation{
		JobInvocation:       ji,
		JobInvocationOutput: *output,
	}
}

func createTestFailedJobInvocation(jobName string, elapsed time.Duration, err error) *JobInvocation {
	ts := time.Now().UTC()
	return &JobInvocation{
		JobInvocation: &cron.JobInvocation{
			ID:       uuid.V4().String(),
			JobName:  jobName,
			Started:  ts,
			Complete: ts.Add(elapsed),
			Status:   cron.JobInvocationStatusErrored,
			Err:      err,
		},
		JobInvocationOutput: JobInvocationOutput{
			Output: &bufferutil.Buffer{
				Chunks: []bufferutil.BufferChunk{
					createTestBufferChunk(0),
					createTestBufferChunk(1),
				},
			},
		},
	}
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

func TestMain(m *testing.M) {
	assert.Main(m)
}
