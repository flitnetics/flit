package cmd

import (
	"math"
	"os"
	_ "os/signal"
	"strconv"
	"strings"
	_ "syscall"
	"time"

        "github.com/r3labs/sse/v2"
	apiMetrics "github.com/containrrr/watchtower/pkg/api/metrics"
	"github.com/containrrr/watchtower/pkg/api/update"

	"github.com/containrrr/watchtower/internal/actions"
	"github.com/containrrr/watchtower/internal/flags"
	"github.com/containrrr/watchtower/pkg/api"
	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/filters"
	"github.com/containrrr/watchtower/pkg/metrics"
	"github.com/containrrr/watchtower/pkg/notifications"
	t "github.com/containrrr/watchtower/pkg/types"
	_ "github.com/robfig/cron"
	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

var (
	client         container.Client
	scheduleSpec   string
	cleanup        bool
	noRestart      bool
	monitorOnly    bool
	enableLabel    bool
	notifier       *notifications.Notifier
	timeout        time.Duration
	lifecycleHooks bool
	rollingRestart bool
	scope          string
	// Set on build using ldflags
	version = "v0.0.0-unknown"
)

var rootCmd = NewRootCommand()

// NewRootCommand creates the root command for watchtower
func NewRootCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "watchtower",
		Short: "Automatically updates running Docker containers",
		Long: `
	Watchtower automatically updates running Docker containers whenever a new image is released.
	More information available at https://github.com/containrrr/watchtower/.
	`,
		Run:    Run,
		PreRun: PreRun,
	}
}

func init() {
	flags.SetDefaults()
	flags.RegisterDockerFlags(rootCmd)
	flags.RegisterSystemFlags(rootCmd)
	flags.RegisterNotificationFlags(rootCmd)
}

// Execute the root func and exit in case of errors
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// PreRun is a lifecycle hook that runs before the command is executed.
func PreRun(cmd *cobra.Command, _ []string) {
	f := cmd.PersistentFlags()

	if enabled, _ := f.GetBool("no-color"); enabled {
		log.SetFormatter(&log.TextFormatter{
			DisableColors: true,
		})
	} else {
		// enable logrus built-in support for https://bixense.com/clicolors/
		log.SetFormatter(&log.TextFormatter{
			EnvironmentOverrideColors: true,
		})
	}

	if enabled, _ := f.GetBool("debug"); enabled {
		log.SetLevel(log.DebugLevel)
	}
	if enabled, _ := f.GetBool("trace"); enabled {
		log.SetLevel(log.TraceLevel)
	}

	pollingSet := f.Changed("interval")
	schedule, _ := f.GetString("schedule")
	cronLen := len(schedule)

	if pollingSet && cronLen > 0 {
		log.Fatal("Only schedule or interval can be defined, not both.")
	} else if cronLen > 0 {
		scheduleSpec, _ = f.GetString("schedule")
	} else {
		interval, _ := f.GetInt("interval")
		scheduleSpec = "@every " + strconv.Itoa(interval) + "s"
	}

	flags.GetSecretsFromFiles(cmd)
	cleanup, noRestart, monitorOnly, timeout = flags.ReadFlags(cmd)

	if timeout < 0 {
		log.Fatal("Please specify a positive value for timeout value.")
	}

	enableLabel, _ = f.GetBool("label-enable")
	lifecycleHooks, _ = f.GetBool("enable-lifecycle-hooks")
	rollingRestart, _ = f.GetBool("rolling-restart")
	scope, _ = f.GetString("scope")

	log.Debug(scope)

	// configure environment vars for client
	err := flags.EnvConfig(cmd)
	if err != nil {
		log.Fatal(err)
	}

	noPull, _ := f.GetBool("no-pull")
	includeStopped, _ := f.GetBool("include-stopped")
	includeRestarting, _ := f.GetBool("include-restarting")
	reviveStopped, _ := f.GetBool("revive-stopped")
	removeVolumes, _ := f.GetBool("remove-volumes")

	if monitorOnly && noPull {
		log.Warn("Using `WATCHTOWER_NO_PULL` and `WATCHTOWER_MONITOR_ONLY` simultaneously might lead to no action being taken at all. If this is intentional, you may safely ignore this message.")
	}

	client = container.NewClient(
		!noPull,
		includeStopped,
		reviveStopped,
		removeVolumes,
		includeRestarting,
	)

	notifier = notifications.NewNotifier(cmd)
}

