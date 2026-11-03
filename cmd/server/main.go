package main

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/base64"
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
	"regexp"
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
	ServerPubKey   = "5dRNSNB9hA8qxA00mYSXU97lFG/s3yyNGwUyhKKw8Cw="
	ServerEndpoint = "146.235.218.190:51820"
)

var (
	adminTokenPath = "/home/ubuntu/crazyping-admin.token"
	adminToken     string
)

func loadOrGenerateAdminToken() string {
	if data, err := os.ReadFile(adminTokenPath); err == nil {
		tok := strings.TrimSpace(string(data))
		if len(tok) >= 32 {
			return tok
		}
	}
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	tok := hex.EncodeToString(b)
	_ = os.WriteFile(adminTokenPath, []byte(tok+"\n"), 0600)
	log.Printf("[SECURITY] Nouveau token maître généré dans %s (Permissions 0600)", adminTokenPath)
	return tok
}

func isValidWireGuardPubKey(key string) bool {
	clean := strings.TrimSpace(key)
	if len(clean) != 44 {
		return false
	}
	b, err := base64.StdEncoding.DecodeString(clean)
	return err == nil && len(b) == 32
}

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
	VpnQuota             string `json:"vpnQuota"`
	VpnBlocked           bool   `json:"vpnBlocked"`
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
	Action string `json:"action"` // "list_peers", "list_pending", "approve", "reject", "revoke", "audit_peer"
	Token  string `json:"token"`
	ID     string `json:"id"`
	PubKey string `json:"pubKey"`
}

type PingResult struct {
	Transmitted int     `json:"transmitted"`
	Received    int     `json:"received"`
	LossPct     float64 `json:"lossPct"`
	MinMs       float64 `json:"minMs"`
	AvgMs       float64 `json:"avgMs"`
	MaxMs       float64 `json:"maxMs"`
	JitterMs    float64 `json:"jitterMs"`
	Success     bool    `json:"success"`
}

type MtuResult struct {
	TestedSize int    `json:"testedSize"`
	Pass       bool   `json:"pass"`
	Details    string `json:"details"`
}

type GameTargetLatency struct {
	Name        string  `json:"name"`
	IP          string  `json:"ip"`
	Location    string  `json:"location"`
	VpsPingMs   float64 `json:"vpsPingMs"`
	TotalPingMs float64 `json:"totalPingMs"`
}

type PeerAuditReport struct {
	PubKey        string              `json:"pubKey"`
	Alias         string              `json:"alias"`
	ClientIP      string              `json:"clientIp"`
	Endpoint      string              `json:"endpoint"`
	IsOnline      bool                `json:"isOnline"`
	HandshakeAge  int64               `json:"handshakeAge"`
	HandshakeStr  string              `json:"handshakeStr"`
	HonotuaPing   PingResult          `json:"honotuaPing"`
	MtuStatus     MtuResult           `json:"mtuStatus"`
	GameTargets   []GameTargetLatency `json:"gameTargets"`
	OverallStatus string              `json:"overallStatus"` // "OPTIMAL", "WARNING", "OFFLINE"
	Diagnosis     string              `json:"diagnosis"`
	Logs          []string            `json:"logs"`
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

		quotaStr, isBlocked := getServerPeerQuota(cleanIP)

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
			VpnQuota:             quotaStr,
			VpnBlocked:           isBlocked,
		})
	}
	return peers, nil
}

func startAdminHTTPServer(port int) {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/admin/peers", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token != adminToken {
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
		if token != adminToken {
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
		if token != adminToken || id == "" {
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
		if token != adminToken || id == "" {
			http.Error(w, "Accès refusé", http.StatusUnauthorized)
			return
		}
		_ = rejectEnrollment(id)
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/admin/kick", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		pubKey := r.URL.Query().Get("pubkey")
		if token != adminToken || pubKey == "" {
			http.Error(w, "Accès refusé", http.StatusUnauthorized)
			return
		}
		_ = revokePeer(pubKey)
		w.WriteHeader(http.StatusOK)
	})

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	log.Printf("[ADMIN] API d'administration locale écoute sur %s", addr)
	_ = http.ListenAndServe(addr, mux)
}

