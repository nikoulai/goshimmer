package txwrapped_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/iotaledger/goshimmer/packages/registry/registrations"
	. "github.com/iotaledger/goshimmer/packages/transaction_wrapped"

	registry "github.com/iotaledger/goshimmer/packages/registry"
)

var sampleColor = Color{2}

type A struct {
	Inputs  Inputs  `serialize:"true"`
	Outputs Outputs `serialize:"true"`
}

func TestWhatever(t *testing.T) {
	wallets := createWallets(2)
	input := generateOutput(wallets[0].address, 0)
	a := &A{
		Inputs: Inputs{
			NewUTXOInput(input.ID()),
		},
		Outputs: Outputs{
			generateOutput(wallets[0].address, 0),
		},
	}

	serializedBytes, err := registry.Manager.Serialize(a)
	assert.NoError(t, err)

	otherA := &A{}
	require.NoError(t, registry.Manager.Deserialize(otherA, serializedBytes))
}

var result []byte
var resultDeser *Transaction

func BenchmarkTransactionSerialization(b *testing.B) {
	wallets := createWallets(2)
	input := generateOutput(wallets[0].address, 0)
	tx, _ := singleInputTransaction(wallets[0], wallets[1], input)
	var bytes []byte
	var err error
	for n := 0; err == nil && n < b.N; n++ {
		bytes, err = registry.Manager.Serialize(tx)
	}

	t := &Transaction{}
	err = registry.Manager.Deserialize(t, bytes)
	require.NoError(b, err)
	result = bytes
	resultDeser = t
	assert.Equal(b, t.ID(), tx.ID())
}

func BenchmarkTransactionReflectionDeserialization(b *testing.B) {
	wallets := createWallets(2)
	input := generateOutput(wallets[0].address, 0)
	tx, _ := singleInputTransaction(wallets[0], wallets[1], input)
	var bytes []byte
	var err error
	bytes, err = registry.Manager.Serialize(tx)

	t := &Transaction{}
	for n := 0; err == nil && n < b.N; n++ {
		t = &Transaction{}
		err = registry.Manager.Deserialize(t, bytes)
	}
	require.NoError(b, err)
	result = bytes
	resultDeser = t
	assert.Equal(b, t.ID(), tx.ID())
}

func TestTransaction_Serialization(t *testing.T) {
	wallets := createWallets(2)
	input := generateOutput(wallets[0].address, 0)
	tx, _ := singleInputTransaction(wallets[0], wallets[1], input)

	serializedBytes, err := registry.Manager.Serialize(tx)
	assert.NoError(t, err)

	otherTx := &Transaction{}
	require.NoError(t, registry.Manager.Deserialize(otherTx, serializedBytes))
	assert.Equal(t, tx.ID(), otherTx.ID())

	//assert.Equal(t, tx, otherTx)
}

func TestTransaction_TransactionStorage(t *testing.T) {
	store := mapdb.NewMapDB()
	cacheTimeProvider := database.NewCacheTimeProvider(0)
	options := []objectstorage.Option{
		cacheTimeProvider.CacheTime(10 * time.Second),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
	}

	osFactory := objectstorage.NewFactory(store, database.PrefixLedgerState)

	txStorage := osFactory.New(byte(0), TransactionFromObjectStorage, options...)
	//outputStorage := osFactory.New(byte(1), OutputFromObjectStorage, options...)

	wallets := createWallets(2)
	input := generateOutput(wallets[0].address, 0)
	tx, _ := singleInputTransaction(wallets[0], wallets[1], input)
	txStorage.Store(tx)
	idBytes, _ := registry.Manager.Serialize(tx.ID())

	ct := &CachedTransaction{CachedObject: txStorage.Load(idBytes)}
	ct.Consume(func(transaction *Transaction) {
		assert.Equal(t, transaction.ID(), tx.ID())
	})

	//outputStorage.Store(nil)
}

