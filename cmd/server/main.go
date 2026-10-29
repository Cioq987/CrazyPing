package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"crazyping/pkg/dedup"
	"crazyping/pkg/protocol"
)

const (
	AdminToken     = "crazyping-tahiti-2026"
	ServerPubKey   = "5dRNSNB9hA8qxA00mYSXU97lFG/s3yyNGwUyhKKw8Cw="
	ServerEndpoint = "146.235.218.190:51820"
)

type Stats struct {
	TotalReceived   uint64
	DuplicatesSaved uint64
	PingsHandled    uint64
	ForwardsToGame  uint64
	RepliesFromGame uint64
}

type PeerInfo struct {
	PubKey               string `json:"pubKey"`
	Endpoint             string `json:"endpoint"`
	AllowedIPs           string `json:"allowedIps"`
	LatestHandshakeEpoch int64  `json:"latestHandshakeEpoch"`
	LatestHandshakeHuman string `json:"latestHandshakeHuman"`
	IsOnline             bool   `json:"isOnline"`
	RxBytes              uint64 `json:"rxBytes"`
	TxBytes              uint64 `json:"txBytes"`
	RxFormatted          string `json:"rxFormatted"`
	TxFormatted          string `json:"txFormatted"`
	Alias                string `json:"alias"`
}

type EnrollmentRequest struct {
	ID         string `json:"id"`
	PubKey     string `json:"pubKey"`
	Alias      string `json:"alias"`
	Hostname   string `json:"hostname"`
	ClientIP   string `json:"clientIp"`
	CreatedAt  int64  `json:"createdAt"`
	Status     string `json:"status"` // "pending", "approved", "rejected"
	AssignedIP string `json:"assignedIp"`
}

type AdminCommand struct {
	Action string `json:"action"` // "list_peers", "list_pending", "approve", "reject", "revoke"
	Token  string `json:"token"`
	ID     string `json:"id"`
	PubKey string `json:"pubKey"`
}

type EnrollCommand struct {
	Action   string `json:"action"` // "request", "check"
	PubKey   string `json:"pubKey"`
	Alias    string `json:"alias"`
	Hostname string `json:"hostname"`
}

var (
	aliasFile  = "/home/ubuntu/crazyping-aliases.json"
	aliasMu    sync.Mutex
	enrollFile = "/home/ubuntu/crazyping-enrollments.json"
	enrollMu   sync.Mutex
)

func loadAliases() map[string]string {
	aliasMu.Lock()
	defer aliasMu.Unlock()
	res := make(map[string]string)
	data, err := os.ReadFile(aliasFile)
	if err == nil {
		_ = json.Unmarshal(data, &res)
	}
	return res
}

func saveAlias(key, name string) {
	aliasMu.Lock()
	defer aliasMu.Unlock()
	res := make(map[string]string)
	data, err := os.ReadFile(aliasFile)
	if err == nil {
		_ = json.Unmarshal(data, &res)
	}
	res[key] = name
	encoded, _ := json.MarshalIndent(res, "", "  ")
	_ = os.WriteFile(aliasFile, encoded, 0644)
}

func loadEnrollments() map[string]*EnrollmentRequest {
	enrollMu.Lock()
	defer enrollMu.Unlock()
	res := make(map[string]*EnrollmentRequest)
	data, err := os.ReadFile(enrollFile)
	if err == nil {
		_ = json.Unmarshal(data, &res)
	}
	return res
}

func saveEnrollments(m map[string]*EnrollmentRequest) {
	enrollMu.Lock()
	defer enrollMu.Unlock()
	data, _ := json.MarshalIndent(m, "", "  ")
	_ = os.WriteFile(enrollFile, data, 0644)
}