func executePing(ip string, count int, intervalSec float64, timeoutSec int, payloadSize int, doNotFragment bool) PingResult {
	var args []string
	args = append(args, "-c", fmt.Sprintf("%d", count))
	if intervalSec > 0 {
		args = append(args, "-i", fmt.Sprintf("%.2f", intervalSec))
	}
	args = append(args, "-W", fmt.Sprintf("%d", timeoutSec))
	if payloadSize > 0 {
		args = append(args, "-s", fmt.Sprintf("%d", payloadSize))
	}
	if doNotFragment {
		args = append(args, "-M", "do")
	}
	args = append(args, ip)

	cmd := exec.Command("ping", args...)
	out, _ := cmd.CombinedOutput()
	outStr := string(out)

	res := PingResult{
		Transmitted: count,
		LossPct:     100.0,
	}

	reStats := regexp.MustCompile(`(\d+)\s+packets transmitted,\s+(\d+)\s+(?:packets\s+)?received,\s+([\d\.]+)%\s+packet loss`)
	if m := reStats.FindStringSubmatch(outStr); len(m) >= 4 {
		tx, _ := strconv.Atoi(m[1])
		rx, _ := strconv.Atoi(m[2])
		loss, _ := strconv.ParseFloat(m[3], 64)
		res.Transmitted = tx
		res.Received = rx
		res.LossPct = loss
	}

	reRtt := regexp.MustCompile(`rtt min/avg/max/mdev = ([\d\.]+)/([\d\.]+)/([\d\.]+)/([\d\.]+)`)
	if m := reRtt.FindStringSubmatch(outStr); len(m) >= 5 {
		minVal, _ := strconv.ParseFloat(m[1], 64)
		avgVal, _ := strconv.ParseFloat(m[2], 64)
		maxVal, _ := strconv.ParseFloat(m[3], 64)
		mdevVal, _ := strconv.ParseFloat(m[4], 64)
		res.MinMs = minVal
		res.AvgMs = avgVal
		res.MaxMs = maxVal
		res.JitterMs = mdevVal
		res.Success = res.Received > 0
	} else if res.Received > 0 {
		reSingle := regexp.MustCompile(`time=([\d\.]+)\s*ms`)
		if m := reSingle.FindStringSubmatch(outStr); len(m) >= 2 {
			tVal, _ := strconv.ParseFloat(m[1], 64)
			res.MinMs = tVal
			res.AvgMs = tVal
			res.MaxMs = tVal
			res.Success = true
		}
	}

	return res
}

