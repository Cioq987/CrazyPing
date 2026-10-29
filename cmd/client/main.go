package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"crazyping/pkg/protocol"
)

type ClientMetrics struct {
	PingsSent     uint64
	PongsReceived uint64
	LostPackets   uint64
	TotalRttMs    float64
	MinRttMs      float64
	MaxRttMs      float64
	JitterMs      float64
	lastRttMs     float64
	mu            sync.Mutex
}

func main() {
	relayHost := flag.String("relay", "127.0.0.1", "Adresse IP ou domaine du serveur VPS Relais")
	relayPort := flag.Int("port", 4141, "Port UDP du serveur Relais")
	multipath := flag.Bool("multipath", true, "Activer la duplication de paquets (Multipath Redundancy)")
	duplicateDelay := flag.Int("delay-ms", 2, "Delai en ms entre le 1er paquet et sa replique")
	mode := flag.String("mode", "bench", "Mode de fonctionnement: 'bench' (test RTT/loss) ou 'proxy' (tunnel jeu)")
	listenPort := flag.Int("listen", 8888, "Port d'ecoute local en mode proxy")
	targetGame := flag.String("game-target", "", "Cible du serveur de jeu en mode proxy (ex: 104.160.131.3:5000)")
	flag.Parse()

	relayAddrStr := fmt.Sprintf("%s:%d", *relayHost, *relayPort)
	rAddr, err := net.ResolveUDPAddr("udp", relayAddrStr)
	if err != nil {
		log.Fatalf("[FATAL] Impossible de resoudre l'adresse du relais: %v", err)
	}

	conn, err := net.DialUDP("udp", nil, rAddr)
	if err != nil {
		log.Fatalf("[FATAL] Erreur de connexion UDP vers le relais: %v", err)
	}
	defer conn.Close()

	// Optimisation buffers socket
	_ = conn.SetReadBuffer(4 * 1024 * 1024)
	_ = conn.SetWriteBuffer(4 * 1024 * 1024)

	fmt.Println("==================================================================")
	fmt.Printf("   CrazyPing Client (Tahiti Gaming Optimizer)\n")
	fmt.Printf("   Relais VPS        : %s\n", relayAddrStr)
	fmt.Printf("   Multipath         : %v (Delai replique: %d ms)\n", *multipath, *duplicateDelay)
	fmt.Printf("   Mode              : %s\n", *mode)
	fmt.Println("==================================================================")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	if *mode == "bench" {
		runBenchMode(conn, *multipath, *duplicateDelay, sigChan)
	} else if *mode == "proxy" {
		runProxyMode(conn, *listenPort, *targetGame, *multipath, *duplicateDelay, sigChan)
	}
}

