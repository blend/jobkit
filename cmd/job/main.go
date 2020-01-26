package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/blend/go-sdk/ansi"
	"github.com/blend/go-sdk/configutil"
	"github.com/blend/go-sdk/cron"
	"github.com/blend/go-sdk/datadog"
	"github.com/blend/go-sdk/email"
	"github.com/blend/go-sdk/env"
	"github.com/blend/go-sdk/ex"
	"github.com/blend/go-sdk/graceful"
	"github.com/blend/go-sdk/logger"
	"github.com/blend/go-sdk/ref"
	"github.com/blend/go-sdk/sentry"
	"github.com/blend/go-sdk/slack"
	"github.com/blend/go-sdk/stats"
	"github.com/blend/go-sdk/stringutil"

	"github.com/blend/jobkit"
)

var (
	flagTitle                                *string
	flagBind                                 *string
	flagConfigPath                           *string
	flagDisableServer                        *bool
	flagUseViewFiles                         *bool
	flagDefaultJobName                       *string
	flagDefaultJobExec                       *string
	flagDefaultJobSchedule                   *string
	flagDefaultJobHistoryDisabled            *bool
	flagDefaultJobHistoryPersistenceDisabled *bool
	flagDefaultJobHistoryPath                *string
	flagDefaultJobHistoryMaxCount            *int
	flagDefaultJobHistoryMaxAge              *time.Duration
	flagDefaultJobTimeout                    *time.Duration
	flagDefaultJobShutdownGracePeriod        *time.Duration
	flagDefaultJobLabels                     *[]string
	flagDefaultJobSkipExpandEnv              *bool
	flagDefaultJobDiscardOutput              *bool
	flagDefaultJobHideOutput                 *bool
)

func initFlags(cmd *cobra.Command) {
	flagTitle = cmd.Flags().String("title", "", "The title for the jobkit instance.")
	flagBind = cmd.Flags().String("bind", "", "The management http server bind address.")
	flagConfigPath = cmd.Flags().StringP("config", "c", "", "The config path.")
	flagUseViewFiles = cmd.Flags().Bool("use-view-files", false, "If we should use view files vs. statically linked assets.")
	flagDisableServer = cmd.Flags().Bool("disable-server", false, "If the management server should be disabled.")
	flagDefaultJobName = cmd.Flags().StringP("name", "n", "", "The job name (will default to a random string of 8 letters).")
	flagDefaultJobSchedule = cmd.Flags().StringP("schedule", "s", "", "The job schedule in cron format (ex: '*/5 * * * *')")
	flagDefaultJobHistoryPath = cmd.Flags().String("history-path", "", "The job history path.")
	flagDefaultJobHistoryDisabled = cmd.Flags().Bool("history-disabled", jobkit.DefaultHistoryDisabled, "If job history should be tracked in memory.")
	flagDefaultJobHistoryPersistenceDisabled = cmd.Flags().Bool("history-persistence-disabled", jobkit.DefaultHistoryPersistenceDisabled, "If job history should be saved to disk.")
	flagDefaultJobHistoryMaxCount = cmd.Flags().Int("history-max-count", 0, "Maximum number of history items to maintain (defaults unbounded).")
	flagDefaultJobHistoryMaxAge = cmd.Flags().Duration("history-max-age", 0, "Maximum age of history items to maintain (defaults unbounded).")
	flagDefaultJobTimeout = cmd.Flags().Duration("timeout", 0, "The job execution timeout as a duration (ex: 5s)")
	flagDefaultJobShutdownGracePeriod = cmd.Flags().Duration("shutdown-grace-period", 0, "The grace period to wait for the job to complete on stop (ex: 5s)")
	flagDefaultJobLabels = cmd.Flags().StringSlice("label", nil, "Labels for the job that can be used with filtering or tagging.")
	flagDefaultJobSkipExpandEnv = cmd.Flags().Bool("skip-expand-env", jobkit.DefaultSkipExpandEnv, "If the job exec should skip expanding environment variables.")
	flagDefaultJobDiscardOutput = cmd.Flags().Bool("discard-output", jobkit.DefaultDiscardOutput, "If jobs should not save console output from the action in job history.")
	flagDefaultJobHideOutput = cmd.Flags().Bool("hide-output", jobkit.DefaultHideOutput, "If jobs should hide console output from the action.")
}

