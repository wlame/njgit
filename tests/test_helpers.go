// Package tests contains shared test helpers
package tests

// Helper functions for creating pointers to primitive types
// Used by both unit and integration tests

func stringToPtr(s string) *string {
	return &s
}

func intToPtr(i int) *int {
	return &i
}
