# Journal des Évolutions Logicielles et Historique des Versions - CrazyPing

Ce document consigne l'ensemble des montées de version, évolutions fonctionnelles, correctifs de sécurité et ajustements d'infrastructure apportés à la plateforme **CrazyPing**.

La numérotation des versions respecte les principes du **Semantic Versioning (SemVer)** : `vMAJOR.MINOR.PATCH`.

---

## [1.2.0] - 2026-11-01

### Nouvelles Fonctionnalités
- **Auto-Détection Native des Jeux & Tunneling Automatique** :
  - Intégration d'un module de surveillance d'arrière-plan haute performance utilisant les APIs Win32 natives (`CreateToolhelp32Snapshot`).
  - Détection automatique et transparente de l'ouverture des jeux supportés (*Valorant*, *Fortnite*, *Mistfall Hunter*, *League of Legends*, *Overwatch 2*).
  - Démarrage automatique du tunnel d'optimisation Wintun dès la détection de l'exécutable de jeu, sans aucune action requise de la part du joueur.
  - Bascule automatique de profil réseau si le joueur change de titre au cours de sa session.
- **Période de Grâce Anti-Flapping (30 secondes)** :
  - Maintien du tunnel actif pendant 30 secondes lors de la fermeture d'un jeu ou d'un rechargement de carte pour prévenir toute coupure de connexion intempestive.
  - Restauration automatique du routage internet standard à l'issue de la période de grâce si aucune session de jeu n'est active.
- **Contrôle & Télémétrie dans l'Interface** :
  - Ajout d'un commutateur d'activation/désactivation de l'auto-détection dans le panneau de configuration.
  - Badge d'état dynamique en temps réel indiquant l'état de veille ou le processus de jeu actuellement intercepté.
  - Priorité donnée aux actions manuelles de l'utilisateur en cas de débrayage volontaire.

---

## [1.1.1] - 2026-10-31

### Améliorations de Résilience & Robustesse Réseau
- **Watchdog Anti-Throttling** :
  - Extension du délai d'expiration du chien de garde local de 5 à 30 secondes pour prévenir les fermetures intempestives dues à la mise en sommeil des processus d'arrière-plan par le gestionnaire d'énergie Windows ou Edge WebView.
  - Ajout d'écouteurs d'événements JavaScript `window.focus` et `document.visibilitychange` pour émettre un battement de cœur (`heartbeat`) immédiat dès la reprise de premier plan de la fenêtre.
- **Résilience des Requêtes Administratives UDP** :
  - Implémentation d'un mécanisme de re-tentative automatique (2 essais consécutifs, timeout de 3 000 ms par tentative) pour les échanges de contrôle administratif `queryVpsAdminAction`.
  - Absorption complète des pertes transitoires de paquets sur le câble sous-marin Honotua lors de l'interrogation de la supervision VPS.
- **Interface & Transparence des Versions** :
  - Intégration d'un visualiseur de changelog interactif directement accessible dans l'application avec fallback sur l'API GitHub.
  - Présentation automatique des Release Notes dans une modale dédiée avant tout déclenchement de mise à jour.

---

## [1.1.0] - 2026-10-30

### Nouvelles Fonctionnalités
- **Support Élargi des Titres E-Sport** :
  - Intégration du profil réseau dédié pour **League of Legends** (`CrazyPing-LoL.conf`) ciblant les serveurs de jeu Riot Games Amérique du Nord (Chicago / San José).
  - Intégration du profil réseau dédié pour **Overwatch 2** (`CrazyPing-Overwatch.conf`) routant les plages Blizzard Entertainment et AWS NA-West.
- **Actualisation des Tables de Routage Split-Tunneling** :
  - Spécification des plages CIDR et optimisation de l'assignation MTU à 1380 octets.

---

## [1.0.1] - 2026-10-28

### Sécurité & Durcissement Infrastructure
- **Élimination des Clés en Dur** :
  - Suppression intégrale des tokens secrets et clés statiques dans le code source suivi par Git.
  - Déplacement du jeton maître serveur vers un fichier d'environnement isolé `/home/ubuntu/crazyping-admin.token` avec permissions restreintes (`chmod 0600`).
  - Modèle de signature par challenge non rejouable pour l'administration distante (`admin.key`).
- **Cantonnement d'Écoute Réseau** :
  - Restriction stricte du serveur HTTP de contrôle local à l'interface de bouclage `127.0.0.1` (fermeture de l'exposition non chiffrée sur `0.0.0.0`).
- **Validation Cryptographique en Profondeur** :
  - Contrôle systématique du format, de la longueur exacte (32 octets) et du décodage base64 des clés publiques WireGuard entrantes sur l'endpoint d'auto-enrôlement.
- **Isolation L3 Inter-Clients VPS** :
  - Déploiement d'une règle `iptables -I FORWARD -i wg0 -o wg0 -j REJECT` interdisant tout routage latéral ou analyse de port entre joueurs connectés sur le sous-réseau `10.10.0.0/24`.

### Fonctionnalités
- **Module Live Auto-Updater** :
  - Intégration de la détection et du téléchargement automatique des nouvelles versions via l'API publique GitHub Releases (`/releases/latest`).
  - Déclenchement transparent de l'installateur autonome sans nécessiter de réinstallation manuelle.

---

## [1.0.0] - 2026-10-25

### Lancement Initial
- **Moteur Réseau Hybride Client/Relais** :
  - Client Windows natif compilé en Go avec intégration du pilote virtuel de niveau 3 Wintun (`wintun.sys`).
  - Serveur relais Linux autonome sur architecture AMD64 orchestré via WireGuard et `iptables`.
- **Split-Tunneling Spécialisé Tahiti -> USA** :
  - Profils initiaux de réduction de latence pour *Valorant*, *Fortnite* et *Mistfall Hunter*.
- **Système d'Auto-Enrôlement Dynamique** :
  - Génération autonome des clés cryptographiques Curve25519 côté client et assignation d'adresses IPv4 dans `10.10.0.0/24` via le protocole binaire UDP `4141`.
- **Télémétrie Haute Précision** :
  - Émission de sondes RTT cadencées à 1 Hz avec calcul milliseconde du temps de propagation aller-retour sur la dorsale Honotua.
- **Packaging de Déploiement** :
  - Script d'installation automatisé Inno Setup 6 (`CrazyPing-Installer.iss`) avec détection de privilèges et signature d'application.
