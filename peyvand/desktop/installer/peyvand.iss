; Inno Setup script. Built by scripts/build.ps1 (needs dist/ filled first):
;   ISCC /DVersion=0.1.0 installer\peyvand.iss
#ifndef Version
  #define Version "0.0.0"
#endif

[Setup]
AppId={{9162A318-A2BD-45BC-83D7-FEC155A0D86E}
AppName=Peyvand
AppVersion={#Version}
AppVerName=Peyvand {#Version}
AppPublisher=Peyvand contributors
#ifdef RepoUrl
AppPublisherURL={#RepoUrl}
AppSupportURL={#RepoUrl}/issues
AppUpdatesURL={#RepoUrl}/releases
#endif
VersionInfoVersion={#Version}
DefaultDirName={autopf}\peyvand
DefaultGroupName=Peyvand
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
OutputBaseFilename=peyvand-setup-{#Version}
UninstallDisplayIcon={app}\peyvand.ico
CloseApplications=no
; Uninstall removes %LOCALAPPDATA%\peyvand (status/log files) of the user who installed.
UsedUserAreasWarning=no

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "autostart"; Description: "Start Peyvand when I sign in (no UAC prompt)"; GroupDescription: "Startup:"; Flags: unchecked
Name: "desktopicon"; Description: "Create a &desktop shortcut"; GroupDescription: "Shortcuts:"; Flags: unchecked

[Files]
Source: "dist\peyvand.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "dist\peyvand-tray.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "dist\wintun.dll"; DestDir: "{app}"; Flags: ignoreversion
Source: "dist\wintun-LICENSE.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "dist\peyvand.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "THIRD-PARTY-NOTICES.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "README.md"; DestDir: "{app}"; Flags: ignoreversion isreadme

[Icons]
Name: "{autoprograms}\Peyvand"; Filename: "{app}\peyvand-tray.exe"; IconFilename: "{app}\peyvand.ico"; Comment: "Use all your connections at once (پیوند)"
Name: "{autodesktop}\Peyvand"; Filename: "{app}\peyvand-tray.exe"; IconFilename: "{app}\peyvand.ico"; Tasks: desktopicon

[Run]
Filename: "{app}\peyvand.exe"; Parameters: "autostart on"; Flags: runhidden; Tasks: autostart; StatusMsg: "Enabling start at sign-in..."
; The installer is already elevated, so the tray starts without a second UAC prompt.
Filename: "{app}\peyvand-tray.exe"; Description: "Start peyvand now"; Flags: nowait postinstall skipifsilent

; Booster: let phones' announcements on the local network reach the tray (UDP 47800).
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall add rule name=""Peyvand booster"" dir=in action=allow protocol=UDP localport=47800 profile=private,domain program=""{app}\peyvand-tray.exe"""; Flags: runhidden; StatusMsg: "Allowing booster phones through the firewall..."

[UninstallRun]
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall delete rule name=""Peyvand booster"""; Flags: runhidden; RunOnceId: "FirewallOff"
Filename: "{app}\peyvand.exe"; Parameters: "autostart off"; Flags: runhidden; RunOnceId: "AutostartOff"

[UninstallDelete]
Type: filesandordirs; Name: "{localappdata}\peyvand"

[Code]
// Turn peyvand off and make sure no copy is running before files are replaced or removed.
// Killing it is safe: the TUN adapter and its routes disappear with the process.
procedure StopPeyvand;
var
  rc: Integer;
begin
  if FileExists(ExpandConstant('{app}\peyvand.exe')) then
    Exec(ExpandConstant('{app}\peyvand.exe'), 'quit', '', SW_HIDE, ewWaitUntilTerminated, rc);
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /IM peyvand-tray.exe', '', SW_HIDE, ewWaitUntilTerminated, rc);
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /IM peyvand.exe', '', SW_HIDE, ewWaitUntilTerminated, rc);
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  StopPeyvand;
  Result := '';
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usUninstall then
    StopPeyvand;
end;
