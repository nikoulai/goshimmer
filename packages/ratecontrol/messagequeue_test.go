package ratecontrol

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

func TestMessageQueue_Queue(t *testing.T) {
	m := NewMessageQueue(WithMaxSize(2))

	m.Queue(newTestMessage(t, "Message1", 10))
	m.Queue(newTestMessage(t, "Message2", 5))
	m.Queue(newTestMessage(t, "Message3", 12))

	fmt.Println(m.Size())

	fmt.Println(m.Poll(false))
	fmt.Println(m.Poll(false))
	fmt.Println(m.Poll(false))
}

func newTestMessage(t *testing.T, alias string, priority uint64) *Message {
	issuerPublicKey, issuerPrivateKey, err := ed25519.GenerateKey()
	require.NoError(t, err)

	message, err := tangle.NewMessage(
		tangle.MessageIDs{tangle.EmptyMessageID},
		nil,
		nil,
		nil,
		time.Now(),
		issuerPublicKey,
		0,
		payload.NewGenericDataPayload([]byte(alias)),
		0,
		issuerPrivateKey.Sign([]byte("")),
	)
	require.NoError(t, err)

	tangle.RegisterMessageIDAlias(message.ID(), t.Name() + "_" + alias)

	return &Message{
		Message:  message,
		Priority: priority,
	}
}