func runBenchMode(conn *net.UDPConn, multipath bool, duplicateDelayMs int, sigChan chan os.Signal) {
	metrics := &ClientMetrics{
		MinRttMs: 999999.0,
	}

	var seqCounter uint32

	// Goroutine de reception des Pongs
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

			now := time.Now().UnixNano()
			rttNs := now - pkt.Timestamp
			rttMs := float64(rttNs) / 1e6

			metrics.mu.Lock()
			atomic.AddUint64(&metrics.PongsReceived, 1)
			metrics.TotalRttMs += rttMs
			if rttMs < metrics.MinRttMs {
				metrics.MinRttMs = rttMs
			}
			if rttMs > metrics.MaxRttMs {
				metrics.MaxRttMs = rttMs
			}
			if metrics.lastRttMs > 0 {
				diff := math.Abs(rttMs - metrics.lastRttMs)
				metrics.JitterMs = (metrics.JitterMs * 0.9) + (diff * 0.1) // RFC 3550 lissage gigue
			}
			metrics.lastRttMs = rttMs
			metrics.mu.Unlock()
		}
	}()

	ticker := time.NewTicker(200 * time.Millisecond) // 5 pings / seconde
	reportTicker := time.NewTicker(2 * time.Second)

	for {
		select {
		case <-sigChan:
			fmt.Println("\nFin du test CrazyPing.")
			return

		case <-ticker.C:
			seq := atomic.AddUint32(&seqCounter, 1)
			pingPkt := protocol.NewPing(seq)
			encoded := pingPkt.Encode()

			// Premier envoi
			_, _ = conn.Write(encoded)
			atomic.AddUint64(&metrics.PingsSent, 1)

			// Si multipath est actif, on envoie une seconde replique
			if multipath {
				go func(p []byte, delayMs int) {
					if delayMs > 0 {
						time.Sleep(time.Duration(delayMs) * time.Millisecond)
					}
					_, _ = conn.Write(p)
				}(encoded, duplicateDelayMs)
			}

		case <-reportTicker.C:
			sent := atomic.LoadUint64(&metrics.PingsSent)
			rcvd := atomic.LoadUint64(&metrics.PongsReceived)

			metrics.mu.Lock()
			var avgRtt float64
			if rcvd > 0 {
				avgRtt = metrics.TotalRttMs / float64(rcvd)
			}
			minR := metrics.MinRttMs
			if minR > 900000 {
				minR = 0
			}
			maxR := metrics.MaxRttMs
			jitter := metrics.JitterMs
			metrics.mu.Unlock()

			lossPct := 0.0
			if sent > 0 && sent >= rcvd {
				lossPct = float64(sent-rcvd) / float64(sent) * 100.0
			}

			fmt.Printf("\r[RTT] Min: %5.1f ms | Moy: %5.1f ms | Max: %5.1f ms | Gigue: %4.1f ms | Paquets: %d envoyes / %d reçus (Perte: %.1f%%)  ",
				minR, avgRtt, maxR, jitter, sent, rcvd, lossPct)
		}
	}
}

func runProxyMode(relayConn *net.UDPConn, listenPort int, targetGame string, multipath bool, duplicateDelayMs int, sigChan chan os.Signal) {
	if targetGame == "" {
		log.Fatalf("[ERREUR] En mode proxy, specifiez -game-target IP:Port (ex: -game-target 104.160.131.3:5000)")
	}

	gameAddr, err := net.ResolveUDPAddr("udp", targetGame)
	if err != nil {
		log.Fatalf("[FATAL] Impossible de resoudre le serveur de jeu %s: %v", targetGame, err)
	}

	localAddr := net.UDPAddr{Port: listenPort, IP: net.ParseIP("127.0.0.1")}
	localConn, err := net.ListenUDP("udp", &localAddr)
	if err != nil {
		log.Fatalf("[FATAL] Impossible d'ecouter sur le port local %d: %v", listenPort, err)
	}
	defer localConn.Close()

	log.Printf("[PROXY] Tunnel actif: configurez votre jeu sur 127.0.0.1:%d -> redirection vers %s via Relais", listenPort, targetGame)

	var seqCounter uint32
	var clientGameAddr *net.UDPAddr
	var mu sync.Mutex

	// Goroutine pour retourner les paquets du jeu vers le client local
	go func() {
		buf := make([]byte, 65535)
		for {
			n, err := relayConn.Read(buf)
			if err != nil {
				return
			}
			pkt, err := protocol.Decode(buf[:n])
			if err != nil || pkt.Type != protocol.TypeData {
				continue
			}

			mu.Lock()
			dest := clientGameAddr
			mu.Unlock()

			if dest != nil {
				_, _ = localConn.WriteToUDP(pkt.Payload, dest)
			}
		}
	}()

	// Boucle principale d'interception du jeu local
	buf := make([]byte, 65535)
	for {
		n, cAddr, err := localConn.ReadFromUDP(buf)
		if err != nil {
			break
		}

		mu.Lock()
		clientGameAddr = cAddr
		mu.Unlock()

		seq := atomic.AddUint32(&seqCounter, 1)
		pkt := protocol.NewData(seq, gameAddr.IP, uint16(gameAddr.Port), buf[:n])
		encoded := pkt.Encode()

		// Envoi au relais
		_, _ = relayConn.Write(encoded)

		// Duplication Multipath
		if multipath {
			go func(p []byte, delayMs int) {
				if delayMs > 0 {
					time.Sleep(time.Duration(delayMs) * time.Millisecond)
				}
				_, _ = relayConn.Write(p)
			}(encoded, duplicateDelayMs)
		}
	}
}
