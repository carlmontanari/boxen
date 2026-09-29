package util

import (
	"fmt"
	"math"
)

// MustIntToUint32 converts i to a uint32 or fatals.
func MustIntToUint32(i int) uint32 {
	if i < 0 || i > math.MaxUint32 {
		panic(fmt.Sprintf("%d does not fit in uint32", i))
	}

	return uint32(i)
}
