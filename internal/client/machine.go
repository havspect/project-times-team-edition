package client

import (
	"os"
	"strings"
	"time"
)

// MachineTimezone is the IANA name of the machine's zone, so a CLI or MCP
// process logs in the zone the user is sitting in. Go exposes no name for
// time.Local, so read TZ or the /etc/localtime link; fallback covers Windows
// and containers where that file is a copy.
func MachineTimezone(fallback string) string {
	if name := os.Getenv("TZ"); name != "" {
		if _, err := time.LoadLocation(name); err == nil {
			return name
		}
	}
	if path, err := os.Readlink("/etc/localtime"); err == nil {
		if i := strings.Index(path, "zoneinfo/"); i >= 0 {
			name := path[i+len("zoneinfo/"):]
			if _, err := time.LoadLocation(name); err == nil {
				return name
			}
		}
	}
	return fallback
}