func allocateNextFreeIP() (string, error) {
	peers, err := getWireGuardPeers()
	if err != nil {
		return "", err
	}
	usedIPs := make(map[string]bool)
	usedIPs["10.10.0.1"] = true
	usedIPs["10.10.0.2"] = true // Master IP

	for _, p := range peers {
		cleanIP := strings.TrimSuffix(p.AllowedIPs, "/32")
		usedIPs[cleanIP] = true
	}

	for i := 3; i < 254; i++ {
		candidate := fmt.Sprintf("10.10.0.%d", i)
		if !usedIPs[candidate] {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("aucune adresse IP disponible dans le pool")
}

func approveEnrollment(id string) (*EnrollmentRequest, error) {
	enrollments := loadEnrollments()
	req, ok := enrollments[id]
	if !ok {
		return nil, fmt.Errorf("demande introuvable")
	}

	ip, err := allocateNextFreeIP()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("wg", "set", "wg0", "peer", req.PubKey, "allowed-ips", fmt.Sprintf("%s/32", ip))
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("erreur wireguard: %v (%s)", err, string(out))
	}

	req.Status = "approved"
	req.AssignedIP = ip
	enrollments[id] = req
	saveEnrollments(enrollments)

	saveAlias(ip, req.Alias)
	saveAlias(req.PubKey, req.Alias)

	log.Printf("[SUPERVISEUR] Ami approuvé : %s (%s) -> %s", req.Alias, req.Hostname, ip)
	return req, nil
}

func rejectEnrollment(id string) error {
	enrollments := loadEnrollments()
	if req, ok := enrollments[id]; ok {
		req.Status = "rejected"
		saveEnrollments(enrollments)
	}
	return nil
}

func revokePeer(pubKey string) error {
	_ = exec.Command("wg", "set", "wg0", "peer", pubKey, "remove").Run()
	enrollments := loadEnrollments()
	for _, req := range enrollments {
		if req.PubKey == pubKey {
			req.Status = "revoked"
		}
	}
	saveEnrollments(enrollments)
	log.Printf("[SUPERVISEUR] Accès révoqué pour la clé : %s", pubKey)
	return nil
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d o", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %co", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	s := int(d.Seconds())
	if s < 60 {
		return fmt.Sprintf("Il y a %ds", s)
	}
	m := s / 60
	if m < 60 {
		return fmt.Sprintf("Il y a %dmin", m)
	}
	h := m / 60
	return fmt.Sprintf("Il y a %dh", h)
}

func getWireGuardPeers() ([]PeerInfo, error) {
	cmd := exec.Command("wg", "show", "wg0", "dump")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	aliases := loadAliases()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) <= 1 {
		return []PeerInfo{}, nil
	}

	var peers []PeerInfo
	now := time.Now().Unix()

	for _, line := range lines[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) < 8 {
			continue
		}

		pubKey := fields[0]
		endpoint := fields[2]
		if endpoint == "(none)" {
			endpoint = "-"
		}
		allowedIPs := fields[3]
		hs, _ := strconv.ParseInt(fields[4], 10, 64)
		rx, _ := strconv.ParseUint(fields[5], 10, 64)
		tx, _ := strconv.ParseUint(fields[6], 10, 64)

		isOnline := false
		humanHs := "Jamais"
		if hs > 0 {
			diff := now - hs
			if diff < 180 {
				isOnline = true
			}
			humanHs = formatDuration(time.Duration(diff) * time.Second)
		}

		cleanIP := strings.TrimSuffix(allowedIPs, "/32")
		alias := aliases[cleanIP]
		if alias == "" {
			alias = aliases[pubKey]
		}
		if alias == "" {
			if cleanIP == "10.10.0.2" {
				alias = "Moi (Tahiti)"
			} else {
				alias = fmt.Sprintf("Joueur %s", cleanIP)
			}
		}

		peers = append(peers, PeerInfo{
			PubKey:               pubKey,
			Endpoint:             endpoint,
			AllowedIPs:           allowedIPs,
			LatestHandshakeEpoch: hs,
			LatestHandshakeHuman: humanHs,
			IsOnline:             isOnline,
			RxBytes:              rx,
			TxBytes:              tx,
			RxFormatted:          formatBytes(rx),
			TxFormatted:          formatBytes(tx),
			Alias:                alias,
		})
	}
	return peers, nil
}

func startAdminHTTPServer(port int) {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/admin/peers", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token != AdminToken {
			http.Error(w, "Accès refusé", http.StatusUnauthorized)
			return
		}
		peers, err := getWireGuardPeers()
		if err != nil {
			http.Error(w, fmt.Sprintf("Erreur: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(peers)
	})

	mux.HandleFunc("/api/admin/pending", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token != AdminToken {
			http.Error(w, "Accès refusé", http.StatusUnauthorized)
			return
		}
		enrollments := loadEnrollments()
		var pending []*EnrollmentRequest
		for _, e := range enrollments {
			if e.Status == "pending" {
				pending = append(pending, e)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pending)
	})

	mux.HandleFunc("/api/admin/approve", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		id := r.URL.Query().Get("id")
		if token != AdminToken || id == "" {
			http.Error(w, "Accès refusé", http.StatusUnauthorized)
			return
		}
		req, err := approveEnrollment(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(req)
	})

	mux.HandleFunc("/api/admin/reject", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		id := r.URL.Query().Get("id")
		if token != AdminToken || id == "" {
			http.Error(w, "Accès refusé", http.StatusUnauthorized)
			return
		}
		_ = rejectEnrollment(id)
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/admin/kick", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		pubKey := r.URL.Query().Get("pubkey")
		if token != AdminToken || pubKey == "" {
			http.Error(w, "Accès refusé", http.StatusUnauthorized)
			return
		}
		_ = revokePeer(pubKey)
		w.WriteHeader(http.StatusOK)
	})

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	log.Printf("[ADMIN] API d'administration écoute sur HTTP %s", addr)
	_ = http.ListenAndServe(addr, mux)
}

