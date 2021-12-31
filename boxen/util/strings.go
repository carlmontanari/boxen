package util

// StringSliceContains checks if slice of strings contains a given string.
func StringSliceContains(s string, l []string) bool {
	for _, ss := range l {
		if ss == s {
			return true
		}
	}

	return false
}
