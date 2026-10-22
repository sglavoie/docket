package syncexchange

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/synclog"
)

// DeviceAdvertisement is the tt-sync-device record each device keeps at
// devices/<device-id>.json and only it rewrites.
type DeviceAdvertisement struct {
	Format             string         `json:"format"`
	FormatVersion      int            `json:"format_version"`
	DeviceID           string         `json:"device_id"`
	DeviceName         string         `json:"device_name"`
	Platform           string         `json:"platform"`
	AppVersion         string         `json:"app_version"`
	BridgeVersions     []int          `json:"bridge_versions"`
	SyncFormatVersions []int          `json:"sync_format_versions"`
	Schemas            map[string]int `json:"schemas"`
	Capabilities       []string       `json:"capabilities"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

// advertise rewrites this device's advertisement when what it says changed.
func advertise(history *synclog.History, root string, opts FolderOptions, at time.Time) error {
	path := filepath.Join(root, "devices", history.DeviceID()+".json")
	ad := DeviceAdvertisement{
		Format:             deviceAdvertisementFormat,
		FormatVersion:      exchangeFormatVersion,
		DeviceID:           history.DeviceID(),
		DeviceName:         opts.DeviceName,
		Platform:           platform(),
		AppVersion:         opts.AppVersion,
		BridgeVersions:     []int{},
		SyncFormatVersions: []int{synclog.ManifestFormatVersion},
		Schemas:            history.Schemas(),
		Capabilities:       history.Capabilities(),
		UpdatedAt:          at.UTC(),
	}
	if data, err := os.ReadFile(path); err == nil {
		var existing DeviceAdvertisement
		if json.Unmarshal(data, &existing) == nil {
			existing.UpdatedAt = ad.UpdatedAt
			if sameJSON(existing, ad) {
				return nil
			}
		}
	}
	encoded, err := json.MarshalIndent(ad, "", "  ")
	if err != nil {
		return err
	}
	return writeExchangeFile(path, append(encoded, '\n'))
}

func platform() string {
	if runtime.GOOS == "darwin" {
		return "macos"
	}
	return runtime.GOOS
}

func sameJSON(a, b any) bool {
	left, errA := json.Marshal(a)
	right, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(left, right)
}

// readPeers lists the other devices' advertisements. One that is unreadable
// or not downloaded is skipped: advertisements inform and never gate.
func readPeers(root, self string) ([]DeviceAdvertisement, error) {
	if exchangeFileUnavailable(filepath.Join(root, "devices")) {
		return []DeviceAdvertisement{}, nil
	}
	entries, err := os.ReadDir(filepath.Join(root, "devices"))
	if errors.Is(err, fs.ErrNotExist) {
		return []DeviceAdvertisement{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing device advertisements: %w", err)
	}
	peers := []DeviceAdvertisement{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") || strings.HasPrefix(name, ".") || strings.TrimSuffix(name, ".json") == self {
			continue
		}
		data, readable := readExchangeFile(filepath.Join(root, "devices", name))
		if !readable {
			continue
		}
		var ad DeviceAdvertisement
		if json.Unmarshal(data, &ad) == nil && ad.Format == deviceAdvertisementFormat {
			peers = append(peers, ad)
		}
	}
	sort.Slice(peers, func(i, j int) bool { return peers[i].DeviceID < peers[j].DeviceID })
	return peers, nil
}
