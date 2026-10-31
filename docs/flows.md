# Flux Réseau et Protocoles de Communication - CrazyPing

Ce document détaille les échanges de données, protocoles binaires et séquences opérationnelles régissant les interactions entre le client Windows, la passerelle relais Linux et les serveurs de jeu distants.

---

## 1. Format des Paquets de Contrôle et Télémétrie

CrazyPing s'appuie sur un protocole binaire compact véhiculé sur UDP (port `4141`) conçu pour minimiser l'overhead réseau sur la liaison Honotua.

### 1.1 En-tête Binaire Minimaliste (23 Octets)

```text
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|   Magic ('C') |   Magic ('P') |   Type (1B)   |   Seq (4B)    |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                      Timestamp (UnixNano 8B)                  |
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                       Target IPv4 (4B)                        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|       Target Port (2B)        |      Payload Length (2B)      |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                       Payload (Variable)                      |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### 1.2 Types de Messages (`protocol.Type`)

| Identifiant Hex | Constante | Description |
| :---: | :--- | :--- |
| `0x01` | `TypePing` | Sonde de latence client vers relais (mesure RTT). |
| `0x02` | `TypePong` | Réponse immédiate émise par le démon relais. |
| `0x03` | `TypeEnrollReq` | Requête d'auto-enrôlement client (provisioning de clé WireGuard). |
| `0x04` | `TypeEnrollResp` | Réponse d'attribution d'adresse IP et clé publique serveur. |
| `0x05` | `TypeAdminReq` | Requête d'administration sécurisée (challenge cryptographique). |
| `0x06` | `TypeAdminResp` | Réponse administrative (état des pairs WireGuard, métriques). |

---

## 2. Diagrammes de Séquence des Flux Majeurs

### 2.1 Enrôlement Automatique d'un Nouveau Joueur

Lors de la première initialisation du client ou lors d'une demande explicite d'activation, le client génère localement sa paire de clés cryptographiques Curve25519 et s'enregistre auprès du relais sans intervention manuelle.

```mermaid
sequenceDiagram
    autonumber
    participant UI as Interface Graphique (Web)
    participant Client as Moteur Local (Go)
    participant Relay as Démon Relais (Linux)
    participant WG as Interface Noyau wg0

    UI->>Client: Soumission du Pseudo Joueur
    Client->>Client: Génération paire Curve25519 (PrivKey / PubKey)
    Client->>Relay: TypeEnrollReq [UDP 4141] (PubKey, Pseudo)
    Relay->>Relay: Validation stricte clé (taille 32 octets, base64)
    Relay->>Relay: Allocation dynamique IP suivante (10.10.0.X/32)
    Relay->>WG: Exécution commande wg set peer <PubKey> allowed-ips 10.10.0.X/32
    WG-->>Relay: Confirmation mise à jour table de routage
    Relay-->>Client: TypeEnrollResp (AssignedIP, ServerPubKey, Endpoint)
    Client->>Client: Génération des fichiers .conf locaux sécurisés
    Client-->>UI: Statut Enrôlé avec Succès (Prêt à jouer)
