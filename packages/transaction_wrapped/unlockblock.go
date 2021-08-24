package txwrapped

import (
	"strconv"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
)

// region UnlockBlockType //////////////////////////////////////////////////////////////////////////////////////////////

const (
	// SignatureUnlockBlockType represents the type of a SignatureUnlockBlock.
	SignatureUnlockBlockType UnlockBlockType = iota

	// ReferenceUnlockBlockType represents the type of a ReferenceUnlockBlock.
	ReferenceUnlockBlockType

	// AliasUnlockBlockType represents the type of a AliasUnlockBlock
	AliasUnlockBlockType
)

// UnlockBlockType represents the type of the UnlockBlock. Different types of UnlockBlocks can unlock different types of
// Outputs.
type UnlockBlockType uint8

// String returns a human readable representation of the UnlockBlockType.
func (a UnlockBlockType) String() string {
	return [...]string{
		"SignatureUnlockBlockType",
		"ReferenceUnlockBlockType",
		"AliasUnlockBlockType",
	}[a]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UnlockBlock //////////////////////////////////////////////////////////////////////////////////////////////////

// UnlockBlock represents a generic interface to Address the different kinds of unlock information that are required to
// authorize the spending of different Output types.
type UnlockBlock interface {
	// Type returns the UnlockBlockType of the UnlockBlock.
	Type() UnlockBlockType

	// Bytes returns a marshaled version of the UnlockBlock.
	Bytes() []byte

	// String returns a human readable version of the UnlockBlock.
	String() string
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UnlockBlocks /////////////////////////////////////////////////////////////////////////////////////////////////

// UnlockBlocks is slice of UnlockBlocks that offers additional methods for easier marshaling and unmarshaling.
type UnlockBlocks []UnlockBlock

// Bytes returns a marshaled version of the UnlockBlocks.
func (u UnlockBlocks) Bytes() []byte {
	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint16(uint16(len(u)))
	for _, unlockBlock := range u {
		marshalUtil.WriteBytes(unlockBlock.Bytes())
	}

	return marshalUtil.Bytes()
}

// String returns a human readable version of the UnlockBlocks.
func (u UnlockBlocks) String() string {
	structBuilder := stringify.StructBuilder("UnlockBlocks")
	for i, unlockBlock := range u {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), unlockBlock))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SignatureUnlockBlock /////////////////////////////////////////////////////////////////////////////////////////

// SignatureUnlockBlock represents an UnlockBlock that contains a Signature for an Address.

type SignatureUnlockBlock struct {
	signatureUnlockBlockInner `serialize:"unpack"`
}

type signatureUnlockBlockInner struct {
	Signature Signature `serialize:"true"`
}

// NewSignatureUnlockBlock is the constructor for SignatureUnlockBlock objects.
func NewSignatureUnlockBlock(signature Signature) *SignatureUnlockBlock {
	return &SignatureUnlockBlock{signatureUnlockBlockInner{
		Signature: signature,
	},
	}
}

// AddressSignatureValid returns true if the UnlockBlock correctly signs the given Address.
func (s *SignatureUnlockBlock) AddressSignatureValid(address Address, signedData []byte) bool {
	return s.AddressSignatureValid(address, signedData)
}

// Type returns the UnlockBlockType of the UnlockBlock.
func (s *SignatureUnlockBlock) Type() UnlockBlockType {
	return SignatureUnlockBlockType
}

// Bytes returns a marshaled version of the UnlockBlock.
func (s *SignatureUnlockBlock) Bytes() []byte {
	return byteutils.ConcatBytes([]byte{byte(SignatureUnlockBlockType)}, s.Bytes())
}

// String returns a human readable version of the UnlockBlock.
func (s *SignatureUnlockBlock) String() string {
	return stringify.Struct("SignatureUnlockBlock",
		stringify.StructField("signature", s.Signature),
	)
}

// Signature return the signature itself
func (s *SignatureUnlockBlock) Signature() Signature {
	return s.signatureUnlockBlockInner.Signature
}

// code contract (make sure the type implements all required methods)
var _ UnlockBlock = &SignatureUnlockBlock{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ReferenceUnlockBlock /////////////////////////////////////////////////////////////////////////////////////////

// ReferenceUnlockBlock defines an UnlockBlock which references a previous UnlockBlock (which must not be another
// ReferenceUnlockBlock).
type ReferenceUnlockBlock struct {
	referenceUnlockBlockInner `serialize:"unpack"`
}
type referenceUnlockBlockInner struct {
	ReferencedIndex uint16 `serialize:"true"`
}

// NewReferenceUnlockBlock is the constructor for ReferenceUnlockBlocks.
func NewReferenceUnlockBlock(referencedIndex uint16) *ReferenceUnlockBlock {
	return &ReferenceUnlockBlock{
		referenceUnlockBlockInner{
			ReferencedIndex: referencedIndex,
		},
	}
}

// ReferencedIndex returns the index of the referenced UnlockBlock.
func (r *ReferenceUnlockBlock) ReferencedIndex() uint16 {
	return r.referenceUnlockBlockInner.ReferencedIndex
}

// Type returns the UnlockBlockType of the UnlockBlock.
func (r *ReferenceUnlockBlock) Type() UnlockBlockType {
	return ReferenceUnlockBlockType
}

// Bytes returns a marshaled version of the UnlockBlock.
func (r *ReferenceUnlockBlock) Bytes() []byte {
	return marshalutil.New(1 + marshalutil.Uint16Size).
		WriteByte(byte(ReferenceUnlockBlockType)).
		WriteUint16(r.referenceUnlockBlockInner.ReferencedIndex).
		Bytes()
}

// String returns a human readable version of the UnlockBlock.
func (r *ReferenceUnlockBlock) String() string {
	return stringify.Struct("ReferenceUnlockBlock",
		stringify.StructField("referencedIndex", int(r.referenceUnlockBlockInner.ReferencedIndex)),
	)
}

// code contract (make sure the type implements all required methods)
var _ UnlockBlock = &ReferenceUnlockBlock{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AliasUnlockBlock /////////////////////////////////////////////////////////////////////////////////////////

// AliasUnlockBlock defines an UnlockBlock which contains an index of corresponding AliasOutput
type AliasUnlockBlock struct {
	aliasUnlockBlockInner `serialize:"unpack"`
}

type aliasUnlockBlockInner struct {
	ReferencedIndex uint16 `serialize:"true"`
}

// NewAliasUnlockBlock is the constructor for AliasUnlockBlocks.
func NewAliasUnlockBlock(chainInputIndex uint16) *AliasUnlockBlock {
	return &AliasUnlockBlock{
		aliasUnlockBlockInner{
			ReferencedIndex: chainInputIndex,
		},
	}
}

// AliasInputIndex returns the index of the input, the AliasOutput which contains AliasAddress
func (r *AliasUnlockBlock) AliasInputIndex() uint16 {
	return r.aliasUnlockBlockInner.ReferencedIndex
}

// Type returns the UnlockBlockType of the UnlockBlock.
func (r *AliasUnlockBlock) Type() UnlockBlockType {
	return AliasUnlockBlockType
}

// Bytes returns a marshaled version of the UnlockBlock.
func (r *AliasUnlockBlock) Bytes() []byte {
	return marshalutil.New(1 + marshalutil.Uint16Size).
		WriteByte(byte(AliasUnlockBlockType)).
		WriteUint16(r.aliasUnlockBlockInner.ReferencedIndex).
		Bytes()
}

// String returns a human readable version of the UnlockBlock.
func (r *AliasUnlockBlock) String() string {
	return stringify.Struct("AliasUnlockBlock",
		stringify.StructField("referencedIndex", int(r.aliasUnlockBlockInner.ReferencedIndex)),
	)
}

// code contract (make sure the type implements all required methods)
var _ UnlockBlock = &AliasUnlockBlock{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