type config struct {
	jobkit.Config `yaml:",inline"`
	DisableServer *bool              `yaml:"disableServer"`
	Jobs          []jobkit.JobConfig `yaml:"jobs"`
}

func (c *config) Resolve() error {
	if len(c.Logger.Flags) == 0 {
		c.Logger.Flags = []string{"all"}
	}
	if err := c.Config.Resolve(); err != nil {
		return err
	}
	return configutil.AnyError(
		configutil.SetString(&c.Title, configutil.String(*flagTitle), configutil.Env("HOSTNAME"), configutil.String(c.Title)),
		configutil.SetString(&c.Web.BindAddr, configutil.String(*flagBind), configutil.Env("BIND_ADDR"), configutil.String(c.Web.BindAddr)),
		configutil.SetBool(&c.DisableServer, configutil.Bool(flagDisableServer), configutil.Bool(c.DisableServer), configutil.Bool(ref.Bool(false))),
		configutil.SetBool(&c.UseViewFiles, configutil.Bool(flagUseViewFiles), configutil.Bool(c.UseViewFiles), configutil.Bool(ref.Bool(false))),
	)
}

type defaultJobConfig struct {
	jobkit.JobConfig
}

func (djc *defaultJobConfig) Resolve() error {
	if *flagDefaultJobLabels != nil && len(*flagDefaultJobLabels) > 0 {
		if djc.Labels == nil {
			djc.Labels = map[string]string{}
		}
		for _, label := range *flagDefaultJobLabels {
			vars, err := env.Parse(strings.TrimSpace(label))
			if err != nil {
				return err
			}
			for key, value := range vars {
				djc.Labels[key] = value
			}
		}
	}
	return configutil.AnyError(
		configutil.SetString(&djc.Name, configutil.String(*flagDefaultJobName), configutil.String(env.Env().ServiceName()), configutil.String(djc.Name), configutil.String(stringutil.Letters.Random(8))),
		configutil.SetString(&djc.Schedule, configutil.String(*flagDefaultJobSchedule), configutil.String(djc.Schedule)),
		configutil.SetBool(&djc.HistoryDisabled, configutil.Bool(flagDefaultJobHistoryDisabled), configutil.Bool(djc.HistoryDisabled), configutil.Bool(ref.Bool(jobkit.DefaultHistoryDisabled))),
		configutil.SetBool(&djc.HistoryPersistenceDisabled, configutil.Bool(flagDefaultJobHistoryPersistenceDisabled), configutil.Bool(djc.HistoryPersistenceDisabled), configutil.Bool(ref.Bool(jobkit.DefaultHistoryPersistenceDisabled))),
		configutil.SetString(&djc.HistoryPath, configutil.String(*flagDefaultJobHistoryPath), configutil.String(djc.HistoryPath)),
		configutil.SetInt(djc.HistoryMaxCount, configutil.Int(*flagDefaultJobHistoryMaxCount), configutil.Int(jobkit.DefaultHistoryMaxCount)),
		configutil.SetDuration(djc.HistoryMaxAge, configutil.Duration(*flagDefaultJobHistoryMaxAge), configutil.Duration(jobkit.DefaultHistoryMaxAge)),
		configutil.SetDuration(&djc.Timeout, configutil.Duration(*flagDefaultJobTimeout), configutil.Duration(djc.Timeout)),
		configutil.SetDuration(&djc.ShutdownGracePeriod, configutil.Duration(*flagDefaultJobShutdownGracePeriod), configutil.Duration(djc.ShutdownGracePeriod)),
		configutil.SetBool(&djc.SkipExpandEnv, configutil.Bool(flagDefaultJobSkipExpandEnv), configutil.Bool(djc.SkipExpandEnv), configutil.Bool(ref.Bool(jobkit.DefaultSkipExpandEnv))),
		configutil.SetBool(&djc.DiscardOutput, configutil.Bool(flagDefaultJobDiscardOutput), configutil.Bool(djc.DiscardOutput), configutil.Bool(ref.Bool(jobkit.DefaultDiscardOutput))),
		configutil.SetBool(&djc.HideOutput, configutil.Bool(flagDefaultJobHideOutput), configutil.Bool(djc.HideOutput), configutil.Bool(ref.Bool(jobkit.DefaultHideOutput))),
	)
}

