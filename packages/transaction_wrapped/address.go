package txwrapped

import (
	"bytes"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
)

// region AddressType //////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// ED25519AddressType represents an Address secured by the ED25519 signature scheme.
	ED25519AddressType AddressType = iota

	// BLSAddressType represents an Address secured by the BLS signature scheme.
	BLSAddressType

	// AliasAddressType represents ID used in AliasOutput and AliasLockOutput
	AliasAddressType
)

// AddressLength contains the length of an Address (type length = 1, Digest length = 32).
const AddressLength = 33

// AddressType represents the type of the Address (different types encode different signature schemes).
type AddressType byte

// String returns a human readable representation of the AddressType.
func (a AddressType) String() string {
	return [...]string{
		"AddressTypeED25519",
		"AddressTypeBLS",
		"AliasAddress",
	}[a]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Address //////////////////////////////////////////////////////////////////////////////////////////////////////

// Address is an interface for the different kind of Addresses that are supported by the ledger state.
type Address interface {
	// Type returns the AddressType of the Address.
	Type() AddressType

	// Digest returns the hashed version of the Addresses public key.
	Digest() []byte

	// Clone creates a copy of the Address.
	Clone() Address

	// Equals returns true if the two Addresses are equal.
	Equals(other Address) bool

	// Bytes returns a marshaled version of the Address.
	Bytes() []byte

	// Array returns an array of bytes that contains the marshaled version of the Address.
	Array() [AddressLength]byte

	// Base58 returns a base58 encoded version of the Address.
	Base58() string

	// String returns a human readable version of the Address for debug purposes.
	String() string
}

// AddressFromSignature returns Address corresponding to the signature if it has one (for ed25519 and BLS).
func AddressFromSignature(sig Signature) (Address, error) {
	switch s := sig.(type) {
	case *ED25519Signature:
		return NewED25519Address(s.PublicKey), nil
	case *BLSSignature:
		return NewBLSAddress(s.Signature.PublicKey.Bytes()), nil
	}
	return nil, errors.New("signature has no corresponding Address")
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ED25519Address ///////////////////////////////////////////////////////////////////////////////////////////////

// ED25519Address represents an Address that is secured by the ED25519 signature scheme.
type ED25519Address struct {
	eD25519AddressInner `serialize:"unpack"`
}
type eD25519AddressInner struct {
	Digest []byte `serialize:"true"`
}

// NewED25519Address creates a new ED25519Address from the given public key.
func NewED25519Address(publicKey ed25519.PublicKey) *ED25519Address {
	digest := blake2b.Sum256(publicKey[:])

	return &ED25519Address{
		eD25519AddressInner: eD25519AddressInner{
			Digest: digest[:],
		},
	}
}

// Type returns the AddressType of the Address.
func (e *ED25519Address) Type() AddressType {
	return ED25519AddressType
}

// Digest returns the hashed version of the Addresses public key.
func (e *ED25519Address) Digest() []byte {
	return e.eD25519AddressInner.Digest
}

// Clone creates a copy of the Address.
func (e *ED25519Address) Clone() Address {
	clonedDigest := make([]byte, len(e.eD25519AddressInner.Digest))
	copy(clonedDigest, e.eD25519AddressInner.Digest)

	return &ED25519Address{
		eD25519AddressInner: eD25519AddressInner{
			Digest: clonedDigest,
		}}
}

// Equals returns true if the two Addresses are equal.
func (e *ED25519Address) Equals(other Address) bool {
	return e.Type() == other.Type() && bytes.Equal(e.eD25519AddressInner.Digest, other.Digest())
}

// Bytes returns a marshaled version of the Address.
func (e *ED25519Address) Bytes() []byte {
	return byteutils.ConcatBytes([]byte{byte(ED25519AddressType)}, e.eD25519AddressInner.Digest)
}

// Array returns an array of bytes that contains the marshaled version of the Address.
func (e *ED25519Address) Array() (array [AddressLength]byte) {
	copy(array[:], e.Bytes())

	return
}

// Base58 returns a base58 encoded version of the Address.
func (e *ED25519Address) Base58() string {
	return base58.Encode(e.Bytes())
}

// String returns a human readable version of the addresses for debug purposes.
func (e *ED25519Address) String() string {
	return stringify.Struct("ED25519Address",
		stringify.StructField("Digest", e.Digest()),
		stringify.StructField("Base58", e.Base58()),
	)
}

// code contract (make sure the struct implements all required methods)
var _ Address = &ED25519Address{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BLSAddress ///////////////////////////////////////////////////////////////////////////////////////////////////

// BLSAddress represents an Address that is secured by the BLS signature scheme.
type BLSAddress struct {
	bLSAddressInner `serialize:"unpack"`
}
type bLSAddressInner struct {
	Digest []byte `serialize:"true"`
}

// NewBLSAddress creates a new BLSAddress from the given public key.
func NewBLSAddress(publicKey []byte) *BLSAddress {
	digest := blake2b.Sum256(publicKey)

	return &BLSAddress{
		bLSAddressInner: bLSAddressInner{
			Digest: digest[:],
		},
	}
}

// Type returns the AddressType of the Address.
func (b *BLSAddress) Type() AddressType {
	return BLSAddressType
}

// Digest returns the hashed version of the Addresses public key.
func (b *BLSAddress) Digest() []byte {
	return b.bLSAddressInner.Digest
}

// Clone creates a copy of the Address.
func (b *BLSAddress) Clone() Address {
	clonedDigest := make([]byte, len(b.bLSAddressInner.Digest))
	copy(clonedDigest, b.bLSAddressInner.Digest)

	return &BLSAddress{bLSAddressInner: bLSAddressInner{
		Digest: clonedDigest,
	},
	}
}

// Equals returns true if the two Addresses are equal.
func (b *BLSAddress) Equals(other Address) bool {
	return b.Type() == other.Type() && bytes.Equal(b.bLSAddressInner.Digest, other.Digest())
}

// Bytes returns a marshaled version of the Address.
func (b *BLSAddress) Bytes() []byte {
	return byteutils.ConcatBytes([]byte{byte(BLSAddressType)}, b.bLSAddressInner.Digest)
}

// Array returns an array of bytes that contains the marshaled version of the Address.
func (b *BLSAddress) Array() (array [AddressLength]byte) {
	copy(array[:], b.Bytes())

	return
}

// Base58 returns a base58 encoded version of the Address.
func (b *BLSAddress) Base58() string {
	return base58.Encode(b.Bytes())
}

// String returns a human readable version of the addresses for debug purposes.
func (b *BLSAddress) String() string {
	return stringify.Struct("BLSAddress",
		stringify.StructField("Digest", b.Digest()),
		stringify.StructField("Base58", b.Base58()),
	)
}

// code contract (make sure the struct implements all required methods)
var _ Address = &BLSAddress{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AliasAddress ///////////////////////////////////////////////////////////////////////////////////////////////////

// AliasAddressDigestSize defines the length of the alias Address Digest in bytes.
const AliasAddressDigestSize = 32

// AliasAddress represents a special type of Address which is not backed by a private key directly,
// but is indirectly backed by a private key defined by corresponding AliasOutput parameters
type AliasAddress struct {
	aliasAddressInner `serialize:"unpack"`
}

type aliasAddressInner struct {
	Digest [AliasAddressDigestSize]byte `serialize:"true"`
}

// NewAliasAddress creates a new AliasAddress from the given bytes used as seed.
// Normally the seed is an OutputID.
func NewAliasAddress(data []byte) *AliasAddress {
	return &AliasAddress{
		aliasAddressInner: aliasAddressInner{
			Digest: blake2b.Sum256(data),
		},
	}
}

// Type returns the AddressType of the Address.
func (a *AliasAddress) Type() AddressType {
	return AliasAddressType
}

// Digest returns the hashed version of the Addresses public key.
func (a *AliasAddress) Digest() []byte {
	return a.aliasAddressInner.Digest[:]
}

// Clone creates a copy of the Address.
func (a *AliasAddress) Clone() Address {
	return &AliasAddress{aliasAddressInner: aliasAddressInner{Digest: a.aliasAddressInner.Digest}}
}

// Bytes returns a marshaled version of the Address.
func (a *AliasAddress) Bytes() []byte {
	return byteutils.ConcatBytes([]byte{byte(AliasAddressType)}, a.aliasAddressInner.Digest[:])
}

// Array returns an array of bytes that contains the marshaled version of the Address.
func (a *AliasAddress) Array() (array [AddressLength]byte) {
	copy(array[:], a.Bytes())

	return
}

// Equals returns true if the two Addresses are equal.
func (a *AliasAddress) Equals(other Address) bool {
	return a.Type() == other.Type() && bytes.Equal(a.Digest(), other.Digest())
}

// Base58 returns a base58 encoded version of the Address.
func (a *AliasAddress) Base58() string {
	return base58.Encode(a.Bytes())
}

// String returns a human readable version of the addresses for debug purposes.
func (a *AliasAddress) String() string {
	return stringify.Struct("AliasAddress",
		stringify.StructField("Digest", a.Digest()),
		stringify.StructField("Base58", a.Base58()),
	)
}

// IsNil returns if the alias Address is zero value (uninitialized).
func (a *AliasAddress) IsNil() bool {
	return a.aliasAddressInner.Digest == [32]byte{}
}

// code contract (make sure the struct implements all required methods)
var _ Address = &AliasAddress{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
