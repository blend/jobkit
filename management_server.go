package jobkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blend/go-sdk/bufferutil"
	"github.com/blend/go-sdk/cron"
	"github.com/blend/go-sdk/ex"
	"github.com/blend/go-sdk/logger"
	"github.com/blend/go-sdk/selector"
	"github.com/blend/go-sdk/stringutil"
	"github.com/blend/go-sdk/uuid"
	"github.com/blend/go-sdk/web"
	"github.com/blend/go-sdk/webutil"

	"github.com/blend/jobkit/static"
	"github.com/blend/jobkit/views"
)

// NewServer returns a new management server that lets you
// trigger jobs or look at job statuses via. a json api.
func NewServer(jm *cron.JobManager, cfg Config, options ...web.Option) *web.App {
	options = append([]web.Option{web.OptConfig(cfg.Web)}, options...)
	app := web.MustNew(options...)
	app.Register(ManagementServer{Cron: jm, Config: cfg})
	return app
}

// ManagementServer is the jobkit management server.
type ManagementServer struct {
	Config Config
	Cron   *cron.JobManager
}

// Register registers the management server.
func (ms ManagementServer) Register(app *web.App) {
	if ms.Config.UseViewFilesOrDefault() {
		app.Views.LiveReload = true
		app.Views.AddPaths(ms.ViewPaths()...)
	} else {
		app.Views.LiveReload = false
		for _, viewPath := range ms.ViewPaths() {
			vf, err := views.GetBinaryAsset(viewPath)
			if err != nil {
				panic(ex.New(err, ex.OptMessagef("view path: %s", viewPath)))
			}
			contents, err := vf.Contents()
			if err != nil {
				panic(err)
			}
			app.Views.AddLiterals(string(contents))
		}
	}
	app.DefaultMiddleware = append(app.DefaultMiddleware, ms.addContextStateConfig)
	app.PanicAction = func(r *web.Ctx, err interface{}) web.Result {
		return r.Views.InternalError(ex.New(err))
	}
	app.Views.FuncMap["format_environ"] = func(params cron.JobParameters) string {
		if len(params) == 0 {
			return "-"
		}
		return strings.Join(ParameterValuesAsEnviron(params), ",")
	}
	app.Views.FuncMap["job"] = func(viewmodel interface{}) *Job {
		jobScheduler, ok := viewmodel.(*cron.JobScheduler)
		if !ok {
			return nil
		}
		job, ok := jobScheduler.Job.(*Job)
		if !ok {
			return nil
		}
		return job
	}

	// web specific routes
	app.GET("/status.json", ms.getStatus)
	app.GET("/static/*filepath", ms.getStatic)

	// manager routes
	app.GET("/", ms.getIndex)
	app.GET("/search", ms.getSearch)
	app.GET("/pause", ms.getPause)
	app.GET("/resume", ms.getResume)

	// job routes
	app.GET("/job/:jobName", ms.getJob)
	app.GET("/job/:jobName/:id", ms.getJobInvocation)
	app.GET("/job.parameters/:jobName", ms.getJobParameters)
	app.GET("/job.run/:jobName", ms.getJobRun)
	app.GET("/job.enable/:jobName", ms.getJobEnable)
	app.GET("/job.disable/:jobName", ms.getJobDisable)
	app.GET("/job.cancel/:jobName", ms.getJobCancel)

	// api routes
	app.POST("/api/pause", ms.postAPIPause)
	app.POST("/api/resume", ms.postAPIResume)
	app.GET("/api/jobs", ms.getAPIJobs)
	app.GET("/api/jobs.running", ms.getAPIJobsRunning)
	app.GET("/api/job/:jobName", ms.getAPIJob)
	app.GET("/api/job.parameters/:jobName", ms.getAPIJobParameters)
	app.POST("/api/job.run/:jobName", ms.postAPIJobRun)
	app.POST("/api/job.cancel/:jobName", ms.postAPIJobCancel)
	app.POST("/api/job.disable/:jobName", ms.postAPIJobDisable)
	app.POST("/api/job.enable/:jobName", ms.postAPIJobEnable)
	app.GET("/api/job/:jobName/:id", ms.getAPIJobInvocation)
	app.GET("/api/job.output/:jobName/:id", ms.getAPIJobOutput)
	app.GET("/api/job.output.stream/:jobName/:id", ms.getAPIJobOutputStream)

	// debug things
	app.GET("/api/debug/error", func(r *web.Ctx) web.Result {
		return web.JSON.InternalError(ex.New("this is only a test"))
	})
}

