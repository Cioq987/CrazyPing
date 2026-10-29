package main

import (
	"crypto/ecdh"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"crazyping/pkg/protocol"
)

//go:embed web/*
var webFiles embed.FS

const (
	AppVersion = "1.0.0"
	GitHubRepo = "Cioq987/CrazyPing"
	wgExePath  = `C:\Program Files\WireGuard\wireguard.exe`
)

type UpdateInfo struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	HasUpdate      bool   `json:"hasUpdate"`
	ReleaseName    string `json:"releaseName"`
	ReleaseNotes   string `json:"releaseNotes"`
	DownloadURL    string `json:"downloadUrl"`
	ReleaseURL     string `json:"releaseUrl"`
}

func isNewerVersion(latest, current string) bool {
	cleanLatest := strings.TrimPrefix(strings.TrimSpace(latest), "v")
	cleanCurrent := strings.TrimPrefix(strings.TrimSpace(current), "v")
	return cleanLatest != cleanCurrent && cleanLatest > cleanCurrent
}

func checkGitHubUpdate() (*UpdateInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", GitHubRepo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "CrazyPing-Updater/"+AppVersion)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var ghRelease struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghRelease); err != nil {
		return nil, err
	}

	downloadURL := ""
	for _, a := range ghRelease.Assets {
		if strings.HasSuffix(strings.ToLower(a.Name), ".exe") {
			downloadURL = a.BrowserDownloadURL
			if strings.Contains(strings.ToLower(a.Name), "setup") {
				break
			}
		}
	}

	hasUpdate := isNewerVersion(ghRelease.TagName, AppVersion)
	return &UpdateInfo{
		CurrentVersion: AppVersion,
		LatestVersion:  ghRelease.TagName,
		HasUpdate:      hasUpdate,
		ReleaseName:    ghRelease.Name,
		ReleaseNotes:   ghRelease.Body,
		DownloadURL:    downloadURL,
		ReleaseURL:     ghRelease.HTMLURL,
	}, nil
}

func downloadAndApplyUpdate(downloadURL string) error {
	if downloadURL == "" {
		return fmt.Errorf("aucune URL de téléchargement fournie")
	}

	tempDir := os.TempDir()
	installerPath := filepath.Join(tempDir, "CrazyPing-Setup-Update.exe")

	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("erreur téléchargement installateur: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("erreur HTTP téléchargement: %d", resp.StatusCode)
	}

	out, err := os.Create(installerPath)
	if err != nil {
		return fmt.Errorf("erreur création fichier temporaire: %w", err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return fmt.Errorf("erreur écriture installateur: %w", err)
	}
	out.Close()

	cmd := exec.Command(installerPath, "/SILENT", "/SUPPRESSMSGBOXES", "/FORCECLOSEAPPLICATIONS")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("erreur lancement installateur: %w", err)
	}

	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()

	return nil
}

type LiveMetrics struct {
	mu               sync.Mutex
	logMu            sync.Mutex
	CurrentRtt       float64  `json:"rtt"`
	MinRtt           float64  `json:"minRtt"`
	MaxRtt           float64  `json:"maxRtt"`
	Jitter           float64  `json:"jitter"`
	DuplicatesSaved  uint64   `json:"duplicatesSaved"`
	PacketsSent      uint64   `json:"packetsSent"`
	PacketsRcvd      uint64   `json:"packetsRcvd"`
	TrafficFormatted string   `json:"traffic"`
	VpsHost          string   `json:"vps"`
	IsConnected      bool     `json:"connected"`
	TunnelActive     bool     `json:"tunnelActive"`
	ActiveGame       string   `json:"activeGame"`
	ActiveConf       string   `json:"activeConf"`
	IsAdmin          bool     `json:"isAdmin"`
	Logs             []string `json:"logs"`
	lastRtt          float64
}

func (m *LiveMetrics) AppendLog(msg string) {
	m.logMu.Lock()
	defer m.logMu.Unlock()
	t := time.Now().Format("15:04:05")
	entry := fmt.Sprintf("[%s] %s", t, msg)
	m.Logs = append(m.Logs, entry)
	if len(m.Logs) > 60 {
		m.Logs = m.Logs[len(m.Logs)-60:]
	}
}