```

---

### 2.2 Activation du Tunnel Split-Tunneling et Mesure RTT

Lorsque l'utilisateur sélectionne un jeu et active l'optimisation, CrazyPing instancie l'adaptateur Wintun, injecte les tables de routage CIDR ciblées et démarre la boucle de télémétrie RTT.

```mermaid
sequenceDiagram
    autonumber
    participant Client as Moteur Local (Go)
    participant Wintun as Adaptateur Wintun L3
    participant Relay as Serveur Relais (VPS)
    participant GameSrv as Serveur de Jeu (US)

    Client->>Wintun: Injection configuration WireGuard (MTU 1380, AllowedIPs)
    Wintun->>Wintun: Création de l'interface et activation des routes statiques
    
    loop Télémétrie RTT (toutes les 1000 ms)
        Client->>Relay: TypePing [UDP 4141] (Timestamp Nano)
        Relay-->>Client: TypePong [UDP 4141] (Miroir Timestamp)
        Client->>Client: Calcul RTT = (NowNano - TimestampNano) / 10^6 ms
    end

    Note over Wintun,GameSrv: Trafic réel du jeu en cours de partie
    Client->>Wintun: Paquet UDP Jeu vers Serveur US (ex: 24.105.x.x)
    Wintun->>Relay: Tunnel chiffré WireGuard (Noise_IK)
    Relay->>Relay: iptables FORWARD + NAT MASQUERADE
    Relay->>GameSrv: Paquet UDP routé via Peering Direct (< 1 ms)
    GameSrv-->>Relay: Réponse du serveur de jeu
    Relay-->>Wintun: Acheminement retour chiffré
    Wintun-->>Client: Restitution paquet au processus de jeu
```

---

### 2.3 Plan de Contrôle Administratif & Supervision

Le tableau de bord administrateur interroge l'état des pairs WireGuard via une authentification symétrique à challenge non rejouable.

```mermaid
sequenceDiagram
    autonumber
    participant AdminUI as Interface Supervision
    participant LocalClient as Client Local (Admin)
    participant RelayDaemon as Démon Relais (VPS)
    participant WGKernel as Noyau Linux wg0

    AdminUI->>LocalClient: GET /api/admin/peers
    LocalClient->>LocalClient: Lecture clé secrète locale admin.key
    LocalClient->>LocalClient: Génération Nonce (Timestamp UnixNano)
    LocalClient->>LocalClient: Calcul HMAC/Hash = SHA256(Nonce + admin.key)
    LocalClient->>RelayDaemon: TypeAdminReq [UDP 4141] (Action: "list", TokenHash, Nonce)
    RelayDaemon->>RelayDaemon: Vérification validité Nonce (< 5s) & TokenHash
    RelayDaemon->>WGKernel: Lecture dump wg0 (wg show wg0 dump)
    WGKernel-->>RelayDaemon: Données brutes (Tx/Rx, Handshake, Endpoint)
    RelayDaemon-->>LocalClient: TypeAdminResp (Payload JSON structuré)
    LocalClient-->>AdminUI: Affichage métriques temps-réel (Débits, Pings, Joueurs)
```

---

### 2.4 Chien de Garde (Watchdog) et Extinction Propre

Afin de garantir qu'aucun résidu d'adaptateur réseau ou de processus orphelin ne persiste sur le système hôte lors de la fermeture de la fenêtre, CrazyPing met en œuvre un mécanisme de battement de cœur synchronisé.

```mermaid
sequenceDiagram
    autonumber
    participant UI as Interface Web (Navigateur)
    participant Client as Moteur CrazyPing (Go)
    participant Wintun as Service Réseau Wintun

    loop Battement de cœur nominal (toutes les 1500 ms)
        UI->>Client: POST /api/heartbeat
        Client->>Client: Mise à jour atomique atomic.StoreInt64(&lastHeartbeat, Now)
    end

    alt Fermeture normale de la fenêtre
        UI->>Client: POST /api/exit (via navigator.sendBeacon)
        Client->>Wintun: Déconnexion immédiate des tunnels actifs
        Wintun-->>Client: Suppression de l'adaptateur virtuel
        Client->>Client: os.Exit(0)
    else Crash ou interruption brutale du navigateur
        Note over UI: Fenêtre fermée anormalement (aucun beacon)
        loop Contrôle Watchdog (toutes les 3000 ms)
            Client->>Client: Évaluation (Now - lastHeartbeat)
            opt Si (Now - lastHeartbeat) > 30 secondes
                Client->>Wintun: Déconnexion forcée des tunnels réseau
                Wintun-->>Client: Libération des ressources de routage
                Client->>Client: os.Exit(0) [Arrêt de sécurité]
            end
        end
    end
```
