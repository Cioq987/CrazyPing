package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	ServerDailyQuotaBytes = 10 * 1024 * 1024 * 1024 // 10 Go par jour
)

var (
	quotaDir = "/home/ubuntu/crazyping-quotas"
	quotaMu  sync.Mutex

	// GameCIDRs regroupe les sous-réseaux officiels de jeu (exclus du quota VPN)
	GameCIDRs = []string{
		// Fortnite / Epic Games
		"52.88.0.0/15", "52.10.0.0/15", "52.24.0.0/14", "52.32.0.0/11",
		"54.68.0.0/14", "54.148.0.0/15", "54.184.0.0/13", "54.200.0.0/13",
		"54.212.0.0/14", "54.218.0.0/15", "54.244.0.0/15", "34.208.0.0/12",
		"35.160.0.0/13", "44.224.0.0/11", "199.255.40.0/22", "199.192.112.0/22",
		"205.185.192.0/20",

		// Riot Games (Valorant / LoL)
		"104.160.128.0/19", "162.249.72.0/22", "192.64.174.0/23",
		"192.207.0.0/17", "66.23.16.0/20", "64.224.0.0/18",

		// Mistfall Hunter / NetEase / Steam SDR NA
		"155.133.224.0/19", "155.133.248.0/21", "162.254.192.0/19", "162.254.194.0/23",
		"101.32.0.0/16", "129.226.0.0/16", "43.128.0.0/16", "52.0.0.0/11",
		"54.153.0.0/16", "54.183.0.0/16", "54.193.0.0/16", "54.215.0.0/16",
		"54.219.0.0/16", "54.241.0.0/16", "52.8.0.0/14", "52.52.0.0/15",

		// Blizzard (Overwatch 2)
		"24.105.0.0/18", "137.221.64.0/18", "37.244.0.0/18",
		"5.42.160.0/20", "66.40.176.0/20", "158.115.192.0/18",
		"34.94.0.0/16", "35.235.0.0/16", "35.236.0.0/16",
	}
)

type ServerPeerQuota struct {
	IP         string `json:"ip"`
	Date       string `json:"date"`
	BytesUsed  uint64 `json:"bytesUsed"`
	DailyLimit uint64 `json:"dailyLimit"`
	Blocked    bool   `json:"blocked"`
}

func getTahitiDate() string {
	loc, err := time.LoadLocation("Pacific/Tahiti")
	if err != nil {
		loc = time.FixedZone("HST", -10*3600)
	}
	return time.Now().In(loc).Format("2006-01-02")
}

func initQuotaSubsystem() {
	_ = os.MkdirAll(quotaDir, 0755)

	// 0. Règle fondamentale : autoriser le trafic retour établi/relié depuis Internet vers wg0
	cmdRelCheck := exec.Command("sudo", "iptables", "-C", "FORWARD", "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT")
	if cmdRelCheck.Run() != nil {
		_ = exec.Command("sudo", "iptables", "-I", "FORWARD", "1", "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT").Run()
	}

	// Création du set ipset pour les jeux s'il n'existe pas
	_ = exec.Command("sudo", "ipset", "create", "cp_game_cidrs", "hash:net", "-exist").Run()
	for _, cidr := range GameCIDRs {
		_ = exec.Command("sudo", "ipset", "add", "cp_game_cidrs", cidr, "-exist").Run()
	}

	// 1. Règle prioritaire : jeu vidéo illimité (outrepasse le comptage VPN)
	_ = exec.Command("sudo", "iptables", "-C", "FORWARD", "-i", "wg0", "-o", "ens3", "-m", "set", "--match-set", "cp_game_cidrs", "dst", "-j", "ACCEPT").Run()
	cmdCheck := exec.Command("sudo", "iptables", "-C", "FORWARD", "-i", "wg0", "-o", "ens3", "-m", "set", "--match-set", "cp_game_cidrs", "dst", "-j", "ACCEPT")
	if cmdCheck.Run() != nil {
		_ = exec.Command("sudo", "iptables", "-I", "FORWARD", "2", "-i", "wg0", "-o", "ens3", "-m", "set", "--match-set", "cp_game_cidrs", "dst", "-j", "ACCEPT").Run()
	}

	// 2. Chaîne dédiée au comptage et filtrage VPN
	_ = exec.Command("sudo", "iptables", "-N", "CP_VPN_QUOTA").Run()
	_ = exec.Command("sudo", "iptables", "-D", "FORWARD", "-i", "wg0", "-o", "ens3", "-j", "CP_VPN_QUOTA").Run()
	_ = exec.Command("sudo", "iptables", "-I", "FORWARD", "4", "-i", "wg0", "-o", "ens3", "-j", "CP_VPN_QUOTA").Run()

	// Initialisation des règles d'accounting par IP pour 10.10.0.2 à 10.10.0.30
	for i := 2; i <= 30; i++ {
		ip := fmt.Sprintf("10.10.0.%d", i)
		ensurePeerAccountingRule(ip)
	}

	log.Printf("[QUOTA] Moteur de quotas serveur initialisé avec %d plages CIDR de jeux protégées", len(GameCIDRs))
}

