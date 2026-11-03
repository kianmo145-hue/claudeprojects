# Smoke test on a Windows runner: start the app headless, add servers through the API, test them,
# connect in TUN mode (creates the Wintun adapter and routes), then disconnect and quit.
#   scripts\windows-smoke.ps1 dist\paydar.exe
param([Parameter(Mandatory = $true)][string]$Exe)
$ErrorActionPreference = 'Stop'

$data = Join-Path $env:RUNNER_TEMP 'paydar-smoke'
if (Test-Path $data) { Remove-Item $data -Recurse -Force }
$env:PAYDAR_DATA = $data
$proc = Start-Process -FilePath (Resolve-Path $Exe) -ArgumentList '-headless', '-port', '18555' -PassThru

$urlFile = Join-Path $data 'ui-url'
for ($i = 0; $i -lt 50 -and -not (Test-Path $urlFile); $i++) { Start-Sleep -Milliseconds 200 }
if (-not (Test-Path $urlFile)) { throw 'the app did not start (no ui-url file)' }
$url = Get-Content $urlFile -Raw
$base = ($url -split '/#')[0]
$token = ($url -split 't=')[1].Trim()
Write-Host "UI at $base"

function Call([string]$Method, $Body = @{}) {
    $r = Invoke-RestMethod -Method Post -Uri "$base/api/$Method" -Headers @{ 'X-Token' = $token } `
        -ContentType 'application/json' -Body ($Body | ConvertTo-Json -Compress)
    if (-not $r.ok) { throw "$Method failed: $($r.error)" }
    return $r.data
}

$page = Invoke-WebRequest -Uri "$base/" -UseBasicParsing
if ($page.Content -notmatch 'app.js') { throw 'UI page not served' }

$hello = Call 'hello'
Write-Host "platform $($hello.platform), version $($hello.version)"
$added = Call 'add' @{ text = "ss://aes-128-gcm:pw@127.0.0.1:9#dead-1`ntrojan://pw@127.0.0.1:10#dead-2" }
if ($added.added -ne 2) { throw "expected 2 servers, got $($added.added)" }

Call 'test' | Out-Null
Start-Sleep -Seconds 8
$servers = Call 'servers'
foreach ($s in $servers) { Write-Host "$($s.name): $($s.health.state)" }
if (($servers | Where-Object { $_.health.state -eq 'down' }).Count -ne 2) { throw 'dead servers not detected' }

Call 'connect' | Out-Null
for ($i = 0; $i -lt 30; $i++) {
    $st = Call 'status'
    if ($st.state -ne 'connecting') { break }
    Start-Sleep -Milliseconds 300
}
Write-Host "after connect: $($st.state) mode=$($st.mode) error=$($st.error)"
if ($st.state -ne 'connected') { throw "connect failed: $($st.error)" }
$adapter = Get-NetAdapter -Name 'Paydar' -ErrorAction SilentlyContinue
if (-not $adapter) { throw 'TUN adapter "Paydar" not found' }
Write-Host "adapter: $($adapter.InterfaceDescription) $($adapter.Status)"

Call 'disconnect' | Out-Null
Start-Sleep -Seconds 2
$st = Call 'status'
if ($st.state -ne 'disconnected') { throw "still $($st.state) after disconnect" }

Call 'quit' | Out-Null
if (-not $proc.WaitForExit(15000)) { $proc.Kill(); throw 'the app did not quit' }
Write-Host 'smoke test passed'