func command() *cobra.Command {
	return &cobra.Command{
		Use:   "job",
		Short: "Job runs a command on a schedule, and tracks limited job history.",
		Long:  "Job runs a command on a schedule, and tracks limited job history.",
		Example: `
# echo 'hello world' with the default schedule
job -- echo 'hello world'

# echo 'hello world' every 30 seconds
job --schedule='*/30 * * * *' -- echo 'hello world'

# set the job name
job -n echo --schedule='*/30 * * * *' -- echo 'hello world'

# use a config
job -c config.yml'

# where the config can specify multiple jobs.
# it can also set general configuration options like the bind address (located in the web config).
"""
logger:
  flags: [all, -http.request]

web:
  bindAddr: :8080

jobs:
- name: echo
  schedule: '*/30 * * * *'
  exec: [echo, 'hello world']
- name: echo2
  schedule: '*/30 * * * *'
  exec: [echo, 'hello again']
"""
`,
	}
}

func main() {
	cmd := command()
	initFlags(cmd)
	cmd.Run = fatalExit(run)
	if err := cmd.Execute(); err != nil {
		logger.FatalExit(err)
	}
}

func fatalExit(action func(*cobra.Command, []string) error) func(*cobra.Command, []string) {
	return func(parent *cobra.Command, args []string) {
		if err := action(parent, args); err != nil {
			logger.FatalExit(err)
		}
	}
}

type configUpdater struct {
	Log    logger.Log
	Config *config
}

func (cu *configUpdater) Start() error {
	return nil
}

func (cu *configUpdater) Stop() error {
	return nil
}

func (cu *configUpdater) Update() error {
	if _, err := configutil.Read(cu.Config, configutil.OptPaths(*flagConfigPath)); !configutil.IsIgnored(err) {
		return err
	}
	return nil
}

