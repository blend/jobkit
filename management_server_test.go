package jobkit

import (
	"bufio"
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/blend/go-sdk/assert"
	"github.com/blend/go-sdk/bufferutil"
	"github.com/blend/go-sdk/cron"
	"github.com/blend/go-sdk/ex"
	"github.com/blend/go-sdk/r2"
	"github.com/blend/go-sdk/uuid"
	"github.com/blend/go-sdk/web"
)

func TestManagmentServerGetRequestJob(t *testing.T) {
	assert := assert.New(t)

	jm := createTestJobManager()
	ms := ManagementServer{
		Cron: jm,
	}

	r := web.MockCtx("GET", "/job/test2+job.foo", web.OptCtxRouteParamValue("jobName", "test2+job.foo"))
	job, res := ms.getRequestJob(r, web.Text)
	assert.Nil(res)
	assert.NotNil(job)
	assert.Equal("test2 job.foo", job.Name)
}

func TestManagmentServerGetRequestJobInvocation(t *testing.T) {
	assert := assert.New(t)

	jm := createTestJobManager()
	ms := ManagementServer{
		Cron: jm,
	}
	job := firstJobScheduler(jm)

	// test failure cases ...
	r := web.MockCtx("GET", "/job/"+url.QueryEscape(job.Name()),
		web.OptCtxRouteParamValue("jobName", job.Name()),
	)
	found, res := ms.getRequestJobInvocation(r, web.Text)
	assert.NotNil(res)
	assert.Nil(found)

	ji := firstInvocation(jm)
	id := ji.JobInvocation.ID

	r = web.MockCtx("GET", "/job/"+url.QueryEscape(job.Name())+"/"+url.QueryEscape(id),
		web.OptCtxRouteParamValue("jobName", job.Name()),
		web.OptCtxRouteParamValue("id", id),
	)
	found, res = ms.getRequestJobInvocation(r, web.Text)
	assert.Nil(res)
	assert.NotNil(found)
	assert.Equal(job.Name(), found.JobInvocation.JobName)
	assert.Equal(id, found.JobInvocation.ID)
}

func TestManagementServerStatic(t *testing.T) {
	assert := assert.New(t)

	_, app := createTestManagementServer()

	meta, err := web.MockGet(app, "/static/js/zepto.min.js").Discard()
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)
}

func TestManagementServerIndex(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()
	contents, meta, err := web.MockGet(app, "/").Bytes()
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)
	jobName := firstJobScheduler(jm).Name()
	assert.Contains(string(contents), fmt.Sprintf("/job/%s", jobName))
	assert.Contains(string(contents), "Show job stats and history")
}

func TestManagementServerSearch(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()
	jobName := firstJobScheduler(jm).Name()

	contents, meta, err := web.MockGet(app, "/search", r2.OptQueryValue("selector", "name="+jobName)).Bytes()
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)
	assert.Contains(string(contents), fmt.Sprintf("/job/%s", jobName))
	assert.Contains(string(contents), "Show job stats and history")
}

func TestManagementServerSearchInvalidSelector(t *testing.T) {
	assert := assert.New(t)

	_, app := createTestManagementServer()

	meta, err := web.MockGet(app, "/search", r2.OptQueryValue("selector", "~~")).Discard()
	assert.Nil(err)
	assert.Equal(http.StatusBadRequest, meta.StatusCode)
}

func TestManagementServerPause(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()
	jm.StartAsync()

	meta, err := web.MockGet(app, "/pause").Discard()
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)
	assert.True(jm.Latch.IsStopped())
}

func TestManagementServerResume(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()

	jm.StartAsync()
	<-jm.Latch.NotifyStarted()
	defer jm.Stop()

	assert.Nil(jm.Stop())
	assert.True(jm.Latch.IsStopped())

	meta, err := web.MockGet(app, "/resume").Discard()
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)

	assert.True(jm.Latch.IsStarted())
}

func TestManagementServerJob(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()

	ji := firstInvocation(jm)
	invocationID := ji.JobInvocation.ID

	contents, meta, err := web.MockGet(app, fmt.Sprintf("/job/%s", ji.JobName)).Bytes()
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)
	assert.Contains(string(contents), html.EscapeString(ji.JobName))
	assert.Contains(string(contents), html.EscapeString(invocationID))

	assert.Contains(string(contents), fmt.Sprintf("/job/%s", html.EscapeString(ji.JobName)))
	assert.NotContains(string(contents), "Show job stats and history")
}

