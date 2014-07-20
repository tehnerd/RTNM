package utils

func StringInSlice(slice []string, key string) bool {
	for cntr := range slice {
		if slice[cntr] == key {
			return true
		}
	}
	return false
}

func SliceWOString(slice []string, key string) []string {
	new_slice := make([]string, 0)
	for cntr := range slice {
		if slice[cntr] != key {
			new_slice = append(new_slice, slice[cntr])
		}
	}
	return new_slice
}