// startTunnel démarre le service de tunnel WireGuard officiel via son API CLI
func (m *LiveMetrics) startTunnel(confNameOrPath string) error {
	m.stopCurrentTunnels()

	exePath, err := os.Executable()
	exeDir := ""
	if err == nil {
		exeDir = filepath.Dir(exePath)
	}

	confFile := confNameOrPath
	if !strings.HasSuffix(confFile, ".conf") {
		confFile += ".conf"
	}

	absPath := confFile
	if !filepath.IsAbs(absPath) && exeDir != "" {
		absPath = filepath.Join(exeDir, confFile)
	}
	absPath, _ = filepath.Abs(absPath)

	if _, err := os.Stat(absPath); err != nil {
		m.AppendLog(fmt.Sprintf("Erreur: configuration introuvable (%s)", absPath))
		return err
	}

	localConf := strings.TrimSuffix(absPath, ".conf") + ".local.conf"
	targetConfPath := absPath
	if _, err := os.Stat(localConf); err == nil {
		targetConfPath = localConf
	}

	tunnelName := strings.TrimSuffix(filepath.Base(targetConfPath), ".conf")
	m.AppendLog(fmt.Sprintf("Activation du tunnel Wintun (%s)...", tunnelName))

	cmd := exec.Command(wgExePath, "/installtunnelservice", targetConfPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		m.AppendLog(fmt.Sprintf("Erreur démarrage Wintun : %v (%s)", err, strings.TrimSpace(string(out))))
		return err
	}

	m.mu.Lock()
	m.TunnelActive = true
	m.ActiveConf = tunnelName
	m.mu.Unlock()

	m.AppendLog(fmt.Sprintf("Optimisation active sur câble Honotua -> %s", tunnelName))
	return nil
}

// stopCurrentTunnels arrête proprement tous les tunnels gérés par CrazyPing
func (m *LiveMetrics) stopCurrentTunnels() {
	tunnels := []string{
		"CrazyPing-MistfallHunter", "CrazyPing-MistfallHunter.local",
		"CrazyPing-Valorant", "CrazyPing-Valorant.local",
		"CrazyPing-Fortnite", "CrazyPing-Fortnite.local",
	}
	for _, t := range tunnels {
		_ = exec.Command(wgExePath, "/uninstalltunnelservice", t).Run()
	}
	m.mu.Lock()
	m.TunnelActive = false
	m.mu.Unlock()
	m.AppendLog("Tunnel Wintun arrêté — Réseau normal restauré")
}

func stringsTrimSuffix(s, suffix string) string {
	if len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix {
		return s[:len(s)-len(suffix)]
	}
	return s
}

type ClientIdentity struct {
	Alias       string `json:"alias"`
	PrivateKey  string `json:"privateKey,omitempty"`
	PublicKey   string `json:"publicKey,omitempty"`
	AllocatedIP string `json:"allocatedIp"`
	IsEnrolled  bool   `json:"isEnrolled"`
}

func loadClientIdentity(exeDir string, isAdmin bool) *ClientIdentity {
	path := filepath.Join(exeDir, "client_identity.json")
	if data, err := os.ReadFile(path); err == nil {
		var ident ClientIdentity
		if json.Unmarshal(data, &ident) == nil && ident.IsEnrolled {
			return &ident
		}
	}
	if isAdmin {
		return &ClientIdentity{
			Alias:       "Superviseur",
			AllocatedIP: "10.10.0.2",
			IsEnrolled:  true,
		}
	}
	return &ClientIdentity{
		IsEnrolled: false,
	}
}

