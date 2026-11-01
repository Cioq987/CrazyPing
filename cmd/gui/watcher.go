package main

import (
	"fmt"
	"time"
)

// GameProcessDef associe un titre de jeu et son profil réseau à ses exécutables Windows réels
type GameProcessDef struct {
	GameName    string
	ConfName    string
	Executables []string
}

// SupportedGameProcesses recense les processus officiels des jeux supportés par CrazyPing
var SupportedGameProcesses = []GameProcessDef{
	{
		GameName: "Valorant",
		ConfName: "CrazyPing-Valorant",
		Executables: []string{
			"valorant-win64-shipping.exe",
			"valorant.exe",
		},
	},
	{
		GameName: "Fortnite",
		ConfName: "CrazyPing-Fortnite",
		Executables: []string{
			"fortniteclient-win64-shipping.exe",
			"fortniteclient-win64-shipping_eac.exe",
			"fortniteclient-win64-shipping_be.exe",
			"fortnite.exe",
		},
	},
	{
		GameName: "Mistfall Hunter",
		ConfName: "CrazyPing-MistfallHunter",
		Executables: []string{
			"mistfallhunter-win64-shipping.exe",
			"mistfallhunter.exe",
		},
	},
	{
		GameName: "League of Legends",
		ConfName: "CrazyPing-LoL",
		Executables: []string{
			"league of legends.exe",
		},
	},
	{
		GameName: "Overwatch 2",
		ConfName: "CrazyPing-Overwatch",
		Executables: []string{
			"overwatch.exe",
		},
	},
}

// startProcessWatcher scrute périodiquement la table des processus pour automatiser le tunnel Wintun
func (m *LiveMetrics) startProcessWatcher() {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		var missedIterations int
		const gracePeriodIterations = 15 // 15 cycles * 2s = 30s de période de grâce anti-flapping

		for range ticker.C {
			m.mu.Lock()
			enabled := m.AutoDetectEnabled
			m.mu.Unlock()

			if !enabled {
				missedIterations = 0
				continue
			}

			gameDef, exeName := findRunningGame(SupportedGameProcesses)

			m.mu.Lock()
			isTunnelActive := m.TunnelActive
			activeConf := m.ActiveConf
			isAutoTriggered := m.AutoTriggered
			m.mu.Unlock()

			if gameDef != nil {
				missedIterations = 0
				m.mu.Lock()
				m.DetectedGame = gameDef.GameName
				m.DetectedExe = exeName
				m.mu.Unlock()

				if !isTunnelActive {
					m.mu.Lock()
					m.ActiveGame = gameDef.GameName
					m.ActiveConf = gameDef.ConfName
					m.AutoTriggered = true
					m.mu.Unlock()

					m.AppendLog(fmt.Sprintf("[Auto-Détection] Processus détecté : %s -> Démarrage automatique de l'optimisation pour %s", exeName, gameDef.GameName))
					_ = m.startTunnel(gameDef.ConfName + ".conf")
				} else if isAutoTriggered && activeConf != gameDef.ConfName {
					// L'utilisateur a changé de jeu en cours de route
					m.mu.Lock()
					m.ActiveGame = gameDef.GameName
					m.ActiveConf = gameDef.ConfName
					m.mu.Unlock()

					m.AppendLog(fmt.Sprintf("[Auto-Détection] Nouveau jeu détecté : %s -> Bascule automatique du tunnel vers %s", exeName, gameDef.GameName))
					_ = m.startTunnel(gameDef.ConfName + ".conf")
				}
			} else {
				// Aucun processus de jeu surveillé dans la table
				m.mu.Lock()
				if m.DetectedGame != "" {
					m.DetectedGame = ""
					m.DetectedExe = ""
				}
				m.mu.Unlock()

				if isTunnelActive && isAutoTriggered {
					missedIterations++
					if missedIterations == 1 {
						m.AppendLog("[Auto-Détection] Processus de jeu fermé. Maintien du tunnel pendant 30s (chargement / reconnexion)...")
					} else if missedIterations >= gracePeriodIterations {
						m.AppendLog("[Auto-Détection] Délai de grâce écoulé : arrêt automatique du tunnel Wintun.")
						m.stopCurrentTunnels()
						m.mu.Lock()
						m.AutoTriggered = false
						m.mu.Unlock()
						missedIterations = 0
					}
				} else {
					missedIterations = 0
				}
			}
		}
	}()
}
