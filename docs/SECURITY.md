# Modèle de Sécurité et Analyse des Menaces - CrazyPing

Ce document détaille l'architecture de sécurité réseau et logicielle de l'infrastructure **CrazyPing**, le modèle de menaces analysé, les mécanismes d'isolation cryptographique et de niveau 3, ainsi que les contre-mesures face à l'analyse binaire et au reverse engineering.

---

## 1. Modèle de Confiance et Architecture Zero-Trust

CrazyPing repose sur un paradigme **Zero-Trust Network Access (ZTNA)** adapté aux contraintes de latence temps-réel du gaming compétitif. Aucun client ne bénéficie d'une confiance implicite sur le réseau overlay.

```
+-------------------------------------------------------------+
|                     Poste Client (Joueur)                  |
|  - Wintun Virtual Adapter                                   |
|  - CrazyPing Service (Go Runtime)                           |
|  - Clé Privée Éphémère (Curve25519)                        |
+------------------------------+------------------------------+
                               |
             WireGuard Noise_IK Tunnel (UDP 51820)
             ChaCha20-Poly1305 / Cryptokey Routing
                               |
+------------------------------v------------------------------+
|               Passerelle Relais VPS (San José)              |
|  - Noyau Linux Hardened (sysctl L3)                         |
|  - iptables FORWARD Isolation Filter                        |
|  - CrazyPing Daemon (UDP 4141 - Token-authenticated)       |
+-------------------------------------------------------------+
```

### Principes Directeurs
1. **Confidentialité et Intégrité Cryptographique** : Tout le trafic réseau d'optimisation est chiffré de bout en bout entre l'adaptateur virtuel Wintun du client et le noyau Linux du VPS via le protocole WireGuard (Noise Protocol Framework).
2. **Cloisonnement Strict L3** : Les clients connectés au relais ne peuvent en aucun cas communiquer entre eux, sonder leurs ports locaux ou intercepter le trafic d'un autre utilisateur.
3. **Contrôle d'Accès Supervision Séparé** : Le plan de contrôle administratif (Superviseur) est strictement isolé du plan de données et protégé par challenge cryptographique symétrique (SHA-256 + nonce).

---

## 2. Isolation Réseau des Clients (L3 Layer Enforcement)

### 2.1 Problématique de l'Inter-Client Routing
Par défaut sous Linux, lorsqu'un routeur/passerelle active l'IP Forwarding (`net.ipv4.ip_forward = 1`) pour permettre l'accès Internet aux peers d'une interface overlay (`wg0`), tout client du sous-réseau `10.10.0.0/24` pourrait techniquement émettre des paquets vers l'adresse IP d'un autre client connecté.

### 2.2 Règle d'Isolation Active
Pour interdire strictement toute communication inter-clients, la chaîne `FORWARD` du pare-feu `iptables` de la passerelle applique un rejet immédiat sur toute tentative de routage dont l'interface d'entrée et de sortie est `wg0` :

```bash
# Règle d'isolation stricte inter-clients
sudo iptables -I FORWARD -i wg0 -o wg0 -j REJECT --reject-with icmp-port-unreachable
```

**Effet garanti :**
- Un scan ARP/ICMP/TCP de type `nmap 10.10.0.0/24` depuis un poste client est instantanément bloqué par le noyau Linux du relais.
- Aucun paquet ne transite d'un peer à un autre.
- Seul le transit `wg0 -> ens3 (Internet)` et `ens3 -> wg0 (Return Path)` est autorisé.

---

## 3. Cryptographie et Routage par Clé (Cryptokey Routing)

### 3.1 Primitives Cryptographiques
CrazyPing s'appuie sur la suite cryptographique moderne et formellement vérifiée de WireGuard :
- **Échange de clés / Authentification** : Curve25519 (ECDH).
- **Chiffrement authentifié (AEAD)** : ChaCha20-Poly1305.
- **Hachage et KDF** : BLAKE2s.
- **Résistance au rejeu** : Compteurs d'états 64-bit et horodatages TAI64N.

### 3.2 Prévention de l'IP Spoofing (Cryptokey Routing)
Chaque clé publique WireGuard autorisée sur le serveur est explicitement liée à une unique adresse IP interne (ex. `10.10.0.2/32`). 
- Si un client malveillant modifie son paquet local pour forger l'adresse IP source d'un autre joueur, le pilote WireGuard sur le serveur déchiffre le paquet avec la clé du client, compare l'IP source interne avec l'entrée `AllowedIPs` associée à cette clé, et jette silencieusement le paquet (`drop`).
- **Aucune usurpation d'identité ou de session IP n'est possible sur le tunnel.**

