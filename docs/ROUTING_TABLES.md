# Tables de Routage et Spécifications Split-Tunneling - CrazyPing

Ce document recense les spécifications d'acheminement réseau, le calcul des Unités Maximales de Transmission (**MTU**) et les tables de sous-réseaux IP (CIDR) configurées pour chaque titre compétitif supporté par l'infrastructure **CrazyPing**.

---

## 1. Philosophie du Split-Tunneling Sélectif

Contrairement aux solutions VPN conventionnelles (Full-Tunnel) qui encapsulent l'intégralité du trafic de la station cliente (`0.0.0.0/0`), CrazyPing applique un filtrage strict de type **Split-Tunneling** :

```
                  Poste Joueur (Tahiti)
                         |
       +-----------------+-----------------+
       |                                   |
[ Trafic Quotidien ]             [ Flux Temps-Réel Jeu ]
- Navigation Web / DNS           - Télémétrie & Game State
- Streaming (YouTube, Twitch)    - Voice & Matchmaking direct
- Téléchargements / Discord                |
       |                                   |
       v                                   v
Route Locale FAI (Vini / Vodafone)   Interface Wintun CrazyPing
       |                                   |
Câbles Pacifique standards           Câble Honotua direct (108 ms)
                                           |
                                     Passerelle VPS (San José)
                                           |
                                     Dorsale Edge Cloud US-West
```

### Avantages Majeurs
1. **Zéro Latence Supplémentaire sur les Tâches de Fond** : Les appels vocaux Discord et la navigation web restent sur les routes locales sans subir d'encapsulation inutile.
2. **Économie de Bande Passante Serveur** : Seuls les paquets UDP légers du jeu traversent la passerelle relais, préservant la capacité de traitement CPU et réseau du VPS.
3. **Stabilité du Débit** : Aucun téléchargement en arrière-plan ne vient saturer la file d'attente du tunnel WireGuard.

---

## 2. Optimisation MTU et Prévention de la Fragmentation

Sur les liaisons sous-marines transpacifiques (Honotua -> Hawaï -> Côte Ouest des États-Unis), la fragmentation des paquets IP est l'une des causes principales des micro-saccades (*jitter*) et des pertes de paquets (*packet loss*).

### Calcul du MTU Optimal

L'en-tête WireGuard standard sur IPv4 impose une surcharge de **60 octets** :
- En-tête IPv4 standard : **20 octets**
- En-tête UDP standard : **8 octets**
- En-tête WireGuard + AEAD Poly1305 Tag : **32 octets**
- **Total Overhead** : **60 octets**

Sur un lien de base à MTU 1500 (ou 1492 en liaison PPPoE) :
$$\text{MTU}_{\text{WireGuard}} = 1500 - 60 = 1440 \text{ octets}$$

Pour garantir une marge de sécurité absolue face aux encapsulations intermédiaires (MPLS/QinQ des opérateurs de transit transocéaniques), CrazyPing fixe le MTU client à :
$$\mathbf{MTU = 1380 \text{ octets}}$$

Cette valeur élimine 100% de la fragmentation IP tout en garantissant un ratio charge utile / en-tête optimal pour les trames de jeu (souvent inférieures à 1200 octets).

---

## 3. Profils de Jeux et Plages CIDR

### 3.1 Fortnite (Epic Games / AWS GameLift US-West)
- **Fichier de configuration** : `CrazyPing-Fortnite.conf`
- **Infrastructures cibles** : Epic Online Services (EOS) & AWS GameLift (Oregon `us-west-2` et California `us-west-1`).

```ini
[Interface]
PrivateKey = <Client_Private_Key>
Address = 10.10.0.X/32
MTU = 1380

[Peer]
PublicKey = 5K893Q2Yy9XyZtOaQy7zF+k9V8vB3N1mR0xL4jP2wEg=
Endpoint = 146.235.218.190:51820
AllowedIPs = 52.88.0.0/15, 52.10.0.0/15, 52.24.0.0/14, 52.32.0.0/11, 54.68.0.0/14, 54.148.0.0/15, 54.184.0.0/13, 54.200.0.0/13, 54.212.0.0/14, 54.218.0.0/15, 54.244.0.0/15, 34.208.0.0/12, 35.160.0.0/13, 44.224.0.0/11, 199.255.40.0/22, 199.192.112.0/22, 205.185.192.0/20
PersistentKeepalive = 15
```

