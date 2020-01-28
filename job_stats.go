package jobkit

import (
	"time"

	"github.com/blend/go-sdk/cron"
	"github.com/blend/go-sdk/mathutil"
)

// HistoryStats returns job history stats.
func HistoryStats(history []*JobInvocation) JobStats {
	output := JobStats{
		RunsTotal: len(history),
	}
	var elapsedTimes []time.Duration
	for _, ji := range history {
		switch ji.Status {
		case cron.JobInvocationStatusSuccess:
			output.RunsSuccessful++
		case cron.JobInvocationStatusErrored:
			output.RunsErrored++
		case cron.JobInvocationStatusCancelled:
			output.RunsCancelled++
		}

		elapsedTimes = append(elapsedTimes, ji.JobInvocation.Elapsed())
		if ji.JobInvocation.Elapsed() > output.ElapsedMax {
			output.ElapsedMax = ji.JobInvocation.Elapsed()
		}
		if ji.JobInvocationOutput.Output != nil {
			output.OutputBytes += len(ji.JobInvocationOutput.Output.Bytes())
		}
	}
	if output.RunsTotal > 0 {
		output.SuccessRate = float64(output.RunsSuccessful) / float64(output.RunsTotal)
	}
	output.Elapsed50th = mathutil.PercentileOfDuration(elapsedTimes, 50.0)
	output.Elapsed95th = mathutil.PercentileOfDuration(elapsedTimes, 95.0)
	return output
}

// JobStats represent stats about a job scheduler.
type JobStats struct {
	SuccessRate    float64       `json:"successRate"`
	OutputBytes    int           `json:"outputBytes"`
	RunsTotal      int           `json:"runsTotal"`
	RunsSuccessful int           `json:"runsSuccessful"`
	RunsErrored    int           `json:"runsErrored"`
	RunsCancelled  int           `json:"runsCancelled"`
	ElapsedMax     time.Duration `json:"elapsedMax"`
	ElapsedMin     time.Duration `json:"elapsedMin"`
	Elapsed50th    time.Duration `json:"elapsed50th"`
	Elapsed95th    time.Duration `json:"elapsed95th"`
}
