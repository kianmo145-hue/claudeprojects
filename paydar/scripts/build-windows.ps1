# Builds dist\: paydar.exe, the installer and a portable zip.
#   scripts\build-windows.ps1 -Version 0.1.0
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

$tags = 'with_gvisor,with_quic,with_utls,badlinkname,tfogo_checklinkname0'
if (Test-Path dist) { Remove-Item dist -Recurse -Force }
New-Item -ItemType Directory dist | Out-Null

Invoke-Native 'go vet'  { go vet -tags $tags ./... }
Invoke-Native 'go test' { go test -tags $tags -ldflags=-checklinkname=0 ./... }

# The icon is drawn in code; a plain build writes it out first.
Invoke-Native 'icon' { go run ./cmd/paydar -write-icon dist\paydar.ico }

# Version info, icon and an admin manifest (the TUN adapter needs elevation). Antivirus ML models
# flag bare unsigned Go exes, so the resources matter.
Invoke-Native 'resources' { go run github.com/tc-hib/go-winres@v0.3.3 simply --arch amd64 --out cmd\paydar\rsrc --icon dist\paydar.ico --manifest gui --admin --product-name Paydar --file-description 'Paydar VPN client' --original-filename paydar.exe --product-version $Version --file-version $Version --copyright 'Paydar contributors' }
Invoke-Native 'build' { go build -tags $tags -trimpath -ldflags "-s -w -H=windowsgui -checklinkname=0 -X main.version=$Version" -o dist\paydar.exe .\cmd\paydar }
Remove-Item cmd\paydar\rsrc_windows_amd64.syso

$iscc = $env:ISCC
if (-not $iscc) { $iscc = (Get-Command ISCC.exe -ErrorAction SilentlyContinue).Source }
if (-not $iscc) {
    $iscc = Get-ChildItem 'C:\Program Files (x86)\Inno Setup 6\ISCC.exe', 'C:\Program Files\Inno Setup 6\ISCC.exe' -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty FullName
}
if (-not $iscc) { throw 'ISCC.exe not found. Install Inno Setup 6 or set $env:ISCC.' }
$isccArgs = @("/DVersion=$Version")
if ($RepoUrl) { $isccArgs += "/DRepoUrl=$RepoUrl" }
Invoke-Native 'inno setup' { & $iscc @isccArgs installer\paydar.iss }

$portable = "dist\paydar-$Version-portable.zip"
Compress-Archive -Path dist\paydar.exe, LICENSE, README.md, README.en.md -DestinationPath $portable
Get-ChildItem dist\paydar-setup-$Version.exe, $portable | ForEach-Object { '{0}  {1:N1} MB' -f $_.Name, ($_.Length / 1MB) }
