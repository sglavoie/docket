//go:build !darwin && !linux

package store

func hardwareID() string { return "" }
