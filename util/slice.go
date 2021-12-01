package util

import (
	"errors"
)

// Contains - checks that a slice has a specific element
func Contains(s []string, target string) (bool, error) {
	if len(s) == 0 || len(target) == 0 {
		return false, errors.New("invalid Contains arguments")
	}
	for _, element := range s {
		if element == target {
			return true, nil
		}
	}
	return false, nil
}
