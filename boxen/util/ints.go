package util

func IntSliceContains(s []int, i int) bool {
	for _, ss := range s {
		if ss == i {
			return true
		}
	}

	return false
}

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
