package chat

import (
	"strconv"
	"sync"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

const (
	// PluginName contains the human readable name of the plugin.
	PluginName = "Chat"
)

var (
	// App is the "plugin" instance of the chat application.
	app  *node.Plugin
	once sync.Once
)

// App gets the plugin instance.
func App() *node.Plugin {
	once.Do(func() {
		app = node.NewPlugin(PluginName, node.Enabled, configure)
	})
	return app
}

func configure(_ *node.Plugin) {
	messagelayer.Tangle().Booker.Events.MessageBooked.Attach(events.NewClosure(onReceiveMessageFromMessageLayer))
	// we can also attach this event at some other part for instance
	// `messagelayer.Tangle().TipManager.Events.TipAdded.Attach(events.NewClosure(onReceiveMessageFromMessageLayer))`
	configureWebAPI()
}

func onReceiveMessageFromMessageLayer(messageID tangle.MessageID) {
	var chatEvent *ChatEvent
	messagelayer.Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
		if message.Payload().Type() != Type {
			return
		}

		chatPayload, _, err := FromBytes(message.Payload().Bytes())
		if err != nil {
			app.LogError(err)
			return
		}

		chatEvent = &ChatEvent{
			From:      chatPayload.From,
			To:        chatPayload.To,
			Message:   chatPayload.Message,
			Timestamp: message.IssuingTime(),
			MessageID: message.ID().Base58(),
		}
	})

	if chatEvent == nil {
		return
	}
	go func() {
		if chatEvent.Message == "tips" {
			strongTips := messagelayer.Tangle().TipManager.AllStrongTips()
			numberTips := len(strongTips)
			ReponseChat := NewPayload("", "", strconv.Itoa(numberTips))
			messagelayer.Tangle().IssuePayload(ReponseChat)
		}
		if chatEvent.Message == "4" {
			ReponseChat := NewPayload("", "", "increase number of parents to 4 (if possible)")
			messagelayer.Tangle().IssuePayload(ReponseChat, 4)
		}
	}()
	Events.MessageReceived.Trigger(chatEvent)
}