func TestManagementServerJobNotFound(t *testing.T) {
	assert := assert.New(t)

	_, app := createTestManagementServer()

	meta, err := web.MockGet(app, fmt.Sprintf("/job/%s", uuid.V4().String())).Discard()
	assert.Nil(err)
	assert.Equal(http.StatusNotFound, meta.StatusCode)
}

func TestManagementServerJobDisable(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()

	job, err := jm.Job("test1")
	assert.Nil(err)
	assert.NotNil(job)
	jobName := job.Name()
	assert.False(job.Disabled())

	meta, err := web.MockGet(app, fmt.Sprintf("/job.disable/%s", jobName)).Discard()
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)

	assert.True(job.Disabled())
}

func TestManagementServerJobEnable(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()

	job, err := jm.Job("test1")
	assert.Nil(err)
	assert.NotNil(job)
	jobName := job.Name()
	job.Disable()

	meta, err := web.MockGet(app, fmt.Sprintf("/job.enable/%s", jobName)).Discard()
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)
	assert.False(job.Disabled())
}

func TestManagementServerJobRun(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()

	job, err := jm.Job("test1")
	assert.Nil(err)
	assert.NotNil(job)
	jobName := job.Name()

	contents, meta, err := web.MockGet(app, fmt.Sprintf("/job.run/%s", jobName)).Bytes()
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode, string(contents))
	assert.NotNil(job.Last)
}

func TestManagementServerJobCancel(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()

	called := make(chan struct{})
	cancelled := make(chan struct{})

	job := MustNewJob(cron.NewJob(cron.OptJobName("cancel-test"), cron.OptJobAction(func(ctx context.Context) error {
		close(called)
		<-ctx.Done()
		close(cancelled)
		return nil
	})))
	jm.LoadJobs(job)

	js, err := jm.Job("cancel-test")
	assert.Nil(err)
	assert.NotNil(js)

	meta, err := web.MockGet(app, fmt.Sprintf("/job.run/%s", job.Name())).Discard()
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)

	<-called

	meta, err = web.MockGet(app, fmt.Sprintf("/job.cancel/%s", job.Name())).Discard()
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)

	<-cancelled
}

func TestManagementServerJobInvocation(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()

	ji := firstInvocation(jm)
	invocationID := ji.JobInvocation.ID

	contents, meta, err := web.MockGet(app, fmt.Sprintf("/job/%s/%s", ji.JobName, invocationID)).Bytes()
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode, string(contents))
	assert.Contains(string(contents), ji.JobName)
	assert.Contains(string(contents), invocationID)
}

func TestManagementServerJobInvocationCurrent(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()

	job := firstJobScheduler(jm)
	assert.NotNil(job)

	jobName := job.Name()
	invocationID := job.Current().ID

	contents, meta, err := web.MockGet(app, fmt.Sprintf("/job/%s/current", jobName)).Bytes()
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode, string(contents))
	assert.Contains(string(contents), jobName)
	assert.Contains(string(contents), invocationID)
}

func TestManagementServerJobInvocationNotFound(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()

	meta, err := web.MockGet(app, fmt.Sprintf("/job/%s/%s", uuid.V4().String(), uuid.V4().String())).Discard()
	assert.Nil(err)
	assert.Equal(http.StatusNotFound, meta.StatusCode)

	job := firstJobScheduler(jm)
	assert.NotNil(job)
	jobName := job.Name()

	meta, err = web.MockGet(app, fmt.Sprintf("/job/%s/%s", jobName, uuid.V4().String())).Discard()
	assert.Nil(err)
	assert.Equal(http.StatusNotFound, meta.StatusCode)
}

//
// api tests
//

func TestManagementServerAPIJobsRunning(t *testing.T) {
	assert := assert.New(t)

	_, app := createTestManagementServer()
	var jobs []JobViewModel
	meta, err := web.MockGet(app, "/api/jobs.running").JSON(&jobs)
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)
	assert.Len(jobs, 1)
	assert.Equal("test0", jobs[0].Name)
}