func handleAdminUDP(pkt *protocol.Packet, conn *net.UDPConn, clientAddr *net.UDPAddr) {
	var cmd AdminCommand
	if err := json.Unmarshal(pkt.Payload, &cmd); err != nil {
		// Format brut hérité : juste lister les pairs
		cmd.Action = "list_peers"
		cmd.Token = AdminToken
	}

	if cmd.Token != AdminToken {
		return
	}

	var respPayload []byte
	switch cmd.Action {
	case "list_pending":
		enrollments := loadEnrollments()
		var pending []*EnrollmentRequest
		for _, e := range enrollments {
			if e.Status == "pending" {
				pending = append(pending, e)
			}
		}
		respPayload, _ = json.Marshal(pending)

	case "approve":
		if req, err := approveEnrollment(cmd.ID); err == nil {
			respPayload, _ = json.Marshal(req)
		} else {
			respPayload = []byte(fmt.Sprintf(`{"error":"%s"}`, err.Error()))
		}

	case "reject":
		_ = rejectEnrollment(cmd.ID)
		respPayload = []byte(`{"status":"rejected"}`)

	case "revoke":
		_ = revokePeer(cmd.PubKey)
		respPayload = []byte(`{"status":"revoked"}`)

	default: // "list_peers"
		peers, err := getWireGuardPeers()
		if err == nil {
			respPayload, _ = json.Marshal(peers)
		}
	}

	if respPayload != nil {
		resp := &protocol.Packet{
			Type:      protocol.TypeAdminResp,
			Seq:       pkt.Seq,
			Timestamp: time.Now().UnixNano(),
			Payload:   respPayload,
		}
		_, _ = conn.WriteToUDP(resp.Encode(), clientAddr)
	}
}

func handleEnrollUDP(pkt *protocol.Packet, conn *net.UDPConn, clientAddr *net.UDPAddr) {
	var cmd EnrollCommand
	if err := json.Unmarshal(pkt.Payload, &cmd); err != nil {
		return
	}

	enrollments := loadEnrollments()
	var respData map[string]interface{}

	switch cmd.Action {
	case "request":
		// Vérifier si la pubkey existe déjà
		var found *EnrollmentRequest
		for _, e := range enrollments {
			if e.PubKey == cmd.PubKey {
				found = e
				break
			}
		}

		if found != nil {
			if found.Status == "approved" {
				respData = map[string]interface{}{
					"status":        "approved",
					"assigned_ip":   found.AssignedIP,
					"server_pubkey": ServerPubKey,
					"endpoint":      ServerEndpoint,
				}
			} else {
				respData = map[string]interface{}{
					"status": found.Status,
					"id":     found.ID,
				}
			}
		} else {
			// Nouvelle demande d'ami
			idBytes := make([]byte, 6)
			_, _ = rand.Read(idBytes)
			id := hex.EncodeToString(idBytes)

			alias := strings.TrimSpace(cmd.Alias)
			if alias == "" {
				alias = fmt.Sprintf("Ami-%s", id[:4])
			}
			hostname := strings.TrimSpace(cmd.Hostname)
			if hostname == "" {
				hostname = "PC-Inconnu"
			}

			newReq := &EnrollmentRequest{
				ID:        id,
				PubKey:    cmd.PubKey,
				Alias:     alias,
				Hostname:  hostname,
				ClientIP:  clientAddr.IP.String(),
				CreatedAt: time.Now().Unix(),
				Status:    "pending",
			}
			enrollments[id] = newReq
			saveEnrollments(enrollments)

			log.Printf("[AMI] Nouvelle demande d'accès : %s (%s) depuis %s (ID: %s)", alias, hostname, clientAddr.IP.String(), id)
			respData = map[string]interface{}{
				"status":  "pending",
				"id":      id,
				"message": "Demande envoyée, en attente d'approbation",
			}
		}

	case "check":
		var found *EnrollmentRequest
		for _, e := range enrollments {
			if e.PubKey == cmd.PubKey {
				found = e
				break
			}
		}

		if found != nil {
			if found.Status == "approved" {
				respData = map[string]interface{}{
					"status":        "approved",
					"assigned_ip":   found.AssignedIP,
					"server_pubkey": ServerPubKey,
					"endpoint":      ServerEndpoint,
				}
			} else {
				respData = map[string]interface{}{
					"status": found.Status,
				}
			}
		} else {
			respData = map[string]interface{}{
				"status": "not_found",
			}
		}
	}

	if respData != nil {
		data, _ := json.Marshal(respData)
		resp := &protocol.Packet{
			Type:      protocol.TypeEnrollResp,
			Seq:       pkt.Seq,
			Timestamp: time.Now().UnixNano(),
			Payload:   data,
		}
		_, _ = conn.WriteToUDP(resp.Encode(), clientAddr)
	}
}