// ViewPaths returns the view paths for the management server.
func (ms ManagementServer) ViewPaths() []string {
	return []string{
		"_views/header.html",
		"_views/footer.html",
		"_views/index.html",
		"_views/job.html",
		"_views/invocation.html",
		"_views/parameters.html",
		"_views/partials/job_table.html",
		"_views/partials/job_row.html",
		"_views/status/error.html",
		"_views/status/not_found.html",
		"_views/status/bad_request.html",
	}
}

// getStatus is mapped to GET /status.json
func (ms ManagementServer) getStatus(r *web.Ctx) web.Result {
	return web.JSON.Result(ms.Cron.State())
}

// getStatic is mapped to GET /static/*filepath
func (ms ManagementServer) getStatic(r *web.Ctx) web.Result {
	path, err := r.RouteParam("filepath")
	if err != nil {
		web.Text.NotFound()
	}
	path = filepath.Join("_static", path)

	if ms.Config.UseViewFilesOrDefault() {
		r.WithContext(logger.WithLabels(r.Context(), logger.Labels{
			"web.static_file": path,
		}))
		return web.Static(path)
	}

	r.WithContext(logger.WithLabels(r.Context(), logger.Labels{
		"web.static_file_cached": path,
	}))

	file, err := static.GetBinaryAsset(path)
	if err == os.ErrNotExist {
		return web.Text.NotFound()
	}
	contents, err := file.Contents()
	if err != nil {
		return web.Text.InternalError(err)
	}
	http.ServeContent(r.Response, r.Request, path, time.Unix(file.ModTime, 0), bytes.NewReader(contents))
	return nil
}

//
// api or view routes
//

// getIndex is mapped to GET /
func (ms ManagementServer) getIndex(r *web.Ctx) web.Result {
	r.State.Set("show-job-history-link", true)
	jobs, err := NewJobViewModels(ms.Cron.Jobs)
	if err != nil {
		return r.Views.InternalError(err)
	}
	return r.Views.View("index", jobs)
}

// getIndex is mapped to GET /search?selector=<SELECTOR>
func (ms ManagementServer) getSearch(r *web.Ctx) web.Result {
	selectorParam := web.StringValue(r.QueryValue("selector"))
	if selectorParam == "" {
		return web.RedirectWithMethod("GET", "/")
	}
	sel, err := selector.Parse(selectorParam)
	if err != nil {
		return r.Views.BadRequest(err)
	}
	r.State.Set("selector", sel.String())

	jobs, err := NewJobViewModels(ms.Cron.Jobs)
	if err != nil {
		return r.Views.InternalError(err)
	}
	jobs = FilterJobViewModels(jobs, func(job *JobViewModel) bool {
		return sel.Matches(job.Labels)
	})
	r.State.Set("show-job-history-link", true)
	return r.Views.View("index", jobs)
}

// getPause is mapped to GET /pause
func (ms ManagementServer) getPause(r *web.Ctx) web.Result {
	if err := ms.Cron.Stop(); err != nil {
		return r.Views.BadRequest(err)
	}
	return web.RedirectWithMethod("GET", "/")
}

// getResume is mapped to GET /resume
func (ms ManagementServer) getResume(r *web.Ctx) web.Result {
	if err := ms.Cron.StartAsync(); err != nil {
		return r.Views.BadRequest(err)
	}
	return web.RedirectWithMethod("GET", "/")
}

// getJob is mapped to GET /job/:jobName
func (ms ManagementServer) getJob(r *web.Ctx) web.Result {
	job, result := ms.getRequestJob(r, web.JSON)
	if result != nil {
		return result
	}
	return r.Views.View("job", job)
}

// getJobParameters is mapped to GET /job.parameters/:jobName
func (ms ManagementServer) getJobParameters(r *web.Ctx) web.Result {
	job, result := ms.getRequestJob(r, web.JSON)
	if result != nil {
		return result
	}

	parameters := job.Config.Parameters
	if len(parameters) == 0 {
		return web.Redirect("/job.run/" + job.Name)
	}
	return r.Views.View("parameters", job)
}

