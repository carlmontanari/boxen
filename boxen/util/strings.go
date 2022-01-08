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

// AnyStringVal returns true if any string in variadic s matches the string val.
func AnyStringVal(val string, s ...string) bool {
	for _, v := range s {
		if v == val {
			return true
		}
	}

	return false
}

// AllStringVal returns true if all strings in variadic s matches the string val.
func AllStringVal(val string, s ...string) bool {
	for _, v := range s {
		if v != val {
			return false
		}
	}

	return true
}
