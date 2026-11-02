; Inno Setup script. Built by scripts/build.ps1 (needs dist/ filled first):
;   ISCC /DVersion=0.1.0 installer\braid.iss
#ifndef Version
  #define Version "0.0.0"
#endif

[Setup]
AppId={{3C8AAD2B-1C76-4427-8617-4F455B7C40AE}
AppName=Braid
AppVersion={#Version}
AppVerName=Braid {#Version}
AppPublisher=Braid contributors
#ifdef RepoUrl
AppPublisherURL={#RepoUrl}
AppSupportURL={#RepoUrl}/issues
AppUpdatesURL={#RepoUrl}/releases
#endif
VersionInfoVersion={#Version}
DefaultDirName={autopf}\braid
DefaultGroupName=Braid
DisableProgramGroupPage=yes
; The TUN adapter and routes need admin, and so does writing to Program Files.
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0.17763
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
LicenseFile=LICENSE
SourceDir=..
OutputDir=dist
OutputBaseFilename=braid-setup-{#Version}
UninstallDisplayIcon={app}\braid.ico
CloseApplications=no
; Uninstall removes %LOCALAPPDATA%\braid (status/log files) of the user who installed.
UsedUserAreasWarning=no

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "autostart"; Description: "Start Braid when I sign in (no UAC prompt)"; GroupDescription: "Startup:"; Flags: unchecked
Name: "desktopicon"; Description: "Create a &desktop shortcut"; GroupDescription: "Shortcuts:"; Flags: unchecked

[Files]
Source: "dist\braid.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "dist\braid-tray.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "dist\wintun.dll"; DestDir: "{app}"; Flags: ignoreversion
Source: "dist\wintun-LICENSE.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "dist\braid.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "THIRD-PARTY-NOTICES.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "README.md"; DestDir: "{app}"; Flags: ignoreversion isreadme

[Icons]
Name: "{autoprograms}\Braid"; Filename: "{app}\braid-tray.exe"; IconFilename: "{app}\braid.ico"; Comment: "Use all your connections at once"
Name: "{autodesktop}\Braid"; Filename: "{app}\braid-tray.exe"; IconFilename: "{app}\braid.ico"; Tasks: desktopicon

[Run]
Filename: "{app}\braid.exe"; Parameters: "autostart on"; Flags: runhidden; Tasks: autostart; StatusMsg: "Enabling start at sign-in..."
; The installer is already elevated, so the tray starts without a second UAC prompt.
Filename: "{app}\braid-tray.exe"; Description: "Start braid now"; Flags: nowait postinstall skipifsilent

; Booster: let phones' announcements on the local network reach the tray (UDP 47800).
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall add rule name=""Braid booster"" dir=in action=allow protocol=UDP localport=47800 profile=private,domain program=""{app}\braid-tray.exe"""; Flags: runhidden; StatusMsg: "Allowing booster phones through the firewall..."

[UninstallRun]
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall delete rule name=""Braid booster"""; Flags: runhidden; RunOnceId: "FirewallOff"
Filename: "{app}\braid.exe"; Parameters: "autostart off"; Flags: runhidden; RunOnceId: "AutostartOff"

[UninstallDelete]
Type: filesandordirs; Name: "{localappdata}\braid"

[Code]
// Turn braid off and make sure no copy is running before files are replaced or removed.
// Killing it is safe: the TUN adapter and its routes disappear with the process.
procedure StopBraid;
var
  rc: Integer;
begin
  if FileExists(ExpandConstant('{app}\braid.exe')) then
    Exec(ExpandConstant('{app}\braid.exe'), 'quit', '', SW_HIDE, ewWaitUntilTerminated, rc);
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /IM braid-tray.exe', '', SW_HIDE, ewWaitUntilTerminated, rc);
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /IM braid.exe', '', SW_HIDE, ewWaitUntilTerminated, rc);
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  StopBraid;
  Result := '';
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usUninstall then
    StopBraid;
end;