func ensurePeerAccountingRule(ip string) {
	check := exec.Command("sudo", "iptables", "-C", "CP_VPN_QUOTA", "-s", ip, "-j", "ACCEPT")
	if check.Run() != nil {
		_ = exec.Command("sudo", "iptables", "-A", "CP_VPN_QUOTA", "-s", ip, "-j", "ACCEPT").Run()
	}
}

func startQuotaDaemon() {
	go func() {
		lastDate := getTahitiDate()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			currentDate := getTahitiDate()
			isNewDay := currentDate != lastDate

			if isNewDay {
				log.Printf("[QUOTA] Minuit passé à Tahiti (%s). Réinitialisation des quotas journaliers VPN.", currentDate)
				resetServerDailyQuotas(currentDate)
				lastDate = currentDate
			}

			// Lecture des compteurs iptables
			out, err := exec.Command("sudo", "iptables", "-L", "CP_VPN_QUOTA", "-v", "-n", "-x").Output()
			if err != nil {
				continue
			}

			parseAndApplyQuotas(string(out), currentDate)
		}
	}()
}

func parseAndApplyQuotas(iptablesOut, currentDate string) {
	quotaMu.Lock()
	defer quotaMu.Unlock()

	lines := strings.Split(iptablesOut, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 9 {
			// Format: pkts bytes target prot opt in out source destination
			bytesStr := fields[1]
			target := fields[2]
			src := fields[7]

			if strings.HasPrefix(src, "10.10.0.") && target == "ACCEPT" {
				b, _ := strconv.ParseUint(bytesStr, 10, 64)
				updatePeerQuotaFile(src, b, currentDate)
			}
		}
	}
}

func updatePeerQuotaFile(ip string, iptablesBytes uint64, currentDate string) {
	filePath := filepath.Join(quotaDir, fmt.Sprintf("%s.json", ip))
	var q ServerPeerQuota
	data, err := os.ReadFile(filePath)
	if err == nil {
		_ = json.Unmarshal(data, &q)
	}

	if q.Date != currentDate {
		q = ServerPeerQuota{
			IP:         ip,
			Date:       currentDate,
			BytesUsed:  0,
			DailyLimit: ServerDailyQuotaBytes,
			Blocked:    false,
		}
	}

	q.BytesUsed = iptablesBytes
	if q.BytesUsed >= ServerDailyQuotaBytes && !q.Blocked {
		q.Blocked = true
		log.Printf("[QUOTA] Joueur %s a atteint son quota VPN journalier (10 Go). Blocage du forwarding web.", ip)
		_ = exec.Command("sudo", "iptables", "-I", "CP_VPN_QUOTA", "1", "-s", ip, "-j", "REJECT").Run()
	}

	raw, err := json.MarshalIndent(q, "", "  ")
	if err == nil {
		_ = os.WriteFile(filePath, raw, 0644)
	}
}

func resetServerDailyQuotas(newDate string) {
	quotaMu.Lock()
	defer quotaMu.Unlock()

	// Réinitialisation des compteurs iptables
	_ = exec.Command("sudo", "iptables", "-F", "CP_VPN_QUOTA").Run()

	// Réinsertion des règles ACCEPT par IP
	for i := 2; i <= 30; i++ {
		ip := fmt.Sprintf("10.10.0.%d", i)
		_ = exec.Command("sudo", "iptables", "-A", "CP_VPN_QUOTA", "-s", ip, "-j", "ACCEPT").Run()

		filePath := filepath.Join(quotaDir, fmt.Sprintf("%s.json", ip))
		q := ServerPeerQuota{
			IP:         ip,
			Date:       newDate,
			BytesUsed:  0,
			DailyLimit: ServerDailyQuotaBytes,
			Blocked:    false,
		}
		raw, _ := json.MarshalIndent(q, "", "  ")
		_ = os.WriteFile(filePath, raw, 0644)
	}
}

func getServerPeerQuota(ip string) (formatted string, blocked bool) {
	quotaMu.Lock()
	defer quotaMu.Unlock()

	filePath := filepath.Join(quotaDir, fmt.Sprintf("%s.json", ip))
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "0.00 Go / 10 Go", false
	}

	var q ServerPeerQuota
	if json.Unmarshal(data, &q) != nil {
		return "0.00 Go / 10 Go", false
	}

	gb := float64(q.BytesUsed) / (1024 * 1024 * 1024)
	return fmt.Sprintf("%.2f Go / 10 Go", gb), q.Blocked
}
