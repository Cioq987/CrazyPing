# CrazyPing — Accélérateur Réseau & Réducteur de Latence Haute Performance

[![Version](https://img.shields.io/badge/Version-v1.4.3-cyan.svg?style=flat)](https://github.com/Cioq987/CrazyPing/releases/latest)
[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux-0078D6?style=flat)](https://github.com/Cioq987/CrazyPing)
[![Latency](https://img.shields.io/badge/Tahiti%20%E2%86%92%20San%20Jos%C3%A9-~108%20ms-brightgreen)](docs/ROUTING_TABLES.md)
[![Auto--Updater](https://img.shields.io/badge/Auto--Updater-GitHub%20Releases-blueviolet)](https://github.com/Cioq987/CrazyPing/releases/latest)
[![License](https://img.shields.io/badge/License-Proprietary-red.svg)](#)

**CrazyPing** est une solution logicielle et réseau d'optimisation de latence, de stabilisation de gigue (*jitter*) et d'élimination de la perte de paquets (*packet loss*), spécialement conçue pour les liaisons sous-marines transpacifiques reliant la Polynésie française (**Tahiti**) aux centres de données nord-américains (San José / Oregon / Los Angeles).

L'application s'appuie sur une pile réseau hybride de haute précision :
- **Pilote Noyau Haute Performance** : Moteur virtuel **Wintun** (opérant dans le Ring 0 de Windows) intégré silencieusement dans le package d'installation sans configuration complexe.
- **Protocole Overlay Sécurisé & MTU 1380** : Chiffrement asymétrique Curve25519 et AEAD ChaCha20-Poly1305 calibré à MTU 1380 pour éliminer toute fragmentation IP sur le câble sous-marin Honotua.
- **Routage Sélectif (Split-Tunneling)** : Seuls les paquets UDP/TCP des jeux ciblés empruntent la dorsale optimisée ; la navigation web et les appels Discord restent sur les routes locales sans aucune latence additionnelle.
- **Auto-Détection des Jeux en Temps Réel** : Surveillance dynamique des processus exécutables et commutation automatique du profil de tunnel dès le lancement du jeu.
- **Mode VPN Global USA & Quota Hybride** : Navigation complète via une adresse IP américaine dédiée avec jauge journalière de 10 Go/jour.
- **Auto-Mise à Jour Transparente (Auto-Updater)** : Détection asynchrone, affichage des notes de version avant application et mise à niveau en 1 clic avec élévation administrative Windows (`RunAs`).

---

## 🌴 Contexte Géographique & Performance Réseau

Depuis Tahiti, les flux réseau conventionnels souffrent souvent d'un acheminement sous-optimal imposé par les politiques de transit BGP des FAI locaux (détours par Hawaï, saturations aux heures de pointe, variations de ping de +40 à +100 ms).

```
   [ Tahiti (Vini / Vodafone) ]
                |
          (Câble Honotua)
                |
                v  ~108 ms
    [ Relais VPS CrazyPing (San José, CA) ]
        |                 |                 |
        v                 v                 v
[ Riot Direct ]   [ AWS GameLift ]   [ Valve SDR ]
(Valorant / LoL)     (Fortnite)          (CS2)
   110-114 ms        112-118 ms        114-120 ms
```

### Résultats de Mesure en Conditions Réelles
- **Ping Moyen Tahiti $\rightarrow$ San José** : **~108 ms** (liaison stable sans gigue).
- **Packet Loss Résiduel** : **0.0%** grâce aux tampons socket de 4 Mo et au MTU ajusté à 1380 octets.
- **Stabilité de Jitter** : $\Delta < 1.5 \text{ ms}$ sur sessions prolongées.

---

## 🎮 Jeux Supportés et Statut des Profils

| Profil / Jeu Compétitif | Statut | Infrastructure Cible | Profil Configuration |
| :--- | :---: | :--- | :--- |
| **VPN Global USA** | `● Prêt` | San José, CA (IP Américaine 146.235.218.190) | `CrazyPing-GlobalVPN.conf` (Quota 10 Go/j) |
| **Fortnite** | `● Prêt` | Epic Online Services + AWS GameLift US-West | `CrazyPing-Fortnite.conf` |
| **Mistfall Hunter** | `● Prêt` | NetEase Cloud Nodes NA-West | `CrazyPing-MistfallHunter.conf` |
| **Valorant** | `● Prêt` | Riot Direct US-West (San José / LA / Seattle) | `CrazyPing-Valorant.conf` |
| **League of Legends** | `● Prêt` | Riot Direct NA (Chicago / US-West) | `CrazyPing-LoL.conf` |
| **Overwatch 2** | `● Prêt` | Blizzard Entertainment (NA-West / lax1) | `CrazyPing-Overwatch.conf` |
| **Marvel Rivals** | `🚧 En dév.` | NetEase / AWS US-West | En intégration |
| **Counter-Strike 2** | `🚧 En dév.` | Valve SDR (Steam Datagram Relay US-West) | En intégration |
| **Apex Legends** | `🚧 En dév.` | Multiplay / Electronic Arts NA | En intégration |
| **Call of Duty: Warzone** | `🚧 En dév.` | Demonware / Activision Blizzard NA | En intégration |

---

## ⚡ Auto-Activation & Expérience Utilisateur Instantanée

CrazyPing intègre un système d'enrôlement automatique à haute fluidité :
1. **Premier Lancement** : Une modale invite le joueur à saisir son **Pseudo** (ex: *Teiki*).
2. **Génération Cryptographique Locale** : Une paire de clés **Curve25519** est générée localement sur le poste client.
3. **Provisioning Serveur en 100 ms** : Le serveur Linux réserve la prochaine IP libre du sous-réseau `10.10.0.0/24`, injecte la clé publique dans la table noyau `wg0`, et configure instantanément les profils locaux du joueur.
4. Le joueur clique sur **OPTIMISER** et joue immédiatement !

---

## 🔄 Module d'Auto-Mise à Jour en Direct (Live Auto-Updater)

L'application intègre un moteur de mise à jour autonome connecté directement aux Releases GitHub :
- **Vérification Asynchrone** : 2.5 secondes après l'ouverture, l'application vérifie discrètement si une version plus récente que la version locale a été publiée.
- **Bannière Interactive Néon & Modale Dédiée** : Une notification dynamique apparaît avec le nom et les nouveautés de la version. L'utilisateur peut consulter les notes officielles détaillées avant de valider.
- **Installation 1-Clic avec Élévation UAC** : Le clic sur *« Mettre à jour maintenant »* télécharge l'installateur dans `%TEMP%`, déclenche l'installation administrative silencieuse (`RunAs`), ferme proprement les tunnels actifs et relance la nouvelle version.

---

## 🧪 Validation Réseau & Batterie de Tests Automatisés

Le projet fait l'objet d'un protocole de validation continue rigoureux :

```
[ Banc d'Essai Automatisé ]
  ├── 1. Service Démon Linux       [ PASS ] -> Active (crazyping-server.service)
  ├── 2. Interface WireGuard Kernel[ PASS ] -> wg0 écoute sur UDP 51820
  ├── 3. Pare-Feu d'Isolation L3   [ PASS ] -> FORWARD -i wg0 -o wg0 -j REJECT
  ├── 4. Transit Honotua TCP/UDP   [ PASS ] -> RTT ~108-112 ms sans perte
  ├── 5. Auto-Enrollment Live Test [ PASS ] -> Requête UDP traitée en 112.45 ms
  ├── 6. Intégrité Trames Réseau MTU [ PASS ] -> Flux sous le seuil critique des 1 380 octets
  └── 7. Téléchargement Public Setup[ PASS ] -> HTTP 200 OK (9.65 Mo avec pilote Wintun)
```

Chaque modification du serveur ou de l'application est vérifiée par sonde directe avant mise en production.

---

## 📚 Documentation Technique Complète

Pour approfondir les détails d'implémentation, protocoles et sécurité, consultez les guides modulaires dans le dossier [`docs/`](docs/README.md) :

- [**Index & Cartographie Documentaire**](docs/README.md) : Sommaire unifié et diagramme d'architecture système global.
- [**Architecture Réseau et Logicielle**](docs/ARCHITECTURE.md) : Décomposition en couches (Kernel, Engine Go, UI), cycle de vie, watchdog HTTP/Beacon et protocole de contrôle.
- [**Flux Réseau et Protocoles**](docs/flows.md) : Format binaire 23 octets, diagrammes de séquence Mermaid (enrôlement Curve25519, télémétrie RTT).
- [**Cas d'Utilisation Système**](docs/use-cases.md) : Scénarios nominaux et d'erreur (démarrage, split-tunneling, auto-updater, arrêt sécurisé).
- [**Tables de Routage et Split-Tunneling**](docs/ROUTING_TABLES.md) : Liste exhaustive des sous-réseaux CIDR qualifiés et calcul du MTU optimal (1380 octets).
- [**Modèle de Sécurité et Analyse des Menaces**](docs/SECURITY.md) : Cloisonnement L3 inter-clients, cryptokey routing, enrôlement dynamique ZTNA et comparatif légal.
- [**Guide de Déploiement et d'Exploitation**](docs/DEPLOYMENT.md) : Provisionnement du serveur Linux, configuration systemd, sysctl BBR et packaging Inno Setup.
- [**Journal des Évolutions Logicielles**](docs/changelog.md) : Historique chronologique des versions et suivi des correctifs de résilience.

---

## 🚀 Démarrage Rapide

### Installation du Client
Téléchargez la dernière version officielle de CrazyPing :

👉 [**Télécharger CrazyPing-Setup.exe (Dernière Version)**](https://github.com/Cioq987/CrazyPing/releases/latest/download/CrazyPing-Setup.exe)

L'installateur configure automatiquement l'exécutable, les raccourcis bureau et installe de façon totalement transparente le pilote réseau officiel Wintun / WireGuard.

---

## 🛠 Compilation depuis les Sources

### Compiler le Client Windows
```powershell
# Générer les ressources Windows (Manifeste UAC + Icône)
rsrc -manifest cmd/gui/CrazyPing.exe.manifest -ico icon.ico -o cmd/gui/rsrc.syso

# Compiler le binaire autonome
go build -ldflags="-s -w -H=windowsgui" -o CrazyPing.exe ./cmd/gui
```

### Compiler le Démon Serveur Linux
```powershell
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -ldflags="-s -w" -o crazyping-server-linux-amd64 ./cmd/server
```

---

## 📄 Propriété Intellectuelle & Clause de Non-Affiliation

**CrazyPing** est un logiciel propriétaire indépendant. Copyright © 2026 CrazyPing Project. Tous droits réservés.

L'utilisation de ce logiciel, de ses binaires et de son code source est strictement réservée à un cadre privé et personnel non commercial. Toute reproduction, redistribution, décompilation, modification ou exploitation commerciale sans autorisation écrite préalable est formellement interdite.

Ce projet n'est affilié, sponsorisé ni approuvé par aucune société éditrice de jeux ou de solutions logicielles tierces mentionnées (Epic Games, Riot Games, Valve, NetEase, ExitLag). Tous les noms de marques et logos demeurent la propriété exclusive de leurs détenteurs respectifs.
