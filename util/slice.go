package util

import (
	"errors"
)

func contains(s []string, target string) (bool, error) {
	if len(s) == 0 || len(target) == 0 {
		return false, errors.New("invalid contains arguments")
	}
	for _, element := range s {
		if element == target {
			return true, nil
		}
	}
	return false, nil
}
