@echo off
title CrazyPing - Reinitialisation Reseau d'Urgence
echo ========================================================
echo   CrazyPing - Restauration d'Urgence de la Connexion
echo ========================================================
echo.
echo Demontage de tous les adaptateurs Wintun / WireGuard...

set WG="C:\Program Files\WireGuard\wireguard.exe"
if exist %WG% (
    %WG% /uninstalltunnelservice CrazyPing-GlobalVPN >nul 2>&1
    %WG% /uninstalltunnelservice CrazyPing-GlobalVPN.local >nul 2>&1
    %WG% /uninstalltunnelservice CrazyPing-MistfallHunter >nul 2>&1
    %WG% /uninstalltunnelservice CrazyPing-MistfallHunter.local >nul 2>&1
    %WG% /uninstalltunnelservice CrazyPing-Valorant >nul 2>&1
    %WG% /uninstalltunnelservice CrazyPing-Valorant.local >nul 2>&1
    %WG% /uninstalltunnelservice CrazyPing-Fortnite >nul 2>&1
    %WG% /uninstalltunnelservice CrazyPing-Fortnite.local >nul 2>&1
    %WG% /uninstalltunnelservice CrazyPing-LoL >nul 2>&1
    %WG% /uninstalltunnelservice CrazyPing-LoL.local >nul 2>&1
    %WG% /uninstalltunnelservice CrazyPing-Overwatch >nul 2>&1
    %WG% /uninstalltunnelservice CrazyPing-Overwatch.local >nul 2>&1
)

echo Nettoyage des routes residuelles...
route delete 0.0.0.0 mask 0.0.0.0 10.10.0.1 >nul 2>&1
route delete 0.0.0.0 mask 0.0.0.0 10.10.0.2 >nul 2>&1

echo.
echo [SUCCES] Tous les adaptateurs de tunnel ont ete demontes.
echo Votre connexion Internet standard est restauree.
echo.
pause
