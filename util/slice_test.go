package util

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContains(t *testing.T) {
	var testCases = []struct {
		inputSlice     []string
		inputTarget    string
		expectedOutput bool
		expectedError  error
	}{
		{inputSlice: []string{"foo", "bar"}, inputTarget: "bar", expectedOutput: true, expectedError: nil},
		{inputSlice: []string{"foo", "bar"}, inputTarget: "baz", expectedOutput: false, expectedError: nil},
		{inputSlice: []string{}, inputTarget: "baz", expectedOutput: false, expectedError: errors.New("invalid contains arguments")},
		{inputSlice: []string{"foo", "bar"}, inputTarget: "", expectedOutput: false, expectedError: errors.New("invalid contains arguments")},
	}

	for _, tc := range testCases {
		actualOutput, actualError := contains(tc.inputSlice, tc.inputTarget)

		require.Equal(t, tc.expectedError, actualError)
		assert.Equal(t, tc.expectedOutput, actualOutput)
	}
}