func auditPeer(pubKey string) (*PeerAuditReport, error) {
	peers, err := getWireGuardPeers()
	if err != nil {
		return nil, err
	}

	var targetPeer *PeerInfo
	for _, p := range peers {
		if p.PubKey == pubKey {
			targetPeer = &p
			break
		}
	}

	if targetPeer == nil {
		return nil, fmt.Errorf("peer introuvable pour la clé fournie")
	}

	cleanIP := strings.TrimSuffix(targetPeer.AllowedIPs, "/32")
	now := time.Now().Unix()
	handshakeAge := int64(0)
	if targetPeer.LatestHandshakeEpoch > 0 {
		handshakeAge = now - targetPeer.LatestHandshakeEpoch
	}

	report := &PeerAuditReport{
		PubKey:       targetPeer.PubKey,
		Alias:        targetPeer.Alias,
		ClientIP:     cleanIP,
		Endpoint:     targetPeer.Endpoint,
		IsOnline:     targetPeer.IsOnline,
		HandshakeAge: handshakeAge,
		HandshakeStr: targetPeer.LatestHandshakeHuman,
		Logs:         []string{},
	}

	report.Logs = append(report.Logs, fmt.Sprintf("[AUDIT] Démarrage du diagnostic pour %s (%s)", targetPeer.Alias, cleanIP))
	report.Logs = append(report.Logs, fmt.Sprintf("[AUDIT] Endpoint: %s • Handshake: %s", targetPeer.Endpoint, targetPeer.LatestHandshakeHuman))

	// 1. Sonde de la liaison Honotua via l'interface WireGuard
	report.Logs = append(report.Logs, fmt.Sprintf("[AUDIT] Sonde ICMP vers %s (tunnel WireGuard / Wintun)...", cleanIP))
	honotuaPing := executePing(cleanIP, 3, 0.2, 1, 56, false)
	report.HonotuaPing = honotuaPing

	if honotuaPing.Success {
		report.Logs = append(report.Logs, fmt.Sprintf("[AUDIT] Honotua RTT: %.1f ms (min: %.1f, max: %.1f, gigue: %.2f ms, perte: %.1f%%)",
			honotuaPing.AvgMs, honotuaPing.MinMs, honotuaPing.MaxMs, honotuaPing.JitterMs, honotuaPing.LossPct))
	} else {
		report.Logs = append(report.Logs, fmt.Sprintf("[AUDIT] Injoignable : 100%% de paquets perdus vers %s", cleanIP))
	}

	// 2. Vérification MTU Clamp (paquet IP 1380 octets -> 1352 octets de payload ICMP avec DF bit)
	report.Logs = append(report.Logs, "[AUDIT] Test MTU 1380 octets avec bit DF (Don't Fragment)...")
	mtuPing := executePing(cleanIP, 1, 0, 1, 1352, true)
	report.MtuStatus = MtuResult{
		TestedSize: 1380,
		Pass:       mtuPing.Success,
	}
	if mtuPing.Success {
		report.MtuStatus.Details = "Paquet 1380 octets transmis sans fragmentation"
		report.Logs = append(report.Logs, "[AUDIT] Intégrité MTU : Conforme (MTU 1380 clamp actif)")
	} else {
		report.MtuStatus.Details = "Échec ou fragmentation requise pour 1380 octets"
		report.Logs = append(report.Logs, "[AUDIT] Intégrité MTU : Anomalie ou client injoignable")
	}

	// 3. Mesure de latence vers les cibles E-Sport clés
	targets := []struct {
		Name     string
		IP       string
		Location string
	}{
		{Name: "Steam SDR LAX", IP: "162.254.195.1", Location: "Los Angeles (US-West)"},
		{Name: "Steam SDR Seattle", IP: "162.254.193.1", Location: "Seattle (US-NW)"},
		{Name: "Cloudflare Edge", IP: "1.1.1.1", Location: "San José Gateway"},
	}

	report.Logs = append(report.Logs, "[AUDIT] Mesure des latences amont vers les infrastructures E-Sport...")
	report.GameTargets = make([]GameTargetLatency, len(targets))

	clientHonotuaRtt := honotuaPing.AvgMs
	if clientHonotuaRtt == 0 && honotuaPing.Success {
		clientHonotuaRtt = honotuaPing.MinMs
	}

	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func(idx int, name, ip, loc string) {
			defer wg.Done()
			p := executePing(ip, 2, 0.2, 1, 56, false)
			vpsRtt := p.AvgMs
			totalRtt := 0.0
			if p.Success && honotuaPing.Success {
				totalRtt = clientHonotuaRtt + vpsRtt
			} else if p.Success {
				totalRtt = vpsRtt
			}
			report.GameTargets[idx] = GameTargetLatency{
				Name:        name,
				IP:          ip,
				Location:    loc,
				VpsPingMs:   vpsRtt,
				TotalPingMs: totalRtt,
			}
		}(i, t.Name, t.IP, t.Location)
	}
	wg.Wait()

	for _, gt := range report.GameTargets {
		report.Logs = append(report.Logs, fmt.Sprintf("[AUDIT] %s (%s) : VPS %.1f ms | Joueur estimé: %.1f ms",
			gt.Name, gt.IP, gt.VpsPingMs, gt.TotalPingMs))
	}

	// 4. Évaluation globale de la santé de la session
	if !targetPeer.IsOnline && !honotuaPing.Success {
		report.OverallStatus = "OFFLINE"
		report.Diagnosis = "Client inactif ou application CrazyPing fermée. Vérifiez que CrazyPing est lancé chez le joueur."
		report.Logs = append(report.Logs, "[AUDIT] Statut global : HORS LIGNE")
	} else if honotuaPing.LossPct > 5.0 || honotuaPing.JitterMs > 15.0 || !report.MtuStatus.Pass {
		report.OverallStatus = "WARNING"
		report.Diagnosis = "Liaison instable : gigue élevée ou perte de paquets détectée sur le câble Honotua."
		report.Logs = append(report.Logs, "[AUDIT] Statut global : PERFORMANCES DÉGRADÉES")
	} else {
		report.OverallStatus = "OPTIMAL"
		report.Diagnosis = "Connexion E-Sport optimale et conforme. Tunnel Wintun opérationnel, 0% perte, MTU 1380 respecté."
		report.Logs = append(report.Logs, "[AUDIT] Statut global : OPTIMAL")
	}

	return report, nil
}

