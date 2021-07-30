package nodeqsizetracker

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
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
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PluginName)
	stopChan = make(chan struct{})
	getQSize = make(chan map[identity.ID]int, 1024)
	closure = events.NewClosure(func() {
		getQSize <- messagelayer.Tangle().Scheduler.NodeQueueSizes()
	})
	webapi.Server().GET("nodeqsizetracker", handleRequest)
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("nodeqsizetracker", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal

		stop()
	}, shutdown.PrioritySpammer); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func start() {
	nodeQSizeMap = make(map[int64]map[identity.ID]int)
	stopChan = make(chan struct{})
	getQSize = make(chan map[identity.ID]int, 1024)
	messagelayer.Tangle().Scheduler.Events.SchedulerTicked.Attach(closure)

	go func() {
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
	messagelayer.Tangle().Scheduler.Events.SchedulerTicked.Detach(closure)
	close(getQSize)
	close(stopChan)
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
