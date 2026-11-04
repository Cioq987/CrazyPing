# Journal des Évolutions Logicielles et Historique des Versions - CrazyPing

Ce document consigne l'ensemble des montées de version, évolutions fonctionnelles, correctifs de sécurité et ajustements d'infrastructure apportés à la plateforme **CrazyPing**.

La numérotation des versions respecte les principes du **Semantic Versioning (SemVer)** : `vMAJOR.MINOR.PATCH`.

---

## [1.4.3] - 2026-11-03

### Amélioration du Déploiement & Expérience Utilisateur
- **Finalisation de l'Auto-Updater Silencieux (%LOCALAPPDATA%\\Programs)** :
  - Installation utilisateur sans friction et élimination du code d'élévation 740.
  - Déclenchement automatique de la nouvelle version et purge des anciens raccourcis publics.
- **Routage et Stabilité Réseau** :
  - Préservation continue des identités, profils de jeux et supervision temps réel.

---

## [1.4.2] - 2026-11-03

### Architecture de Déploiement & Expérience Utilisateur
- **Migration Espace Utilisateur (%LOCALAPPDATA%\\Programs)** :
  - Déplacement de l'installation vers l'espace applicatif utilisateur pour des mises à jour 100% silencieuses sans blocage UAC.
  - Élimination définitive des blocages d'écriture et chargement direct du profil graphique natif sombre.
- **Migration Automatique et Transparente des Données Joueur** :
  - Rapatriement automatique des identités Curve25519 (`client_identity.json`), jetons et profils de jeux locaux depuis l'ancien répertoire `Program Files`.
  - Préservation intégrale de la configuration joueur sans aucune intervention manuelle.

---

## [1.4.1] - 2026-11-02

### Correctifs & Améliorations de Stabilité
- **Compression Gzip UDP & Respect Strict du MTU 1380 (Câble Honotua)** :
  - Compression dynamique des trames d'administration sur le VPS éliminant tout rejet de paquet lié au MTU sous-marin.
  - Décompression transparente côté client réduisant la charge utile réseau de plus de 75%.
- **Stabilité Permanente du Superviseur VPS en Arrière-Plan** :
  - Suppression de l'arrêt intempestif causé par le bridage des temporisateurs Chromium en arrière-plan.
  - Maintien continu de la supervision et de la liaison de télémétrie locale sans interruption inopinée.
- **Élévation UAC Automatique du Programme de Mise à Jour** :
  - Lancement de l'installateur avec élévation des privilèges administrateur (`RunAs`), garantissant le remplacement des binaires dans `C:\Program Files\CrazyPing`.
- **Habillage Sombre Natif & Intégration Visuelle** :
  - Intégration de la directive `<meta name="theme-color">` et du fichier `favicon.ico` pour éliminer tout cadre blanc et habiller la barre de titre de la fenêtre native.
- **Synchronisation Dynamique des Versions** :
  - Liaison directe des indicateurs visuels de version avec la configuration de build de l'exécutable.

---

## [1.4.0] - 2026-11-02

### Nouvelles Fonctionnalités & Supervision Avancée
- **Module d'Audit Réseau & Diagnostic Télécom Distant en 1 Clic** :
  - Intégration d'un module d'audit et de santé pour chaque client connecté dans la console d'administration ("Superviseur Infrastructure Honotua").
  - Déclenchement d'un audit complet à la demande vers n'importe quel joueur (`10.10.0.X`) depuis la passerelle de San José via trame d'administration sécurisée UDP.
- **Télémétrie Câble Sous-Marin Honotua & Analyse de Gigue** :
  - Sondes ICMP calibrées à haute fréquence mesurant le RTT réel aller-retour Tahiti &rarr; San José, l'écart-type de gigue (*jitter*) et le taux de perte de paquets.
- **Validation d'Intégrité MTU 1380 & Détection de Fragmentation (DF-bit)** :
  - Émission de trames de test de 1380 octets avec drapeau *Don't Fragment* (DF) pour vérifier la conformité du clamp MTU Wintun et éliminer tout risque de fragmentation ou de rejet pare-feu.
- **Projection Temps Réel des Latences E-Sport Cibles** :
  - Mesure continue des latences vers les points d'interconnexion majeurs (Steam Datagram Relay Los Angeles / Seattle, passerelle Cloudflare) et calcul instantané de la latence de jeu perçue par le joueur.
- **Console de Traçabilité Technique & Évaluation de Conformité** :
  - Restitution détaillée étape par étape dans l'interface superviseur avec diagnostic de conformité (Optimal, Dégradé, Hors ligne) et recommandations opérationnelles.

---

## [1.3.1] - 2026-11-01

### Nouvelles Fonctionnalités & Optimisations Réseau
- **Intégration Embarquée 100% Silencieuse de WireGuard & Wintun** :
  - Inclusion directe du package MSI officiel WireGuard AMD64 (`DO_NOT_LAUNCH=1`) au sein de l'installateur autonome `CrazyPing-Setup.exe`.
  - Déploiement silencieux du pilote Wintun sans aucune intervention de l'utilisateur, supprimant toute redirection vers un site web externe et toute manipulation manuelle.
