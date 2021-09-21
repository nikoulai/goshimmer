package txwrapped

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"

	"github.com/iotaledger/goshimmer/packages/registry"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/iotaledger/hive.go/typeutils"
)

// region InputType ////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// UTXOInputType is the type of an Input that references an UTXO Output.
	UTXOInputType InputType = iota

	// MinInputCount defines the minimum amount of Inputs in a Transaction.
	MinInputCount = 1

	// MaxInputCount defines the maximum amount of Inputs in a Transaction.
	MaxInputCount = 127
)

// InputType represents the type of an Input.
type InputType uint8

// InputTypeNames contains the names of the existing InputTypes.
var InputTypeNames = [...]string{
	"UTXOInputType",
}

// String returns a human readable representation of the InputType.
func (i InputType) String() string {
	if i > InputType(len(InputTypeNames)-1) {
		return fmt.Sprintf("InputType(%X)", uint8(i))
	}

	return InputTypeNames[i]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Input ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Input is a generic interface for different kinds of Inputs.
type Input interface {
	// Type returns the type of the Input.
	Type() InputType

	// String returns a human readable version of the Input.
	String() string

	// Base58 returns the base58 encoded input.
	Base58() string

	// Compare offers a comparator for Inputs which returns -1 if other Input is bigger, 1 if it is smaller and 0 if they
	// are the same.
	Compare(other Input) int
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Inputs ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Inputs represents a collection of Inputs that ensures a deterministic order.
type Inputs []Input

// NewInputs returns a deterministically ordered collection of Inputs removing existing duplicates.
func NewInputs(optionalInputs ...Input) (inputs Inputs) {
	seenInputs := make(map[string]types.Empty)
	sortedInputs := make([]struct {
		input           Input
		inputSerialized []byte
	}, 0)

	// filter duplicates (store marshaled version so we don't need to marshal a second time during sort)
	for _, input := range optionalInputs {
		marshaledInput, err := registry.Manager.Serialize(input)
		if err != nil {
			panic(err)
		}
		marshaledInputAsString := typeutils.BytesToString(marshaledInput)

		if _, seenAlready := seenInputs[marshaledInputAsString]; seenAlready {
			continue
		}
		seenInputs[marshaledInputAsString] = types.Void

		sortedInputs = append(sortedInputs, struct {
			input           Input
			inputSerialized []byte
		}{input, marshaledInput})
	}

	// sort inputs
	sort.Slice(sortedInputs, func(i, j int) bool {
		return bytes.Compare(sortedInputs[i].inputSerialized, sortedInputs[j].inputSerialized) < 0
	})

	// create result
	inputs = make(Inputs, len(sortedInputs))
	for i, sortedInput := range sortedInputs {
		inputs[i] = sortedInput.input
	}

	return
}

// Clone creates a copy of the Inputs.
func (i Inputs) Clone() (clonedInputs Inputs) {
	clonedInputs = make(Inputs, len(i))
	copy(clonedInputs[:], i)

	return
}

// String returns a human readable version of the Inputs.
func (i Inputs) String() string {
	structBuilder := stringify.StructBuilder("Inputs")
	for i, input := range i {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), input))
	}

	return structBuilder.String()
}

// Strings returns the Inputs in the form []transactionID:index.
func (i Inputs) Strings() (result []string) {
	for _, input := range i {
		if input.Type() == UTXOInputType {
			outputID := input.(*UTXOInput).ReferencedOutputID()
			result = append(result, fmt.Sprintf("%s:%d", outputID.TransactionID().Base58(), outputID.OutputIndex()))
		}
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UTXOInput ////////////////////////////////////////////////////////////////////////////////////////////////////

// UTXOInput represents a reference to an Output in the UTXODAG.
type UTXOInput struct {
	uTXOInputInner `serialize:"unpack"`
}

type uTXOInputInner struct {
	ReferencedOutputID OutputID `serialize:"true"`
}

// NewUTXOInput is the constructor for UTXOInputs.
func NewUTXOInput(referencedOutputID OutputID) *UTXOInput {
	return &UTXOInput{
		uTXOInputInner{
			ReferencedOutputID: referencedOutputID,
		},
	}
}

// Type returns the type of the Input.
func (u *UTXOInput) Type() InputType {
	return UTXOInputType
}

// ReferencedOutputID returns the OutputID that this Input references.
func (u *UTXOInput) ReferencedOutputID() OutputID {
	return u.uTXOInputInner.ReferencedOutputID
}

// Base58 returns the base58 encoded referenced output ID of this input.
func (u *UTXOInput) Base58() string {
	return u.uTXOInputInner.ReferencedOutputID.Base58()
}

// Compare offers a comparator for Inputs which returns -1 if other Input is bigger, 1 if it is smaller and 0 if they
// are the same.
func (u *UTXOInput) Compare(other Input) int {
	uBytes, err := registry.Manager.Serialize(u)
	if err != nil {
		panic(err)
	}
	otherBytes, err := registry.Manager.Serialize(other)
	if err != nil {
		panic(err)
	}
	return bytes.Compare(uBytes, otherBytes)
}

// String returns a human readable version of the Input.
func (u *UTXOInput) String() string {
	return stringify.Struct("UTXOInput",
		stringify.StructField("referencedOutputID", u.uTXOInputInner.ReferencedOutputID),
	)
}

// code contract (make sure the struct implements all required methods)
var _ Input = &UTXOInput{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
