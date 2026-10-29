; Script Inno Setup de Déploiement - CrazyPing Client
; Génère CrazyPing-Setup.exe avec privilèges d'élévation UAC

#define MyAppName "CrazyPing"
#define MyAppVersion "1.0.0"
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
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
LicenseFile=
OutputDir=..\dist
OutputBaseFilename=CrazyPing-Setup
SetupIconFile=..\icon.ico
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
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
Source: "..\docs\*"; DestDir: "{app}\docs"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\icon.ico"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\icon.ico"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

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

procedure CurStepChanged(CurStep: TSetupStep);
var
  ErrorCode: Integer;
begin
  if CurStep = ssPostInstall then
  begin
    if not IsWireGuardInstalled() then
    begin
      if MsgBox('CrazyPing nécessite le pilote réseau Wintun / WireGuard pour fonctionner.'#13#10 +
                'Souhaitez-vous ouvrir la page officielle de téléchargement de WireGuard Windows ?', 
                mbConfirmation, MB_YESNO) = IDYES then
      begin
        ShellExec('open', 'https://www.wireguard.com/install/', '', '', SW_SHOWNORMAL, ewNoWait, ErrorCode);
      end;
    end;
  end;
end;
