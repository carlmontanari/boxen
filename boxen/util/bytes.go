package util

import "bytes"

// ByteSliceAllNull checks if a byte slice contains onlyl null bytes.
func ByteSliceAllNull(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}

	return true
}

// ByteSliceContains checks if slice of bytes contains a given byte subslice.
func ByteSliceContains(b [][]byte, l []byte) bool {
	for _, bs := range b {
		if bytes.Contains(l, bs) {
			return true
		}
	}

	return false
}
