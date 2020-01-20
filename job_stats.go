package jobkit

import "time"

// JobStats represent stats about a job scheduler.
type JobStats struct {
	Name           string        `json:"name"`
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