---

## 4. Protocole d'Enrôlement Dynamique et Modération Superviseur

Pour éviter la diffusion statique de fichiers de configuration contenant des clés pré-partagées ou pré-générées, CrazyPing intègre un système d'enrôlement dynamique à validation humaine :

### Séquence d'Approbation

```
Client (Nouveau Joueur)               Serveur Relais VPS             Superviseur (Admin)
          |                                   |                              |
          |--- TypeEnrollReq (Nom, PubKey) -->|                              |
          |    (UDP 4141)                     |-- Stocke en mémoire pending  |
          |                                   |                              |
          |                                   |<-- Polling GET /pending -----|
          |                                   |--- Renvoie liste demandes -->|
          |                                   |                              |
          |                                   |<-- Action Approve (UUID) ----|
          |                                   |    (Signé Token Admin)       |
          |                                   |                              |
          |                                   |-- Alloue IP (10.10.0.X)      |
          |                                   |-- `wg set wg0 peer ...`      |
          |<-- TypeEnrollResp (Status=OK) ----|                              |
          |    (IP allouée + Clé Pub VPS)     |                              |
          |                                   |                              |
    Tunnel WireGuard Actif                    |                              |
```

1. **Génération locale des secrets** : La clé privée du client est générée sur sa machine et ne transite jamais sur le réseau.
2. **Demande en attente (Quarantine Queue)** : Tant que l'administrateur n'a pas validé explicitement la demande depuis le dashboard superviseur, aucun accès réseau n'est injecté dans `wg0`.
3. **Révocation Instantanée** : Le superviseur peut révoquer un accès à tout instant via un simple clic (`wg set wg0 peer <PubKey> remove`), rompant le tunnel en moins de 10 millisecondes.

---

## 5. Analyse Face au Reverse-Engineering (Analyse Statique et Dynamique)

Si un tiers ou un analyste tente de décompiler ou d'inspecter `CrazyPing.exe` (à l'aide d'outils tels que REA, Ghidra, IDA Pro, x64dbg ou Wireshark) :

### Ce qui est visible dans le binaire :
1. **L'adresse IP publique du VPS** (`146.235.218.190`) et les ports de communication (`UDP 4141`, `UDP 51820`).
2. **Le bytecode Go compilé** : Fonctions internes de gestion d'adaptateur Wintun, parsers de métriques JSON, logique UI.
3. **Les plages d'adresses IP des serveurs de jeu** : Les listes de sous-réseaux AWS GameLift, Riot Direct, Valve SDR.

### Ce qui est protégé et invulnérable :
1. **Aucun secret d'infrastructure** : Le binaire client ne contient aucune clé d'accès SSH, aucun mot de passe root, et aucun token administrateur. Le token d'administration `admin.key` n'est présent que sur la station de supervision.
2. **Intégrité de l'hôte Linux** : Les ports SSH (`22`) sont protégés par clé asymétrique RSA 4096-bit / Ed25519 (authentification par mot de passe désactivée). La surface d'exposition UDP (ports WireGuard et API relais) est imperméable à l'exécution de code à distance.
3. **Absence de fuite de données utilisateurs** : L'architecture ne conserve aucune donnée nominative ou télémétrique intrusive sur disque.

---

## 6. Conformité et Indépendance Technologique

CrazyPing a été développé selon une approche **Clean-Room** stricte, en s'appuyant exclusivement sur des standards et des briques logicielles open-source éprouvées :

| Composant | CrazyPing | Solutions Propriétaires (ex. ExitLag) | Statut Juridique / Technique |
| :--- | :--- | :--- | :--- |
| **Langage de base** | 100% Go (Golang) standard | C++ / MFC / Qt | Indépendant |
| **Driver Réseau Windows** | **Wintun** (Jason Donenfeld, WireGuard Project) | Driver NDIS propriétaire (`ndextlag.sys`, NetFilter SDK) | Open-Source officiel (GPLv2 / MIT) |
| **Tunnel / Chiffrement** | Protocole WireGuard standard (Noise Protocol) | Multipath UDP propriétaire, encapsulation custom | Standard IETF / RFC |
| **Protocole de Contrôle** | Format binaire UDP minimal CrazyPing | RPC Protobuf chiffré propriétaire | Spécification originale CrazyPing |

CrazyPing n'embarque, ne réutilise, ni ne rétro-ingénie aucun composant, bibliothèque ou protocole d'éditeurs tiers d'optimiseurs réseau. L'implémentation est 100% originale et conforme aux standards de l'art du génie logiciel réseau.
