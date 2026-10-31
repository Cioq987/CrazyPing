# Cas d'Utilisation Système et Utilisateur - CrazyPing

Ce document recense les scénarios d'utilisation majeurs de l'application **CrazyPing**, définissant les prérequis, le déroulement nominal étape par étape, les cas d'erreur associés et les conditions de sortie pour chacun des flux opérationnels.

---

## 1. Sommaire des Cas d'Utilisation

| Identifiant | Intitulé | Acteur Principal | Description |
| :---: | :--- | :--- | :--- |
| **UC-01** | Initialisation & Démarrage du Client | Utilisateur / OS | Lancement de l'exécutable, vérification des privilèges administrateur et démarrage du serveur HTTP interne. |
| **UC-02** | Enrôlement Automatique du Joueur | Joueur | Déclaration du pseudo, négociation dynamique de bail IP et génération locale des configurations WireGuard. |
| **UC-03** | Activation d'un Profil de Split-Tunneling | Joueur | Sélection d'un titre (Valorant, LoL, etc.), instanciation du pilote Wintun et injection des routes CIDR. |
| **UC-04** | Désactivation du Tunnel | Joueur | Fermeture de l'interface Wintun et rétablissement transparent des routes par défaut. |
| **UC-05** | Supervision Administrative du Relais | Administrateur | Consultation de l'état des pairs WireGuard, des débits temps-réel et révocation d'accès via `admin.key`. |
| **UC-06** | Mise à Jour Automatique du Binaire | Joueur / Système | Détection d'une nouvelle version sur GitHub Releases, téléchargement et exécution de l'installateur autonome. |
| **UC-07** | Surveillance et Extinction Sécurisée (Watchdog) | Système (Chien de garde) | Détection de l'inactivité de l'interface graphique et libération forcée des adaptateurs réseau. |

---

## 2. Spécification Détaillée des Cas d'Utilisation

### UC-01 : Initialisation & Démarrage du Client
- **Acteur** : Utilisateur (Joueur ou Administrateur).
- **Prérequis** : Système d'exploitation Windows 10/11 64-bit avec privilèges administratifs (nécessaires pour manipuler les interfaces Wintun).
- **Déroulement Nominal** :
  1. L'utilisateur lance `CrazyPing.exe`.
  2. Le programme vérifie les droits d'administration Windows. Si non élevés, élévation demandée via UAC.
  3. Le serveur HTTP local démarre sur l'interface loopback `127.0.0.1:4142`.
  4. L'application lance la fenêtre Edge WebView en mode application autonome (`--app=http://127.0.0.1:4142`).
  5. Le watchdog interne démarre son compte à rebours initial de 20 secondes.
  6. L'interface charge les ressources HTML/CSS/JS embarquées et transmet le premier battement de cœur (`/api/heartbeat`).
- **Cas d'Erreur** :
  - *Refus UAC* : L'application se termine immédiatement avec un message explicatif dans la console ou journal d'événements.
  - *Port 4142 indisponible* : Le système recherche le port incrémental suivant ou retourne un code d'erreur de socket bind.

---

### UC-02 : Enrôlement Automatique du Joueur
- **Acteur** : Joueur lors du premier lancement de l'application.
- **Prérequis** : Connectivité Internet active vers le relais VPS (UDP 4141).
- **Déroulement Nominal** :
  1. L'interface affiche l'écran d'accueil demandant la saisie d'un pseudo (sans espaces, caractères alphanumériques).
  2. L'utilisateur saisit son pseudo et clique sur **Activer CrazyPing**.
  3. Le client Go génère une clé privée et une clé publique Curve25519 (32 octets).
  4. Le client forge un paquet binaire `TypeEnrollReq` (code `0x03`) contenant le pseudo et la clé publique, expédié au relais via UDP 4141.
  5. Le démon relais valide le format de la clé, réserve la prochaine adresse disponible dans `10.10.0.0/24` (ex: `10.10.0.5/32`) et configure l'interface `wg0` Linux en temps réel.
  6. Le relais renvoie un paquet `TypeEnrollResp` (code `0x04`) contenant l'adresse IP allouée, la clé publique du serveur et l'adresse de terminaison.
  7. Le client écrit les fichiers `CrazyPing-*.local.conf` sur le disque local en intégrant la clé privée du joueur.
  8. L'interface bascule en mode prêt avec la liste des profils de jeu activables.
- **Cas d'Erreur** :
  - *Perte de paquet UDP sur Honotua* : Le client retente la transmission jusqu'à 3 fois avec un timeout de 2,5 s par essai. En cas d'échec total, une alerte invite l'utilisateur à vérifier sa liaison Internet.
  - *Clé invalide rejetée par le serveur* : Message d'erreur signalant un refus d'enrôlement par le relais.

---

