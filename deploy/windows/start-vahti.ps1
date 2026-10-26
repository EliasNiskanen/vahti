# Starts Vahti and points this computer's DNS at it.
# DNS settings are restored automatically when the Vahti window is closed.
#
#   powershell -ExecutionPolicy Bypass -File .\start-vahti.ps1
#   powershell -ExecutionPolicy Bypass -File .\start-vahti.ps1 -Router 192.168.1.1
param([string]$Router = '')
$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'VahtiDns.psm1') -Force

$exe = Join-Path $PSScriptRoot 'vahti.exe'
if (-not (Test-Path $exe)) { Write-Host "vahti.exe is missing from $PSScriptRoot" -ForegroundColor Red; exit 1 }
if (Test-VahtiDns) { Write-Host 'Vahti is already running.' -ForegroundColor Yellow; exit 0 }

$vahtiArgs = @('local')
if ($Router) { $vahtiArgs += @('-router', $Router) }
$p = Start-Process -FilePath $exe -ArgumentList $vahtiArgs -WorkingDirectory $PSScriptRoot -PassThru

Write-Host 'Starting Vahti...'
$ok = $false
for ($i = 0; $i -lt 30; $i++) {
    if ($p.HasExited) { break }
    if (Test-VahtiDns) { $ok = $true; break }
    Start-Sleep -Milliseconds 500
}
if (-not $ok) {
    Write-Host 'Vahti did not start. See the error in the Vahti window.' -ForegroundColor Red
    exit 1
}

Write-Host 'Windows will now ask for permission to change the DNS settings.'
try {
    Start-VahtiGuard -VahtiPid $p.Id
} catch {
    Write-Host 'Permission was not given, so the DNS settings were not changed.' -ForegroundColor Yellow
    Write-Host "Vahti is running, but it is not filtering this computer's traffic."
    exit 1
}

Start-Sleep -Seconds 2
Write-Host ''
Write-Host "Vahti is running and filtering this computer's traffic." -ForegroundColor Green
Write-Host '  Manage:  http://127.0.0.1:8080'
Write-Host '  Stop:    close the Vahti window or press Ctrl+C in it.'
Write-Host '           DNS settings are restored automatically.'
Write-Host '  Check:   powershell -ExecutionPolicy Bypass -File .\check-dns.ps1'
