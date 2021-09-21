package txwrapped_test

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/registry"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"

	. "github.com/iotaledger/goshimmer/packages/transaction_wrapped"
)

var (
	color1 = Color{1}
	color2 = Color{2}
)

type wallet struct {
	keyPair ed25519.KeyPair
	address *ED25519Address
}

func (w wallet) privateKey() ed25519.PrivateKey {
	return w.keyPair.PrivateKey
}

func (w wallet) publicKey() ed25519.PublicKey {
	return w.keyPair.PublicKey
}

func createWallets(n int) []wallet {
	wallets := make([]wallet, n)
	for i := 0; i < n; i++ {
		kp := ed25519.GenerateKeyPair()
		wallets[i] = wallet{
			kp,
			NewED25519Address(kp.PublicKey),
		}
	}
	return wallets
}

func (w wallet) sign(txEssence *TransactionEssence) *ED25519Signature {
	essenceBytes, err := registry.Manager.Serialize(txEssence)
	if err != nil {
		panic(nil)
	}
	return NewED25519Signature(w.publicKey(), w.privateKey().Sign(essenceBytes))
}

func (w wallet) unlockBlocks(txEssence *TransactionEssence) []UnlockBlock {
	unlockBlock := NewSignatureUnlockBlock(w.sign(txEssence))
	unlockBlocks := make([]UnlockBlock, len(txEssence.Inputs()))
	for i := range txEssence.Inputs() {
		unlockBlocks[i] = unlockBlock
	}
	return unlockBlocks
}

func generateOutput(address Address, index uint16) *SigLockedSingleOutput {
	output := NewSigLockedSingleOutput(100, address)
	output.SetID(NewOutputID(GenesisTransactionID, index))

	return output
}

func generateOutputs(address Address, numOutputs int) (outputs []*SigLockedSingleOutput) {
	outputs = make([]*SigLockedSingleOutput, numOutputs)
	for i := 0; i < numOutputs; i++ {
		outputs[i] = NewSigLockedSingleOutput(100, address)
		outputs[i].SetID(NewOutputID(GenesisTransactionID, uint16(i)))
		i++
	}

	return
}

func singleInputTransaction(a, b wallet, outputToSpend *SigLockedSingleOutput) (*Transaction, *SigLockedSingleOutput) {
	input := NewUTXOInput(outputToSpend.ID())
	output := NewSigLockedSingleOutput(100, b.address)

	txEssence := NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, NewInputs(input), NewOutputs(output))

	tx := NewTransaction(txEssence, a.unlockBlocks(txEssence))

	return tx, output
}

func multipleInputsTransaction(a, b wallet, outputsToSpend []*SigLockedSingleOutput) *Transaction {
	inputs := make(Inputs, len(outputsToSpend))
	for i, outputToSpend := range outputsToSpend {
		inputs[i] = NewUTXOInput(outputToSpend.ID())
	}

	output := NewSigLockedSingleOutput(100, b.address)

	txEssence := NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, inputs, NewOutputs(output))

	tx := NewTransaction(txEssence, a.unlockBlocks(txEssence))
	return tx
}

func buildTransaction(a, b wallet, outputsToSpend []*SigLockedSingleOutput) *Transaction {
	inputs := make(Inputs, len(outputsToSpend))
	sum := uint64(0)
	for i, outputToSpend := range outputsToSpend {
		inputs[i] = NewUTXOInput(outputToSpend.ID())
		outputToSpend.Balances().ForEach(func(color Color, balance uint64) bool {
			if color == ColorIOTA {
				sum += balance
			}

			return true
		})
	}

	output := NewSigLockedSingleOutput(sum, b.address)

	txEssence := NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, inputs, NewOutputs(output))

	tx := NewTransaction(txEssence, a.unlockBlocks(txEssence))

	return tx
}
