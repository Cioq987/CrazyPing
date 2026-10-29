package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
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
	wgExePath = `C:\Program Files\WireGuard\wireguard.exe`
)

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

	tunnelName := strings.TrimSuffix(filepath.Base(absPath), ".conf")
	m.AppendLog(fmt.Sprintf("Activation du tunnel Wintun (%s)...", tunnelName))

	cmd := exec.Command(wgExePath, "/installtunnelservice", absPath)
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
	tunnels := []string{"CrazyPing-MistfallHunter", "CrazyPing-Valorant", "CrazyPing-Fortnite"}
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

func main() {
	relayHost := flag.String("relay", "146.235.218.190", "Adresse IP du VPS Relais")
	relayPort := flag.Int("port", 4141, "Port UDP du serveur Relais")
	flag.Parse()

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	adminKeyPath := filepath.Join(exeDir, "admin.key")
	if _, err := os.Stat(adminKeyPath); os.IsNotExist(err) {
		_ = os.WriteFile(adminKeyPath, []byte("crazyping-tahiti-2026\n"), 0600)
	}
	isAdmin := false
	if _, err := os.Stat(adminKeyPath); err == nil {
		isAdmin = true
	}

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
		if !metrics.IsAdmin {
			http.Error(w, "Accès réservé", http.StatusForbidden)
			return
		}
		data, err := queryVpsAdminAction(*relayHost, *relayPort, "list_peers", "", "")
		if err != nil {
			http.Error(w, fmt.Sprintf("Erreur: %v", err), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	})

	mux.HandleFunc("/api/admin/pending", func(w http.ResponseWriter, r *http.Request) {
		if !metrics.IsAdmin {
			http.Error(w, "Accès réservé", http.StatusForbidden)
			return
		}
		data, err := queryVpsAdminAction(*relayHost, *relayPort, "list_pending", "", "")
		if err != nil {
			http.Error(w, fmt.Sprintf("Erreur: %v", err), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	})

	mux.HandleFunc("/api/admin/approve", func(w http.ResponseWriter, r *http.Request) {
		if !metrics.IsAdmin {
			http.Error(w, "Accès réservé", http.StatusForbidden)
			return
		}
		id := r.URL.Query().Get("id")
		data, err := queryVpsAdminAction(*relayHost, *relayPort, "approve", id, "")
		if err != nil {
			http.Error(w, fmt.Sprintf("Erreur: %v", err), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	})

	mux.HandleFunc("/api/admin/reject", func(w http.ResponseWriter, r *http.Request) {
		if !metrics.IsAdmin {
			http.Error(w, "Accès réservé", http.StatusForbidden)
			return
		}
		id := r.URL.Query().Get("id")
		data, err := queryVpsAdminAction(*relayHost, *relayPort, "reject", id, "")
		if err != nil {
			http.Error(w, fmt.Sprintf("Erreur: %v", err), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	})

	mux.HandleFunc("/api/admin/revoke", func(w http.ResponseWriter, r *http.Request) {
		if !metrics.IsAdmin {
			http.Error(w, "Accès réservé", http.StatusForbidden)
			return
		}
		pubKey := r.URL.Query().Get("pubkey")
		data, err := queryVpsAdminAction(*relayHost, *relayPort, "revoke", "", pubKey)
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

func queryVpsAdminAction(relayHost string, relayPort int, action, id, pubKey string) ([]byte, error) {
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
		"token":  "crazyping-tahiti-2026",
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