// getJobRun is mapped to GET /job.run/:jobName
func (ms ManagementServer) getJobRun(r *web.Ctx) web.Result {
	job, result := ms.getRequestJob(r, web.JSON)
	if result != nil {
		return result
	}
	if err := r.Request.ParseForm(); err != nil {
		return r.Views.BadRequest(err)
	}

	jobScheduler, err := ms.Cron.Job(job.Name)
	if err != nil {
		return r.Views.BadRequest(err)
	}

	parameters := job.Config.Parameters
	parameterValues := ParameterValuesFromForm(parameters, r.Request.Form)
	ji, _, err := jobScheduler.RunAsyncContext(cron.WithJobParameterValues(context.Background(), parameterValues))
	if err != nil {
		return r.Views.BadRequest(err)
	}
	return web.RedirectWithMethodf("GET", "/job/%s/%s", url.QueryEscape(job.Name), ji.ID)
}

// getJobEnable is mapped to GET /job.enable/:jobName
func (ms ManagementServer) getJobEnable(r *web.Ctx) web.Result {
	job, result := ms.getRequestJob(r, web.JSON)
	if result != nil {
		return result
	}
	if err := ms.Cron.EnableJobs(job.Name); err != nil {
		return r.Views.BadRequest(err)
	}
	return web.RedirectWithMethod("GET", "/")
}

// getJobDisable is mapped to GET /job.disable/:jobName
func (ms ManagementServer) getJobDisable(r *web.Ctx) web.Result {
	job, result := ms.getRequestJob(r, web.JSON)
	if result != nil {
		return result
	}
	if err := ms.Cron.DisableJobs(job.Name); err != nil {
		return r.Views.BadRequest(err)
	}
	return web.RedirectWithMethod("GET", "/")
}

// getJobCancel is mapped to GET /job.cancel;/:jobName
func (ms ManagementServer) getJobCancel(r *web.Ctx) web.Result {
	job, result := ms.getRequestJob(r, web.JSON)
	if result != nil {
		return result
	}
	err := ms.Cron.CancelJob(job.Name)
	if err != nil {
		return r.Views.BadRequest(err)
	}
	return web.RedirectWithMethod("GET", "/")
}

// getJobInvocation is mapped to GET /job/:jobName/:id
func (ms ManagementServer) getJobInvocation(r *web.Ctx) web.Result {
	invocation, result := ms.getRequestJobInvocation(r, r.Views)
	if result != nil {
		return result
	}
	return r.Views.View("invocation", invocation)
}

// getAPIJobs is mapped to GET /api/jobs
func (ms ManagementServer) getAPIJobs(r *web.Ctx) web.Result {
	jobs, err := NewJobViewModels(ms.Cron.Jobs)
	if err != nil {
		return r.Views.InternalError(err)
	}
	return web.JSON.Result(jobs)
}

// getAPIJobsRunning is mapped to GET /api/jobs.running
func (ms ManagementServer) getAPIJobsRunning(r *web.Ctx) web.Result {
	jobs, err := NewJobViewModels(ms.Cron.Jobs)
	if err != nil {
		return r.Views.InternalError(err)
	}
	return web.JSON.Result(FilterJobViewModels(jobs, func(jvm *JobViewModel) bool {
		return jvm.Current != nil
	}))
}

// postAPIPause is mapped to POST /api/pause
func (ms ManagementServer) postAPIPause(r *web.Ctx) web.Result {
	if err := ms.Cron.Stop(); err != nil {
		return r.Views.BadRequest(err)
	}
	return web.JSON.OK()
}

// postAPIResume is mapped to POST /api/resume
func (ms ManagementServer) postAPIResume(r *web.Ctx) web.Result {
	if err := ms.Cron.StartAsync(); err != nil {
		return r.Views.BadRequest(err)
	}
	return web.JSON.OK()
}

// getAPIJob is mapped to GET /api/job/:jobName
func (ms ManagementServer) getAPIJob(r *web.Ctx) web.Result {
	job, result := ms.getRequestJob(r, web.JSON)
	if result != nil {
		return result
	}
	return web.JSON.Result(job)
}

// getAPIJobParameters is mapped to GET /api/job.parameters/:jobName
func (ms ManagementServer) getAPIJobParameters(r *web.Ctx) web.Result {
	job, result := ms.getRequestJob(r, web.JSON)
	if result != nil {
		return result
	}
	return web.JSON.Result(job.Config.Parameters)
}

