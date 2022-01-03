package util

// IntSliceContains is a convenience function to check if a provided int i is in an int slice s.
func IntSliceContains(s []int, i int) bool {
	for _, ss := range s {
		if ss == i {
			return true
		}
	}

	return false
}

// IntSliceUniqify removes any duplicated entries in a slice of integers s.
func IntSliceUniqify(s []int) []int {
	var unique []int

	keys := make(map[int]bool)

	for _, entry := range s {
		if _, value := keys[entry]; !value {
			keys[entry] = true

			unique = append(unique, entry)
		}
	}

	return unique
}
