#!/bin/bash
set -e

echo "=== Configuration WireGuard sur Ubuntu ==="

# 1. Activation du routage IPv4
sudo sysctl -w net.ipv4.ip_forward=1
sudo sed -i '/net.ipv4.ip_forward=1/d' /etc/sysctl.conf
echo "net.ipv4.ip_forward=1" | sudo tee -a /etc/sysctl.conf

# 2. Generation des cles
mkdir -p /tmp/wg-keys && cd /tmp/wg-keys
wg genkey | tee server_priv.key | wg pubkey > server_pub.key
wg genkey | tee client_priv.key | wg pubkey > client_pub.key

SERVER_PRIV=$(cat server_priv.key)
SERVER_PUB=$(cat server_pub.key)
CLIENT_PRIV=$(cat client_priv.key)
CLIENT_PUB=$(cat client_pub.key)

# 3. Creation de la configuration serveur avec interface ens3
sudo tee /etc/wireguard/wg0.conf > /dev/null <<EOF
[Interface]
Address = 10.10.0.1/24
ListenPort = 51820
PrivateKey = $SERVER_PRIV
PostUp = iptables -A FORWARD -i wg0 -j ACCEPT; iptables -t nat -A POSTROUTING -o ens3 -j MASQUERADE
PostDown = iptables -D FORWARD -i wg0 -j ACCEPT; iptables -t nat -D POSTROUTING -o ens3 -j MASQUERADE

[Peer]
PublicKey = $CLIENT_PUB
AllowedIPs = 10.10.0.2/32
EOF

# 4. Pare-feu local
sudo iptables -I INPUT -p udp --dport 51820 -j ACCEPT

# 5. Demarrage du service
sudo systemctl restart wg-quick@wg0
sudo systemctl enable wg-quick@wg0

echo "===STATUS_SUCCESS==="
echo "SERVER_PUB=$SERVER_PUB"
echo "CLIENT_PRIV=$CLIENT_PRIV"
