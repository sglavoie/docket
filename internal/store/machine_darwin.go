package store

import "syscall"

func hardwareID() string {
	id, _ := syscall.Sysctl("kern.uuid")
	return id
}
