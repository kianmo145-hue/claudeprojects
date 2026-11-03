; Inno Setup script. Built by scripts/build-windows.ps1 (needs dist/ filled first):
;   ISCC /DVersion=0.1.0 installer\paydar.iss
#ifndef Version
  #define Version "0.0.0"
#endif

[Setup]
AppId={{6B0D51E4-0C55-4D7E-9A37-2F1C7B0E9A41}
AppName=Paydar
AppVersion={#Version}
AppVerName=Paydar {#Version}
AppPublisher=Paydar contributors
#ifdef RepoUrl
AppPublisherURL={#RepoUrl}
AppSupportURL={#RepoUrl}/issues
AppUpdatesURL={#RepoUrl}/releases
#endif
VersionInfoVersion={#Version}
DefaultDirName={autopf}\Paydar
DefaultGroupName=Paydar
DisableProgramGroupPage=yes
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
OutputBaseFilename=paydar-setup-{#Version}
UninstallDisplayIcon={app}\paydar.exe
SetupIconFile=dist\paydar.ico
CloseApplications=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a &desktop shortcut"; GroupDescription: "Shortcuts:"

[Files]
Source: "dist\paydar.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "README.en.md"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\Paydar"; Filename: "{app}\paydar.exe"; Comment: "Stable VPN client with automatic failover"
Name: "{autodesktop}\Paydar"; Filename: "{app}\paydar.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\paydar.exe"; Description: "Start Paydar"; Flags: nowait postinstall skipifsilent shellexec

[UninstallRun]
Filename: "{sys}\taskkill.exe"; Parameters: "/F /IM paydar.exe"; Flags: runhidden; RunOnceId: "KillPaydar"