func TestManagementServerAPIPause(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()
	jm.StartAsync()

	meta, err := web.MockPost(app, "/api/pause", nil).Discard()
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)
	assert.True(jm.Latch.IsStopped())
}

func TestManagementServerAPIResume(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()
	jm.StartAsync()
	defer jm.Stop()

	jm.Stop()
	assert.True(jm.Latch.IsStopped())

	meta, err := web.MockPost(app, "/api/resume", nil).Discard()
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)
	assert.True(jm.Latch.IsStarted())
}

func TestManagementServerAPIJob(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()

	job := firstJobScheduler(jm)
	assert.NotNil(job)
	jobName := job.Name()

	var jvm JobViewModel
	meta, err := web.MockGet(app, fmt.Sprintf("/api/job/%s", jobName)).JSON(&jvm)
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)
	assert.Equal(jobName, job.Name())
}

func TestManagementServerAPIJobNotFound(t *testing.T) {
	assert := assert.New(t)

	_, app := createTestManagementServer()
	meta, err := web.MockGet(app, fmt.Sprintf("/api/job/%s", uuid.V4().String())).Discard()
	assert.Nil(err)
	assert.Equal(http.StatusNotFound, meta.StatusCode)
}

func TestManagementServerAPIJobRun(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()

	job, err := jm.Job("test1")
	assert.Nil(err)
	assert.NotNil(job)
	jobName := job.Name()

	var ji cron.JobInvocation
	meta, err := web.MockPost(app, fmt.Sprintf("/api/job.run/%s", jobName), nil).JSON(&ji)
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)
	assert.NotEmpty(ji.ID)
	assert.Equal("test1", ji.JobName)
}

func TestManagementServerAPIJobCancel(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()

	called := make(chan struct{})
	cancelled := make(chan struct{})

	job := MustNewJob(cron.NewJob(
		cron.OptJobName("cancel-test"),
		cron.OptJobAction(func(ctx context.Context) error {
			close(called)
			<-ctx.Done()
			close(cancelled)
			return nil
		}),
	))
	jm.LoadJobs(job)

	meta, err := web.MockPost(app, fmt.Sprintf("/api/job.run/%s", job.Name()), nil).Discard()
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)

	<-called

	meta, err = web.MockPost(app, fmt.Sprintf("/api/job.cancel/%s", job.Name()), nil).Discard()
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)

	<-cancelled
}

func TestManagementServerAPIJobDisable(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()

	job, err := jm.Job("test1")
	assert.Nil(err)
	assert.NotNil(job)
	jobName := job.Name()
	assert.False(job.Disabled())

	meta, err := web.MockPost(app, fmt.Sprintf("/api/job.disable/%s", jobName), nil).Discard()
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)

	assert.True(job.Disabled())
}

func TestManagementServerAPIJobEnable(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()

	job, err := jm.Job("test1")
	assert.Nil(err)
	assert.NotNil(job)
	jobName := job.Name()
	job.Disable()

	meta, err := web.MockPost(app, fmt.Sprintf("/api/job.enable/%s", jobName), nil).Discard()
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)
	assert.False(job.Disabled())
}

func TestManagementServerAPIJobInvocation(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()

	invocation := firstInvocation(jm)
	invocationID := invocation.JobInvocation.ID

	var ji cron.JobInvocation
	meta, err := web.MockGet(app, fmt.Sprintf("/api/job/%s/%s", invocation.JobName, invocationID)).JSON(&ji)
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)
	assert.Equal(invocation.JobName, ji.JobName)
	assert.Equal(invocationID, ji.ID)
}

func TestManagementServerAPIJobOutput(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()

	invocation := firstInvocation(jm)
	invocationID := invocation.JobInvocation.ID

	var output struct {
		ServerTimeNanos int64                    `json:"serverTimeNanos"`
		Chunks          []bufferutil.BufferChunk `json:"chunks"`
	}
	meta, err := web.MockGet(app, fmt.Sprintf("/api/job.output/%s/%s", invocation.JobName, invocationID)).JSON(&output)
	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)
	assert.NotZero(output.ServerTimeNanos)
	assert.Len(output.Chunks, 5)
}

