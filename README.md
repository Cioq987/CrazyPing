# CrazyPing — Accélérateur Réseau & Réducteur de Latence Haute Performance

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux-0078D6?style=flat)](https://github.com)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Latency](https://img.shields.io/badge/Tahiti%20%E2%86%92%20San%20Jos%C3%A9-~108%20ms-brightgreen)](docs/ROUTING_TABLES.md)

**CrazyPing** est une solution logicielle et réseau d'optimisation de latence, de stabilisation de gigue (*jitter*) et d'élimination de la perte de paquets (*packet loss*), spécialement conçue pour les liaisons sous-marines transpacifiques reliant la Polynésie française (**Tahiti**) aux centres de données nord-américains (San José / Oregon / Los Angeles).

L'application s'appuie sur une pile réseau hybride de haute précision :
- **Niveau Pilote Réseau** : Pilote de tunnel virtuel haute vitesse **Wintun** (développé par le projet WireGuard) opérant dans le Ring 0 de Windows.
- **Protocole Overlay Sécurisé** : Chiffrement asymétrique Curve25519 et AEAD ChaCha20-Poly1305 pour un transit ultra-léger et un cloisonnement strict au niveau 3.
- **Routage Sélectif (Split-Tunneling)** : Seuls les paquets UDP/TCP des jeux ciblés empruntent la dorsale optimisée ; la navigation web et les appels Discord restent sur les routes locales sans aucune latence additionnelle.
- **Supervision en Temps Réel** : Tableau de bord de télémétrie live et système d'enrôlement dynamique d'utilisateurs ("Demandes d'amis") géré directement par le Superviseur.

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

## 📚 Documentation Technique Complète

Pour approfondir les détails d'implémentation et de sécurité, consultez les guides dédiés dans le dossier [`docs/`](docs/) :

- [**Architecture Réseau et Logicielle**](docs/ARCHITECTURE.md) : Décomposition en couches (Kernel, Engine Go, UI), cycle de vie, watchdog HTTP/Beacon et protocole de contrôle.
- [**Modèle de Sécurité et Analyse des Menaces**](docs/SECURITY.md) : Cloisonnement L3 inter-clients, cryptokey routing, enrôlement dynamique ZTNA et comparatif légal.
- [**Tables de Routage et Split-Tunneling**](docs/ROUTING_TABLES.md) : Liste exhaustive des sous-réseaux CIDR qualifiés et calcul du MTU optimal.
- [**Guide de Déploiement et d'Exploitation**](docs/DEPLOYMENT.md) : Provisionnement du serveur Linux, configuration systemd, sysctl BBR et packaging Inno Setup.

---

## 🚀 Démarrage Rapide

### 1. Prérequis Système
- **Poste Client** : Windows 10 ou 11 (64-bit), droits administrateur (requis pour instancier l'interface Wintun).
- **Pilote Réseau** : Pilote officiel WireGuard / Wintun installé.

### 2. Lancement du Client
Exécutez `CrazyPing.exe` en tant qu'administrateur. L'application démarre son moteur local et ouvre une fenêtre d'interface épurée sans bordure de navigateur.

1. Sélectionnez votre jeu (ex. **Fortnite** ou **Valorant**).
2. Cliquez sur **OPTIMISER LA CONNEXION**.
3. Observez la jauge de télémétrie passer au vert avec la latence stabilisée.

### 3. Panneau Superviseur (Mode Administrateur)
Pour administrer les connexions actives et approuver les nouveaux utilisateurs :
1. Déployez le panneau **Superviseur** en bas de l'interface.
2. Entrez le mot de passe maître (issu de `admin.key`).
3. Les demandes d'accès ("Demande d'ami") apparaissent en direct avec leur clé publique :
   - Cliquez sur `✓ Accepter` pour provisionner instantanément le peer sur le serveur Linux.
   - Cliquez sur `Refuser` pour rejeter la demande.
   - Cliquez sur `Révoquer` dans la table des peers pour interrompre un accès existant.

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

## 📄 Licence et Clause de Non-Affiliation

CrazyPing est un projet indépendant sous licence MIT. Il n'est affilié, sponsorisé ni approuvé par aucune société éditrice de jeux ou de logiciels tiers mentionnés (Epic Games, Riot Games, Valve, NetEase, ExitLag). Tous les noms de marques et logos demeurent la propriété exclusive de leurs détenteurs respectifs.
