# Guide de Déploiement et d'Exploitation Infrastructure - CrazyPing

Ce document décrit l'ensemble des procédures opérationnelles nécessaires pour provisionner, durcir et maintenir en production la passerelle relais Linux ainsi que pour packager et distribuer le client Windows **CrazyPing**.

---

## 1. Spécifications du Nœud Relais (VPS)

- **Localisation Géographique** : San José, Californie (États-Unis).
- **Rôle Topologique** : Point d'atterrissement direct du câble sous-marin Honotua et interconnexion directe (Peering) avec les transitaires de rang 1 (Tier-1) et les dorsales de cloud gaming (AWS, Riot Direct, Valve SDR, Azure).
- **Système d'Exploitation Recommandé** : Ubuntu 22.04 LTS x86_64 ou Debian 12 (Noyau Linux $\ge$ 5.15).
- **Dimensionnement Matériel Minimal** : 1 vCPU, 1 Go RAM, Interface Réseau Gigabit ($\ge$ 1 Gbps).

---

## 2. Préparation du Système d'Exploitation Linux

### 2.1 Optimisation des Paramètres Noyau (`sysctl`)
Créez ou modifiez `/etc/sysctl.d/99-crazyping.conf` pour activer le routage L3, l'algorithme de contrôle de congestion BBR et augmenter les tampons UDP pour le temps-réel :

```ini
# Activation du routage IPv4
net.ipv4.ip_forward = 1

# Algorithme de contrôle de congestion BBR
net.core.default_qdisc = fq
net.ipv4.tcp_congestion_control = bbr

# Augmentation des tampons réseau UDP (évite les drops de paquets sous charge)
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.core.rmem_default = 1048576
net.core.wmem_default = 1048576

# Protection contre les attaques DoS réseau
net.ipv4.tcp_syncookies = 1
net.ipv4.conf.all.rp_filter = 1
```

Appliquez immédiatement les paramètres :
```bash
sudo sysctl -p /etc/sysctl.d/99-crazyping.conf
```

---

## 3. Configuration de WireGuard et du Pare-Feu

### 3.1 Installation des Paquets Requis
```bash
sudo apt update && sudo apt install -y wireguard iptables-persistent
```

### 3.2 Configuration de l'Interface Overlay (`/etc/wireguard/wg0.conf`)
```ini
[Interface]
Address = 10.10.0.1/24
ListenPort = 51820
PrivateKey = <Server_Private_Key>

# Règles de routage NAT et cloisonnement L3
PostUp = iptables -t nat -A POSTROUTING -o ens3 -j MASQUERADE
PostUp = iptables -I FORWARD -i wg0 -o wg0 -j REJECT --reject-with icmp-port-unreachable
PostDown = iptables -t nat -D POSTROUTING -o ens3 -j MASQUERADE
PostDown = iptables -D FORWARD -i wg0 -o wg0 -j REJECT --reject-with icmp-port-unreachable
```

*Remarque : Remplacez `ens3` par le nom de l'interface réseau publique de votre VPS (`ip route show default`).*

Activez et démarrez l'interface WireGuard :
```bash
sudo systemctl enable wg-quick@wg0
sudo systemctl start wg-quick@wg0
```

---

## 4. Déploiement du Démon CrazyPing Server

### 4.1 Compilation Croisée (Cross-Compilation Go)
Depuis votre machine de développement locale (Windows ou macOS) :

```powershell
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -ldflags="-s -w" -o crazyping-server-linux-amd64 ./cmd/server
```

### 4.2 Transfert Sécurisé vers le VPS
```powershell
scp -i "chemin\vers\cle.key" crazyping-server-linux-amd64 ubuntu@<IP_VPS>:/home/ubuntu/crazyping-server.new
```

Sur le serveur distant :
```bash
sudo systemctl stop crazyping-server
sudo mv -f /home/ubuntu/crazyping-server.new /usr/local/bin/crazyping-server
sudo chmod +x /usr/local/bin/crazyping-server
```

### 4.3 Service Systemd (`/etc/systemd/system/crazyping-server.service`)
Créez le service de supervision autonome :

```ini
[Unit]
Description=CrazyPing Relay & Dynamic Enrollment Daemon
After=network.target wg-quick@wg0.service

[Service]
Type=simple
User=root
WorkingDirectory=/usr/local/bin
ExecStart=/usr/local/bin/crazyping-server
Restart=always
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

Enregistrez et démarrez le service :
```bash
sudo systemctl daemon-reload
sudo systemctl enable crazyping-server
sudo systemctl start crazyping-server
```

Vérifiez le bon fonctionnement via `journalctl` :
```bash
sudo journalctl -u crazyping-server -f
```

---

## 5. Construction et Packaging du Client Windows

### 5.1 Compilation du Binaire Client GUI
Le client graphique est compilé avec le runtime Go sous Windows, en incorporant le manifeste d'élévation UAC (`requireAdministrator`) et l'icône de l'application :

```powershell
# Génération de l'objet ressource Windows (icône + manifeste UAC)
rsrc -manifest cmd/gui/CrazyPing.exe.manifest -ico icon.ico -o cmd/gui/rsrc.syso

# Compilation du binaire optimisé sans console résiduelle (-H=windowsgui)
go build -ldflags="-s -w -H=windowsgui" -o CrazyPing.exe ./cmd/gui
```

### 5.2 Création de l'Installateur Autonome (`CrazyPing-Setup.exe`)
Le script Inno Setup (`deploy/CrazyPing-Installer.iss`) automatise l'installation chez l'utilisateur final :
- Installation du binaire `CrazyPing.exe` dans `%ProgramFiles%\CrazyPing`.
- Détection automatique de la présence du pilote WireGuard / Wintun sur la machine cible.
- Installation silencieuse de WireGuard si non présent.
- Création des raccourcis Bureau et Menu Démarrer avec les privilèges administrateurs pré-configurés.
- Déploiement des profils de jeux configurés.
