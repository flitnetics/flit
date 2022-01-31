package cmd

import (
	"math"
	"os"
	"syscall"
	"os/signal"
	"strconv"
	"strings"
	"time"
        "encoding/json"
        "bytes"
        "net/http"
        "fmt"

        "github.com/jinzhu/configor"
	apiMetrics "github.com/containrrr/watchtower/pkg/api/metrics"
	"github.com/containrrr/watchtower/pkg/api/update"
        "github.com/gojektech/heimdall/v6/httpclient"

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

        mqtt "github.com/eclipse/paho.mqtt.golang"
        "github.com/google/uuid"

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

var Config = struct {
	Client struct {
		Location_Id     string `default:"all"`
                Ui_Url          string `default:"localhost"`
                Enable_Ui       bool   `default:"false"`
                Organization    string `required:"true"`
		Mqtt struct {
                        Broker          string  `required:"true"`
                        Port            int  `required:"true"`
                        Username        string
                        Password        string
                }
	}
}{}

type Payload struct {
      Action string `json:"action"`
      LocationId string `json:"locationId"`
      Image []actions.Image `json:"images,omitempty"`
      ImageError string `json:"error,omitempty"`
      LoggingInfo []actions.LoggingInfo `json:"logs,omitempty"`
}

type Agent struct {
   Payload Payload `json:"agent"`
}

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
                if err := configor.Load(&Config, "config.yaml"); err != nil {
                        panic(err)
                }
                configor.Load(&Config, "config.yaml")
                log.Info("Our location id: ", Config.Client.Location_Id)

		notifs := "Using no notifications"
		notifList := notifier.String()
		if len(notifList) > 0 {
			notifs = "Using notifications: " + notifList
		}

		log.Info("Watchtower ", version, "\n", notifs, "\n", filtering)
	}
}

func runUpdates(client container.Client, updateParams t.UpdateParams) {
        _, err := actions.Update(client, updateParams)
        if err != nil {
                 log.Println(err)
        }
}

func runList() []actions.Image {
        containers, err := actions.List()
        if err != nil {
                 log.Println(err)
        }

        return containers
}

func runLogs() []actions.LoggingInfo {
        containers, err := actions.Logs()
        if err != nil {
                 log.Println(err)
        }

        return containers
}

func sendToUI(json []byte, endpoint string) {
                timeout := 1000 * time.Millisecond
                client := httpclient.NewClient(httpclient.WithHTTPTimeout(timeout))

                headers := http.Header{}
                headers.Set("Content-Type", "application/json")
                body := bytes.NewReader([]byte(string(json)))

                // Use the clients GET method to create and execute the request
                _, err := client.Post(fmt.Sprintf("%s/api/v1/agents/%s", Config.Client.Ui_Url, endpoint), body, headers)
                if err != nil{
                       log.Error(err)
                }

}

func listenMqtt(filter t.Filter) {
    if err := configor.Load(&Config, "config.yaml"); err != nil {
            panic(err)
    }

    configor.Load(&Config, "config.yaml")

    fmt.Println(fmt.Sprintf("broker: %s", Config.Client.Mqtt.Broker))
    fmt.Println(fmt.Sprintf("port: %d", Config.Client.Mqtt.Port))

    opts := mqtt.NewClientOptions()
    opts.AddBroker(fmt.Sprintf("tcp://%s:%d", Config.Client.Mqtt.Broker, Config.Client.Mqtt.Port))
    opts.SetClientID(uuid.New().String())
    opts.SetDefaultPublishHandler(messagePubHandler(filter)) // needed for subscriber
    opts.SetUsername(Config.Client.Mqtt.Username)
    opts.SetPassword(Config.Client.Mqtt.Password)
    opts.OnConnect = connectHandler
    opts.OnConnectionLost = connectLostHandler
    client := mqtt.NewClient(opts)
    if token := client.Connect(); token.Wait() && token.Error() != nil {
        panic(token.Error())

    }

    // subscribe (listen) to MQTT
    sub(client)
}

func runUpgradesOnSchedule(c *cobra.Command, filter t.Filter, filtering string) error {
	// keepalive code to ensure mqtt do not exit after running
        keepAlive := make(chan os.Signal)
        signal.Notify(keepAlive, os.Interrupt, syscall.SIGTERM)

        t := time.Date(0001, 1, 1, 00, 00, 00, 00, time.UTC) // need cleanup removal of this
        writeStartupMessage(c, t, filtering)

        // listen for MQTT traffic
        listenMqtt(filter)

        <-keepAlive
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

func messagePubHandler(filter t.Filter) func(mqtt.Client, mqtt.Message) {
    //fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())

    return func(client mqtt.Client, msg mqtt.Message) {
            processMsg(msg, filter)
    }
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
    fmt.Println("Connected")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
    fmt.Printf("Connect lost: %v", err)
}

// we process what we need to do, updates or ping
func processMsg(msg mqtt.Message, filter t.Filter) {
    if err := configor.Load(&Config, "config.yaml"); err != nil {
            panic(err)
    }

    configor.Load(&Config, "config.yaml")

    updateParams := t.UpdateParams{
    //        Filter:         filter,
            Cleanup:        cleanup,
            NoRestart:      noRestart,
            Timeout:        timeout,
            MonitorOnly:    monitorOnly,
            LifecycleHooks: lifecycleHooks,
            RollingRestart: rollingRestart,
    }

    message := string(msg.Payload())
    // location Id
    location := Config.Client.Location_Id
    // Web UI Config
    ui := Config.Client.Enable_Ui

    payload := Payload{}
    json.Unmarshal([]byte(message), &payload)

    // "update image" action
    if ((payload.LocationId == location || location == "all") && payload.Action == "update") {
            runUpdates(client, updateParams)
    }

    // "ping" action
    if ((payload.LocationId == location || location == "all") && payload.Action == "ping") {
            images := runList()
            logs := runLogs()

            var mapD Agent
            var mapB []byte
            mapD = Agent{Payload: Payload{Action: "pong", LocationId: location, ImageError: "Containers are not running"}}
            mapB, _ = json.Marshal(mapD)
 
            // if there are no container images
            if (images != nil) {
                    mapD = Agent{Payload: Payload{Action: "pong", LocationId: location, Image: images, LoggingInfo: logs}}
                    mapB, _ = json.Marshal(mapD)
            }

            // if we enable the UI
            if (ui == true) {
                   sendToUI(mapB, "pong")
            }
    }

    fmt.Println(payload.Action)
}

func sub(client mqtt.Client) {
    configor.Load(&Config, "config.yaml")

    topic := fmt.Sprintf("%s/%s", Config.Client.Organization, Config.Client.Location_Id)
    token := client.Subscribe(topic, 1, nil)
    token.Wait()
    fmt.Printf("Subscribed to topic: %s\n", topic)
}