func handleAdminUDP(pkt *protocol.Packet, conn *net.UDPConn, clientAddr *net.UDPAddr) {
	var cmd AdminCommand
	if err := json.Unmarshal(pkt.Payload, &cmd); err != nil {
		log.Printf("[SECURITY] Rejet trame admin UDP invalide depuis %s", clientAddr.String())
		return
	}

	if cmd.Token == "" || cmd.Token != adminToken {
		log.Printf("[SECURITY] Tentative admin rejetée (token non autorisé) depuis %s", clientAddr.String())
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

	case "audit_peer":
		if rep, err := auditPeer(cmd.PubKey); err == nil {
			respPayload, _ = json.Marshal(rep)
		} else {
			respPayload = []byte(fmt.Sprintf(`{"error":"%s"}`, err.Error()))
		}

	default: // "list_peers"
		peers, err := getWireGuardPeers()
		if err == nil {
			respPayload, _ = json.Marshal(peers)
		}
	}

	if respPayload != nil {
		finalPayload := respPayload
		// Si la charge utile dépasse 600 octets, compression gzip automatique pour respecter
		// impérativement le MTU de 1380 octets du câble sous-marin Honotua
		if len(respPayload) > 600 {
			var b bytes.Buffer
			gz := gzip.NewWriter(&b)
			_, _ = gz.Write(respPayload)
			_ = gz.Close()
			finalPayload = b.Bytes()
		}

		resp := &protocol.Packet{
			Type:      protocol.TypeAdminResp,
			Seq:       pkt.Seq,
			Timestamp: time.Now().UnixNano(),
			Payload:   finalPayload,
		}
		_, _ = conn.WriteToUDP(resp.Encode(), clientAddr)
	}
}