---

### 3.2 Mistfall Hunter (NetEase Games / AWS NA-West)
- **Fichier de configuration** : `CrazyPing-MistfallHunter.conf`
- **Infrastructures cibles** : Serveurs de matchmaking et instances de combat en temps réel hébergées sur les clusters cloud nord-américains ouest.

```ini
[Interface]
PrivateKey = <Client_Private_Key>
Address = 10.10.0.X/32
MTU = 1380

[Peer]
PublicKey = 5K893Q2Yy9XyZtOaQy7zF+k9V8vB3N1mR0xL4jP2wEg=
Endpoint = 146.235.218.190:51820
AllowedIPs = 34.208.0.0/12, 35.160.0.0/13, 44.224.0.0/11, 52.10.0.0/15, 52.24.0.0/14, 52.32.0.0/11, 54.68.0.0/14, 54.148.0.0/15, 54.184.0.0/13, 54.200.0.0/13, 54.212.0.0/14, 54.218.0.0/15, 54.244.0.0/15
PersistentKeepalive = 15
```

---

### 3.3 Valorant (Riot Direct / AWS NA-West)
- **Fichier de configuration** : `CrazyPing-Valorant.conf`
- **Infrastructures cibles** : Réseau privé mondial **Riot Direct** et points de présence Edge (PoP) US-West (San José, Los Angeles, Seattle).

```ini
[Interface]
PrivateKey = <Client_Private_Key>
Address = 10.10.0.X/32
MTU = 1380

[Peer]
PublicKey = 5K893Q2Yy9XyZtOaQy7zF+k9V8vB3N1mR0xL4jP2wEg=
Endpoint = 146.235.218.190:51820
AllowedIPs = 192.207.0.0/17, 192.207.128.0/18, 162.249.72.0/22, 162.249.76.0/22, 162.249.80.0/22, 24.105.30.0/24
PersistentKeepalive = 15
```

---

### 3.4 Spécifications des Jeux en Cours de Portage

Les sous-réseaux suivants sont déjà qualifiés et réservés pour injection immédiate dans les prochains profils :

#### Counter-Strike 2 (Valve SDR - Steam Datagram Relay)
- **Réseaux Valve US-West** :
  - `162.254.192.0/21` (Valve Seattle PoP)
  - `162.254.194.0/23` (Valve California Relays)
  - `155.133.246.0/23` (Valve West Coast SDR)

#### Apex Legends (EA / Respawn / Multiplay)
- **Réseaux Multiplay / AWS West** :
  - `159.153.0.0/16` (Electronic Arts NA)
  - `104.254.152.0/21` (Respawn Entertainment)
  - `3.101.0.0/16` (AWS US-West San José)

#### Call of Duty: Warzone (Activision / Demonware)
- **Réseaux Demonware / Battle.net** :
  - `185.34.104.0/22` (Activision Blizzard Edge)
  - `137.221.64.0/19` (Blizzard US-West)
  - `103.4.115.0/24` (Demonware NA)

---

## 4. Outils de Diagnostic et Validation Réseau

Pour vérifier qu'un flux de jeu emprunte effectivement l'interface Wintun CrazyPing :

### Sous Windows PowerShell :
```powershell
# Vérifier la présence de la route dans la table Windows
Get-NetRoute -DestinationPrefix "192.207.0.0/17" | Format-Table

# Traceur de route avec vérification de l'IP du saut 1 (Gateway 10.10.0.1)
Test-NetConnection -ComputerName 192.207.0.1 -TraceRoute
```

Si le premier saut résolu est bien `10.10.0.1` (IP interne du relais San José) avec une latence d'environ 108 ms, le Split-Tunneling est parfaitement opérationnel.
