; Script Inno Setup de Déploiement - CrazyPing Client
; Génère CrazyPing-Setup.exe avec privilèges d'élévation UAC

#define MyAppName "CrazyPing"
#define MyAppVersion "1.4.4"
#define MyAppPublisher "CrazyPing Project"
#define MyAppURL "https://github.com"
#define MyAppExeName "CrazyPing.exe"

[Setup]
AppId={{E6F7A189-982B-4A7B-9CE1-4B27C1E70341}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={localappdata}\Programs\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
LicenseFile=
OutputDir=..\dist
OutputBaseFilename=CrazyPing-Setup
SetupIconFile=..\icon.ico
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=lowest
ArchitecturesInstallIn64BitMode=x64compatible
UninstallDisplayIcon={app}\{#MyAppExeName}

[Languages]
Name: "french"; MessagesFile: "compiler:Languages\French.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "..\CrazyPing.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\icon.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\CrazyPing-Fortnite.conf"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\CrazyPing-MistfallHunter.conf"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\CrazyPing-Valorant.conf"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\CrazyPing-LoL.conf"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\CrazyPing-Overwatch.conf"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\CrazyPing-GlobalVPN.conf"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\reset-network.bat"; DestDir: "{app}"; Flags: ignoreversion
Source: "wireguard-amd64-1.1.1.msi"; DestDir: "{app}"; DestName: "wireguard-installer.msi"; Flags: ignoreversion
Source: "wireguard-amd64-1.1.1.msi"; DestDir: "{tmp}"; Flags: deleteafterinstall; Check: not IsWireGuardInstalled
Source: "..\docs\*"; DestDir: "{app}\docs"; Flags: ignoreversion recursesubdirs createallsubdirs

[InstallDelete]
Type: files; Name: "{commondesktop}\{#MyAppName}.lnk"
Type: filesandordirs; Name: "{commonprograms}\{#MyAppName}"

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\icon.ico"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\icon.ico"; Tasks: desktopicon

[Run]
Filename: "msiexec.exe"; Parameters: "/i ""{tmp}\wireguard-amd64-1.1.1.msi"" /qn /norestart DO_NOT_LAUNCH=1"; StatusMsg: "Configuration du pilote réseau WireGuard / Wintun..."; Flags: runhidden waituntilterminated; Check: not IsWireGuardInstalled
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: shellexec nowait postinstall skipifsilent
Filename: "{app}\{#MyAppExeName}"; Flags: shellexec nowait skipifnotsilent

[Code]
// Vérification de la présence du binaire wireguard.exe sur la machine hôte
function IsWireGuardInstalled(): Boolean;
var
  InstalledPath: String;
begin
  Result := RegQueryStringValue(HKEY_LOCAL_MACHINE, 'SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\wireguard.exe', '', InstalledPath);
  if not Result then
    Result := FileExists(ExpandConstant('{pf}\WireGuard\wireguard.exe'));
end;

// Migration transparente des configurations et identités depuis l'ancien Program Files vers %LOCALAPPDATA%
procedure MigrateOldProgramFilesData();
var
  OldDir, NewDir: String;
  FilesToMigrate: array of String;
  I: Integer;
  OldFile, NewFile: String;
begin
  OldDir := ExpandConstant('{pf}\CrazyPing');
  NewDir := ExpandConstant('{app}');

  if DirExists(OldDir) and (OldDir <> NewDir) then
  begin
    SetArrayLength(FilesToMigrate, 9);
    FilesToMigrate[0] := 'client_identity.json';
    FilesToMigrate[1] := 'admin.key';
    FilesToMigrate[2] := 'vpn_quota.json';
    FilesToMigrate[3] := 'CrazyPing-GlobalVPN.local.conf';
    FilesToMigrate[4] := 'CrazyPing-MistfallHunter.local.conf';
    FilesToMigrate[5] := 'CrazyPing-Valorant.local.conf';
    FilesToMigrate[6] := 'CrazyPing-Fortnite.local.conf';
    FilesToMigrate[7] := 'CrazyPing-LoL.local.conf';
    FilesToMigrate[8] := 'CrazyPing-Overwatch.local.conf';

    for I := 0 to GetArrayLength(FilesToMigrate) - 1 do
    begin
      OldFile := OldDir + '\' + FilesToMigrate[I];
      NewFile := NewDir + '\' + FilesToMigrate[I];
      if FileExists(OldFile) and not FileExists(NewFile) then
      begin
        CopyFile(OldFile, NewFile, False);
      end;
    end;

    // Nettoyage des anciens raccourcis publics hérités de l'installation Program Files
    DeleteFile(ExpandConstant('{commondesktop}\CrazyPing.lnk'));
    DelTree(ExpandConstant('{commonprograms}\CrazyPing'), True, True, True);
  end;
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
  begin
    MigrateOldProgramFilesData();
  end;
end;
