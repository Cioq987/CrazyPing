package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DailyVpnQuotaBytes = 10 * 1024 * 1024 * 1024 // 10 Go
)

type VpnQuotaInfo struct {
	Date       string `json:"date"`
	BytesUsed  uint64 `json:"bytesUsed"`
	DailyLimit uint64 `json:"dailyLimit"`
	IsBlocked  bool   `json:"isBlocked"`
}

type VpnQuotaTracker struct {
	mu           sync.Mutex
	exeDir       string
	info         VpnQuotaInfo
	lastInBytes  uint64
	lastOutBytes uint64
	hasSampled   bool
}

func newVpnQuotaTracker(exeDir string) *VpnQuotaTracker {
	t := &VpnQuotaTracker{
		exeDir: exeDir,
		info: VpnQuotaInfo{
			Date:       time.Now().Format("2006-01-02"),
			BytesUsed:  0,
			DailyLimit: DailyVpnQuotaBytes,
			IsBlocked:  false,
		},
	}
	t.load()
	return t
}

func (t *VpnQuotaTracker) getQuotaPath() string {
	return filepath.Join(t.exeDir, "vpn_quota.json")
}

func (t *VpnQuotaTracker) load() {
	data, err := os.ReadFile(t.getQuotaPath())
	if err == nil {
		var q VpnQuotaInfo
		if json.Unmarshal(data, &q) == nil {
			today := time.Now().Format("2006-01-02")
			if q.Date == today {
				t.info = q
				t.info.DailyLimit = DailyVpnQuotaBytes
				if t.info.BytesUsed >= DailyVpnQuotaBytes {
					t.info.IsBlocked = true
				}
				return
			}
		}
	}
	// Réinitialisation du jour
	t.info = VpnQuotaInfo{
		Date:       time.Now().Format("2006-01-02"),
		BytesUsed:  0,
		DailyLimit: DailyVpnQuotaBytes,
		IsBlocked:  false,
	}
	t.save()
}

func (t *VpnQuotaTracker) save() {
	data, err := json.MarshalIndent(t.info, "", "  ")
	if err == nil {
		_ = os.WriteFile(t.getQuotaPath(), data, 0600)
	}
}

func (t *VpnQuotaTracker) resetIfNewDay() {
	today := time.Now().Format("2006-01-02")
	if t.info.Date != today {
		t.info = VpnQuotaInfo{
			Date:       today,
			BytesUsed:  0,
			DailyLimit: DailyVpnQuotaBytes,
			IsBlocked:  false,
		}
		t.hasSampled = false
		t.save()
	}
}

func (t *VpnQuotaTracker) recordBytes(delta uint64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.resetIfNewDay()
	t.info.BytesUsed += delta
	if t.info.BytesUsed >= DailyVpnQuotaBytes {
		t.info.IsBlocked = true
	}
	t.save()
	return t.info.IsBlocked
}

func (t *VpnQuotaTracker) getStatus() (usedFormatted string, limitFormatted string, percent float64, isBlocked bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.resetIfNewDay()
	usedGb := float64(t.info.BytesUsed) / (1024 * 1024 * 1024)
	limitGb := float64(t.info.DailyLimit) / (1024 * 1024 * 1024)
	pct := (usedGb / limitGb) * 100.0
	if pct > 100.0 {
		pct = 100.0
	}
	return fmt.Sprintf("%.2f Go", usedGb), fmt.Sprintf("%.0f Go", limitGb), pct, t.info.IsBlocked
}

// sampleVpnInterface interroge l'interface Windows du VPN pour comptabiliser les octets échangés
func (t *VpnQuotaTracker) sampleVpnInterface() (uint64, error) {
	cmd := exec.Command("netsh", "interface", "ipv4", "show", "subinterfaces")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			name := strings.Join(fields[4:], " ")
			if strings.Contains(strings.ToLower(name), "crazyping-globalvpn") {
				inBytes, _ := strconv.ParseUint(fields[2], 10, 64)
				outBytes, _ := strconv.ParseUint(fields[3], 10, 64)

				t.mu.Lock()
				defer t.mu.Unlock()

				if !t.hasSampled {
					t.lastInBytes = inBytes
					t.lastOutBytes = outBytes
					t.hasSampled = true
					return 0, nil
				}

				var delta uint64
				if inBytes >= t.lastInBytes {
					delta += (inBytes - t.lastInBytes)
				}
				if outBytes >= t.lastOutBytes {
					delta += (outBytes - t.lastOutBytes)
				}
				t.lastInBytes = inBytes
				t.lastOutBytes = outBytes
				return delta, nil
			}
		}
	}
	return 0, fmt.Errorf("interface VPN introuvable")
}