func TestTransaction_OutputStorage(t *testing.T) {
	store := mapdb.NewMapDB()
	cacheTimeProvider := database.NewCacheTimeProvider(0)
	options := []objectstorage.Option{
		cacheTimeProvider.CacheTime(10 * time.Second),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
	}

	osFactory := objectstorage.NewFactory(store, database.PrefixLedgerState)

	outputStorage := osFactory.New(byte(1), OutputFromObjectStorage, options...)

	wallets := createWallets(2)
	output := generateOutput(wallets[0].address, 0)

	outputStorage.StoreIfAbsent(output)

	inputIDBytes, _ := registry.Manager.Serialize(output.ID())

	ct := &CachedOutput{CachedObject: outputStorage.Load(inputIDBytes)}
	assert.True(t, ct.Exists())
	ct.Consume(func(outputRecovered Output) {
		fmt.Println(output.ID())
		fmt.Println(outputRecovered.ID())
		assert.Equal(t, output.ID(), outputRecovered.ID())
	})
}

//
//func TestTransaction_Complex(t *testing.T) {
//	// setup variables representing keys and outputs for the two parties that wants to trade tokens
//	party1KeyChain, party1SrcAddress, party1DestAddress, party1RemainderAddress := setupKeyChainAndAddresses(t)
//	party1ControlledOutputID := NewOutputID(GenesisTransactionID, 1)
//	party2KeyChain, party2SrcAddress, party2DestAddress, party2RemainderAddress := setupKeyChainAndAddresses(t)
//	party2ControlledOutputID := NewOutputID(GenesisTransactionID, 2)
//
//	// initialize fake ledger state with unspent Outputs
//	unspentOutputsDB := setupUnspentOutputsDB(map[Address]map[OutputID]map[Color]uint64{
//		party1SrcAddress: {
//			party1ControlledOutputID: {
//				sampleColor: 200,
//			},
//		},
//		party2SrcAddress: {
//			party2ControlledOutputID: {
//				ColorIOTA: 2337,
//			},
//		},
//	})
//
//	// party1 prepares a TransactionEssence that party2 is supposed to complete for the exchange of tokens
//	sentParty1Essence := NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{},
//		// he consumes 200 tokens of Color2
//		NewInputs(unspentOutputsDB[party1ControlledOutputID].Input()),
//
//		NewOutputs(
//			// he wants to receive 1337 IOTA on his destination address
//			NewSigLockedSingleOutput(1337, party1DestAddress),
//
//			// he sends only 100 of the consumed tokens to the remainder leaving 100 unspent
//			NewSigLockedColoredOutput(NewColoredBalances(map[Color]uint64{
//				sampleColor: 100,
//			}), party1RemainderAddress),
//		),
//	).Bytes()
//
//	// party2 unmarshals the prepared TransactionEssence he received from party1
//	receivedParty1Essence, _, err := TransactionEssenceFromBytes(sentParty1Essence)
//	require.NoError(t, err)
//
//	// party2 completes the prepared TransactionEssence by adding his Inputs and his Outputs
//	completedEssence := NewTransactionEssence(0,
//		receivedParty1Essence.Timestamp(),
//		receivedParty1Essence.AccessPledgeID(),
//		receivedParty1Essence.ConsensusPledgeID(),
//		NewInputs(append(receivedParty1Essence.Inputs(), unspentOutputsDB[party2ControlledOutputID].Input())...),
//		NewOutputs(append(receivedParty1Essence.Outputs(),
//			// he wants to receive 100 tokens of Color2 on his destination address
//			NewSigLockedColoredOutput(NewColoredBalances(map[Color]uint64{
//				sampleColor: 100,
//			}), party2DestAddress),
//
//			// he sends only 1000 of the 2337 consumed IOTA to the remainder (leaving 1337 unspent)
//			NewSigLockedSingleOutput(1000, party2RemainderAddress),
//		)...),
//	)
//
//	// party2 prepares the signing process by creating the final transaction with empty UnlockBlocks
//	unlockBlocks := make([]UnlockBlock, len(completedEssence.Inputs()))
//	for i := range completedEssence.Inputs() {
//		unlockBlocks[i] = NewSignatureUnlockBlock(NewED25519Signature(ed25519.PublicKey{}, ed25519.Signature{}))
//	}
//	transaction := NewTransaction(completedEssence, unlockBlocks)
//
//	// both parties sign the transaction
//	signTransaction(transaction, unspentOutputsDB, party2KeyChain)
//	signTransaction(transaction, unspentOutputsDB, party1KeyChain)
//
//	// TODO: ADD VALIDITY CHECKS ONCE WE ADDED THE UTXO DAG.
//	// assert.True(t, utxoDAG.TransactionValid(transaction))
//}