func run(cmd *cobra.Command, args []string) error {
	var cfg config
	if _, err := configutil.Read(&cfg, configutil.OptPaths(*flagConfigPath)); !configutil.IsIgnored(err) {
		return err
	}

	log, err := logger.New(
		logger.OptConfig(cfg.Logger),
		logger.OptPath(cfg.TitleOrDefault()),
	)
	if err != nil {
		return err
	}

	log.Debugf("using logger flags: %v", log.Flags.String())
	log.Debugf("using logger format: %v", cfg.Logger.FormatOrDefault())

	if len(args) > 0 {
		defaultJobCfg, err := createDefaultJobConfig(args...)
		if err != nil {
			return err
		}
		log.Debugf("using default job: %s", strings.Join(defaultJobCfg.Exec, " "))
		cfg.Jobs = append(cfg.Jobs, defaultJobCfg.JobConfig)
	}

	if len(cfg.Jobs) == 0 {
		return ex.New("must supply a command to run with `--exec=...` or `-- command`), or provide a jobs config file")
	}

	// set up myriad of notification targets
	var emailClient email.Sender
	if !cfg.SMTP.IsZero() {
		emailClient = cfg.SMTP
		log.Infof("adding smtp email notifications")
	}

	if !cfg.EmailDefaults.IsZero() {
		log.Debugf("using email defaults from: %s", cfg.EmailDefaults.From)
		log.Debugf("using email defaults to: %s", stringutil.CSV(cfg.EmailDefaults.To))
	}

	var slackClient slack.Sender
	if !cfg.Slack.IsZero() {
		slackClient = slack.New(cfg.Slack)
		log.Infof("adding slack notifications")
	}
	var statsClient stats.Collector
	if !cfg.Datadog.IsZero() {
		statsClient, err = datadog.New(cfg.Datadog)
		if err != nil {
			return err
		}
		log.Infof("adding datadog metrics")
	}
	var sentryClient sentry.Sender
	if !cfg.Sentry.IsZero() {
		sentryClient, err = sentry.New(cfg.Sentry)
		if err != nil {
			return err
		}
		log.Listen(logger.Error, "sentry", logger.NewErrorEventListener(sentryClient.Notify))
		log.Listen(logger.Fatal, "sentry", logger.NewErrorEventListener(sentryClient.Notify))
		log.Infof("adding sentry error collection")
	}

	jobs := cron.New(
		cron.OptLog(log.WithPath("cron")),
	)

	for _, jobCfg := range cfg.Jobs {
		job, err := createJobFromConfig(cfg, jobCfg, jobs.Log)
		if err != nil {
			return err
		}

		job.EmailClient = emailClient
		job.SlackClient = slackClient
		job.StatsClient = statsClient
		job.SentryClient = sentryClient

		enabled := ansi.ColorGreen.Apply("enabled")
		disabled := ansi.ColorRed.Apply("disabled")
		log.Infof("loading job `%s` with exec: %s", jobCfg.Name, ansi.ColorLightWhite.Apply(strings.Join(jobCfg.Exec, " ")))
		log.Infof("loading job `%s` with schedule: %s", jobCfg.Name, ansi.ColorLightWhite.Apply(jobCfg.ScheduleOrDefault()))
		if !jobCfg.HistoryDisabledOrDefault() && !jobCfg.HistoryPersistenceDisabledOrDefault() {
			log.Infof("loading job `%s` with history: %v and persistence: %v to output path: %s", jobCfg.Name, enabled, enabled, ansi.ColorLightWhite.Apply(jobCfg.HistoryPathOrDefault()))
		} else if !jobCfg.HistoryDisabledOrDefault() {
			log.Infof("loading job `%s` with history: %v and persistence: %v", jobCfg.Name, enabled, disabled)
		} else {
			log.Infof("loading job `%s` with history: %v", jobCfg.Name, disabled)
		}
		if !jobCfg.HistoryDisabledOrDefault() {
			if jobCfg.HistoryMaxCountOrDefault() > 0 {
				maxCount := ansi.ColorLightWhite.Apply(fmt.Sprint(jobCfg.HistoryMaxCountOrDefault()))
				log.Infof("loading job `%s` with history max count: %s", jobCfg.Name, maxCount)
			} else {
				log.Infof("loading job `%s` with history max count: %s", jobCfg.Name, disabled)
			}
			if jobCfg.HistoryMaxAgeOrDefault() > 0 {
				maxAge := ansi.ColorLightWhite.Apply(fmt.Sprint(jobCfg.HistoryMaxAgeOrDefault()))
				log.Infof("loading job `%s` with history max age: %s", jobCfg.Name, maxAge)
			} else {
				log.Infof("loading job `%s` with history max age: %s", jobCfg.Name, disabled)
			}
		}
		if err = jobs.LoadJobs(job); err != nil {
			log.Error(err)
		}
	}

	hosted := []graceful.Graceful{jobs}
	if !*flagDisableServer {
		ws := jobkit.NewServer(jobs, cfg.Config)
		if cfg.Config.UseViewFilesOrDefault() {
			log.Debugf("using view files loaded from disk")
		}
		ws.Log = log.WithPath("management server")
		hosted = append(hosted, ws)
	} else {
		log.Infof("management server disabled")
	}
	return graceful.Shutdown(hosted...)
}

func createDefaultJobConfig(args ...string) (*defaultJobConfig, error) {
	cfg := new(defaultJobConfig)
	if err := cfg.Resolve(); err != nil {
		return nil, err
	}
	cfg.Exec = args
	return cfg, nil
}

func createJobFromConfig(base config, cfg jobkit.JobConfig, log logger.Log) (*jobkit.Job, error) {
	if len(cfg.Exec) == 0 {
		return nil, ex.New("job exec and command unset", ex.OptMessagef("job: %s", cfg.Name))
	}
	action := jobkit.NewShellAction(cfg.Exec,
		jobkit.OptShellActionConfig(cfg.ShellActionConfig),
	)
	job, err := jobkit.NewJob(
		cron.NewJob(
			cron.OptJobName(cfg.Name),
			cron.OptJobAction(action.Execute),
			cron.OptJobConfig(cfg.JobConfig),
		),
		jobkit.OptJobConfig(cfg),
		jobkit.OptJobParsedSchedule(cfg.Schedule),
		jobkit.OptJobLog(log),
	)
	if err != nil {
		return nil, err
	}
	job.EmailDefaults = email.MergeMessages(base.EmailDefaults, cfg.Notifications.Email)
	job.SlackDefaults = cfg.Notifications.Slack
	job.WebhookDefaults = cfg.Notifications.Webhook
	return job, nil
}
