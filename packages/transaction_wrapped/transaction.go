package txwrapped

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/stringify"

	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/registry"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
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
	transactionInner `serialize:"unpack"`
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

// TransactionFromObjectStorage restores a Transaction that was stored in the ObjectStorage.
func TransactionFromObjectStorage(key []byte, data []byte) (transaction objectstorage.StorableObject, err error) {
	transactionRestored := &Transaction{}
	if err = registry.Manager.Deserialize(transactionRestored, data); err != nil {
		err = errors.Errorf("failed to parse Transaction from bytes: %w", err)
		return
	}

	transactionID := TransactionID{}
	if err = registry.Manager.Deserialize(transactionID, key); err != nil {
		err = errors.Errorf("failed to parse TransactionID from bytes: %w", err)
		return
	}
	transactionRestored.transactionInner.Id = &transactionID
	transaction = transactionRestored
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

	txBytes, err := registry.Manager.Serialize(t)
	if err != nil {
		panic(err)
	}

	idBytes := blake2b.Sum256(txBytes)
	txID := TransactionID{}
	if err := registry.Manager.Deserialize(&txID, idBytes[:]); err != nil {
		panic(err)
	}

	return txID
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

// String returns a human readable version of the Transaction.
func (t *Transaction) String() string {
	return stringify.Struct("Transaction",
		stringify.StructField("id", t.ID()),
		stringify.StructField("essence", t.Essence()),
		stringify.StructField("unlockBlocks", t.UnlockBlocks()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (t *Transaction) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (t *Transaction) ObjectStorageKey() []byte {
	txIDBytes, _ := registry.Manager.Serialize(t.ID())
	return txIDBytes
}

// ObjectStorageValue marshals the Transaction into a sequence of bytes. The ID is not serialized here as it is only
// used as a key in the ObjectStorage.
func (t *Transaction) ObjectStorageValue() []byte {
	txBytes, _ := registry.Manager.Serialize(t)
	return txBytes
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = &Transaction{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedTransaction ////////////////////////////////////////////////////////////////////////////////////////////

// CachedTransaction is a wrapper for the generic CachedObject returned by the object storage that overrides the
// accessor methods with a type-casted one.
type CachedTransaction struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedTransaction) Retain() *CachedTransaction {
	return &CachedTransaction{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedTransaction) Unwrap() *Transaction {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*Transaction)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedTransaction) Consume(consumer func(transaction *Transaction), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Transaction))
	}, forceRelease...)
}

// String returns a human readable version of the CachedTransaction.
func (c *CachedTransaction) String() string {
	return stringify.Struct("CachedTransaction",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionEssence ///////////////////////////////////////////////////////////////////////////////////////////

// TransactionEssence contains the transfer related information of the Transaction (without the unlocking details).
type TransactionEssence struct {
	transactionEssenceInner `serialize:"unpack"`
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

// String returns a human readable version of the TransactionEssence.
func (t *TransactionEssence) String() string {
	return stringify.Struct("TransactionEssence",
		stringify.StructField("version", t.Version()),
		stringify.StructField("timestamp", t.Timestamp()),
		stringify.StructField("accessPledgeID", t.AccessPledgeID()),
		stringify.StructField("consensusPledgeID", t.ConsensusPledgeID()),
		stringify.StructField("inputs", t.Inputs()),
		stringify.StructField("outputs", t.Outputs()),
		stringify.StructField("payload", t.Payload()),
	)
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
