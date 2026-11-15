package store

import "os"

// machineID names the computer, in a way that outlives a change of host
// name where the system offers one. Tests replace it.
var machineID = func() string {
	if id := hardwareID(); id != "" {
		return id
	}
	name, _ := os.Hostname()
	return name
}
