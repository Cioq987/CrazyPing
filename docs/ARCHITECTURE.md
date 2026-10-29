# Architecture Technique et Fonctionnelle de CrazyPing

## 1. Vue d'Ensemble du Système

CrazyPing est un moteur d'optimisation de transit réseau et de réduction de latence spécifiquement conçu pour les joueurs situés en Polynésie française (Tahiti). Il résout les problématiques d'asymétrie de routage BGP, de fragmentation de paquets PPPoE et de latence résiduelle induite par les fournisseurs d'accès Internet locaux vers les infrastructures de jeux nord-américaines (Californie et Oregon).

L'architecture repose sur un modèle hybride client-relais :
- **CrazyPing Desktop Client (Windows)** : Application native en Go orchestrant un adaptateur virtuel de niveau 3 (Wintun) et une interface de contrôle ergonomique sans runtime lourd.
- **CrazyPing Relay Server (Linux)** : Passerelle de routage haute performance hébergée au point de chute direct du câble sous-marin Honotua (San José, Californie).

---

## 2. Schéma d'Architecture Détaillé

```
+----------------------------------------------------------------------------+
|                            STATION LOCALE (PC)                             |
|                                                                            |
|  [ Jeu Vidéo (ex: Valorant) ]        [ Navigation / Discord / ExpressVPN ] |
|               |                                         |                  |
|        (Paquets UDP Jeu)                         (Trafic Standard)         |
|               v                                         |                  |
|     +-------------------+                               |                  |
|     |  Adaptateur L3    |                               |                  |
|     |   Wintun Driver   |                               |                  |
|     +-------------------+                               |                  |
|               |                                         |                  |
|  (Tunnel WireGuard Chiffré UDP 51820)                   |                  |
|               |                                         |                  |
|               +--------------------+--------------------+                  |
|                                    |                                       |
|                                    v                                       |
|                       [ Interface Physique Ethernet ]                      |
+------------------------------------|---------------------------------------+
                                     |
                       (Câble Sous-Marin Honotua)
                               ~105 - 108 ms
                                     |
                                     v
+----------------------------------------------------------------------------+
|                       SERVEUR RELAIS VPS (San José)                        |
|                                                                            |
|               [ Interface Réseau Physique (ens3) ]                         |
|                                    |                                       |
|                     (Filtre Pare-feu Linux iptables)                       |
|                                    |                                       |
|                                    v                                       |
|                      +--------------------------+                          |
|                      | Interface WireGuard wg0  |                          |
|                      |  10.10.0.1 (Noyau Linux) |                          |
|                      +--------------------------+                          |
|                                    |                                       |
|                       (Isolation Client Inter-Peer)                        |
|                                    |                                       |
|                                    v                                       |
|                       (NAT MASQUERADE Sortant)                             |
|                                    |                                       |
|          +-------------------------+-------------------------+             |
|          |                                                   |             |
|          v                                                   v             |
| [ Riot Direct US (San José) ]                     [ AWS GameLift NA-West ] |
|            < 1 ms                                          ~15 ms          |
+----------------------------------------------------------------------------+
```

---

## 3. Composants Techniques du Client (Windows)

### 3.1 Noyau d'Exécution Go & Pilote Wintun
- **Exécutable Autonome** : Le client est compilé statiquement en Go avec les drapeaux `-ldflags="-s -w -H=windowsgui"`. Il ne dépend d'aucun runtime externe (.NET, Java, Electron).
- **Intégration Wintun** : CrazyPing utilise le pilote réseau L3 officiel `wintun.sys` via l'interface en ligne de commande WireGuard. Wintun opère directement dans l'espace noyau de Windows (Ring 0), assurant un débit de plusieurs gigabits par seconde avec une utilisation processeur quasi-nulle.
- **Cycle de Vie et Watchdog** :
  - L'interface transmet un battement de cœur continu (`/api/heartbeat`) toutes les 1,5 s.
  - La fermeture de la fenêtre déclenche immédiatement un événement `beforeunload` balisé (`navigator.sendBeacon('/api/exit')`).
  - Un chien de garde interne garantit qu'en cas de fermeture imprévue de l'interface, les tunnels réseau sont désinstallés proprement et le processus se termine sans laisser de processus fantôme en mémoire.

### 3.2 Interface Graphique Embarquée
- **Absence d'Electron** : Afin d'éviter la consommation excessive de mémoire vive (200 à 400 Mo) propre aux frameworks web lourds, CrazyPing utilise la directive `//go:embed web/*` pour compiler directement les ressources HTML, Tailwind CSS et Chart.js dans le binaire.
- **Affichage Natif** : L'interface s'exécute via le composant d'application WebView/Edge (`--app=http://127.0.0.1:4142`) avec isolation totale et accélération graphique matérielle.

---

## 4. Composants Techniques du Serveur Relais (Linux)

### 4.1 Noyau WireGuard & Routage L3
- Le serveur VPS exécute un noyau Linux avec le module WireGuard intégré.
- **Sous-réseau Privé** : `10.10.0.0/24`. L'interface serveur est fixée à `10.10.0.1`, le client maître à `10.10.0.2`, et les pairs invités reçoivent des baux de `10.10.0.3` à `10.10.0.254`.
- **Forwarding IP** : Activé via `net.ipv4.ip_forward = 1` avec translation d'adresses source (`iptables -t nat -A POSTROUTING -o ens3 -j MASQUERADE`).

### 4.2 Superviseur et Démon de Contrôle (`crazyping-server`)
- Démon Go autonome géré par `systemd` (`crazyping-server.service`).
- Écoute sur le port UDP `4141` pour le contrôle de latence RTT et les commandes d'administration chiffrées.
- Écoute sur le port TCP `4140` en HTTP REST interne pour la supervision des métriques.
- Parse directement l'état de la table d'états WireGuard (`wg show wg0 dump`) pour calculer les débits entrants/sortants et l'activité temporelle de chaque joueur.

---

## 5. Gestion des Données et Flux Réseau

### 5.1 Protocole de Télémétrie CrazyPing (23 Octets d'En-tête)
CrazyPing encapsule les requêtes de mesure dans un format binaire minimaliste :
```
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

### 5.2 Split-Tunneling Strict
Le split-tunneling garantit que seules les adresses IP des serveurs de jeu ciblés sont routées à travers le tunnel. 
- Les plages réseau de navigation générale, de téléphonie, de DNS et de services de streaming continuent d'emprunter la passerelle par défaut du fournisseur d'accès local.
- Les clients VPN commerciaux (ex: ExpressVPN) ne subissent aucune déconnexion ni rupture de connectivité lorsque CrazyPing est activé.
