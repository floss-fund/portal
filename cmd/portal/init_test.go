package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		// Zero
		{0, "0"},
		{int64(0), "0"},
		{float64(0), "0"},

		// Small positive numbers
		{1, "1"},
		{99, "99"},
		{999, "999"},

		// Thousands
		{1000, "1,000"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},

		// Large numbers
		{9876543, "9,876,543"},
		{5555555, "5,555,555"},

		// int64 types
		{int64(1234567), "1,234,567"},
		{int64(9876543), "9,876,543"},

		// float64 types
		{float64(1234567), "1,234,567"},
		{float64(5555555), "5,555,555"},

		// Negative numbers
		{-1, "-1"},
		{-1000, "-1,000"},
		{-1234567, "-1,234,567"},
		{float64(-2170474), "-2,170,474"},

		// Edge cases
		{10, "10"},
		{100, "100"},
		{10000, "10,000"},
		{100000, "100,000"},
		{1000000, "1,000,000"},
	}

	for _, tt := range tests {
		result := formatNumber(tt.input)
		assert.Equal(t, tt.expected, result, "formatNumber(%v) should equal %s", tt.input, tt.expected)
	}
}