// setupKeyChainAndAddresses generates keys and addresses that are used by the test case.
func setupKeyChainAndAddresses(t *testing.T) (keyChain map[Address]ed25519.KeyPair, sourceAddr Address, destAddr Address, remainderAddr Address) {
	keyChain = make(map[Address]ed25519.KeyPair)

	sourceAddrPublicKey, sourceAddrPrivateKey, err := ed25519.GenerateKey()
	require.NoError(t, err)
	sourceAddr = NewED25519Address(sourceAddrPublicKey)
	keyChain[sourceAddr] = ed25519.KeyPair{PrivateKey: sourceAddrPrivateKey, PublicKey: sourceAddrPublicKey}

	destAddrPublicKey, destAddrPrivateKey, err := ed25519.GenerateKey()
	require.NoError(t, err)
	destAddr = NewED25519Address(destAddrPublicKey)
	keyChain[destAddr] = ed25519.KeyPair{PrivateKey: destAddrPrivateKey, PublicKey: destAddrPublicKey}

	remainderAddrPublicKey, remainderAddrPrivateKey, err := ed25519.GenerateKey()
	require.NoError(t, err)
	remainderAddr = NewED25519Address(remainderAddrPublicKey)
	keyChain[destAddr] = ed25519.KeyPair{PrivateKey: remainderAddrPrivateKey, PublicKey: remainderAddrPublicKey}

	return
}

// setupUnspentOutputsDB creates a fake database with Outputs.
func setupUnspentOutputsDB(outputs map[Address]map[OutputID]map[Color]uint64) (unspentOutputsDB OutputsByID) {
	unspentOutputsDB = make(OutputsByID)
	for address, outputs := range outputs {
		for outputID, balances := range outputs {
			unspentOutputsDB[outputID] = NewSigLockedColoredOutput(NewColoredBalances(balances), address).SetID(outputID)
		}
	}

	return
}

// addressFromInput retrieves the Address belonging to an Input by looking it up in the outputs that we have created for
// the tests.
func addressFromInput(input Input, outputsByID OutputsByID) Address {
	typeCastedInput, ok := input.(*UTXOInput)
	if !ok {
		panic("unexpected Input type")
	}

	switch referencedOutput := outputsByID[typeCastedInput.ReferencedOutputID()]; referencedOutput.Type() {
	case SigLockedSingleOutputType:
		typeCastedOutput, ok := referencedOutput.(*SigLockedSingleOutput)
		if !ok {
			panic("failed to type cast SigLockedSingleOutput")
		}

		return typeCastedOutput.Address()
	case SigLockedColoredOutputType:
		typeCastedOutput, ok := referencedOutput.(*SigLockedColoredOutput)
		if !ok {
			panic("failed to type cast SigLockedColoredOutput")
		}

		return typeCastedOutput.Address()
	default:
		panic("unexpected Output type")
	}
}

// signTransaction is a utility function that iterates through a transactions inputs and signs the addresses that are
// part of the signers key chain.
func signTransaction(transaction *Transaction, unspentOutputsDB OutputsByID, keyChain map[Address]ed25519.KeyPair) {
	essenceBytes, err := registry.Manager.Serialize(transaction.Essence())
	if err != nil {
		panic(nil)
	}
	essenceBytesToSign := essenceBytes

	for i, input := range transaction.Essence().Inputs() {
		if keyPair, keyPairExists := keyChain[addressFromInput(input, unspentOutputsDB)]; keyPairExists {
			transaction.UnlockBlocks()[i] = NewSignatureUnlockBlock(NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(essenceBytesToSign)))
		}
	}
}
