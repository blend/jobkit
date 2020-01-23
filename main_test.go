package jobkit

import (
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

func firstJob(jm *cron.JobManager) *cron.JobScheduler {
	sorted := sortedJobs(jm)
	if len(sorted) > 0 {
		return sorted[0]
	}
	return nil
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
	return &JobInvocation{
		JobInvocation: cron.JobInvocation{
			ID:       uuid.V4().String(),
			JobName:  jobName,
			Started:  ts,
			Complete: ts.Add(elapsed),
			Status:   cron.JobInvocationStatusSuccess,
		},
		JobInvocationOutput: JobInvocationOutput{
			Output: &bufferutil.Buffer{
				Chunks: []bufferutil.BufferChunk{
					createTestBufferChunk(0),
					createTestBufferChunk(1),
					createTestBufferChunk(2),
					createTestBufferChunk(3),
					createTestBufferChunk(4),
				},
			},
		},
	}
}

func createTestFailedJobInvocation(jobName string, elapsed time.Duration, err error) *JobInvocation {
	ts := time.Now().UTC()
	return &JobInvocation{
		JobInvocation: cron.JobInvocation{
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
	test0 := cron.NewJob(cron.OptJobName("test0"))
	test1 := cron.NewJob(cron.OptJobName("test1"))
	test2 := cron.NewJob(cron.OptJobName("test2 job.foo"))

	jm := cron.New()
	jm.LoadJobs(test0, test1, test2)

	test0CurrentOutput := &bufferutil.Buffer{
		Chunks: []bufferutil.BufferChunk{
			createTestBufferChunk(0),
			createTestBufferChunk(1),
			createTestBufferChunk(2),
			createTestBufferChunk(3),
		},
	}
	test0CurrentBufferHandlers := new(bufferutil.BufferHandlers)
	test0CurrentOutput.Handler = test0CurrentBufferHandlers.Handle

	jm.Jobs["test0"].Job.(*Job).Current = &JobInvocation{
		JobInvocation: cron.JobInvocation{
			ID:      uuid.V4().String(),
			JobName: "test0",
			Started: time.Now().UTC(),
		},
		JobInvocationOutput: JobInvocationOutput{
			Output:         test0CurrentOutput,
			OutputHandlers: test0CurrentBufferHandlers,
		},
	}

	jm.Jobs["test0"].Job.(*Job).History = []*JobInvocation{
		createTestCompleteJobInvocation("test0", 200*time.Millisecond),
		createTestCompleteJobInvocation("test0", 250*time.Millisecond),
		createTestFailedJobInvocation("test0", 5*time.Second, fmt.Errorf("this is only a test %s", uuid.V4().String())),
	}
	jm.Jobs["test1"].Job.(*Job).History = []*JobInvocation{
		createTestCompleteJobInvocation("test1", 200*time.Millisecond),
		createTestCompleteJobInvocation("test1", 250*time.Millisecond),
		createTestCompleteJobInvocation("test1", 300*time.Millisecond),
		createTestCompleteJobInvocation("test1", 350*time.Millisecond),
	}
	jm.Jobs["test2 job.foo"].Job.(*Job).History = []*JobInvocation{
		createTestCompleteJobInvocation("test2 job.foo", 200*time.Millisecond),
		createTestCompleteJobInvocation("test2 job.foo", 250*time.Millisecond),
		createTestCompleteJobInvocation("test2 job.foo", 300*time.Millisecond),
		createTestCompleteJobInvocation("test2 job.foo", 350*time.Millisecond),
	}
	return jm
}

func createTestManagementServer() (*cron.JobManager, *web.App) {
	jm := createTestJobManager()
	return jm, NewServer(jm, Config{
		Web: web.Config{
			Port: 5000,
		},
	})
}

func TestMain(m *testing.M) {
	assert.Main(m)
}