// postAPIJobRun is mapped to POST /api/job.run/:jobName
func (ms ManagementServer) postAPIJobRun(r *web.Ctx) web.Result {
	job, result := ms.getRequestJob(r, web.JSON)
	if result != nil {
		return result
	}

	parameters := job.Config.Parameters
	body, err := r.PostBody()
	if err != nil {
		return web.JSON.BadRequest(err)
	}
	var params cron.JobParameters
	if len(body) > 0 {
		params, err = ParameterValuesFromJSON(parameters, body)
		if err != nil {
			return web.JSON.BadRequest(err)
		}
	}
	ji, _, err := ms.Cron.RunJobContext(cron.WithJobParameterValues(context.Background(), params), job.Name)
	if err != nil {
		return web.JSON.BadRequest(err)
	}
	return web.JSON.Result(ji)
}

// postAPIJobCancel is mapped to POST /api/job.cancel/:jobName
func (ms ManagementServer) postAPIJobCancel(r *web.Ctx) web.Result {
	job, result := ms.getRequestJob(r, web.JSON)
	if result != nil {
		return result
	}
	if err := ms.Cron.CancelJob(job.Name); err != nil {
		return web.JSON.BadRequest(err)
	}
	return web.JSON.OK()
}

// postAPIJobDisable is mapped to POST /api/job.disable/:jobName
func (ms ManagementServer) postAPIJobDisable(r *web.Ctx) web.Result {
	job, result := ms.getRequestJob(r, web.JSON)
	if result != nil {
		return result
	}
	if err := ms.Cron.DisableJobs(job.Name); err != nil {
		return web.JSON.BadRequest(err)
	}
	return web.JSON.OK()
}

// postAPIJobEnable is mapped to POST /api/job.enable/:jobName
func (ms ManagementServer) postAPIJobEnable(r *web.Ctx) web.Result {
	job, result := ms.getRequestJob(r, web.JSON)
	if result != nil {
		return result
	}
	if err := ms.Cron.EnableJobs(job.Name); err != nil {
		return web.JSON.BadRequest(err)
	}
	return web.JSON.Result(fmt.Sprintf("%s enabled", job.Name))
}

// getAPIJobInvocation is mapped to GET /api/job.invocation/:jobName/:id
func (ms ManagementServer) getAPIJobInvocation(r *web.Ctx) web.Result {
	invocation, result := ms.getRequestJobInvocation(r, web.JSON)
	if result != nil {
		return result
	}
	return web.JSON.Result(invocation)
}

// getAPIJobOutput is mapped to GET /api/job.output/:jobName/:id
func (ms ManagementServer) getAPIJobOutput(r *web.Ctx) web.Result {
	invocation, result := ms.getRequestJobInvocation(r, web.JSON)
	if result != nil {
		return result
	}
	chunks := invocation.JobInvocationOutput.Output.Chunks
	if afterNanos, _ := web.Int64Value(r.QueryValue("afterNanos")); afterNanos > 0 {
		afterTS := time.Unix(0, afterNanos)

		var filtered []bufferutil.BufferChunk
		for _, chunk := range chunks {
			if chunk.Timestamp.After(afterTS) {
				filtered = append(filtered, chunk)
			}
		}
		return web.JSON.Result(map[string]interface{}{
			"serverTimeNanos": time.Now().UTC().UnixNano(),
			"chunks":          filtered,
		})
	}
	return web.JSON.Result(map[string]interface{}{
		"serverTimeNanos": time.Now().UTC().UnixNano(),
		"chunks":          chunks,
	})
}