### UC-03 : Activation d'un Profil de Split-Tunneling
- **Acteur** : Joueur.
- **Prérequis** : Profils de jeu configurés (`CrazyPing-<Jeu>.local.conf` ou `.conf`).
- **Déroulement Nominal** :
  1. Le joueur sélectionne une fiche de jeu (ex: *League of Legends* ou *Valorant*) et clique sur **Connecter**.
  2. L'application vérifie la présence du binaire `wireguard.exe` ou injecte la configuration dans le service Wintun local.
  3. L'adaptateur virtuel L3 Wintun s'initialise avec l'adresse IP attribuée (`10.10.0.X/32`) et le MTU fixé à `1380`.
  4. Les routes statiques correspondant aux plages d'adresses IP des serveurs de jeu ciblés sont injectées dans la table de routage Windows.
  5. La sonde de télémétrie RTT démarre l'envoi périodique de paquets `TypePing` vers le port UDP 4141 du relais.
  6. Le graphique de latence temps-réel s'actualise avec les métriques millisecondes.
- **Cas d'Erreur** :
  - *Conflit d'interface Wintun existante* : L'application désinstalle préventivement l'adaptateur fantôme avant d'en instancier un nouveau.
  - *Absence de configuration locale* : L'interface invite le joueur à procéder préalablement à l'auto-enrôlement.

---

### UC-04 : Désactivation du Tunnel
- **Acteur** : Joueur.
- **Prérequis** : Tunnel actif en cours d'exécution.
- **Déroulement Nominal** :
  1. Le joueur clique sur **Déconnecter**.
  2. L'application ordonne la fermeture de l'adaptateur réseau virtuel via l'API WireGuard Windows.
  3. Le pilote Wintun supprime l'interface virtuelle et retire les routes CIDR injectées.
  4. Le trafic vers les serveurs de jeu bascule instantanément vers la passerelle Internet par défaut sans coupure de connectivité globale.
  5. Le monitoring de latence RTT s'arrête et l'état de l'interface passe à "Déconnecté".

---

### UC-05 : Supervision Administrative du Relais
- **Acteur** : Ingénieur Réseau / Administrateur de l'infrastructure.
- **Prérequis** : Présence du fichier `admin.key` sur le poste local (clé cryptographique secrète non commitée).
- **Déroulement Nominal** :
  1. L'administrateur clique sur le bouton **Supervision VPS** dans l'interface graphique.
  2. L'application vérifie la présence de `admin.key`. Si la clé est introuvable, l'accès est refusé avec un message de restriction d'autorisation.
  3. Le client émet une requête `TypeAdminReq` vers le port UDP 4141 du relais, signée à l'aide d'un jeton à usage unique calculé par hachage SHA-256 avec horodatage strict (nonce anti-rejeu).
  4. Le démon relais valide l'empreinte et exécute `wg show wg0 dump`.
  5. Le serveur retourne la liste exhaustive des pairs connectés, leur bande passante (octets émis/reçus), le dernier handshake et le statut en ligne/hors-ligne.
  6. L'interface affiche la table de supervision avec possibilité de révocation d'un joueur en temps réel.
- **Cas d'Erreur** :
  - *Clé administrative invalide ou obsolète* : Le relais rejette la requête silencieusement ou renvoie un statut d'erreur cryptographique.
  - *Délai de réponse dépassé* : Un mécanisme de re-tentative automatique (2 essais, timeout 3s) pallie les pertes transitoires de paquets sur le lien sous-marin.

---

### UC-06 : Mise à Jour Automatique du Binaire
- **Acteur** : Joueur / Système.
- **Prérequis** : Accès HTTPS sortant vers `api.github.com`.
- **Déroulement Nominal** :
  1. Au lancement de l'interface, le client interroge l'endpoint GitHub Releases (`https://api.github.com/repos/Cioq/CrazyPing/releases/latest`).
  2. Le client compare le tag distant (ex: `v1.2.0`) avec la constante interne `AppVersion` (`v1.1.0`).
  3. Si une version plus récente est disponible, un bandeau d'alerte s'affiche sur l'interface proposant la mise à jour en un clic.
  4. L'utilisateur clique sur **Télécharger et Installer**.
  5. Le binaire télécharge l'exécutable d'installation autonome `CrazyPing-Setup.exe` dans le dossier temporaire système.
  6. Le programme lance l'installateur avec élévation de privilèges et termine l'instance actuelle pour permettre l'écrasement des fichiers exécutables.

---

### UC-07 : Surveillance et Extinction Sécurisée (Watchdog)
- **Acteur** : Système (Chien de garde autonome).
- **Prérequis** : Processus `CrazyPing.exe` en cours d'exécution.
- **Déroulement Nominal** :
  1. Durant la session normale, l'interface graphique émet une requête POST `/api/heartbeat` toutes les 1500 ms, actualisant le compteur temporel atomique.
  2. Lorsque l'utilisateur ferme la fenêtre (croix de fermeture), un signal `navigator.sendBeacon('/api/exit')` est transmis au serveur local.
  3. L'application interrompt les tunnels Wintun actifs, libère les ressources noyau et termine le processus (`os.Exit(0)`).
  4. *Cas alternatif (crash ou gel de la WebView)* : Si aucun battement de cœur n'est reçu pendant une durée supérieure à 30 secondes, le thread de contrôle Watchdog constate l'inactivité, commande la fermeture préventive de tous les adaptateurs virtuels et stoppe le processus.