func handleEnrollUDP(pkt *protocol.Packet, conn *net.UDPConn, clientAddr *net.UDPAddr) {
	var cmd EnrollCommand
	if err := json.Unmarshal(pkt.Payload, &cmd); err != nil {
		return
	}

	if !isValidWireGuardPubKey(cmd.PubKey) {
		log.Printf("[SECURITY] Rejet enrôlement: clé publique WireGuard invalide depuis %s", clientAddr.String())
		respData := map[string]interface{}{
			"status":  "error",
			"message": "Clé publique invalide (format Curve25519 base64 32 octets requis)",
		}
		data, _ := json.Marshal(respData)
		resp := &protocol.Packet{
			Type:      protocol.TypeEnrollResp,
			Seq:       pkt.Seq,
			Timestamp: time.Now().UnixNano(),
			Payload:   data,
		}
		_, _ = conn.WriteToUDP(resp.Encode(), clientAddr)
		return
	}

	enrollments := loadEnrollments()

	// Protection anti-épuisement d'IPs : maximum 3 allocations par IP publique cliente
	clientIPStr := clientAddr.IP.String()
	countForThisIP := 0
	for _, e := range enrollments {
		if e.ClientIP == clientIPStr && e.PubKey != cmd.PubKey && e.Status == "approved" {
			countForThisIP++
		}
	}
	if countForThisIP >= 3 {
		log.Printf("[SECURITY] Quota d'allocations dépassé pour l'IP %s", clientIPStr)
		respData := map[string]interface{}{
			"status":  "error",
			"message": "Quota d'allocations dépassé pour cette adresse IP",
		}
		data, _ := json.Marshal(respData)
		resp := &protocol.Packet{
			Type:      protocol.TypeEnrollResp,
			Seq:       pkt.Seq,
			Timestamp: time.Now().UnixNano(),
			Payload:   data,
		}
		_, _ = conn.WriteToUDP(resp.Encode(), clientAddr)
		return
	}

	var respData map[string]interface{}

	switch cmd.Action {
	case "auto_enroll":
		alias := strings.TrimSpace(cmd.Alias)
		if alias == "" {
			alias = "Joueur"
		}
		hostname := strings.TrimSpace(cmd.Hostname)
		if hostname == "" {
			hostname = "PC-Client"
		}

		var found *EnrollmentRequest
		for _, e := range enrollments {
			if e.PubKey == cmd.PubKey {
				found = e
				break
			}
		}

		if found != nil && found.Status == "approved" {
			found.Alias = alias
			saveAlias(found.PubKey, alias)
			saveEnrollments(enrollments)
			respData = map[string]interface{}{
				"status":        "approved",
				"assigned_ip":   found.AssignedIP,
				"server_pubkey": ServerPubKey,
				"endpoint":      ServerEndpoint,
				"alias":         alias,
			}
		} else {
			ip, err := allocateNextFreeIP()
			if err != nil {
				respData = map[string]interface{}{
					"status":  "error",
					"message": fmt.Sprintf("Erreur allocation IP : %v", err),
				}
			} else {
				cmdWg := exec.Command("wg", "set", "wg0", "peer", cmd.PubKey, "allowed-ips", fmt.Sprintf("%s/32", ip))
				if out, err := cmdWg.CombinedOutput(); err != nil {
					log.Printf("[AUTO-ENROLL] Erreur wg set: %v (%s)", err, string(out))
				}

				idBytes := make([]byte, 6)
				_, _ = rand.Read(idBytes)
				id := hex.EncodeToString(idBytes)

				req := &EnrollmentRequest{
					ID:         id,
					PubKey:     cmd.PubKey,
					Alias:      alias,
					Hostname:   hostname,
					ClientIP:   clientAddr.IP.String(),
					CreatedAt:  time.Now().Unix(),
					Status:     "approved",
					AssignedIP: ip,
				}
				enrollments[id] = req
				saveEnrollments(enrollments)
				saveAlias(cmd.PubKey, alias)

				log.Printf("[AUTO-ENROLL] Nouvel utilisateur auto-activé : %s (IP: %s, ID: %s) depuis %s", alias, ip, id, clientAddr.IP.String())

				respData = map[string]interface{}{
					"status":        "approved",
					"assigned_ip":   ip,
					"server_pubkey": ServerPubKey,
					"endpoint":      ServerEndpoint,
					"alias":         alias,
				}
			}
		}

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

	adminToken = loadOrGenerateAdminToken()

	// Initialisation du sous-système de quotas VPN et protection des jeux
	initQuotaSubsystem()
	startQuotaDaemon()

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