// getAPIJobOutput is mapped to GET /api/job.output.stream/:jobName/:id
func (ms ManagementServer) getAPIJobOutputStream(r *web.Ctx) web.Result {
	invocation, result := ms.getRequestJobInvocation(r, web.JSON)
	if result != nil {
		return result
	}

	// set up the event source
	es := webutil.NewEventSource(r.Response)
	if err := es.StartSession(); err != nil {
		logger.MaybeError(r.App.Log, err)
		return nil
	}

	// check if the job is running, if not, send the complete event and return
	if !ms.Cron.IsJobRunning(invocation.JobInvocation.JobName) {
		logger.MaybeDebugf(r.App.Log, "output stream; job is not running, closing")
		if err := es.EventData("complete", string(invocation.Status)); err != nil {
			logger.MaybeError(r.App.Log, err)
		}
		return nil
	}

	// this is a helper closure that splits
	// a chunk into lines, and sends each line individually
	// because the server events spec is not ideal
	sendOutputData := func(chunk bufferutil.BufferChunk) {
		for _, line := range stringutil.SplitLines(string(chunk.Data),
			stringutil.OptSplitLinesIncludeNewLine(true),
			stringutil.OptSplitLinesIncludeEmptyLines(true),
		) {
			contents, _ := json.Marshal(map[string]interface{}{"data": strings.TrimSuffix(line, "\n")})
			if strings.HasSuffix(line, "\n") {
				if err := es.EventData("writeln", string(contents)); err != nil {
					logger.MaybeError(r.App.Log, err)
				}
			} else {
				if err := es.EventData("write", string(contents)); err != nil {
					logger.MaybeError(r.App.Log, err)
				}
			}
		}
	}

	// if the caller specifies an afterNanos, ship the catchup chunks from an invocation already in flight
	if afterNanos, _ := web.Int64Value(r.QueryValue("afterNanos")); afterNanos > 0 {
		after := time.Unix(0, afterNanos)
		logger.MaybeDebugf(r.App.Log, "output stream; sending catchup output stream data from: %v", after)
		for _, chunk := range invocation.Output.Chunks {
			if chunk.Timestamp.After(after) {
				sendOutputData(chunk)
			}
		}
	}

	// this is the identifier for the streaming session
	// it won't mean anything after this action returns
	listenerID := uuid.V4().String()
	logger.MaybeDebugf(r.App.Log, "output stream; listening for new chunks")
	// listen for new chunks, this will fire synchronously
	invocation.OutputHandlers.Add(listenerID, func(chunk bufferutil.BufferChunk) {
		sendOutputData(chunk)
	})
	// unhook on exit
	defer func() { invocation.OutputHandlers.Remove(listenerID) }()

	// every 100ms, send a heartbeat, if it errors, return
	// otherwise, listen for the client to have closed, and return
	updateTick := time.Tick(100 * time.Millisecond)
	for {
		select {
		case <-updateTick:
			if !ms.Cron.IsJobRunning(invocation.JobName) { // if the job isn't running, return
				logger.MaybeDebugf(r.App.Log, "output stream; job invocation is complete, closing")
				if err := es.EventData("complete", string(invocation.Status)); err != nil {
					logger.MaybeError(r.App.Log, err)
				}
				return nil
			}
			// send a ping, if a browser is listening it
			// will postpone it closing the connection
			if err := es.Ping(); err != nil { // if the ping fails, log and return
				logger.MaybeError(r.App.Log, err)
				return nil
			}
			// send an elapsed time update
			if err := es.EventData("elapsed", fmt.Sprintf("%v", time.Now().UTC().Sub(invocation.Started).Round(time.Millisecond))); err != nil {
				logger.MaybeError(r.App.Log, err)
				return nil
			}
		case <-r.Context().Done(): // if the context signals done, return
			logger.MaybeDebugf(r.App.Log, "output stream; reader exiting on context done")
			return nil
		}
	}
}

// addContextStateConfig is a middleware that adds the config to a request context's state.
func (ms ManagementServer) addContextStateConfig(action web.Action) web.Action {
	return func(r *web.Ctx) web.Result {
		r.State.Set("config", ms.Config)
		return action(r)
	}
}

func (ms ManagementServer) getRequestJob(r *web.Ctx, resultProvider web.ResultProvider) (*JobViewModel, web.Result) {
	jobName, err := r.RouteParam("jobName")
	if err != nil {
		return nil, resultProvider.BadRequest(err)
	}
	jobName, err = url.QueryUnescape(jobName)
	if err != nil {
		return nil, resultProvider.BadRequest(err)
	}
	jobScheduler, err := ms.Cron.Job(jobName)
	if err != nil || jobScheduler == nil {
		return nil, resultProvider.NotFound()
	}
	jvm, err := NewJobViewModel(jobScheduler)
	if err != nil {
		return nil, resultProvider.InternalError(err)
	}
	return jvm, nil
}

// getRequestJobInvocation pulls a job invocation off a request context.
func (ms ManagementServer) getRequestJobInvocation(r *web.Ctx, resultProvider web.ResultProvider) (*JobInvocation, web.Result) {
	job, result := ms.getRequestJob(r, resultProvider)
	if result != nil {
		return nil, result
	}

	invocationID, err := r.RouteParam("id")
	if err != nil {
		return nil, resultProvider.BadRequest(err)
	}

	if (invocationID == "current" && job.Current != nil) ||
		(job.Current != nil && job.Current.ID == invocationID) {
		return job.Current, nil
	}
	if (invocationID == "last" && job.Last != nil) ||
		(job.Last != nil && job.Last.ID == invocationID) {
		return job.Last, nil
	}

	invocation, ok := job.HistoryLookup[invocationID]
	if !ok {
		return nil, resultProvider.NotFound()
	}
	return invocation, nil
}