- **Diagnostic de Santé Kernel & Réassurance Pédagogique** :
  - Détection proactive de la présence du pilote Wintun dès le lancement de l'application avec affichage d'un indicateur d'état en temps réel.
  - Intégration d'une modale pédagogique présentant les 4 piliers de sécurité (certification WHQL Microsoft, code 100% open-source, zéro télémétrie, empreinte ~2 Mo de RAM).
  - Bouton d'auto-réparation et d'installation silencieuse en 1 clic sans redémarrage.
- **Optimisation Majeure du Routage E-Sport Mistfall Hunter (NA)** :
  - Extension complète des plages réseau avec l'ensemble des blocs AWS GameLift US West (Oregon/Californie) et des relais Steam Datagram Relay (SDR) d'Amérique du Nord (`155.133.248.0/21`, `162.254.194.0/23`, `52.32.0.0/11`, `44.224.0.0/11`, etc.).
  - Élimination des routages asymétriques et des pénalités de fallback SDR, ramenant la latence en jeu à 115-125 ms pour les joueurs tahitiens.

### Correctifs & Stabilité du Cycle de Vie
- **Résolution Définitive des Fenêtres Orphelines Post-Mise à Jour** :
  - Isolation complète du processus de rendu Microsoft Edge via un profil applicatif dédié (`CrazyPing-WebProfile`), éliminant tout rattachement au navigateur personnel de l'utilisateur.
  - Déclenchement d'une fermeture propre et sans délai (`closeNativeWindow`) de l'ancienne fenêtre d'interface avant l'exécution de l'installateur, garantissant une relance sans doublons de fenêtre.
- **Verrou d'Instance Unique Windows (Single-Instance Mutex)** :
  - Intégration d'un mutex nommé Win32 (`Local\CrazyPing_App_Instance`) pour empêcher le lancement concurrent de plusieurs processus d'arrière-plan ou de serveurs locaux sur des ports dupliqués.
- **Réactivité Immédiate à la Clôture de l'Interface** :
  - Détection instantanée de la fermeture de la fenêtre par l'utilisateur, déclenchant l'arrêt propre des tunnels WireGuard et la terminaison du processus père en moins de 100 ms (sans attendre le timeout réseau).

---

## [1.3.0] - 2026-11-01

### Nouvelles Fonctionnalités
- **Mode VPN Global USA (Full-Tunnel)** :
  - Intégration d'un profil de navigation complète (`CrazyPing-GlobalVPN.conf`) avec routage `0.0.0.0/0` et DNS ultra-rapide Cloudflare (`1.1.1.1, 8.8.8.8`) attribuant l'adresse IP américaine de San José (`146.235.218.190`) à l'ensemble du trafic de la machine (Antigravity, Web, Dev, APIs).
- **Architecture de Quota Hybride Co-Responsable (10 Go / jour)** :
  - Garde-fou strict côté serveur Linux (`iptables` et table `ipset cp_game_cidrs`) plafonnant le trafic web non-jeu à 10 Go par joueur et par jour avec réinitialisation automatique chaque nuit à minuit (fuseau horaire de Tahiti).
  - Préservation intégrale du trafic E-Sport : les flux de jeux vidéo sont 100% illimités et exclus du décompte du quota VPN.
  - Ergonomie client réactive : jauge visuelle de quota en temps réel et grisement automatique de la carte avec avertissement dès épuisement du solde quotidien.
- **Priorité E-Sport Intelligente** :
  - Bascule instantanée du profil VPN vers le profil de jeu optimisé dès la détection d'une partie par le module d'auto-détection, garantissant zéro bufferbloat et latence minimale.
  - Restauration automatique du VPN Global USA à la clôture de la session de jeu.
- **Supervision Administrative Élargie** :
  - Ajout du suivi de consommation du quota journalier VPN pour chaque pair dans la console de supervision Honotua.

---

## [1.2.1] - 2026-11-01

### Correctifs & Expérience de Déploiement
- **Relance Automatique Post-Mise à Jour** :
  - Correction de la directive d'exécution Inno Setup (`skipifnotsilent`) pour garantir la relance automatique et sans friction de `CrazyPing.exe` à l'issue de l'installation silencieuse.
- **Cycle de Vie de l'Interface Graphique** :
  - Masquage immédiat de la bannière et fermeture propre de la fenêtre hôte Microsoft Edge avant l'application de la mise à jour pour éliminer l'affichage orphelin de l'ancienne version.
  - Ajout d'un écran de transition pleine page informant l'utilisateur du redémarrage en cours.
- **Fermeture Propre des Tunnels Réseau** :
  - Déconnexion systématique du pilote Wintun et purge des routes temporaires avant le remplacement des fichiers système par l'installateur.

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