func TestManagementServerAPIJobOutputAfterNanos(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()

	invocation := firstInvocation(jm)
	invocationID := invocation.JobInvocation.ID
	afterNanos := invocation.Output.Chunks[2].Timestamp.UnixNano()

	var output struct {
		ServerTimeNanos int64                    `json:"serverTimeNanos"`
		Chunks          []bufferutil.BufferChunk `json:"chunks"`
	}
	meta, err := web.MockGet(app,
		fmt.Sprintf("/api/job.output/%s/%s", invocation.JobName, invocationID),
		r2.OptQueryValue("afterNanos", fmt.Sprint(afterNanos)),
	).JSON(&output)

	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)
	assert.NotZero(output.ServerTimeNanos)
	assert.Len(output.Chunks, 2)
}

func TestManagementServerAPIJobOutputAfterNanosInvalid(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()

	invocation := firstInvocation(jm)
	invocationID := invocation.JobInvocation.ID

	var output struct {
		ServerTimeNanos int64                    `json:"serverTimeNanos"`
		Chunks          []bufferutil.BufferChunk `json:"chunks"`
	}
	meta, err := web.MockGet(app,
		fmt.Sprintf("/api/job.output/%s/%s", invocation.JobName, invocationID),
		r2.OptQueryValue("afterNanos", "baileydog"),
	).JSON(&output)

	assert.Nil(err)
	assert.Equal(http.StatusOK, meta.StatusCode)
	assert.NotZero(output.ServerTimeNanos)
	assert.Len(output.Chunks, 5)
}

func TestManagementServerAPIJobOutputStreamComplete(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()
	invocation := firstCompleteInvocation(jm)

	contents, res, err := web.MockGet(app,
		fmt.Sprintf("/api/job.output.stream/%s/%s", invocation.JobName, invocation.ID),
		r2.OptQueryValue("afterNanos", "baileydog"),
	).Bytes()

	assert.Nil(err)
	defer res.Body.Close()

	assert.Equal(http.StatusOK, res.StatusCode, string(contents))
	assert.Equal("event: ping\n\nevent: complete\ndata: success\n\n", string(contents))
}

func TestManagementServerAPIJobOutputStream(t *testing.T) {
	assert := assert.New(t)

	jm, app := createTestManagementServer()

	jobScheduler := firstJobScheduler(jm)

	jobName := jobScheduler.Name()
	ji := jobScheduler.Current()
	invocationID := ji.ID

	start := make(chan struct{})
	finish := make(chan struct{})
	errors := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errors <- ex.New(r)
			}
		}()
		<-start
		io.WriteString(ji.State.(*JobInvocationOutput).Output, "test1\n")
		io.WriteString(ji.State.(*JobInvocationOutput).Output, "test2\n")
		io.WriteString(ji.State.(*JobInvocationOutput).Output, "test3\n")
		io.WriteString(ji.State.(*JobInvocationOutput).Output, "test4\n")
		io.WriteString(ji.State.(*JobInvocationOutput).Output, "test5\n")

		<-finish
		ji.Status = cron.JobInvocationStatusSuccess
		ji.Complete = time.Now().UTC()

		jobScheduler.SetCurrent(nil)
		jobScheduler.SetLast(ji)
	}()

	res, err := web.MockGet(app,
		fmt.Sprintf("/api/job.output.stream/%s/%s", jobName, invocationID),
	).Do()

	assert.Nil(err)
	defer res.Body.Close()
	assert.Equal(http.StatusOK, res.StatusCode)

	// tell the sidecar to send data
	close(start)

	// read the body out
	scanner := bufio.NewScanner(res.Body)

	expectedScript := []string{
		"event: ping",
		"",
		"event: writeln",
		"data: {\"data\":\"test1\"}",
		"",
		"event: writeln",
		"data: {\"data\":\"test2\"}",
		"",
		"event: writeln",
		"data: {\"data\":\"test3\"}",
		"",
		"event: writeln",
		"data: {\"data\":\"test4\"}",
		"",
		"event: writeln",
		"data: {\"data\":\"test5\"}",
		"",
	}
	for _, expected := range expectedScript {
		scanner.Scan()
		line := scanner.Text()
		assert.Equal(expected, line)
	}
	assert.Empty(errors)
	close(finish)
	scanner.Scan()
	line := scanner.Text()
	assert.Equal("event: complete", line)
}
