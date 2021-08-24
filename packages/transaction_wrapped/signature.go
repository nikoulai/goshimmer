package txwrapped

import (
	"bytes"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/crypto/bls"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
)

// region SignatureType ////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// ED25519SignatureType represents an ED25519 Signature.
	ED25519SignatureType SignatureType = iota

	// BLSSignatureType represents a BLS Signature.
	BLSSignatureType
)

// SignatureType represents the type of the signature scheme.
type SignatureType uint8

// String returns a human readable representation of the SignatureType.
func (s SignatureType) String() string {
	return [...]string{
		"ED25519SignatureType",
		"BLSSignatureType",
	}[s]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Signature ////////////////////////////////////////////////////////////////////////////////////////////////////

// Signature is an interface for the different kinds of Signatures that are supported by the ledger state.
type Signature interface {
	// Type returns the SignatureType of this Signature.
	Type() SignatureType

	// SignatureValid returns true if the Signature signs the given data.
	SignatureValid(data []byte) bool

	// AddressSignatureValid returns true if the Signature signs the given Address.
	AddressSignatureValid(address Address, data []byte) bool

	// Bytes returns a marshaled version of the Signature.
	Bytes() []byte

	// Base58 returns a base58 encoded version of the Signature.
	Base58() string

	// String returns a human readable version of the Signature.
	String() string
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ED25519Signature /////////////////////////////////////////////////////////////////////////////////////////////

// ED25519Signature represents a Signature created with the ed25519 signature scheme.
type ED25519Signature struct {
	eD25519SignatureInner `serialize:"true"`
}
type eD25519SignatureInner struct {
	PublicKey ed25519.PublicKey `serialize:"true"`
	Signature ed25519.Signature `serialize:"true"`
}

// NewED25519Signature is the constructor of an ED25519Signature.
func NewED25519Signature(publicKey ed25519.PublicKey, signature ed25519.Signature) *ED25519Signature {
	return &ED25519Signature{
		eD25519SignatureInner: eD25519SignatureInner{
			PublicKey: publicKey,
			Signature: signature,
		},
	}
}

// Type returns the SignatureType of this Signature.
func (e *ED25519Signature) Type() SignatureType {
	return ED25519SignatureType
}

// SignatureValid returns true if the Signature signs the given data.
func (e *ED25519Signature) SignatureValid(data []byte) bool {
	return e.PublicKey.VerifySignature(data, e.Signature)
}

// AddressSignatureValid returns true if the Signature signs the given Address.
func (e *ED25519Signature) AddressSignatureValid(address Address, data []byte) bool {
	if address.Type() != ED25519AddressType {
		return false
	}

	hashedPublicKey := blake2b.Sum256(e.PublicKey.Bytes())
	if !bytes.Equal(hashedPublicKey[:], address.Digest()) {
		return false
	}

	return e.SignatureValid(data)
}

// Bytes returns a marshaled version of the Signature.
func (e *ED25519Signature) Bytes() []byte {
	return byteutils.ConcatBytes([]byte{byte(ED25519SignatureType)}, e.PublicKey.Bytes(), e.Signature.Bytes())
}

// Base58 returns a base58 encoded version of the Signature.
func (e *ED25519Signature) Base58() string {
	return base58.Encode(e.Bytes())
}

// String returns a human readable version of the Signature.
func (e *ED25519Signature) String() string {
	return stringify.Struct("ED25519Signature",
		stringify.StructField("publicKey", e.PublicKey),
		stringify.StructField("signature", e.Signature),
	)
}

// code contract (make sure the type implements all required methods)
var _ Signature = &ED25519Signature{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BLSSignature /////////////////////////////////////////////////////////////////////////////////////////////////

// BLSSignature represents a Signature created with the BLS signature scheme.
type BLSSignature struct {
	bLSSignatureInner `serialize:"true"`
}
type bLSSignatureInner struct {
	Signature bls.SignatureWithPublicKey `serialize:"true"`
}

// NewBLSSignature is the constructor of a BLSSignature.
func NewBLSSignature(signature bls.SignatureWithPublicKey) *BLSSignature {
	return &BLSSignature{
		bLSSignatureInner: bLSSignatureInner{
			Signature: signature,
		},
	}
}

// Type returns the SignatureType of this Signature.
func (b *BLSSignature) Type() SignatureType {
	return BLSSignatureType
}

// SignatureValid returns true if the Signature signs the given data.
func (b *BLSSignature) SignatureValid(data []byte) bool {
	return b.Signature.IsValid(data)
}

// AddressSignatureValid returns true if the Signature signs the given Address.
func (b *BLSSignature) AddressSignatureValid(address Address, data []byte) bool {
	if address.Type() != BLSAddressType {
		return false
	}

	hashedPublicKey := blake2b.Sum256(b.Signature.PublicKey.Bytes())
	if !bytes.Equal(hashedPublicKey[:], address.Digest()) {
		return false
	}

	return b.SignatureValid(data)
}

// Bytes returns a marshaled version of the Signature.
func (b *BLSSignature) Bytes() []byte {
	return byteutils.ConcatBytes([]byte{byte(BLSSignatureType)}, b.Signature.Bytes())
}

// Base58 returns a base58 encoded version of the Signature.
func (b *BLSSignature) Base58() string {
	return base58.Encode(b.Bytes())
}

// String returns a human readable version of the Signature.
func (b *BLSSignature) String() string {
	return stringify.Struct("BLSSignature",
		stringify.StructField("publicKey", b.Signature.PublicKey),
		stringify.StructField("signature", b.Signature.Signature),
	)
}

// code contract (make sure the type implements all required methods)
var _ Signature = &BLSSignature{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
