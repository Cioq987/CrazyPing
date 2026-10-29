# CrazyPing — Accélérateur Réseau & Réducteur de Latence Haute Performance

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux-0078D6?style=flat)](https://github.com/Cioq987/CrazyPing)
[![License](https://img.shields.io/badge/License-Proprietary-red.svg)](#)
[![Latency](https://img.shields.io/badge/Tahiti%20%E2%86%92%20San%20Jos%C3%A9-~108%20ms-brightgreen)](docs/ROUTING_TABLES.md)
[![Auto--Updater](https://img.shields.io/badge/Auto--Updater-GitHub%20Releases-blueviolet)](https://github.com/Cioq987/CrazyPing/releases)
[![Tests](https://img.shields.io/badge/Tests-100%25%20Passing-success)](#-validation-r%C3%A9seau--batterie-de-tests-automatis%C3%A9s)

**CrazyPing** est une solution logicielle et réseau d'optimisation de latence, de stabilisation de gigue (*jitter*) et d'élimination de la perte de paquets (*packet loss*), spécialement conçue pour les liaisons sous-marines transpacifiques reliant la Polynésie française (**Tahiti**) aux centres de données nord-américains (San José / Oregon / Los Angeles).

L'application s'appuie sur une pile réseau hybride de haute précision :
- **Niveau Pilote Réseau** : Pilote de tunnel virtuel haute vitesse **Wintun** (développé par le projet WireGuard) opérant dans le Ring 0 de Windows.
- **Protocole Overlay Sécurisé** : Chiffrement asymétrique Curve25519 et AEAD ChaCha20-Poly1305 pour un transit ultra-léger et un cloisonnement strict au niveau 3.
- **Routage Sélectif (Split-Tunneling)** : Seuls les paquets UDP/TCP des jeux ciblés empruntent la dorsale optimisée ; la navigation web et les appels Discord restent sur les routes locales sans aucune latence additionnelle.
- **Auto-Activation Instantanée** : Saisie d'un pseudo au premier démarrage et attribution automatique d'une adresse IP WireGuard en moins de 100 ms.
- **Mise à Jour en Direct (Auto-Updater)** : Détection et application silencieuse des nouvelles versions en 1 clic via l'API GitHub Releases.
- **Supervision en Temps Réel** : Tableau de bord de télémétrie live et contrôle d'accès dans le panneau Superviseur.

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

| Jeu Compétitif | Statut | Infrastructure Cible | Profil Configuration |
| :--- | :---: | :--- | :--- |
| **Fortnite** | `● Prêt` | Epic Online Services + AWS GameLift US-West | `CrazyPing-Fortnite.conf` |
| **Mistfall Hunter** | `● Prêt` | NetEase Cloud Nodes NA-West | `CrazyPing-MistfallHunter.conf` |
| **Valorant** | `● Prêt` | Riot Direct US-West (San José / LA / Seattle) | `CrazyPing-Valorant.conf` |
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
- **Bannière Interactive Néon** : Si une mise à jour est disponible, une notification dynamique apparaît avec le nom et les nouveautés de la version.
- **Installation 1-Clic Silencieuse** : Le clic sur *« Mettre à jour maintenant »* télécharge le setup dans `%TEMP%`, déclenche l'installateur en mode silencieux (`/SILENT`), ferme l'ancienne version et relance la nouvelle en moins de 3 secondes.

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
  └── 6. Téléchargement Public Setup[ PASS ] -> HTTP 200 OK (6.52 Mo)
```

Chaque modification du serveur ou de l'application est vérifiée par sonde directe avant mise en production.

---

## 📚 Documentation Technique Complète

Pour approfondir les détails d'implémentation et de sécurité, consultez les guides dédiés dans le dossier [`docs/`](docs/) :

- [**Architecture Réseau et Logicielle**](docs/ARCHITECTURE.md) : Décomposition en couches (Kernel, Engine Go, UI), cycle de vie, watchdog HTTP/Beacon et protocole de contrôle.
- [**Modèle de Sécurité et Analyse des Menaces**](docs/SECURITY.md) : Cloisonnement L3 inter-clients, cryptokey routing, enrôlement dynamique ZTNA et comparatif légal.
- [**Tables de Routage et Split-Tunneling**](docs/ROUTING_TABLES.md) : Liste exhaustive des sous-réseaux CIDR qualifiés et calcul du MTU optimal (1380 octets).
- [**Guide de Déploiement et d'Exploitation**](docs/DEPLOYMENT.md) : Provisionnement du serveur Linux, configuration systemd, sysctl BBR et packaging Inno Setup.

---

## 🚀 Démarrage Rapide

### 1. Installation du Client
Téléchargez l'installateur autonome depuis la dernière [**Release GitHub**](https://github.com/Cioq987/CrazyPing/releases) :
👉 [**CrazyPing-Setup.exe**](https://github.com/Cioq987/CrazyPing/releases/download/v1.0.1/CrazyPing-Setup.exe)

L'installateur configure automatiquement l'exécutable, les raccourcis et vérifie la présence du pilote Wintun / WireGuard.

### 2. Mode Superviseur (Privé)
Le panneau de contrôle administrateur permet à l'administrateur de l'infrastructure de surveiller en direct l'état des liaisons, les adresses IP allouées et les débits effectifs, avec la possibilité d'interrompre ou de révoquer un accès à tout instant.

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