func main() {
	port := flag.Int("port", 4141, "Port UDP d'ecoute du relais CrazyPing")
	adminPort := flag.Int("admin", 4140, "Port TCP d'administration HTTP")
	dedupTTL := flag.Duration("ttl", 3*time.Second, "Duree de retention du filtre anti-doublon")
	flag.Parse()

	go startAdminHTTPServer(*adminPort)

	addr := net.UDPAddr{
		Port: *port,
		IP:   net.ParseIP("0.0.0.0"),
	}

	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		log.Fatalf("[FATAL] Impossible d'ecouter sur le port UDP %d: %v", *port, err)
	}
	defer conn.Close()

	_ = conn.SetReadBuffer(4 * 1024 * 1024)
	_ = conn.SetWriteBuffer(4 * 1024 * 1024)

	d := dedup.New(*dedupTTL)
	var stats Stats

	log.Printf("======================================================")
	log.Printf("   CrazyPing Relay Server (VPS)")
	log.Printf("   Ecoute UDP active sur :%d", *port)
	log.Printf("   Filtre Anti-Doublon TTL : %v", *dedupTTL)
	log.Printf("   Système d'approbation d'amis actif")
	log.Printf("======================================================")

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			total := atomic.LoadUint64(&stats.TotalReceived)
			dups := atomic.LoadUint64(&stats.DuplicatesSaved)
			pings := atomic.LoadUint64(&stats.PingsHandled)
			fwd := atomic.LoadUint64(&stats.ForwardsToGame)
			replies := atomic.LoadUint64(&stats.RepliesFromGame)

			var dupPct float64
			if total > 0 {
				dupPct = float64(dups) / float64(total) * 100.0
			}

			log.Printf("[STATS] Total: %d | Doublons filtres: %d (%.1f%%) | Pings: %d | Envois Jeux: %d | Retours: %d",
				total, dups, dupPct, pings, fwd, replies)
		}
	}()

	var sessionMu sync.RWMutex
	clientSessions := make(map[string]*net.UDPAddr)

	buf := make([]byte, 65535)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nArret du relais CrazyPing...")
		os.Exit(0)
	}()

	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("[WARN] Erreur de lecture UDP: %v", err)
			continue
		}

		atomic.AddUint64(&stats.TotalReceived, 1)

		pkt, err := protocol.Decode(buf[:n])
		if err != nil {
			continue
		}

		switch pkt.Type {
		case protocol.TypePing:
			atomic.AddUint64(&stats.PingsHandled, 1)
			pong := &protocol.Packet{
				Type:      protocol.TypePong,
				Seq:       pkt.Seq,
				Timestamp: pkt.Timestamp,
			}
			_, _ = conn.WriteToUDP(pong.Encode(), clientAddr)

		case protocol.TypeAdminReq:
			handleAdminUDP(pkt, conn, clientAddr)

		case protocol.TypeEnrollReq:
			handleEnrollUDP(pkt, conn, clientAddr)

		case protocol.TypeData:
			if d.IsDuplicate(pkt.Seq) {
				atomic.AddUint64(&stats.DuplicatesSaved, 1)
				continue
			}

			atomic.AddUint64(&stats.ForwardsToGame, 1)

			targetAddr := &net.UDPAddr{
				IP:   pkt.TargetIP,
				Port: int(pkt.TargetPort),
			}

			targetKey := targetAddr.String()
			sessionMu.Lock()
			clientSessions[targetKey] = clientAddr
			sessionMu.Unlock()

			go forwardToGame(pkt.Payload, targetAddr, conn, clientAddr)
		}
	}
}

func forwardToGame(payload []byte, target *net.UDPAddr, relayConn *net.UDPConn, clientAddr *net.UDPAddr) {
	outConn, err := net.DialUDP("udp", nil, target)
	if err != nil {
		return
	}
	defer outConn.Close()

	_ = outConn.SetDeadline(time.Now().Add(5 * time.Second))
	_, err = outConn.Write(payload)
	if err != nil {
		return
	}

	replyBuf := make([]byte, 4096)
	n, err := outConn.Read(replyBuf)
	if err != nil || n == 0 {
		return
	}

	replyPkt := protocol.NewData(0, target.IP, uint16(target.Port), replyBuf[:n])
	_, _ = relayConn.WriteToUDP(replyPkt.Encode(), clientAddr)
}