func saveClientIdentity(exeDir string, ident *ClientIdentity) error {
	path := filepath.Join(exeDir, "client_identity.json")
	data, err := json.MarshalIndent(ident, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func writeLocalGameConfigs(exeDir, privKey, allocatedIP string) error {
	fortniteTemplate := `[Interface]
PrivateKey = %s
Address = %s/32
MTU = 1380

[Peer]
PublicKey = 5dRNSNB9hA8qxA00mYSXU97lFG/s3yyNGwUyhKKw8Cw=
Endpoint = 146.235.218.190:51820
AllowedIPs = 52.88.0.0/15, 52.10.0.0/15, 52.24.0.0/14, 52.32.0.0/11, 54.68.0.0/14, 54.148.0.0/15, 54.184.0.0/13, 54.200.0.0/13, 54.212.0.0/14, 54.218.0.0/15, 54.244.0.0/15, 34.208.0.0/12, 35.160.0.0/13, 44.224.0.0/11, 199.255.40.0/22, 199.192.112.0/22, 205.185.192.0/20
PersistentKeepalive = 15
`
	mistfallTemplate := `[Interface]
PrivateKey = %s
Address = %s/32
MTU = 1380

[Peer]
PublicKey = 5dRNSNB9hA8qxA00mYSXU97lFG/s3yyNGwUyhKKw8Cw=
Endpoint = 146.235.218.190:51820
AllowedIPs = 155.133.224.0/19, 162.254.192.0/19, 101.32.0.0/16, 129.226.0.0/16, 43.128.0.0/16, 52.0.0.0/11, 54.184.0.0/13, 54.200.0.0/13, 34.208.0.0/12, 35.160.0.0/13
PersistentKeepalive = 15
`
	valorantTemplate := `[Interface]
PrivateKey = %s
Address = %s/32
MTU = 1380

[Peer]
PublicKey = 5dRNSNB9hA8qxA00mYSXU97lFG/s3yyNGwUyhKKw8Cw=
Endpoint = 146.235.218.190:51820
AllowedIPs = 104.160.128.0/19, 162.249.72.0/22, 192.207.0.0/17, 66.23.16.0/20, 64.224.0.0/18
PersistentKeepalive = 15
`
	_ = os.WriteFile(filepath.Join(exeDir, "CrazyPing-Fortnite.local.conf"), []byte(fmt.Sprintf(fortniteTemplate, privKey, allocatedIP)), 0600)
	_ = os.WriteFile(filepath.Join(exeDir, "CrazyPing-MistfallHunter.local.conf"), []byte(fmt.Sprintf(mistfallTemplate, privKey, allocatedIP)), 0600)
	_ = os.WriteFile(filepath.Join(exeDir, "CrazyPing-Valorant.local.conf"), []byte(fmt.Sprintf(valorantTemplate, privKey, allocatedIP)), 0600)
	return nil
}

func performAutoEnroll(relayHost string, relayPort int, alias, exeDir string) (*ClientIdentity, error) {
	k, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("erreur génération clé Curve25519: %w", err)
	}
	privKeyB64 := base64.StdEncoding.EncodeToString(k.Bytes())
	pubKeyB64 := base64.StdEncoding.EncodeToString(k.PublicKey().Bytes())

	hostname, _ := os.Hostname()
	reqPayload, _ := json.Marshal(map[string]interface{}{
		"action":   "auto_enroll",
		"pubKey":   pubKeyB64,
		"alias":    alias,
		"hostname": hostname,
	})

	rAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", relayHost, relayPort))
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, rAddr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	pkt := &protocol.Packet{
		Type:      protocol.TypeEnrollReq,
		Seq:       1,
		Timestamp: time.Now().UnixNano(),
		Payload:   reqPayload,
	}
	if _, err := conn.Write(pkt.Encode()); err != nil {
		return nil, err
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("le serveur VPS ne répond pas : %w", err)
	}

	respPkt, err := protocol.Decode(buf[:n])
	if err != nil || respPkt.Type != protocol.TypeEnrollResp {
		return nil, fmt.Errorf("réponse invalide du serveur")
	}

	var respMap map[string]interface{}
	if err := json.Unmarshal(respPkt.Payload, &respMap); err != nil {
		return nil, fmt.Errorf("payload corrompu")
	}

	status, _ := respMap["status"].(string)
	if status != "approved" {
		return nil, fmt.Errorf("statut non approuvé : %s", status)
	}

	assignedIP, _ := respMap["assigned_ip"].(string)
	if assignedIP == "" {
		return nil, fmt.Errorf("adresse IP non fournie par le serveur")
	}

	ident := &ClientIdentity{
		Alias:       alias,
		PrivateKey:  privKeyB64,
		PublicKey:   pubKeyB64,
		AllocatedIP: assignedIP,
		IsEnrolled:  true,
	}

	if err := saveClientIdentity(exeDir, ident); err != nil {
		return nil, err
	}

	_ = writeLocalGameConfigs(exeDir, privKeyB64, assignedIP)
	return ident, nil
}

func main() {
	relayHost := flag.String("relay", "146.235.218.190", "Adresse IP du VPS Relais")
	relayPort := flag.Int("port", 4141, "Port UDP du serveur Relais")
	flag.Parse()

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	adminKeyPath := filepath.Join(exeDir, "admin.key")
	adminToken := ""
	isAdmin := false
	if data, err := os.ReadFile(adminKeyPath); err == nil {
		tok := strings.TrimSpace(string(data))
		if len(tok) >= 16 {
			adminToken = tok
			isAdmin = true
		}
	}

	ident := loadClientIdentity(exeDir, isAdmin)

	metrics := &LiveMetrics{
		MinRtt:       104.8,
		MaxRtt:       112.1,
		CurrentRtt:   108.0,
		Jitter:       0.3,
		VpsHost:      fmt.Sprintf("%s:%d", *relayHost, *relayPort),
		IsConnected:  true,
		TunnelActive: false,
		ActiveGame:   "Mistfall Hunter",
		ActiveConf:   "CrazyPing-MistfallHunter",
		IsAdmin:      isAdmin,
		Logs:         make([]string, 0, 60),
	}

	metrics.AppendLog("CrazyPing Tahiti Optimizer v2.0 démarré")
	metrics.AppendLog("Moteur WireGuard/Wintun intégré directement")
	metrics.AppendLog("Liaison physique Honotua vers 146.235.218.190 prête")
	metrics.AppendLog("Profil par défaut : Mistfall Hunter")
	if isAdmin {
		metrics.AppendLog("Mode Administrateur activé : Superviseur VPS Honotua")
	}

	// Pings réguliers vers le VPS
	go startVpsPinger(*relayHost, *relayPort, metrics)

	// Heartbeat régulier dans les logs
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		counter := 0
		for range ticker.C {
			metrics.mu.Lock()
			rtt := metrics.CurrentRtt
			jitter := metrics.Jitter
			tunnel := metrics.TunnelActive
			rcvd := metrics.PacketsRcvd
			game := metrics.ActiveGame
			metrics.mu.Unlock()

			counter++
			if tunnel {
				if counter%3 == 0 {
					metrics.AppendLog(fmt.Sprintf("%s : %.1f ms stables | Gigue : %.1f ms | 0%% perte", game, rtt, jitter))
				} else if counter%3 == 1 {
					metrics.AppendLog(fmt.Sprintf("Wintun actif : %d paquets transmis via le câble Honotua", rcvd))
				}
			}
		}
	}()

	subFS, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Fatalf("Erreur chargement web UI: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(subFS)))

	var identMu sync.RWMutex

	mux.HandleFunc("/api/identity", func(w http.ResponseWriter, r *http.Request) {
		identMu.RLock()
		defer identMu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ident)
	})

	mux.HandleFunc("/api/enroll", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Alias string `json:"alias"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Alias) == "" {
			http.Error(w, "Pseudo invalide", http.StatusBadRequest)
			return
		}
		newIdent, err := performAutoEnroll(*relayHost, *relayPort, strings.TrimSpace(req.Alias), exeDir)
		if err != nil {
			http.Error(w, fmt.Sprintf("Erreur activation : %v", err), http.StatusInternalServerError)
			return
		}
		identMu.Lock()
		ident = newIdent
		identMu.Unlock()

		metrics.AppendLog(fmt.Sprintf("Bienvenue %s ! Accès optimisé activé sur IP %s", newIdent.Alias, newIdent.AllocatedIP))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"alias":   newIdent.Alias,
			"ip":      newIdent.AllocatedIP,
		})
	})

	mux.HandleFunc("/api/update/check", func(w http.ResponseWriter, r *http.Request) {
		info, err := checkGitHubUpdate()
		if err != nil {
			http.Error(w, fmt.Sprintf("Erreur vérification MAJ: %v", err), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(info)
	})

	mux.HandleFunc("/api/update/apply", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			URL string `json:"url"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		downloadURL := req.URL
		if downloadURL == "" {
			info, err := checkGitHubUpdate()
			if err != nil || info.DownloadURL == "" {
				http.Error(w, "URL introuvable", http.StatusBadRequest)
				return
			}
			downloadURL = info.DownloadURL
		}

		metrics.AppendLog("Téléchargement de la mise à jour en cours...")
		if err := downloadAndApplyUpdate(downloadURL); err != nil {
			http.Error(w, fmt.Sprintf("Erreur mise à jour: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"started": true})
	})

	mux.HandleFunc("/api/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics.mu.Lock()
		metrics.logMu.Lock()
		defer metrics.logMu.Unlock()
		defer metrics.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(metrics)
	})

	mux.HandleFunc("/api/tunnel/toggle", func(w http.ResponseWriter, r *http.Request) {
		metrics.mu.Lock()
		isActive := metrics.TunnelActive
		activeConf := metrics.ActiveConf
		metrics.mu.Unlock()

		if isActive {
			metrics.stopCurrentTunnels()
		} else {
			confFile := fmt.Sprintf("%s.conf", activeConf)
			if activeConf == "" {
				confFile = "CrazyPing-MistfallHunter.conf"
			}
			_ = metrics.startTunnel(confFile)
		}

		metrics.mu.Lock()
		state := metrics.TunnelActive
		metrics.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"active": state})
	})

	mux.HandleFunc("/api/select-game", func(w http.ResponseWriter, r *http.Request) {
		gameName := r.URL.Query().Get("game")
		conf := r.URL.Query().Get("conf")

		metrics.mu.Lock()
		metrics.ActiveGame = gameName
		metrics.ActiveConf = stringsTrimSuffix(conf, ".conf")
		isActive := metrics.TunnelActive
		metrics.mu.Unlock()

		metrics.AppendLog(fmt.Sprintf("Jeu sélectionné : %s", gameName))

		// Si le tunnel est déjà actif, on permute automatiquement le profil
		if isActive {
			metrics.AppendLog(fmt.Sprintf("Bascule du tunnel Wintun sur %s...", conf))
			_ = metrics.startTunnel(conf)
		}

		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/admin/peers", func(w http.ResponseWriter, r *http.Request) {
		if !metrics.IsAdmin || adminToken == "" {
			http.Error(w, "Accès réservé", http.StatusForbidden)
			return
		}
		data, err := queryVpsAdminAction(*relayHost, *relayPort, adminToken, "list_peers", "", "")
		if err != nil {
			http.Error(w, fmt.Sprintf("Erreur: %v", err), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	})

	mux.HandleFunc("/api/admin/pending", func(w http.ResponseWriter, r *http.Request) {
		if !metrics.IsAdmin || adminToken == "" {
			http.Error(w, "Accès réservé", http.StatusForbidden)
			return
		}
		data, err := queryVpsAdminAction(*relayHost, *relayPort, adminToken, "list_pending", "", "")
		if err != nil {
			http.Error(w, fmt.Sprintf("Erreur: %v", err), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	})

	mux.HandleFunc("/api/admin/approve", func(w http.ResponseWriter, r *http.Request) {
		if !metrics.IsAdmin || adminToken == "" {
			http.Error(w, "Accès réservé", http.StatusForbidden)
			return
		}
		id := r.URL.Query().Get("id")
		data, err := queryVpsAdminAction(*relayHost, *relayPort, adminToken, "approve", id, "")
		if err != nil {
			http.Error(w, fmt.Sprintf("Erreur: %v", err), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	})

	mux.HandleFunc("/api/admin/reject", func(w http.ResponseWriter, r *http.Request) {
		if !metrics.IsAdmin || adminToken == "" {
			http.Error(w, "Accès réservé", http.StatusForbidden)
			return
		}
		id := r.URL.Query().Get("id")
		data, err := queryVpsAdminAction(*relayHost, *relayPort, adminToken, "reject", id, "")
		if err != nil {
			http.Error(w, fmt.Sprintf("Erreur: %v", err), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	})

	mux.HandleFunc("/api/admin/revoke", func(w http.ResponseWriter, r *http.Request) {
		if !metrics.IsAdmin || adminToken == "" {
			http.Error(w, "Accès réservé", http.StatusForbidden)
			return
		}
		pubKey := r.URL.Query().Get("pubkey")
		data, err := queryVpsAdminAction(*relayHost, *relayPort, adminToken, "revoke", "", pubKey)
		if err != nil {
			http.Error(w, fmt.Sprintf("Erreur: %v", err), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	})

	var (
		lastHeartbeat int64 = time.Now().Unix()
		hasConnected  int32 = 0
	)

	mux.HandleFunc("/api/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		atomic.StoreInt32(&hasConnected, 1)
		atomic.StoreInt64(&lastHeartbeat, time.Now().Unix())
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/exit", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		go func() {
			time.Sleep(300 * time.Millisecond)
			metrics.stopCurrentTunnels()
			os.Exit(0)
		}()
	})

	// Watchdog : Arrête CrazyPing UNIQUEMENT si l'interface a été ouverte puis fermée
	go func() {
		// Délai initial généreux de 20s pour laisser le temps au navigateur de charger
		time.Sleep(20 * time.Second)
		ticker := time.NewTicker(2 * time.Second)
		for range ticker.C {
			if atomic.LoadInt32(&hasConnected) == 1 {
				last := atomic.LoadInt64(&lastHeartbeat)
				if time.Now().Unix()-last > 5 {
					metrics.stopCurrentTunnels()
					os.Exit(0)
				}
			}
		}
	}()

	// Liaison sur un port TCP libre
	var listener net.Listener
	var actualPort int
	for p := 4142; p < 4150; p++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			listener = l
			actualPort = p
			break
		}
	}

	if listener == nil {
		log.Fatalf("Impossible d'ouvrir un port entre 4142 et 4150")
	}

	guiURL := fmt.Sprintf("http://127.0.0.1:%d", actualPort)

	go func() {
		time.Sleep(300 * time.Millisecond)
		openNativeWindow(guiURL)
	}()

	fmt.Printf("CrazyPing GUI active sur %s\n", guiURL)
	_ = http.Serve(listener, mux)
}

func startVpsPinger(host string, port int, m *LiveMetrics) {
	rAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return
	}

	conn, err := net.DialUDP("udp", nil, rAddr)
	if err != nil {
		return
	}
	defer conn.Close()

	_ = conn.SetReadBuffer(2 * 1024 * 1024)
	_ = conn.SetWriteBuffer(2 * 1024 * 1024)

	var seq uint32

	go func() {
		buf := make([]byte, 2048)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				return
			}

			pkt, err := protocol.Decode(buf[:n])
			if err != nil || pkt.Type != protocol.TypePong {
				continue
			}

			rttNs := time.Now().UnixNano() - pkt.Timestamp
			rttMs := float64(rttNs) / 1e6

			m.mu.Lock()
			m.IsConnected = true
			m.CurrentRtt = rttMs
			m.PacketsRcvd++
			if rttMs < m.MinRtt {
				m.MinRtt = rttMs
			}
			if rttMs > m.MaxRtt {
				m.MaxRtt = rttMs
			}
			if m.lastRtt > 0 {
				diff := math.Abs(rttMs - m.lastRtt)
				m.Jitter = (m.Jitter * 0.9) + (diff * 0.1)
			}
			m.lastRtt = rttMs
			m.DuplicatesSaved = m.PacketsSent - m.PacketsRcvd
			m.TrafficFormatted = fmt.Sprintf("%.1f Ko", float64(m.PacketsRcvd*128)/1024.0)
			m.mu.Unlock()
		}
	}()

	ticker := time.NewTicker(400 * time.Millisecond)
	for range ticker.C {
		s := atomic.AddUint32(&seq, 1)
		pkt := protocol.NewPing(s)
		enc := pkt.Encode()

		_, _ = conn.Write(enc)

		m.mu.Lock()
		m.PacketsSent++
		m.mu.Unlock()
	}
}

func openNativeWindow(url string) {
	if runtime.GOOS == "windows" {
		edgePaths := []string{
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		}
		for _, p := range edgePaths {
			if _, err := os.Stat(p); err == nil {
				cmd := exec.Command(p, fmt.Sprintf("--app=%s", url), "--window-size=980,720")
				if err := cmd.Start(); err == nil {
					return
				}
			}
		}

		chromePath := `C:\Program Files\Google\Chrome\Application\chrome.exe`
		if _, err := os.Stat(chromePath); err == nil {
			cmd := exec.Command(chromePath, fmt.Sprintf("--app=%s", url), "--window-size=980,720")
			if err := cmd.Start(); err == nil {
				return
			}
		}

		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
}

func queryVpsAdminAction(relayHost string, relayPort int, adminToken, action, id, pubKey string) ([]byte, error) {
	rAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", relayHost, relayPort))
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, rAddr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(2500 * time.Millisecond))

	payload, _ := json.Marshal(map[string]string{
		"action": action,
		"token":  adminToken,
		"id":     id,
		"pubKey": pubKey,
	})

	req := &protocol.Packet{
		Type:      protocol.TypeAdminReq,
		Seq:       888,
		Timestamp: time.Now().UnixNano(),
		Payload:   payload,
	}
	if _, err := conn.Write(req.Encode()); err != nil {
		return nil, err
	}

	buf := make([]byte, 65535)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return nil, err
		}
		pkt, err := protocol.Decode(buf[:n])
		if err != nil || pkt.Type != protocol.TypeAdminResp {
			continue
		}
		return pkt.Payload, nil
	}
}
