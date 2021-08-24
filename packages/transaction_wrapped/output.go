package txwrapped

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
)

// region Constraints for syntactical validation ///////////////////////////////////////////////////////////////////////

const (
	// MinOutputCount defines the minimum amount of Outputs in a Transaction.
	MinOutputCount = 1

	// MaxOutputCount defines the maximum amount of Outputs in a Transaction.
	MaxOutputCount = 127

	// MinOutputBalance defines the minimum balance per Output.
	MinOutputBalance = 1

	// MaxOutputBalance defines the maximum balance on an Output (the supply).
	MaxOutputBalance = 2779530283277761
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputType ///////////////////////////////////////////////////////////////////////////////////////////////////

// OutputType represents the type of an Output. Outputs of different types can have different unlock rules and allow for
// some relatively basic smart contract logic.
type OutputType uint8

const (
	// SigLockedSingleOutputType represents an Output holding vanilla IOTA tokens that gets unlocked by a signature.
	SigLockedSingleOutputType OutputType = iota

	// SigLockedColoredOutputType represents an Output that holds colored coins that gets unlocked by a signature.
	SigLockedColoredOutputType

	// AliasOutputType represents an Output which makes a chain with optional governance
	AliasOutputType

	// ExtendedLockedOutputType represents an Output which extends SigLockedColoredOutput with alias locking and fallback
	ExtendedLockedOutputType
)

// String returns a human readable representation of the OutputType.
func (o OutputType) String() string {
	return [...]string{
		"SigLockedSingleOutputType",
		"SigLockedColoredOutputType",
		"AliasOutputType",
		"ExtendedLockedOutputType",
	}[o]
}

// OutputTypeFromString returns the output type from a string.
func OutputTypeFromString(ot string) (OutputType, error) {
	res, ok := map[string]OutputType{
		"SigLockedSingleOutputType":  SigLockedSingleOutputType,
		"SigLockedColoredOutputType": SigLockedColoredOutputType,
		"AliasOutputType":            AliasOutputType,
		"ExtendedLockedOutputType":   ExtendedLockedOutputType,
	}[ot]
	if !ok {
		return res, errors.New(fmt.Sprintf("unsupported output type: %s", ot))
	}
	return res, nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputID /////////////////////////////////////////////////////////////////////////////////////////////////////

// OutputIDLength contains the amount of bytes that a marshaled version of the OutputID contains.
const OutputIDLength = TransactionIDLength + marshalutil.Uint16Size

// OutputID is the data type that represents the identifier of an Output (which consists of a TransactionID and the
// index of the Output in the Transaction that created it).
type OutputID [OutputIDLength]byte

// EmptyOutputID represents the zero-value of an OutputID.
var EmptyOutputID OutputID

// NewOutputID is the constructor for the OutputID.
func NewOutputID(transactionID TransactionID, outputIndex uint16) (outputID OutputID) {
	if outputIndex >= MaxOutputCount {
		panic(fmt.Sprintf("output index exceeds threshold defined by MaxOutputCount (%d)", MaxOutputCount))
	}

	copy(outputID[:TransactionIDLength], transactionID.Bytes())
	binary.LittleEndian.PutUint16(outputID[TransactionIDLength:], outputIndex)

	return
}

// TransactionID returns the TransactionID part of an OutputID.
func (o OutputID) TransactionID() (transactionID TransactionID) {
	copy(transactionID[:], o[:TransactionIDLength])

	return
}

// OutputIndex returns the Output index part of an OutputID.
func (o OutputID) OutputIndex() uint16 {
	return binary.LittleEndian.Uint16(o[TransactionIDLength:])
}

// Bytes marshals the OutputID into a sequence of bytes.
func (o OutputID) Bytes() []byte {
	return o[:]
}

// Base58 returns a base58 encoded version of the OutputID.
func (o OutputID) Base58() string {
	return base58.Encode(o[:])
}

// String creates a human readable version of the OutputID.
func (o OutputID) String() string {
	return stringify.Struct("OutputID",
		stringify.StructField("transactionID", o.TransactionID()),
		stringify.StructField("outputIndex", o.OutputIndex()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Output ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Output is a generic interface for the different types of Outputs (with different unlock behaviors).
type Output interface {
	// ID returns the identifier of the Output that is used to Address the Output in the UTXODAG.
	ID() OutputID

	// SetID allows to set the identifier of the Output. We offer a setter for the property since Outputs that are
	// created to become part of a transaction usually do not have an identifier, yet as their identifier depends on
	// the TransactionID that is only determinable after the Transaction has been fully constructed. The ID is therefore
	// only accessed when the Output is supposed to be persisted.
	SetID(outputID OutputID) Output

	// Type returns the OutputType which allows us to generically handle Outputs of different types.
	Type() OutputType

	// Balances returns the funds that are associated with the Output.
	Balances() *ColoredBalances

	// Address returns the Address that is associated to the output.
	Address() Address

	// UnlockValid determines if the given Transaction and the corresponding UnlockBlock are allowed to spend the
	// Output.
	UnlockValid(tx *Transaction, unlockBlock UnlockBlock, inputs []Output) (bool, error)

	// UpdateMintingColor replaces the ColorMint in the Balances of the Output with the hash of the OutputID. It returns a
	// copy of the original Output with the modified Balances.
	UpdateMintingColor() Output

	// Input returns an Input that references the Output.
	Input() Input

	// Clone creates a copy of the Output.
	Clone() Output

	// Bytes returns a marshaled version of the Output.
	Bytes() []byte

	// String returns a human readable version of the Output for debug purposes.
	String() string

	// Compare offers a comparator for Outputs which returns -1 if the other Output is bigger, 1 if it is smaller and 0
	// if they are the same.
	Compare(other Output) int

	// StorableObject makes Outputs storable in the ObjectStorage.
	objectstorage.StorableObject
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Outputs //////////////////////////////////////////////////////////////////////////////////////////////////////

// Outputs represents a list of Outputs that can be used in a Transaction.
type Outputs []Output

// NewOutputs returns a deterministically ordered collection of Outputs. It removes duplicates in the parameters and
// sorts the Outputs to ensure syntactical correctness.
func NewOutputs(optionalOutputs ...Output) (outputs Outputs) {
	seenOutputs := make(map[string]types.Empty)
	sortedOutputs := make([]struct {
		output           Output
		outputSerialized []byte
	}, 0)

	// filter duplicates (store marshaled version so we don't need to marshal a second time during sort)
	for _, output := range optionalOutputs {
		marshaledOutput := output.Bytes()
		marshaledOutputAsString := typeutils.BytesToString(marshaledOutput)

		if _, seenAlready := seenOutputs[marshaledOutputAsString]; seenAlready {
			continue
		}
		seenOutputs[marshaledOutputAsString] = types.Void

		sortedOutputs = append(sortedOutputs, struct {
			output           Output
			outputSerialized []byte
		}{output, marshaledOutput})
	}

	// sort outputs
	sort.Slice(sortedOutputs, func(i, j int) bool {
		return bytes.Compare(sortedOutputs[i].outputSerialized, sortedOutputs[j].outputSerialized) < 0
	})

	// create result
	outputs = make(Outputs, len(sortedOutputs))
	for i, sortedOutput := range sortedOutputs {
		outputs[i] = sortedOutput.output
	}

	if len(outputs) < MinOutputCount {
		panic(fmt.Sprintf("amount of Outputs (%d) failed to reach MinOutputCount (%d)", len(outputs), MinOutputCount))
	}
	if len(outputs) > MaxOutputCount {
		panic(fmt.Sprintf("amount of Outputs (%d) exceeds MaxOutputCount (%d)", len(outputs), MaxOutputCount))
	}

	return
}

// Inputs returns the Inputs that reference the Outputs.
func (o Outputs) Inputs() Inputs {
	inputs := make([]Input, len(o))
	for i, output := range o {
		inputs[i] = output.Input()
	}

	return NewInputs(inputs...)
}

// ByID returns a map of Outputs where the key is the OutputID.
func (o Outputs) ByID() (outputsByID OutputsByID) {
	outputsByID = make(OutputsByID)
	for _, output := range o {
		outputsByID[output.ID()] = output
	}

	return
}

// Clone creates a copy of the Outputs.
func (o Outputs) Clone() (clonedOutputs Outputs) {
	clonedOutputs = make(Outputs, len(o))
	for i, output := range o {
		clonedOutputs[i] = output.Clone()
	}

	return
}

// Filter removes all elements from the Outputs that do not pass the given condition.
func (o Outputs) Filter(condition func(output Output) bool) (filteredOutputs Outputs) {
	filteredOutputs = make(Outputs, 0)
	for _, output := range o {
		if condition(output) {
			filteredOutputs = append(filteredOutputs, output)
		}
	}

	return
}

// Bytes returns a marshaled version of the Outputs.
func (o Outputs) Bytes() []byte {
	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint16(uint16(len(o)))
	for _, output := range o {
		marshalUtil.WriteBytes(output.Bytes())
	}

	return marshalUtil.Bytes()
}

// String returns a human readable version of the Outputs.
func (o Outputs) String() string {
	structBuilder := stringify.StructBuilder("Outputs")
	for i, output := range o {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), output))
	}

	return structBuilder.String()
}

// Strings returns the Outputs in the form []transactionID:index.
func (o Outputs) Strings() (result []string) {
	for _, output := range o {
		result = append(result, fmt.Sprintf("%s:%d", output.ID().TransactionID().Base58(), output.ID().OutputIndex()))
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputsByID //////////////////////////////////////////////////////////////////////////////////////////////////

// OutputsByID represents a map of Outputs where every Output is stored with its corresponding OutputID as the key.
type OutputsByID map[OutputID]Output

// NewOutputsByID returns a map of Outputs where every Output is stored with its corresponding OutputID as the key.
func NewOutputsByID(optionalOutputs ...Output) (outputsByID OutputsByID) {
	outputsByID = make(OutputsByID)
	for _, optionalOutput := range optionalOutputs {
		outputsByID[optionalOutput.ID()] = optionalOutput
	}

	return
}

// Inputs returns the Inputs that reference the Outputs.
func (o OutputsByID) Inputs() Inputs {
	inputs := make([]Input, 0, len(o))
	for _, output := range o {
		inputs = append(inputs, output.Input())
	}

	return NewInputs(inputs...)
}

// Outputs returns a list of Outputs from the OutputsByID.
func (o OutputsByID) Outputs() Outputs {
	outputs := make([]Output, 0, len(o))
	for _, output := range o {
		outputs = append(outputs, output)
	}

	return NewOutputs(outputs...)
}

// Clone creates a copy of the OutputsByID.
func (o OutputsByID) Clone() (clonedOutputs OutputsByID) {
	clonedOutputs = make(OutputsByID)
	for id, output := range o {
		clonedOutputs[id] = output.Clone()
	}

	return
}

// String returns a human readable version of the OutputsByID.
func (o OutputsByID) String() string {
	structBuilder := stringify.StructBuilder("OutputsByID")
	for id, output := range o {
		structBuilder.AddField(stringify.StructField(id.String(), output))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SigLockedSingleOutput ////////////////////////////////////////////////////////////////////////////////////////

// SigLockedSingleOutput is an Output that holds exactly one uncolored balance and that can be unlocked by providing a
// signature for an Address.
type SigLockedSingleOutput struct {
	sigLockedSingleOutputInner `serialize:"unpack"`
	objectstorage.StorableObjectFlags
}

type sigLockedSingleOutputInner struct {
	Id      OutputID `serialize:"true"`
	idMutex sync.RWMutex
	Balance uint64  `serialize:"true"`
	Address Address `serialize:"true"`
}

// NewSigLockedSingleOutput is the constructor for a SigLockedSingleOutput.
func NewSigLockedSingleOutput(balance uint64, address Address) *SigLockedSingleOutput {
	return &SigLockedSingleOutput{
		sigLockedSingleOutputInner: sigLockedSingleOutputInner{
			Balance: balance,
			Address: address,
		},
	}
}

// ID returns the identifier of the Output that is used to Address the Output in the UTXODAG.
func (s *SigLockedSingleOutput) ID() OutputID {
	s.sigLockedSingleOutputInner.idMutex.RLock()
	defer s.sigLockedSingleOutputInner.idMutex.RUnlock()

	return s.sigLockedSingleOutputInner.Id
}

// SetID allows to set the identifier of the Output. We offer a setter for the property since Outputs that are
// created to become part of a transaction usually do not have an identifier, yet as their identifier depends on
// the TransactionID that is only determinable after the Transaction has been fully constructed. The ID is therefore
// only accessed when the Output is supposed to be persisted by the node.
func (s *SigLockedSingleOutput) SetID(outputID OutputID) Output {
	s.sigLockedSingleOutputInner.idMutex.Lock()
	defer s.sigLockedSingleOutputInner.idMutex.Unlock()

	s.sigLockedSingleOutputInner.Id = outputID

	return s
}

// Type returns the type of the Output which allows us to generically handle Outputs of different types.
func (s *SigLockedSingleOutput) Type() OutputType {
	return SigLockedSingleOutputType
}

// Balances returns the funds that are associated with the Output.
func (s *SigLockedSingleOutput) Balances() *ColoredBalances {
	balances := NewColoredBalances(map[Color]uint64{
		ColorIOTA: s.sigLockedSingleOutputInner.Balance,
	})

	return balances
}

// UnlockValid determines if the given Transaction and the corresponding UnlockBlock are allowed to spend the Output.
func (s *SigLockedSingleOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock, inputs []Output) (unlockValid bool, err error) {
	switch blk := unlockBlock.(type) {
	case *SignatureUnlockBlock:
		// unlocking by signature
		unlockValid = blk.AddressSignatureValid(s.sigLockedSingleOutputInner.Address, tx.Essence().Bytes())

	case *AliasUnlockBlock:
		// unlocking by alias reference. The unlock is valid if:
		// - referenced alias output has same alias Address
		// - it is not unlocked for governance
		if s.sigLockedSingleOutputInner.Address.Type() != AliasAddressType {
			return false, errors.Errorf("SigLockedSingleOutput: %s Address can't be unlocked by alias reference", s.sigLockedSingleOutputInner.Address.Type().String())
		}
		refAliasOutput, isAlias := inputs[blk.AliasInputIndex()].(*AliasOutput)
		if !isAlias {
			return false, errors.New("SigLockedSingleOutput: referenced input must be AliasOutput")
		}
		if !s.sigLockedSingleOutputInner.Address.Equals(refAliasOutput.GetAliasAddress()) {
			return false, errors.New("SigLockedSingleOutput: wrong alias referenced")
		}
		unlockValid = !refAliasOutput.hasToBeUnlockedForGovernanceUpdate(tx)

	default:
		err = errors.Errorf("SigLockedSingleOutput: unsupported unlock block type: %w", cerrors.ErrParseBytesFailed)
	}

	return
}

// Address returns the Address that the Output is associated to.
func (s *SigLockedSingleOutput) Address() Address {
	return s.sigLockedSingleOutputInner.Address
}

// Input returns an Input that references the Output.
func (s *SigLockedSingleOutput) Input() Input {
	if s.ID() == EmptyOutputID {
		panic("Outputs that haven't been assigned an ID yet cannot be converted to an Input")
	}

	return NewUTXOInput(s.ID())
}

// Clone creates a copy of the Output.
func (s *SigLockedSingleOutput) Clone() Output {
	return &SigLockedSingleOutput{
		sigLockedSingleOutputInner: sigLockedSingleOutputInner{
			Id:      s.sigLockedSingleOutputInner.Id,
			Balance: s.sigLockedSingleOutputInner.Balance,
			Address: s.sigLockedSingleOutputInner.Address.Clone(),
		},
	}
}

// Bytes returns a marshaled version of the Output.
func (s *SigLockedSingleOutput) Bytes() []byte {
	return s.ObjectStorageValue()
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (s *SigLockedSingleOutput) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// UpdateMintingColor does nothing for SigLockedSingleOutput
func (s *SigLockedSingleOutput) UpdateMintingColor() Output {
	return s
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (s *SigLockedSingleOutput) ObjectStorageKey() []byte {
	return s.ID().Bytes()
}

// ObjectStorageValue marshals the Output into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (s *SigLockedSingleOutput) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteByte(byte(SigLockedSingleOutputType)).
		WriteUint64(s.sigLockedSingleOutputInner.Balance).
		WriteBytes(s.sigLockedSingleOutputInner.Address.Bytes()).
		Bytes()
}

// Compare offers a comparator for Outputs which returns -1 if the other Output is bigger, 1 if it is smaller and 0 if
// they are the same.
func (s *SigLockedSingleOutput) Compare(other Output) int {
	return bytes.Compare(s.Bytes(), other.Bytes())
}

// String returns a human readable version of the Output.
func (s *SigLockedSingleOutput) String() string {
	return stringify.Struct("SigLockedSingleOutput",
		stringify.StructField("Id", s.ID()),
		stringify.StructField("Address", s.sigLockedSingleOutputInner.Address),
		stringify.StructField("balance", s.sigLockedSingleOutputInner.Balance),
	)
}

// code contract (make sure the type implements all required methods)
var _ Output = &SigLockedSingleOutput{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SigLockedColoredOutput ///////////////////////////////////////////////////////////////////////////////////////

// SigLockedColoredOutput is an Output that holds colored Balances and that can be unlocked by providing a signature for
// an Address.
type SigLockedColoredOutput struct {
	sigLockedColoredOutputInner `serialize:"unpack"`
	objectstorage.StorableObjectFlags
}

type sigLockedColoredOutputInner struct {
	Id       OutputID `serialize:"true"`
	idMutex  sync.RWMutex
	Balances *ColoredBalances `serialize:"true"`
	Address  Address          `serialize:"true"`
}

// NewSigLockedColoredOutput is the constructor for a SigLockedColoredOutput.
func NewSigLockedColoredOutput(balances *ColoredBalances, address Address) *SigLockedColoredOutput {
	return &SigLockedColoredOutput{
		sigLockedColoredOutputInner: sigLockedColoredOutputInner{
			Balances: balances,
			Address:  address,
		},
	}
}

// ID returns the identifier of the Output that is used to Address the Output in the UTXODAG.
func (s *SigLockedColoredOutput) ID() OutputID {
	s.idMutex.RLock()
	defer s.idMutex.RUnlock()

	return s.sigLockedColoredOutputInner.Id
}

// SetID allows to set the identifier of the Output. We offer a setter for the property since Outputs that are
// created to become part of a transaction usually do not have an identifier, yet as their identifier depends on
// the TransactionID that is only determinable after the Transaction has been fully constructed. The ID is therefore
// only accessed when the Output is supposed to be persisted by the node.
func (s *SigLockedColoredOutput) SetID(outputID OutputID) Output {
	s.idMutex.Lock()
	defer s.idMutex.Unlock()

	s.sigLockedColoredOutputInner.Id = outputID

	return s
}

// Type returns the type of the Output which allows us to generically handle Outputs of different types.
func (s *SigLockedColoredOutput) Type() OutputType {
	return SigLockedColoredOutputType
}

// Balances returns the funds that are associated with the Output.
func (s *SigLockedColoredOutput) Balances() *ColoredBalances {
	return s.sigLockedColoredOutputInner.Balances
}

// UnlockValid determines if the given Transaction and the corresponding UnlockBlock are allowed to spend the Output.
func (s *SigLockedColoredOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock, inputs []Output) (unlockValid bool, err error) {
	switch blk := unlockBlock.(type) {
	case *SignatureUnlockBlock:
		// unlocking by signature
		unlockValid = blk.AddressSignatureValid(s.sigLockedColoredOutputInner.Address, tx.Essence().Bytes())

	case *AliasUnlockBlock:
		// unlocking by alias reference. The unlock is valid if:
		// - referenced alias output has same alias Address
		// - it is not unlocked for governance
		if s.sigLockedColoredOutputInner.Address.Type() != AliasAddressType {
			return false, errors.Errorf("SigLockedColoredOutput: %s Address can't be unlocked by alias reference", s.sigLockedColoredOutputInner.Address.Type().String())
		}
		refAliasOutput, isAlias := inputs[blk.AliasInputIndex()].(*AliasOutput)
		if !isAlias {
			return false, errors.New("SigLockedColoredOutput: referenced input must be AliasOutput")
		}
		if !s.sigLockedColoredOutputInner.Address.Equals(refAliasOutput.GetAliasAddress()) {
			return false, errors.New("SigLockedColoredOutput: wrong alias referenced")
		}
		unlockValid = !refAliasOutput.hasToBeUnlockedForGovernanceUpdate(tx)

	default:
		err = errors.Errorf("SigLockedColoredOutput: unsupported unlock block type: %w", cerrors.ErrParseBytesFailed)
	}

	return
}

// Address returns the Address that the Output is associated to.
func (s *SigLockedColoredOutput) Address() Address {
	return s.sigLockedColoredOutputInner.Address
}

// Input returns an Input that references the Output.
func (s *SigLockedColoredOutput) Input() Input {
	if s.ID() == EmptyOutputID {
		panic("Outputs that haven't been assigned an ID, yet cannot be converted to an Input")
	}

	return NewUTXOInput(s.ID())
}

// Clone creates a copy of the Output.
func (s *SigLockedColoredOutput) Clone() Output {
	return &SigLockedColoredOutput{
		sigLockedColoredOutputInner: sigLockedColoredOutputInner{
			Id:       s.sigLockedColoredOutputInner.Id,
			Balances: s.sigLockedColoredOutputInner.Balances.Clone(),
			Address:  s.sigLockedColoredOutputInner.Address.Clone(),
		},
	}
}

// UpdateMintingColor replaces the ColorMint in the Balances of the Output with the hash of the OutputID. It returns a
// copy of the original Output with the modified Balances.
func (s *SigLockedColoredOutput) UpdateMintingColor() (updatedOutput Output) {
	coloredBalances := s.Balances().Map()
	if mintedCoins, mintedCoinsExist := coloredBalances[ColorMint]; mintedCoinsExist {
		delete(coloredBalances, ColorMint)
		coloredBalances[Color(blake2b.Sum256(s.ID().Bytes()))] = mintedCoins
	}
	updatedOutput = NewSigLockedColoredOutput(NewColoredBalances(coloredBalances), s.Address())
	updatedOutput.SetID(s.ID())

	return
}

// Bytes returns a marshaled version of the Output.
func (s *SigLockedColoredOutput) Bytes() []byte {
	return s.ObjectStorageValue()
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (s *SigLockedColoredOutput) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (s *SigLockedColoredOutput) ObjectStorageKey() []byte {
	return s.sigLockedColoredOutputInner.Id.Bytes()
}

// ObjectStorageValue marshals the Output into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (s *SigLockedColoredOutput) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteByte(byte(SigLockedColoredOutputType)).
		WriteBytes(s.sigLockedColoredOutputInner.Balances.Bytes()).
		WriteBytes(s.sigLockedColoredOutputInner.Address.Bytes()).
		Bytes()
}

// Compare offers a comparator for Outputs which returns -1 if the other Output is bigger, 1 if it is smaller and 0 if
// they are the same.
func (s *SigLockedColoredOutput) Compare(other Output) int {
	return bytes.Compare(s.Bytes(), other.Bytes())
}

// String returns a human readable version of the Output.
func (s *SigLockedColoredOutput) String() string {
	return stringify.Struct("SigLockedColoredOutput",
		stringify.StructField("Id", s.ID()),
		stringify.StructField("Address", s.sigLockedColoredOutputInner.Address),
		stringify.StructField("Balances", s.sigLockedColoredOutputInner.Balances),
	)
}

// code contract (make sure the type implements all required methods)
var _ Output = &SigLockedColoredOutput{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AliasOutput ///////////////////////////////////////////////////////////////////////////////////////

// DustThresholdAliasOutputIOTA is minimum number of iotas enforced for the output to be correct
// TODO protocol-wide dust threshold configuration
const DustThresholdAliasOutputIOTA = uint64(100)

// MaxOutputPayloadSize size limit on the data Payload in the output.
const MaxOutputPayloadSize = 4 * 1024

// flags use to compress serialized bytes
const (
	flagAliasOutputGovernanceUpdate = uint(iota)
	flagAliasOutputGovernanceSet
	flagAliasOutputStateDataPresent
	flagAliasOutputGovernanceMetadataPresent
	flagAliasOutputImmutableDataPresent
	flagAliasOutputIsOrigin
	flagAliasOutputDelegationConstraint
	flagAliasOutputDelegationTimelockPresent
)

// AliasOutput represents output which defines as AliasAddress.
// It can only be used in a chained manner
type AliasOutput struct {
	aliasOutputInner `serialize:"unpack"`
	objectstorage.StorableObjectFlags
}

type aliasOutputInner struct {
	// common for all outputs
	OutputID      OutputID `serialize:"true"`
	outputIDMutex sync.RWMutex
	Balances      *ColoredBalances `serialize:"true"`

	// aliasAddress becomes immutable after created for a lifetime. It is returned as Address()
	AliasAddress AliasAddress `serialize:"true"`
	// Address which controls the state and state data
	// It can only be changed by governing entity, if set. Otherwise it is self governed.
	// Can also be an AliasAddress
	StateAddress Address `serialize:"true"`
	// state index is enforced incremental counter of state updates. The constraint is:
	// - start at 0 when chain in minted
	// - increase by 1 with each new chained output with isGovernanceUpdate == false
	// - do not change with any new chained output with isGovernanceUpdate == true
	StateIndex uint32 `serialize:"true"`
	// optional state metadata. nil means absent
	StateData []byte `serialize:"true"`
	// optional governance level metadata, that can only be changed with a governance update
	GovernanceMetadata []byte `serialize:"true"`
	// optional immutable data. It is set when AliasOutput is minted and can't be changed since
	// Useful for NFTs
	ImmutableData []byte `serialize:"true"`
	// if the AliasOutput is chained in the transaction, the flags states if it is updating state or governance data.
	// unlock validation of the corresponding input depends on it.
	IsGovernanceUpdate bool `serialize:"true"`
	// governance Address if set. It can be any Address, unlocked by signature of alias Address. Nil means self governed
	GoverningAddress Address `serialize:"true"`
	// true if it is the first output in the chain
	IsOrigin bool `serialize:"true"`
	// true if the output is subject to the "delegation" constraint: upon transition tokens cannot be changed
	IsDelegated bool `serialize:"true"`
	// delegation Timelock (optional). Before the Timelock, only state transition is permitted, after the Timelock, only
	// governance transition
	DelegationTimelock time.Time `serialize:"true"`
}

// NewAliasOutputMint creates new AliasOutput as minting output, i.e. the one which does not contain corresponding input.
func NewAliasOutputMint(balances map[Color]uint64, stateAddr Address, immutableData ...[]byte) (*AliasOutput, error) {
	if !IsAboveDustThreshold(balances) {
		return nil, errors.New("AliasOutput: colored Balances are below dust threshold")
	}
	if stateAddr == nil {
		return nil, errors.New("AliasOutput: mandatory state Address cannot be nil")
	}
	ret := &AliasOutput{
		aliasOutputInner: aliasOutputInner{
			Balances:     NewColoredBalances(balances),
			StateAddress: stateAddr,
			IsOrigin:     true,
		},
	}
	if len(immutableData) > 0 {
		ret.aliasOutputInner.ImmutableData = immutableData[0]
	}
	if err := ret.checkBasicValidity(); err != nil {
		return nil, err
	}
	return ret, nil
}

// NewAliasOutputNext creates new AliasOutput as state transition from the previous one
func (a *AliasOutput) NewAliasOutputNext(governanceUpdate ...bool) *AliasOutput {
	ret := a.clone()
	ret.aliasOutputInner.IsOrigin = false
	ret.aliasOutputInner.IsGovernanceUpdate = false
	if len(governanceUpdate) > 0 {
		ret.aliasOutputInner.IsGovernanceUpdate = governanceUpdate[0]
	}
	if !ret.aliasOutputInner.IsGovernanceUpdate {
		ret.aliasOutputInner.StateIndex = a.aliasOutputInner.StateIndex + 1
	}
	return ret
}

// WithDelegation returns the output as a delegateds alias output.
func (a *AliasOutput) WithDelegation() *AliasOutput {
	a.aliasOutputInner.IsDelegated = true
	return a
}

// WithDelegationAndTimelock returns the output as a delegated alias output and a set delegation Timelock.
func (a *AliasOutput) WithDelegationAndTimelock(lockUntil time.Time) *AliasOutput {
	a.aliasOutputInner.IsDelegated = true
	a.aliasOutputInner.DelegationTimelock = lockUntil
	return a
}

// SetBalances sets colored Balances of the output
func (a *AliasOutput) SetBalances(balances map[Color]uint64) error {
	if !IsAboveDustThreshold(balances) {
		return errors.New("AliasOutput: Balances are less than dust threshold")
	}
	a.aliasOutputInner.Balances = NewColoredBalances(balances)
	return nil
}

// GetAliasAddress calculates new ID if it is a minting output. Otherwise it takes stored value
func (a *AliasOutput) GetAliasAddress() *AliasAddress {
	if a.aliasOutputInner.AliasAddress.IsNil() {
		return NewAliasAddress(a.ID().Bytes())
	}
	return &a.aliasOutputInner.AliasAddress
}

// SetAliasAddress sets the alias Address of the alias output.
func (a *AliasOutput) SetAliasAddress(addr *AliasAddress) {
	a.aliasOutputInner.AliasAddress = *addr
}

// IsOrigin returns true if it starts the chain
func (a *AliasOutput) IsOrigin() bool {
	return a.aliasOutputInner.IsOrigin
}

// SetIsOrigin sets the isOrigin field of the output.
func (a *AliasOutput) SetIsOrigin(isOrigin bool) {
	a.aliasOutputInner.IsOrigin = isOrigin
}

// IsDelegated returns true if the output is delegated.
func (a *AliasOutput) IsDelegated() bool {
	return a.aliasOutputInner.IsDelegated
}

// SetIsDelegated sets the isDelegated field of the output.
func (a *AliasOutput) SetIsDelegated(isDelegated bool) {
	a.aliasOutputInner.IsDelegated = isDelegated
}

// IsSelfGoverned returns if governing Address is not set which means that stateAddress is same as governingAddress
func (a *AliasOutput) IsSelfGoverned() bool {
	return a.aliasOutputInner.GoverningAddress == nil
}

// GetStateAddress return state controlling Address
func (a *AliasOutput) GetStateAddress() Address {
	return a.aliasOutputInner.StateAddress
}

// SetStateAddress sets the state controlling Address
func (a *AliasOutput) SetStateAddress(addr Address) error {
	if addr == nil {
		return errors.New("AliasOutput: mandatory state Address cannot be nil")
	}
	a.aliasOutputInner.StateAddress = addr
	return nil
}

// SetGoverningAddress sets the governing Address or nil for self-governing
func (a *AliasOutput) SetGoverningAddress(addr Address) {
	if addr == nil {
		a.aliasOutputInner.GoverningAddress = nil
		return
	}
	// calling Array on nil panics
	if addr.Array() == a.aliasOutputInner.StateAddress.Array() {
		addr = nil // self governing
	}
	a.aliasOutputInner.GoverningAddress = addr
}

// GetGoverningAddress return governing Address. If self-governed, it is the same as state controlling Address
func (a *AliasOutput) GetGoverningAddress() Address {
	if a.IsSelfGoverned() {
		return a.aliasOutputInner.StateAddress
	}
	return a.aliasOutputInner.GoverningAddress
}

// SetStateData sets state data
func (a *AliasOutput) SetStateData(data []byte) error {
	if len(data) > MaxOutputPayloadSize {
		return errors.New("AliasOutput: state data too big")
	}
	a.aliasOutputInner.StateData = make([]byte, len(data))
	copy(a.aliasOutputInner.StateData, data)
	return nil
}

// GetStateData gets the state data
func (a *AliasOutput) GetStateData() []byte {
	return a.aliasOutputInner.StateData
}

// SetGovernanceMetadata sets governance metadata
func (a *AliasOutput) SetGovernanceMetadata(data []byte) error {
	if len(data) > MaxOutputPayloadSize {
		return errors.New("AliasOutput: governance metadata too big")
	}
	a.aliasOutputInner.GovernanceMetadata = make([]byte, len(data))
	copy(a.aliasOutputInner.GovernanceMetadata, data)
	return nil
}

// GetGovernanceMetadata gets the governance metadata
func (a *AliasOutput) GetGovernanceMetadata() []byte {
	return a.aliasOutputInner.GovernanceMetadata
}

// SetStateIndex sets the state index in the input. It is enforced to increment by 1 with each state transition
func (a *AliasOutput) SetStateIndex(index uint32) {
	a.aliasOutputInner.StateIndex = index
}

// GetIsGovernanceUpdated returns if the output was unlocked for governance in the transaction.
func (a *AliasOutput) GetIsGovernanceUpdated() bool {
	return a.aliasOutputInner.IsGovernanceUpdate
}

// SetIsGovernanceUpdated sets the isGovernanceUpdated flag.
func (a *AliasOutput) SetIsGovernanceUpdated(i bool) {
	a.aliasOutputInner.IsGovernanceUpdate = i
}

// GetStateIndex returns the state index
func (a *AliasOutput) GetStateIndex() uint32 {
	return a.aliasOutputInner.StateIndex
}

// GetImmutableData gets the state data
func (a *AliasOutput) GetImmutableData() []byte {
	return a.aliasOutputInner.ImmutableData
}

// SetImmutableData sets the immutable data field of the alias output.
func (a *AliasOutput) SetImmutableData(data []byte) error {
	if len(data) > MaxOutputPayloadSize {
		return errors.New("AliasOutput: immutable data too big")
	}
	a.aliasOutputInner.ImmutableData = make([]byte, len(data))
	copy(a.aliasOutputInner.ImmutableData, data)
	return nil
}

// SetDelegationTimelock sets the delegation Timelock. An error is returned if the output is not a delegated.
func (a *AliasOutput) SetDelegationTimelock(timelock time.Time) error {
	if !a.aliasOutputInner.IsDelegated {
		return errors.Errorf("AliasOutput: delegation Timelock can only be set on a delegated output")
	}
	a.aliasOutputInner.DelegationTimelock = timelock
	return nil
}

// DelegationTimelock returns the delegation Timelock. If the output is not delegated, or delegation Timelock is
// not set, it returns the zero time object.
func (a *AliasOutput) DelegationTimelock() time.Time {
	if !a.aliasOutputInner.IsDelegated {
		return time.Time{}
	}
	return a.aliasOutputInner.DelegationTimelock
}

// DelegationTimeLockedNow determines if the alias output is delegation timelocked at a given time.
func (a *AliasOutput) DelegationTimeLockedNow(nowis time.Time) bool {
	if !a.aliasOutputInner.IsDelegated || a.aliasOutputInner.DelegationTimelock.IsZero() {
		return false
	}
	return a.aliasOutputInner.DelegationTimelock.After(nowis)
}

// Clone clones the structure
func (a *AliasOutput) Clone() Output {
	return a.clone()
}

func (a *AliasOutput) clone() *AliasOutput {
	a.mustValidate()
	ret := &AliasOutput{
		aliasOutputInner: aliasOutputInner{
			OutputID:           a.OutputID,
			Balances:           a.aliasOutputInner.Balances.Clone(),
			AliasAddress:       *a.GetAliasAddress(),
			StateAddress:       a.aliasOutputInner.StateAddress.Clone(),
			StateIndex:         a.aliasOutputInner.StateIndex,
			StateData:          make([]byte, len(a.aliasOutputInner.StateData)),
			GovernanceMetadata: make([]byte, len(a.aliasOutputInner.GovernanceMetadata)),
			ImmutableData:      make([]byte, len(a.aliasOutputInner.ImmutableData)),
			DelegationTimelock: a.aliasOutputInner.DelegationTimelock,
			IsOrigin:           a.aliasOutputInner.IsOrigin,
			IsDelegated:        a.aliasOutputInner.IsDelegated,
			IsGovernanceUpdate: a.aliasOutputInner.IsGovernanceUpdate,
		},
	}
	if a.aliasOutputInner.GoverningAddress != nil {
		ret.aliasOutputInner.GoverningAddress = a.aliasOutputInner.GoverningAddress.Clone()
	}
	copy(ret.aliasOutputInner.StateData, a.aliasOutputInner.StateData)
	copy(ret.aliasOutputInner.GovernanceMetadata, a.aliasOutputInner.GovernanceMetadata)
	copy(ret.aliasOutputInner.ImmutableData, a.aliasOutputInner.ImmutableData)
	ret.mustValidate()
	return ret
}

// ID is the ID of the output
func (a *AliasOutput) ID() OutputID {
	a.aliasOutputInner.outputIDMutex.RLock()
	defer a.aliasOutputInner.outputIDMutex.RUnlock()

	return a.aliasOutputInner.OutputID
}

// SetID set the output ID after unmarshalling
func (a *AliasOutput) SetID(outputID OutputID) Output {
	a.outputIDMutex.Lock()
	defer a.outputIDMutex.Unlock()

	a.aliasOutputInner.OutputID = outputID
	return a
}

// Type return the type of the output
func (a *AliasOutput) Type() OutputType {
	return AliasOutputType
}

// Balances return colored Balances of the output
func (a *AliasOutput) Balances() *ColoredBalances {
	return a.aliasOutputInner.Balances
}

// Address AliasOutput is searchable in the ledger through its AliasAddress
func (a *AliasOutput) Address() Address {
	return a.GetAliasAddress()
}

// Input makes input from the output
func (a *AliasOutput) Input() Input {
	if a.ID() == EmptyOutputID {
		panic("AliasOutput: Outputs that haven't been assigned an ID, yet cannot be converted to an Input")
	}

	return NewUTXOInput(a.ID())
}

// Bytes serialized form
func (a *AliasOutput) Bytes() []byte {
	return a.ObjectStorageValue()
}

// String human readable form
func (a *AliasOutput) String() string {
	ret := "AliasOutput:\n"

	return ret
}

// Compare the two outputs
func (a *AliasOutput) Compare(other Output) int {
	return bytes.Compare(a.Bytes(), other.Bytes())
}

// Update is disabled
func (a *AliasOutput) Update(other objectstorage.StorableObject) {
	panic("AliasOutput: storage object updates disabled")
}

// ObjectStorageKey a key
func (a *AliasOutput) ObjectStorageKey() []byte {
	return a.ID().Bytes()
}

// ObjectStorageValue binary form
func (a *AliasOutput) ObjectStorageValue() []byte {
	flags := a.mustFlags()
	ret := marshalutil.New().
		WriteByte(byte(AliasOutputType)).
		WriteByte(byte(flags)).
		WriteBytes(a.aliasOutputInner.AliasAddress.Bytes()).
		WriteBytes(a.aliasOutputInner.Balances.Bytes()).
		WriteBytes(a.aliasOutputInner.StateAddress.Bytes()).
		WriteUint32(a.aliasOutputInner.StateIndex)
	if flags.HasBit(flagAliasOutputStateDataPresent) {
		ret.WriteUint16(uint16(len(a.aliasOutputInner.StateData))).
			WriteBytes(a.aliasOutputInner.StateData)
	}
	if flags.HasBit(flagAliasOutputGovernanceMetadataPresent) {
		ret.WriteUint16(uint16(len(a.aliasOutputInner.GovernanceMetadata))).
			WriteBytes(a.aliasOutputInner.GovernanceMetadata)
	}
	if flags.HasBit(flagAliasOutputImmutableDataPresent) {
		ret.WriteUint16(uint16(len(a.aliasOutputInner.ImmutableData))).
			WriteBytes(a.aliasOutputInner.ImmutableData)
	}
	if flags.HasBit(flagAliasOutputGovernanceSet) {
		ret.WriteBytes(a.aliasOutputInner.GoverningAddress.Bytes())
	}
	if flags.HasBit(flagAliasOutputDelegationTimelockPresent) {
		ret.WriteTime(a.aliasOutputInner.DelegationTimelock)
	}
	return ret.Bytes()
}

// UnlockValid check unlock and validates chain
func (a *AliasOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock, inputs []Output) (bool, error) {
	// find the chained output in the tx
	chained, err := a.findChainedOutputAndCheckFork(tx)
	if err != nil {
		return false, err
	}
	switch blk := unlockBlock.(type) {
	case *SignatureUnlockBlock:
		// check signatures and validate transition
		if chained != nil {
			// chained output is present
			if chained.aliasOutputInner.IsGovernanceUpdate {
				// check if signature is valid against governing Address
				if !blk.AddressSignatureValid(a.GetGoverningAddress(), tx.Essence().Bytes()) {
					return false, errors.New("signature is invalid for governance unlock")
				}
			} else {
				// check if signature is valid against state Address
				if !blk.AddressSignatureValid(a.GetStateAddress(), tx.Essence().Bytes()) {
					return false, errors.New("signature is invalid for state unlock")
				}
			}
			// validate if transition passes the constraints
			if err := a.validateTransition(chained, tx); err != nil {
				return false, err
			}
		} else {
			// no chained output found. Alias is being destroyed?
			// check if governance is unlocked
			if !blk.AddressSignatureValid(a.GetGoverningAddress(), tx.Essence().Bytes()) {
				return false, errors.New("signature is invalid for chain output deletion")
			}
			// validate deletion constraint
			if err := a.validateDestroyTransitionNow(tx.Essence().Timestamp()); err != nil {
				return false, err
			}
		}
		return true, nil
	case *AliasUnlockBlock:
		// The referenced alias output should always be unlocked itself for state transition. But since the state Address
		// can be an AliasAddress, the referenced alias may be unlocked by in turn an other referenced alias. This can cause
		// circular dependency among the unlock blocks, that results in all of them being unlocked without anyone having to
		// provide a signature. As a consequence, the circular dependencies of the alias unlock blocks is checked before
		// the UnlockValid() methods are called on any unlock blocks. We assume in this function that there is no such
		// circular dependency.
		if chained != nil {
			// chained output is present
			if chained.aliasOutputInner.IsGovernanceUpdate {
				if valid, err := a.unlockedGovernanceTransitionByAliasIndex(tx, blk.AliasInputIndex(), inputs); !valid {
					return false, errors.Errorf("referenced alias does not unlock alias for governance transition: %w", err)
				}
			} else {
				if valid, err := a.unlockedStateTransitionByAliasIndex(tx, blk.AliasInputIndex(), inputs); !valid {
					return false, errors.Errorf("referenced alias does not unlock alias for state transition: %w", err)
				}
			}
			// validate if transition passes the constraints
			if err := a.validateTransition(chained, tx); err != nil {
				return false, err
			}
		} else {
			// no chained output is present. Alias being destroyed?
			// check if alias is unlocked for governance transition by the referenced
			if valid, err := a.unlockedGovernanceTransitionByAliasIndex(tx, blk.AliasInputIndex(), inputs); !valid {
				return false, errors.Errorf("referenced alias does not unlock alias for governance transition: %w", err)
			}
			// validate deletion constraint
			if err := a.validateDestroyTransitionNow(tx.Essence().Timestamp()); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	return false, errors.New("unsupported unlock block type")
}

// UpdateMintingColor replaces minting code with computed color code, and calculates the alias Address if it is a
// freshly minted alias output
func (a *AliasOutput) UpdateMintingColor() Output {
	coloredBalances := a.Balances().Map()
	if mintedCoins, mintedCoinsExist := coloredBalances[ColorMint]; mintedCoinsExist {
		delete(coloredBalances, ColorMint)
		coloredBalances[Color(blake2b.Sum256(a.ID().Bytes()))] = mintedCoins
	}
	updatedOutput := a.clone()
	_ = updatedOutput.SetBalances(coloredBalances)
	updatedOutput.SetID(a.ID())

	if a.IsOrigin() {
		updatedOutput.SetAliasAddress(NewAliasAddress(a.ID().Bytes()))
	}

	return updatedOutput
}

// checkBasicValidity checks basic validity of the output
func (a *AliasOutput) checkBasicValidity() error {
	if !IsAboveDustThreshold(a.aliasOutputInner.Balances.Map()) {
		return errors.New("AliasOutput: Balances are below dust threshold")
	}
	if a.aliasOutputInner.StateAddress == nil {
		return errors.New("AliasOutput: state Address must not be nil")
	}
	if a.IsOrigin() && a.aliasOutputInner.StateIndex != 0 {
		return errors.New("AliasOutput: origin must have stateIndex == 0")
	}
	// a.aliasAddress is not set if the output is origin. It is only set after the output has been included in a tx, and
	// its outputID is known. To cover this edge case, TransactionFromMarshalUtil() performs the two checks below after
	// the ID has been set.
	if a.GetStateAddress().Equals(&a.aliasOutputInner.AliasAddress) {
		return errors.New("state Address cannot be the output's own alias Address")
	}
	if a.GetGoverningAddress().Equals(&a.aliasOutputInner.AliasAddress) {
		return errors.New("governing Address cannot be the output's own alias Address")
	}
	if len(a.aliasOutputInner.StateData) > MaxOutputPayloadSize {
		return errors.Errorf("AliasOutput: size of the stateData (%d) exceeds maximum allowed (%d)",
			len(a.aliasOutputInner.StateData), MaxOutputPayloadSize)
	}
	if len(a.aliasOutputInner.GovernanceMetadata) > MaxOutputPayloadSize {
		return errors.Errorf("AliasOutput: size of the governance metadata (%d) exceeds maximum allowed (%d)",
			len(a.aliasOutputInner.GovernanceMetadata), MaxOutputPayloadSize)
	}
	if len(a.aliasOutputInner.ImmutableData) > MaxOutputPayloadSize {
		return errors.Errorf("AliasOutput: size of the immutableData (%d) exceeds maximum allowed (%d)",
			len(a.aliasOutputInner.ImmutableData), MaxOutputPayloadSize)
	}
	if !a.aliasOutputInner.IsDelegated && !a.aliasOutputInner.DelegationTimelock.IsZero() {
		return errors.Errorf("AliasOutput: delegation Timelock is present, but output is not delegated")
	}
	return nil
}

// mustValidate internal validity assertion
func (a *AliasOutput) mustValidate() {
	if err := a.checkBasicValidity(); err != nil {
		panic(err)
	}
}

// mustFlags produces flags for serialization
func (a *AliasOutput) mustFlags() bitmask.BitMask {
	a.mustValidate()
	var ret bitmask.BitMask
	if a.aliasOutputInner.IsOrigin {
		ret = ret.SetBit(flagAliasOutputIsOrigin)
	}
	if a.aliasOutputInner.IsGovernanceUpdate {
		ret = ret.SetBit(flagAliasOutputGovernanceUpdate)
	}
	if len(a.aliasOutputInner.ImmutableData) > 0 {
		ret = ret.SetBit(flagAliasOutputImmutableDataPresent)
	}
	if len(a.aliasOutputInner.StateData) > 0 {
		ret = ret.SetBit(flagAliasOutputStateDataPresent)
	}
	if a.aliasOutputInner.GoverningAddress != nil {
		ret = ret.SetBit(flagAliasOutputGovernanceSet)
	}
	if len(a.aliasOutputInner.GovernanceMetadata) > 0 {
		ret = ret.SetBit(flagAliasOutputGovernanceMetadataPresent)
	}
	if a.aliasOutputInner.IsDelegated {
		ret = ret.SetBit(flagAliasOutputDelegationConstraint)
	}
	if !a.aliasOutputInner.DelegationTimelock.IsZero() {
		ret = ret.SetBit(flagAliasOutputDelegationTimelockPresent)
	}
	return ret
}

// findChainedOutputAndCheckFork finds corresponding chained output.
// If it is not unique, returns an error
// If there's no such output, return nil and no error
func (a *AliasOutput) findChainedOutputAndCheckFork(tx *Transaction) (*AliasOutput, error) {
	var ret *AliasOutput
	aliasAddress := a.GetAliasAddress()
	for _, out := range tx.Essence().Outputs() {
		if out.Type() != AliasOutputType {
			continue
		}
		outAlias := out.(*AliasOutput)
		if !aliasAddress.Equals(outAlias.GetAliasAddress()) {
			continue
		}
		if ret != nil {
			return nil, errors.Errorf("duplicated alias output: %s", aliasAddress.String())
		}
		ret = outAlias
	}
	return ret, nil
}

// equalColoredBalances utility to compare colored Balances
func equalColoredBalances(b1, b2 *ColoredBalances) bool {
	allColors := make(map[Color]bool)
	b1.ForEach(func(col Color, bal uint64) bool {
		allColors[col] = true
		return true
	})
	b2.ForEach(func(col Color, bal uint64) bool {
		allColors[col] = true
		return true
	})
	for col := range allColors {
		v1, ok1 := b1.Get(col)
		v2, ok2 := b2.Get(col)
		if ok1 != ok2 || v1 != v2 {
			return false
		}
	}
	return true
}

// IsAboveDustThreshold internal utility to check if Balances pass dust constraint
func IsAboveDustThreshold(m map[Color]uint64) bool {
	if iotas, ok := m[ColorIOTA]; ok && iotas >= DustThresholdAliasOutputIOTA {
		return true
	}
	return false
}

// IsExactDustMinimum checks if colored Balances are exactly what is required by dust constraint
func IsExactDustMinimum(b *ColoredBalances) bool {
	bals := b.Map()
	if len(bals) != 1 {
		return false
	}
	bal, ok := bals[ColorIOTA]
	if !ok || bal != DustThresholdAliasOutputIOTA {
		return false
	}
	return true
}

// validateTransition enforces transition constraints between input and output chain outputs
func (a *AliasOutput) validateTransition(chained *AliasOutput, tx *Transaction) error {
	// enforce immutability of alias Address and immutable data
	if !a.GetAliasAddress().Equals(chained.GetAliasAddress()) {
		return errors.New("AliasOutput: can't modify alias Address")
	}
	if !bytes.Equal(a.aliasOutputInner.ImmutableData, chained.aliasOutputInner.ImmutableData) {
		return errors.New("AliasOutput: can't modify immutable data")
	}
	// depending on update type, enforce valid transition
	if chained.aliasOutputInner.IsGovernanceUpdate {
		// GOVERNANCE TRANSITION
		// should not modify state data
		if !bytes.Equal(a.aliasOutputInner.StateData, chained.aliasOutputInner.StateData) {
			return errors.New("AliasOutput: state data is not unlocked for modification")
		}
		// should not modify state index
		if a.aliasOutputInner.StateIndex != chained.aliasOutputInner.StateIndex {
			return errors.New("AliasOutput: state index is not unlocked for modification")
		}
		// should not modify tokens
		if !equalColoredBalances(a.aliasOutputInner.Balances, chained.aliasOutputInner.Balances) {
			return errors.New("AliasOutput: tokens are not unlocked for modification")
		}
		// if delegation Timelock is set and active, governance transition is invalid
		// It means delegating party can't take funds back before Timelock deadline
		if a.IsDelegated() && a.DelegationTimeLockedNow(tx.Essence().Timestamp()) {
			return errors.Errorf("AliasOutput: governance transition not allowed until %s, transaction timestamp is: %s",
				a.aliasOutputInner.DelegationTimelock.String(), tx.Essence().Timestamp().String())
		}
		// can modify state Address
		// can modify governing Address
		// can modify governance metadata
		// can modify delegation status
		// can modify delegation Timelock
	} else {
		// STATE TRANSITION
		// can modify state data
		// should increment state index
		if a.aliasOutputInner.StateIndex+1 != chained.aliasOutputInner.StateIndex {
			return errors.Errorf("AliasOutput: expected state index is %d found %d", a.aliasOutputInner.StateIndex+1, chained.aliasOutputInner.StateIndex)
		}
		// can modify tokens
		// should not modify stateAddress
		if !a.aliasOutputInner.StateAddress.Equals(chained.aliasOutputInner.StateAddress) {
			return errors.New("AliasOutput: state Address is not unlocked for modification")
		}
		// should not modify governing Address
		if a.IsSelfGoverned() != chained.IsSelfGoverned() ||
			(a.aliasOutputInner.GoverningAddress != nil && !a.aliasOutputInner.GoverningAddress.Equals(chained.aliasOutputInner.GoverningAddress)) {
			return errors.New("AliasOutput: governing Address is not unlocked for modification")
		}
		// should not modify governance metadata
		if !bytes.Equal(a.aliasOutputInner.GovernanceMetadata, chained.aliasOutputInner.GovernanceMetadata) {
			return errors.New("AliasOutput: governance metadata is not unlocked for modification")
		}
		// should not modify token Balances if delegation constraint is set
		if a.IsDelegated() && !equalColoredBalances(a.aliasOutputInner.Balances, chained.aliasOutputInner.Balances) {
			return errors.New("AliasOutput: delegated output funds can't be changed")
		}
		// should not modify delegation status in state transition
		if a.IsDelegated() != chained.IsDelegated() {
			return errors.New("AliasOutput: delegation status can't be changed")
		}
		// should not modify delegation Timelock
		if !a.DelegationTimelock().Equal(chained.DelegationTimelock()) {
			return errors.New("AliasOutput: delegation Timelock can't be changed")
		}
		// can only be accepted:
		//    - if no delegation Timelock, state update can happen whenever
		//    - if delegation Timelock is present, need to check if the Timelock is active, otherwise state update not allowed
		if a.IsDelegated() && !a.DelegationTimelock().IsZero() && !a.DelegationTimeLockedNow(tx.Essence().Timestamp()) {
			return errors.Errorf("AliasOutput: state transition of delegated output not allowed after %s, transaction timestamp is %s",
				a.aliasOutputInner.DelegationTimelock.String(), tx.Essence().Timestamp().String())
		}
	}
	return nil
}

// validateDestroyTransitionNow check validity if input is not chained (destroyed)
func (a *AliasOutput) validateDestroyTransitionNow(nowis time.Time) error {
	if !a.IsDelegated() && !IsExactDustMinimum(a.aliasOutputInner.Balances) {
		// if the output is delegated, it can be destroyed with more than minimum balance
		return errors.New("AliasOutput: didn't find chained output and there are more tokens then upper limit for non-delegated alias destruction")
	}
	if a.IsDelegated() && a.DelegationTimeLockedNow(nowis) {
		return errors.New("AliasOutput: didn't find expected chained output for delegated output")
	}
	return nil
}

// unlockedGovernanceTransitionByAliasIndex unlocks one step of alias dereference for governance transition
func (a *AliasOutput) unlockedGovernanceTransitionByAliasIndex(tx *Transaction, refIndex uint16, inputs []Output) (bool, error) {
	// when output is self governed, a.GetGoverningAddress() returns the state Address
	if a.GetGoverningAddress().Type() != AliasAddressType {
		return false, errors.New("AliasOutput: expected governing Address of AliasAddress type")
	}
	if int(refIndex) > len(inputs) {
		return false, errors.New("AliasOutput: wrong alias reference index")
	}
	refInput, ok := inputs[refIndex].(*AliasOutput)
	if !ok {
		return false, errors.New("AliasOutput: the referenced output is not of AliasOutput type")
	}
	if !refInput.GetAliasAddress().Equals(a.GetGoverningAddress().(*AliasAddress)) {
		return false, errors.New("AliasOutput: wrong alias reference Address")
	}
	// the referenced output must be unlocked for state update
	return !refInput.hasToBeUnlockedForGovernanceUpdate(tx), nil
}

// unlockedStateTransitionByAliasIndex unlocks one step of alias dereference for state transition
func (a *AliasOutput) unlockedStateTransitionByAliasIndex(tx *Transaction, refIndex uint16, inputs []Output) (bool, error) {
	// when output is self governed, a.GetGoverningAddress() returns the state Address
	if a.GetStateAddress().Type() != AliasAddressType {
		return false, errors.New("AliasOutput: expected state Address of AliasAddress type")
	}
	if int(refIndex) > len(inputs) {
		return false, errors.New("AliasOutput: wrong alias reference index")
	}
	refInput, ok := inputs[refIndex].(*AliasOutput)
	if !ok {
		return false, errors.New("AliasOutput: the referenced output is not of AliasOutput type")
	}
	if !refInput.GetAliasAddress().Equals(a.GetStateAddress().(*AliasAddress)) {
		return false, errors.New("AliasOutput: wrong alias reference Address")
	}
	// the referenced output must be unlocked for state update
	return !refInput.hasToBeUnlockedForGovernanceUpdate(tx), nil
}

// hasToBeUnlockedForGovernanceUpdate finds chained output and checks if it is unlocked governance flags set
// If there's no chained output it means governance unlock is required to destroy the output
func (a *AliasOutput) hasToBeUnlockedForGovernanceUpdate(tx *Transaction) bool {
	chained, err := a.findChainedOutputAndCheckFork(tx)
	if err != nil {
		return false
	}
	if chained == nil {
		// the corresponding chained output not found, it means it is being destroyed,
		// for this we need governance unlock
		return true
	}
	return chained.aliasOutputInner.IsGovernanceUpdate
}

// code contract (make sure the type implements all required methods)
var _ Output = &AliasOutput{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ExtendedLockedOutput /////////////////////////////////////////////////////////////////////////////////////////////////

// ExtendedLockedOutput is an Extension of SigLockedColoredOutput. If extended options not enabled,
// it behaves as SigLockedColoredOutput.
// In addition it has options:
// - fallback Address and fallback timeout
// - can be unlocked by AliasUnlockBlock (if Address is of AliasAddress type)
// - can be time locked until deadline
// - data Payload for arbitrary metadata (size limits apply)
type ExtendedLockedOutput struct {
	extendedLockedOutputInner `serialize:"unpack"`

	objectstorage.StorableObjectFlags
}
type extendedLockedOutputInner struct {
	Id       OutputID `serialize:"true"`
	idMutex  sync.RWMutex
	Balances *ColoredBalances `serialize:"true"`
	Address  Address          `serialize:"true"` // any Address type

	// optional part
	// Fallback Address after timeout. If nil, fallback action not set
	FallbackAddress  Address   `serialize:"true"`
	FallbackDeadline time.Time `serialize:"true"`

	// Deadline since when output can be unlocked
	Timelock time.Time `serialize:"true"`

	// any attached data (subject to size limits)
	Payload []byte `serialize:"true"`
}

const (
	flagExtendedLockedOutputFallbackPresent = uint(iota)
	flagExtendedLockedOutputTimeLockPresent
	flagExtendedLockedOutputPayloadPresent
)

// NewExtendedLockedOutput is the constructor for a ExtendedLockedOutput.
func NewExtendedLockedOutput(balances map[Color]uint64, address Address) *ExtendedLockedOutput {
	return &ExtendedLockedOutput{
		extendedLockedOutputInner: extendedLockedOutputInner{
			Balances: NewColoredBalances(balances),
			Address:  address.Clone(),
		},
	}
}

// WithFallbackOptions adds fallback options to the output and returns the updated version.
func (o *ExtendedLockedOutput) WithFallbackOptions(addr Address, deadline time.Time) *ExtendedLockedOutput {
	if addr != nil {
		o.extendedLockedOutputInner.FallbackAddress = addr.Clone()
	} else {
		o.extendedLockedOutputInner.FallbackAddress = nil
	}
	o.extendedLockedOutputInner.FallbackDeadline = deadline
	return o
}

// WithTimeLock adds Timelock to the output and returns the updated version.
func (o *ExtendedLockedOutput) WithTimeLock(timelock time.Time) *ExtendedLockedOutput {
	o.extendedLockedOutputInner.Timelock = timelock
	return o
}

// SetPayload sets the Payload field of the output.
func (o *ExtendedLockedOutput) SetPayload(data []byte) error {
	if len(data) > MaxOutputPayloadSize {
		return errors.Errorf("ExtendedLockedOutput: data Payload size (%d bytes) is bigger than maximum allowed (%d bytes)", len(data), MaxOutputPayloadSize)
	}
	o.extendedLockedOutputInner.Payload = make([]byte, len(data))
	copy(o.extendedLockedOutputInner.Payload, data)
	return nil
}

// compressFlags examines the optional fields of the output and returns the combined flags as a byte.
func (o *ExtendedLockedOutput) compressFlags() bitmask.BitMask {
	var ret bitmask.BitMask
	if o.extendedLockedOutputInner.FallbackAddress != nil {
		ret = ret.SetBit(flagExtendedLockedOutputFallbackPresent)
	}
	if !o.extendedLockedOutputInner.Timelock.IsZero() {
		ret = ret.SetBit(flagExtendedLockedOutputTimeLockPresent)
	}
	if len(o.extendedLockedOutputInner.Payload) > 0 {
		ret = ret.SetBit(flagExtendedLockedOutputPayloadPresent)
	}
	return ret
}

// ID returns the identifier of the Output that is used to Address the Output in the UTXODAG.
func (o *ExtendedLockedOutput) ID() OutputID {
	o.extendedLockedOutputInner.idMutex.RLock()
	defer o.extendedLockedOutputInner.idMutex.RUnlock()

	return o.extendedLockedOutputInner.Id
}

// SetID allows to set the identifier of the Output. We offer a setter for the property since Outputs that are
// created to become part of a transaction usually do not have an identifier, yet as their identifier depends on
// the TransactionID that is only determinable after the Transaction has been fully constructed. The ID is therefore
// only accessed when the Output is supposed to be persisted by the node.
func (o *ExtendedLockedOutput) SetID(outputID OutputID) Output {
	o.extendedLockedOutputInner.idMutex.Lock()
	defer o.extendedLockedOutputInner.idMutex.Unlock()

	o.extendedLockedOutputInner.Id = outputID

	return o
}

// Type returns the type of the Output which allows us to generically handle Outputs of different types.
func (o *ExtendedLockedOutput) Type() OutputType {
	return ExtendedLockedOutputType
}

// Balances returns the funds that are associated with the Output.
func (o *ExtendedLockedOutput) Balances() *ColoredBalances {
	return o.extendedLockedOutputInner.Balances
}

// UnlockValid determines if the given Transaction and the corresponding UnlockBlock are allowed to spend the Output.
func (o *ExtendedLockedOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock, inputs []Output) (unlockValid bool, err error) {
	if o.TimeLockedNow(tx.Essence().Timestamp()) {
		return false, nil
	}
	addr := o.UnlockAddressNow(tx.Essence().Timestamp())

	switch blk := unlockBlock.(type) {
	case *SignatureUnlockBlock:
		// unlocking by signature
		unlockValid = blk.AddressSignatureValid(addr, tx.Essence().Bytes())

	case *AliasUnlockBlock:
		// unlocking by alias reference. The unlock is valid if:
		// - referenced alias output has same alias Address
		// - it is not unlocked for governance
		if addr.Type() != AliasAddressType {
			return false, errors.Errorf("ExtendedLockedOutput: %s Address can't be unlocked by alias reference", addr.Type().String())
		}
		refAliasOutput, isAlias := inputs[blk.AliasInputIndex()].(*AliasOutput)
		if !isAlias {
			return false, errors.New("ExtendedLockedOutput: referenced input must be AliasOutput")
		}
		if !addr.Equals(refAliasOutput.GetAliasAddress()) {
			return false, errors.New("ExtendedLockedOutput: wrong alias referenced")
		}
		unlockValid = !refAliasOutput.hasToBeUnlockedForGovernanceUpdate(tx)

	default:
		err = errors.Errorf("ExtendedLockedOutput: unsupported unlock block type: %w", cerrors.ErrParseBytesFailed)
	}
	return unlockValid, err
}

// Address returns the Address that the Output is associated to.
func (o *ExtendedLockedOutput) Address() Address {
	return o.extendedLockedOutputInner.Address
}

// FallbackAddress returns the fallback Address that the Output is associated to.
func (o *ExtendedLockedOutput) FallbackAddress() (addy Address) {
	if o.extendedLockedOutputInner.FallbackAddress == nil {
		return
	}
	return o.extendedLockedOutputInner.FallbackAddress
}

// Input returns an Input that references the Output.
func (o *ExtendedLockedOutput) Input() Input {
	if o.ID() == EmptyOutputID {
		panic("ExtendedLockedOutput: Outputs that haven't been assigned an ID, yet cannot be converted to an Input")
	}

	return NewUTXOInput(o.ID())
}

// Clone creates a copy of the Output.
func (o *ExtendedLockedOutput) Clone() Output {
	ret := &ExtendedLockedOutput{
		extendedLockedOutputInner: extendedLockedOutputInner{
			Balances: o.extendedLockedOutputInner.Balances.Clone(),
			Address:  o.extendedLockedOutputInner.Address.Clone(),
		},
	}
	copy(ret.Id[:], o.extendedLockedOutputInner.Id[:])
	if o.extendedLockedOutputInner.FallbackAddress != nil {
		ret.extendedLockedOutputInner.FallbackAddress = o.extendedLockedOutputInner.FallbackAddress.Clone()
	}
	if !o.extendedLockedOutputInner.FallbackDeadline.IsZero() {
		ret.FallbackDeadline = o.extendedLockedOutputInner.FallbackDeadline
	}
	if !o.extendedLockedOutputInner.Timelock.IsZero() {
		ret.Timelock = o.extendedLockedOutputInner.Timelock
	}
	if o.extendedLockedOutputInner.Payload != nil {
		ret.Payload = make([]byte, len(o.extendedLockedOutputInner.Payload))
		copy(ret.Payload, o.extendedLockedOutputInner.Payload)
	}
	return ret
}

// UpdateMintingColor replaces the ColorMint in the Balances of the Output with the hash of the OutputID. It returns a
// copy of the original Output with the modified Balances.
func (o *ExtendedLockedOutput) UpdateMintingColor() Output {
	coloredBalances := o.Balances().Map()
	if mintedCoins, mintedCoinsExist := coloredBalances[ColorMint]; mintedCoinsExist {
		delete(coloredBalances, ColorMint)
		coloredBalances[Color(blake2b.Sum256(o.ID().Bytes()))] = mintedCoins
	}
	updatedOutput := NewExtendedLockedOutput(coloredBalances, o.Address()).
		WithFallbackOptions(o.extendedLockedOutputInner.FallbackAddress, o.extendedLockedOutputInner.FallbackDeadline).
		WithTimeLock(o.extendedLockedOutputInner.Timelock)
	if err := updatedOutput.SetPayload(o.extendedLockedOutputInner.Payload); err != nil {
		panic(errors.Errorf("UpdateMintingColor: %v", err))
	}
	updatedOutput.SetID(o.ID())

	return updatedOutput
}

// Bytes returns a marshaled version of the Output.
func (o *ExtendedLockedOutput) Bytes() []byte {
	return o.ObjectStorageValue()
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (o *ExtendedLockedOutput) Update(objectstorage.StorableObject) {
	panic("ExtendedLockedOutput: updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (o *ExtendedLockedOutput) ObjectStorageKey() []byte {
	return o.Id.Bytes()
}

// ObjectStorageValue marshals the Output into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (o *ExtendedLockedOutput) ObjectStorageValue() []byte {
	flags := o.compressFlags()
	ret := marshalutil.New().
		WriteByte(byte(ExtendedLockedOutputType)).
		WriteBytes(o.extendedLockedOutputInner.Balances.Bytes()).
		WriteBytes(o.extendedLockedOutputInner.Address.Bytes()).
		WriteByte(byte(flags))
	if flags.HasBit(flagExtendedLockedOutputFallbackPresent) {
		ret.WriteBytes(o.extendedLockedOutputInner.FallbackAddress.Bytes()).
			WriteTime(o.extendedLockedOutputInner.FallbackDeadline)
	}
	if flags.HasBit(flagExtendedLockedOutputTimeLockPresent) {
		ret.WriteTime(o.extendedLockedOutputInner.Timelock)
	}
	if flags.HasBit(flagExtendedLockedOutputPayloadPresent) {
		ret.WriteUint16(uint16(len(o.extendedLockedOutputInner.Payload))).
			WriteBytes(o.extendedLockedOutputInner.Payload)
	}
	return ret.Bytes()
}

// Compare offers a comparator for Outputs which returns -1 if the other Output is bigger, 1 if it is smaller and 0 if
// they are the same.
func (o *ExtendedLockedOutput) Compare(other Output) int {
	return bytes.Compare(o.Bytes(), other.Bytes())
}

// String returns a human readable version of the Output.
func (o *ExtendedLockedOutput) String() string {
	return stringify.Struct("ExtendedLockedOutput",
		stringify.StructField("Id", o.ID()),
		stringify.StructField("Address", o.extendedLockedOutputInner.Address),
		stringify.StructField("Balances", o.extendedLockedOutputInner.Balances),
		stringify.StructField("FallbackAddress", o.extendedLockedOutputInner.FallbackAddress),
		stringify.StructField("FallbackDeadline", o.extendedLockedOutputInner.FallbackDeadline),
		stringify.StructField("Timelock", o.extendedLockedOutputInner.Timelock),
	)
}

// GetPayload return a data Payload associated with the output
func (o *ExtendedLockedOutput) GetPayload() []byte {
	return o.extendedLockedOutputInner.Payload
}

// TimeLock is a time after which output can be unlocked
func (o *ExtendedLockedOutput) TimeLock() time.Time {
	return o.extendedLockedOutputInner.Timelock
}

// TimeLockedNow checks if output is unlocked for the specific moment
func (o *ExtendedLockedOutput) TimeLockedNow(nowis time.Time) bool {
	return o.TimeLock().After(nowis)
}

// FallbackOptions returns fallback options of the output. The Address is nil if fallback options are not set
func (o *ExtendedLockedOutput) FallbackOptions() (Address, time.Time) {
	return o.extendedLockedOutputInner.FallbackAddress, o.extendedLockedOutputInner.FallbackDeadline
}

// UnlockAddressNow return unlock Address which is valid for the specific moment of time
func (o *ExtendedLockedOutput) UnlockAddressNow(nowis time.Time) Address {
	if o.extendedLockedOutputInner.FallbackAddress == nil {
		return o.extendedLockedOutputInner.Address
	}
	if nowis.After(o.extendedLockedOutputInner.FallbackDeadline) {
		return o.extendedLockedOutputInner.FallbackAddress
	}
	return o.extendedLockedOutputInner.Address
}

// code contract (make sure the type implements all required methods)
var _ Output = &ExtendedLockedOutput{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
