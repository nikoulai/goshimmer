package nodeqsizetracker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// PluginName is the name of the spammer plugin.
const PluginName = "NodeQSizeTracker"

var (
	// plugin is the plugin instance of the spammer plugin.
	plugin       *node.Plugin
	once         sync.Once
	log          *logger.Logger
	closure      *events.Closure
	nodeQSizeMap map[int64]map[identity.ID]int
	getQSize     chan map[identity.ID]int
	stopChan     chan struct{}
	wg           sync.WaitGroup
	deps         = new(dependencies)
)

type dependencies struct {
	dig.In

	Tangle *tangle.Tangle
	Server *echo.Echo
	Local  *peer.Local
}

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)
	})
	return plugin
}

func configure(plugin *node.Plugin) {
	stopChan = make(chan struct{})
	getQSize = make(chan map[identity.ID]int, 1024)
	closure = events.NewClosure(func() {
		getQSize <- deps.Tangle.Scheduler.NodeQueueSizes()
	})
	deps.Server.GET("nodeqsizetracker", handleRequest)
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("nodeqsizetracker", func(ctx context.Context) {
		<-ctx.Done()

		stop()
	}, shutdown.PrioritySpammer); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func start() {
	nodeQSizeMap = make(map[int64]map[identity.ID]int)
	stopChan = make(chan struct{})
	getQSize = make(chan map[identity.ID]int, 1024)
	deps.Tangle.Scheduler.Events.SchedulerTicked.Attach(closure)
	wg.Add(1)

	go func() {
		defer close(getQSize)
		defer wg.Done()
		for {
			select {
			case nodeQSizes := <-getQSize:
				now := time.Now()
				nodeQSizeMap[now.UnixNano()] = nodeQSizes
			case <-stopChan:
				return
			}
		}
	}()
}

func stop() {
	deps.Tangle.Scheduler.Events.SchedulerTicked.Detach(closure)
	close(stopChan)
	wg.Wait()
}

var nodeQSizeTableDescription = []string{
	"ID",
	"IssuerID",
	"Timestamp",
	"QSize",
}

func nodeQToCSVRow(nodeID, issuerID string, timestamp int64, size int) []string {
	return []string{
		nodeID,
		issuerID,
		fmt.Sprint(timestamp),
		fmt.Sprint(size),
	}
}
