//go:build !darwin

package syncexchange

func cloudDataless(string) bool { return false }

// Other platforms leave delivery to the folder's provider.
func requestCloudDownloads([]string) error { return nil }