// Run is the main execution flow of the command
func Run(c *cobra.Command, names []string) {
	filter, filterDesc := filters.BuildFilter(names, enableLabel, scope)
	runOnce, _ := c.PersistentFlags().GetBool("run-once")
	enableUpdateAPI, _ := c.PersistentFlags().GetBool("http-api-update")
	enableMetricsAPI, _ := c.PersistentFlags().GetBool("http-api-metrics")

	apiToken, _ := c.PersistentFlags().GetString("http-api-token")

	if runOnce {
		writeStartupMessage(c, time.Time{}, filterDesc)
		runUpdatesWithNotifications(filter)
		notifier.Close()
		os.Exit(0)
		return
	}

	if err := actions.CheckForMultipleWatchtowerInstances(client, cleanup, scope); err != nil {
		log.Fatal(err)
	}

	httpAPI := api.New(apiToken)

	if enableUpdateAPI {
		updateHandler := update.New(func() { runUpdatesWithNotifications(filter) })
		httpAPI.RegisterFunc(updateHandler.Path, updateHandler.Handle)
	}

	if enableMetricsAPI {
		metricsHandler := apiMetrics.New()
		httpAPI.RegisterHandler(metricsHandler.Path, metricsHandler.Handle)
	}

	if err := httpAPI.Start(enableUpdateAPI); err != nil {
		log.Error("failed to start API", err)
	}

	if err := runUpgradesOnSchedule(c, filter, filterDesc); err != nil {
		log.Error(err)
	}

	os.Exit(1)
}

func formatDuration(d time.Duration) string {
	sb := strings.Builder{}

	hours := int64(d.Hours())
	minutes := int64(math.Mod(d.Minutes(), 60))
	seconds := int64(math.Mod(d.Seconds(), 60))

	if hours == 1 {
		sb.WriteString("1 hour")
	} else if hours != 0 {
		sb.WriteString(strconv.FormatInt(hours, 10))
		sb.WriteString(" hours")
	}

	if hours != 0 && (seconds != 0 || minutes != 0) {
		sb.WriteString(", ")
	}

	if minutes == 1 {
		sb.WriteString("1 minute")
	} else if minutes != 0 {
		sb.WriteString(strconv.FormatInt(minutes, 10))
		sb.WriteString(" minutes")
	}

	if minutes != 0 && (seconds != 0) {
		sb.WriteString(", ")
	}

	if seconds == 1 {
		sb.WriteString("1 second")
	} else if seconds != 0 || (hours == 0 && minutes == 0) {
		sb.WriteString(strconv.FormatInt(seconds, 10))
		sb.WriteString(" seconds")
	}

	return sb.String()
}

func writeStartupMessage(c *cobra.Command, sched time.Time, filtering string) {
	if noStartupMessage, _ := c.PersistentFlags().GetBool("no-startup-message"); !noStartupMessage {
		schedMessage := "Running a one time update."
		if !sched.IsZero() {
			until := formatDuration(time.Until(sched))
			schedMessage = "Scheduling first run: " + sched.Format("2006-01-02 15:04:05 -0700 MST") +
				"\nNote that the first check will be performed in " + until
		}

		notifs := "Using no notifications"
		notifList := notifier.String()
		if len(notifList) > 0 {
			notifs = "Using notifications: " + notifList
		}

		log.Info("Watchtower ", version, "\n", notifs, "\n", filtering, "\n", schedMessage)
	}
}

func listenSSE(filter t.Filter) {
        sseClient := sse.NewClient("http://localhost:8080/events")
        sseClient.EncodingBase64 = true

        sseClient.Subscribe("messages", func(msg *sse.Event) {
                updateParams := t.UpdateParams{
                        Filter:         filter,
                        Cleanup:        cleanup,
                        NoRestart:      noRestart,
                        Timeout:        timeout,
                        MonitorOnly:    monitorOnly,
                        LifecycleHooks: lifecycleHooks,
                        RollingRestart: rollingRestart,
                }

                _, err := actions.Update(client, updateParams)
                if err != nil {
                         log.Println(err)
                }
        })
}

func runUpgradesOnSchedule(c *cobra.Command, filter t.Filter, filtering string) error {
	listenSSE(filter)
	return nil
}

func runUpdatesWithNotifications(filter t.Filter) *metrics.Metric {
	notifier.StartNotification()
	updateParams := t.UpdateParams{
		Filter:         filter,
		Cleanup:        cleanup,
		NoRestart:      noRestart,
		Timeout:        timeout,
		MonitorOnly:    monitorOnly,
		LifecycleHooks: lifecycleHooks,
		RollingRestart: rollingRestart,
	}
	metricResults, err := actions.Update(client, updateParams)
	if err != nil {
		log.Println(err)
	}
	notifier.SendNotification()
	return metricResults
}
