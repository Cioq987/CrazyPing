#!/usr/bin/env bash
# Script d'installation automatique pour VPS Ubuntu / Debian / Oracle Linux
set -e

echo "=== Installation de CrazyPing Relay Server ==="

# Mise a jour et installation de Docker si non installe
if ! command -v docker &> /dev/null; then
    echo "-> Installation de Docker..."
    curl -fsSL https://get.docker.com | sh
    systemctl enable --now docker
fi

# Ouverture du port UDP dans le pare-feu UFW si present
if command -v ufw &> /dev/null; then
    echo "-> Ouverture du port UDP 4141 dans UFW..."
    ufw allow 4141/udp || true
fi

# Ouverture du port dans iptables (necessaire pour Oracle Cloud)
iptables -I INPUT -p udp --dport 4141 -j ACCEPT || true

echo "-> Lancement du conteneur CrazyPing Relay..."
docker compose -f deploy/docker-compose.yml up -d --build

echo "=== CrazyPing Relay est operationnel sur le port UDP 4141 ! ==="
docker ps --filter "name=crazyping-relay"
