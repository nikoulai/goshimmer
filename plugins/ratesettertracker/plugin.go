package ratesettertracker

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
)

// PluginName is the name of the spammer plugin.
const PluginName = "RateSetterTracker"

// MessageTime holds info about when the message was submitted and issued.
type MessageTime struct {
	SubmittedAt time.Time
	IssuedAt    time.Time
	DiscardedAt time.Time
	Size        int
}

var (
	// plugin is the plugin instance of the spammer plugin.
	plugin                  *node.Plugin
	once                    sync.Once
	log                     *logger.Logger
	messageTimeMap          map[tangle.MessageID]*MessageTime
	ticks                   []time.Time
	messageSubmittedClosure *events.Closure
	messageIssuedClosure    *events.Closure
	messageDiscardedClosure *events.Closure
	tickedClosure           *events.Closure
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
	messageSubmittedClosure = events.NewClosure(func(messageID tangle.MessageID) {
		messageTimeMap[messageID] = &MessageTime{
			SubmittedAt: time.Now(),
			IssuedAt:    time.Time{},
			DiscardedAt: time.Time{},
			Size:        messagelayer.Tangle().RateSetter.Size(),
		}
	})
	messageIssuedClosure = events.NewClosure(func(message *tangle.Message) {
		if _, ok := messageTimeMap[message.ID()]; !ok {
			log.Info("something went wrong")
			return
		}
		messageTimeMap[message.ID()].IssuedAt = time.Now()
	})
	messageDiscardedClosure = events.NewClosure(func(messageID tangle.MessageID) {
		if _, ok := messageTimeMap[messageID]; !ok {
			messageTimeMap[messageID] = &MessageTime{
				DiscardedAt: time.Now(),
				SubmittedAt: time.Time{},
				IssuedAt:    time.Time{},
				Size:        messagelayer.Tangle().RateSetter.Size(),
			}
		}
	})
	tickedClosure = events.NewClosure(func(_ tangle.MessageID) {
		ticks = append(ticks, time.Now())
	})
	webapi.Server().GET("ratesettertracker", handleRequest)
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("RateSetterTracker", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		stop()
	}, shutdown.PrioritySpammer); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func start() {
	messagelayer.Tangle().RateSetter.Events.MessageSubmitted.Attach(messageSubmittedClosure)
	messagelayer.Tangle().RateSetter.Events.MessageDiscarded.Attach(messageDiscardedClosure)
	messagelayer.Tangle().RateSetter.Events.MessageIssued.Attach(messageIssuedClosure)
	messagelayer.Tangle().RateSetter.Events.Ticked.Attach(tickedClosure)
}

func stop() {
	messagelayer.Tangle().RateSetter.Events.MessageSubmitted.Detach(messageSubmittedClosure)
	messagelayer.Tangle().RateSetter.Events.MessageDiscarded.Detach(messageDiscardedClosure)
	messagelayer.Tangle().RateSetter.Events.MessageIssued.Detach(messageIssuedClosure)
	messagelayer.Tangle().RateSetter.Events.Ticked.Detach(tickedClosure)
}
