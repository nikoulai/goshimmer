package txwrapped

import (
	"golang.org/x/crypto/blake2b"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/mr-tron/base58"
)

// region TransactionType //////////////////////////////////////////////////////////////////////////////////////////////

// TransactionType represents the Payload Type of a Transaction.
var TransactionType payload.Type

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionID ////////////////////////////////////////////////////////////////////////////////////////////////

// TransactionIDLength contains the amount of bytes that a marshaled version of the ID contains.
const TransactionIDLength = 32

// TransactionID is the type that represents the identifier of a Transaction.
type TransactionID [TransactionIDLength]byte

// GenesisTransactionID represents the identifier of the genesis Transaction.
var GenesisTransactionID TransactionID

// Bytes returns a marshaled version of the TransactionID.
func (i TransactionID) Bytes() []byte {
	return i[:]
}

// Base58 returns a base58 encoded version of the TransactionID.
func (i TransactionID) Base58() string {
	return base58.Encode(i[:])
}

// String creates a human readable version of the TransactionID.
func (i TransactionID) String() string {
	return "TransactionID(" + i.Base58() + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionIDs ///////////////////////////////////////////////////////////////////////////////////////////////

// TransactionIDs represents a collection of TransactionIDs.
type TransactionIDs map[TransactionID]types.Empty

// String returns a human readable version of the TransactionIDs.
func (t TransactionIDs) String() (result string) {
	return "TransactionIDs(" + strings.Join(t.Base58s(), ",") + ")"
}

// Base58s returns a slice of base58 encoded versions of the contained TransactionIDs.
func (t TransactionIDs) Base58s() (transactionIDs []string) {
	transactionIDs = make([]string, 0, len(t))
	for transactionID := range t {
		transactionIDs = append(transactionIDs, transactionID.Base58())
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Transaction //////////////////////////////////////////////////////////////////////////////////////////////////

// Transaction represents a Payload that executes a value transfer in the ledger state.
type Transaction struct {
	transactionInner `serialize:"true"`
	objectstorage.StorableObjectFlags
}

type transactionInner struct {
	Id           *TransactionID
	idMutex      sync.RWMutex
	Essence      *TransactionEssence `serialize:"true"`
	UnlockBlocks UnlockBlocks        `serialize:"true"`
}

// NewTransaction creates a new Transaction from the given details.
func NewTransaction(essence *TransactionEssence, unlockBlocks UnlockBlocks) (transaction *Transaction) {
	transaction = &Transaction{
		transactionInner: transactionInner{
			Essence:      essence,
			UnlockBlocks: unlockBlocks,
		},
	}
	return
}

// ID returns the identifier of the Transaction. Since calculating the TransactionID is a resource intensive operation
// we calculate this value lazy and use double checked locking.
func (t *Transaction) ID() TransactionID {
	t.transactionInner.idMutex.RLock()
	if t.transactionInner.Id != nil {
		defer t.transactionInner.idMutex.RUnlock()

		return *t.transactionInner.Id
	}

	t.idMutex.RUnlock()
	t.idMutex.Lock()
	defer t.idMutex.Unlock()

	if t.transactionInner.Id != nil {
		return *t.transactionInner.Id
	}

	idBytes := blake2b.Sum256(t.Bytes())
	id, _, err := TransactionIDFromBytes(idBytes[:])
	if err != nil {
		panic(err)
	}
	t.id = &id

	return id
}

// Type returns the Type of the Payload.
func (t *Transaction) Type() payload.Type {
	return TransactionType
}

// Essence returns the TransactionEssence of the Transaction.
func (t *Transaction) Essence() *TransactionEssence {
	return t.transactionInner.Essence
}

// UnlockBlocks returns the UnlockBlocks of the Transaction.
func (t *Transaction) UnlockBlocks() UnlockBlocks {
	return t.transactionInner.UnlockBlocks
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionEssence ///////////////////////////////////////////////////////////////////////////////////////////

// TransactionEssence contains the transfer related information of the Transaction (without the unlocking details).
type TransactionEssence struct {
	transactionEssenceInner `serialize:"true"`
}
type transactionEssenceInner struct {
	Version TransactionEssenceVersion `serialize:"true"`
	// timestamp is the timestamp of the transaction.
	Timestamp time.Time `serialize:"true"`
	// accessPledgeID is the nodeID to which access mana of the transaction is pledged.
	AccessPledgeID identity.ID `serialize:"true"`
	// consensusPledgeID is the nodeID to which consensus mana of the transaction is pledged.
	ConsensusPledgeID identity.ID `serialize:"true"`
	Inputs            Inputs      `serialize:"true"`
	Outputs           Outputs     `serialize:"true"`
	Payload           payload.Payload
}

// NewTransactionEssence creates a new TransactionEssence from the given details.
func NewTransactionEssence(
	version TransactionEssenceVersion,
	timestamp time.Time,
	accessPledgeID identity.ID,
	consensusPledgeID identity.ID,
	inputs Inputs,
	outputs Outputs,
) *TransactionEssence {
	return &TransactionEssence{
		transactionEssenceInner: transactionEssenceInner{
			Version:           version,
			Timestamp:         timestamp,
			AccessPledgeID:    accessPledgeID,
			ConsensusPledgeID: consensusPledgeID,
			Inputs:            inputs,
			Outputs:           outputs,
		},
	}
}

// SetPayload set the optional Payload of the TransactionEssence.
func (t *TransactionEssence) SetPayload(p payload.Payload) {
	t.transactionEssenceInner.Payload = p
}

// Version returns the Version of the TransactionEssence.
func (t *TransactionEssence) Version() TransactionEssenceVersion {
	return t.transactionEssenceInner.Version
}

// Timestamp returns the timestamp of the TransactionEssence.
func (t *TransactionEssence) Timestamp() time.Time {
	return t.transactionEssenceInner.Timestamp
}

// AccessPledgeID returns the access mana pledge nodeID of the TransactionEssence.
func (t *TransactionEssence) AccessPledgeID() identity.ID {
	return t.transactionEssenceInner.AccessPledgeID
}

// ConsensusPledgeID returns the consensus mana pledge nodeID of the TransactionEssence.
func (t *TransactionEssence) ConsensusPledgeID() identity.ID {
	return t.transactionEssenceInner.ConsensusPledgeID
}

// Inputs returns the Inputs of the TransactionEssence.
func (t *TransactionEssence) Inputs() Inputs {
	return t.transactionEssenceInner.Inputs
}

// Outputs returns the Outputs of the TransactionEssence.
func (t *TransactionEssence) Outputs() Outputs {
	return t.transactionEssenceInner.Outputs
}

// Payload returns the optional Payload of the TransactionEssence.
func (t *TransactionEssence) Payload() payload.Payload {
	return t.transactionEssenceInner.Payload
}

// Bytes returns a marshaled version of the TransactionEssence.
func (t *TransactionEssence) Bytes() []byte {
	marshalUtil := marshalutil.New().
		Write(t.transactionEssenceInner.Version).
		WriteTime(t.transactionEssenceInner.Timestamp).
		Write(t.transactionEssenceInner.AccessPledgeID).
		Write(t.transactionEssenceInner.ConsensusPledgeID).
		Write(t.transactionEssenceInner.Inputs).
		Write(t.transactionEssenceInner.Outputs)

	if !typeutils.IsInterfaceNil(t.transactionEssenceInner.Payload) {
		marshalUtil.Write(t.transactionEssenceInner.Payload)
	} else {
		marshalUtil.WriteUint32(0)
	}

	return marshalUtil.Bytes()
}

// String returns a human readable version of the TransactionEssence.
func (t *TransactionEssence) String() string {
	return ""
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionEssenceVersion ////////////////////////////////////////////////////////////////////////////////////

// TransactionEssenceVersion represents a version number for the TransactionEssence which can be used to ensure backward
// compatibility if the structure ever needs to get changed.
type TransactionEssenceVersion uint8

// Bytes returns a marshaled version of the TransactionEssenceVersion.
func (t TransactionEssenceVersion) Bytes() []byte {
	return []byte{byte(t)}
}

// Compare offers a comparator for TransactionEssenceVersions which returns -1 if the other TransactionEssenceVersion is
// bigger, 1 if it is smaller and 0 if they are the same.
func (t TransactionEssenceVersion) Compare(other TransactionEssenceVersion) int {
	switch {
	case t < other:
		return -1
	case t > other:
		return 1
	default:
		return 0
	}
}

// String returns a human readable version of the TransactionEssenceVersion.
func (t TransactionEssenceVersion) String() string {
	return "TransactionEssenceVersion(" + strconv.Itoa(int(t)) + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
