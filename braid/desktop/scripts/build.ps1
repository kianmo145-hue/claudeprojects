# Builds everything into dist\: the two exes, wintun.dll, the installer, and a portable zip.
#   scripts\build.ps1 -Version 0.1.0
# Needs Go and Inno Setup 6 (ISCC.exe on PATH, in a default install dir, or in $env:ISCC).
param(
    [string]$Version = '0.0.0',
    [string]$RepoUrl = ''
)
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)

function Invoke-Native {
    param([string]$What, [scriptblock]$Cmd)
    & $Cmd
    if ($LASTEXITCODE -ne 0) { throw "$What failed (exit $LASTEXITCODE)" }
}

if ($Version -notmatch '^\d+\.\d+\.\d+$') { throw "Version must look like 1.2.3, got '$Version'" }

if (Test-Path dist) { Remove-Item dist -Recurse -Force }
New-Item -ItemType Directory dist | Out-Null

Invoke-Native 'go vet'  { go vet ./... }
Invoke-Native 'go test' { go test ./... }

# Version info, icon and manifest for each exe: antivirus ML models flag bare unsigned Go exes
# (Microsoft's Trojan:Win32/Wacatac.B!ml).
function New-Resources([string]$Exe, [string]$Description) {
    Invoke-Native "resources $Exe" { go run github.com/tc-hib/go-winres@v0.3.3 simply --arch amd64 --out cmd\braid\rsrc --icon dist\braid.ico --manifest cli --product-name Braid --file-description $Description --original-filename $Exe --product-version $Version --file-version $Version --copyright 'Braid contributors' }
}

# The icon is drawn in code (internal/icon): a plain build writes it out first.
Invoke-Native 'icon build' { go build -o dist\icon-tool.exe .\cmd\braid }
Invoke-Native 'icon' { .\dist\icon-tool.exe _icon dist\braid.ico }
Remove-Item dist\icon-tool.exe

# One source, two exes: a console CLI, and a windowless build that defaults to the tray.
New-Resources 'braid.exe' 'Braid'
Invoke-Native 'build cli'  { go build -trimpath -ldflags "-s -w -X main.version=$Version" -o dist\braid.exe .\cmd\braid }
New-Resources 'braid-tray.exe' 'Braid tray'
Invoke-Native 'build tray' { go build -trimpath -ldflags "-s -w -H=windowsgui -X main.defaultCmd=tray -X main.version=$Version" -o dist\braid-tray.exe .\cmd\braid }
Remove-Item cmd\braid\rsrc_windows_amd64.syso

& "$PSScriptRoot\fetch-wintun.ps1" -OutDir dist

$iscc = $env:ISCC
if (-not $iscc) { $iscc = (Get-Command ISCC.exe -ErrorAction SilentlyContinue).Source }
if (-not $iscc) {
    $iscc = Get-ChildItem 'C:\Program Files (x86)\Inno Setup 6\ISCC.exe', 'C:\Program Files\Inno Setup 6\ISCC.exe' -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty FullName
}
if (-not $iscc) { throw 'ISCC.exe not found. Install Inno Setup 6 or set $env:ISCC.' }

$isccArgs = @("/DVersion=$Version")
if ($RepoUrl) { $isccArgs += "/DRepoUrl=$RepoUrl" }
Invoke-Native 'inno setup' { & $iscc @isccArgs installer\braid.iss }

$portable = "dist\braid-$Version-portable.zip"
Compress-Archive -Path dist\braid.exe, dist\braid-tray.exe, dist\wintun.dll, dist\wintun-LICENSE.txt, LICENSE, THIRD-PARTY-NOTICES.md, README.md, README.fa.md -DestinationPath $portable
Get-ChildItem dist\braid-setup-$Version.exe, $portable | ForEach-Object { '{0}  {1:N1} MB' -f $_.Name, ($_.Length / 1MB) }
