package registrations

import (
	"github.com/iotaledger/goshimmer/packages/registry"
	txwrapped "github.com/iotaledger/goshimmer/packages/transaction_wrapped"
)

func init() {
	registry.Manager.RegisterType(&txwrapped.ED25519Address{})
	registry.Manager.RegisterType(&txwrapped.ED25519Signature{})
	registry.Manager.RegisterType(&txwrapped.UTXOInput{})
	registry.Manager.RegisterType(&txwrapped.SignatureUnlockBlock{})
	registry.Manager.RegisterType(&txwrapped.SigLockedSingleOutput{})
	//registry.Manager.RegisterType(txwrapped.Color{})
	//registry.Manager.RegisterType(uint64(0))
}
