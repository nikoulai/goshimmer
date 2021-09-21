package txwrapped

import (
	"bytes"
	"sort"

	"github.com/iotaledger/hive.go/datastructure/orderedmap"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
)

// region Color ////////////////////////////////////////////////////////////////////////////////////////////////////////

// ColorIOTA is the zero value of the Color and represents uncolored tokens.
var ColorIOTA = Color{}

// ColorMint represents a placeholder Color that indicates that tokens should be "colored" in their Output.
var ColorMint = Color{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}

// ColorLength represents the length of a Color (amount of bytes).
const ColorLength = 32

// Color represents a marker that is associated to a token balance and that can give tokens a certain "meaning".
type Color [ColorLength]byte

// Bytes marshals the Color into a sequence of bytes.
func (c Color) Bytes() []byte {
	return c[:]
}

// Base58 returns a base58 encoded version of the Color.
func (c Color) Base58() string {
	return base58.Encode(c.Bytes())
}

// String creates a human readable string of the Color.
func (c Color) String() string {
	switch c {
	case ColorIOTA:
		return "IOTA"
	case ColorMint:
		return "MINT"
	default:
		return c.Base58()
	}
}

// Compare offers a comparator for Colors which returns -1 if otherColor is bigger, 1 if it is smaller and 0 if they are
// the same.
func (c Color) Compare(otherColor Color) int {
	return bytes.Compare(c[:], otherColor[:])
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ColoredBalances //////////////////////////////////////////////////////////////////////////////////////////////

// ColoredBalances represents a collection of Balances associated to their respective Color that maintains a
// deterministic order of the present Colors.
type ColoredBalances struct {
	coloredBalancesInner `serialize:"unpack"`
}
type coloredBalancesInner struct {
	Balances *orderedmap.OrderedMap `serialize:"true"`
}

// NewColoredBalances returns a new deterministically ordered collection of ColoredBalances.
func NewColoredBalances(balances map[Color]uint64) (coloredBalances *ColoredBalances) {
	coloredBalances = &ColoredBalances{coloredBalancesInner: coloredBalancesInner{Balances: orderedmap.New()}}

	// deterministically sort colors
	sortedColors := make([]Color, 0, len(balances))
	for color, balance := range balances {
		if balance == 0 {
			// drop zero Balances
			continue
		}
		sortedColors = append(sortedColors, color)
	}
	sort.Slice(sortedColors, func(i, j int) bool { return sortedColors[i].Compare(sortedColors[j]) < 0 })

	// add sorted colors to the underlying map
	for _, color := range sortedColors {
		coloredBalances.coloredBalancesInner.Balances.Set(color, balances[color])
	}

	return
}

// Get returns the balance of the given Color and a boolean value indicating if the requested Color existed.
func (c *ColoredBalances) Get(color Color) (uint64, bool) {
	balance, exists := c.coloredBalancesInner.Balances.Get(color)
	ret, ok := balance.(uint64)
	if !ok {
		return 0, false
	}
	return ret, exists
}

// ForEach calls the consumer for each element in the collection and aborts the iteration if the consumer returns false.
func (c *ColoredBalances) ForEach(consumer func(color Color, balance uint64) bool) {
	c.coloredBalancesInner.Balances.ForEach(func(key, value interface{}) bool {
		return consumer(key.(Color), value.(uint64))
	})
}

// Size returns the amount of individual Balances in the ColoredBalances.
func (c *ColoredBalances) Size() int {
	return c.coloredBalancesInner.Balances.Size()
}

// Clone returns a copy of the ColoredBalances.
func (c *ColoredBalances) Clone() *ColoredBalances {
	copiedBalances := orderedmap.New()
	c.coloredBalancesInner.Balances.ForEach(copiedBalances.Set)

	return &ColoredBalances{
		coloredBalancesInner: coloredBalancesInner{Balances: copiedBalances},
	}
}

// Map returns a vanilla golang map (unordered) containing the existing Balances. Since the ColoredBalances are
// immutable to ensure the deterministic ordering, this method can be used to retrieve a copy of the current values
// prior to some modification (like setting the updated colors of a minting transaction) which can then be used to
// create a new ColoredBalances object.
func (c *ColoredBalances) Map() (balances map[Color]uint64) {
	balances = make(map[Color]uint64)

	c.ForEach(func(color Color, balance uint64) bool {
		balances[color] = balance

		return true
	})

	return
}

// String returns a human readable version of the ColoredBalances.
func (c *ColoredBalances) String() string {
	structBuilder := stringify.StructBuilder("ColoredBalances")
	c.ForEach(func(color Color, balance uint64) bool {
		structBuilder.AddField(stringify.StructField(color.String(), balance))

		return true
	})

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
