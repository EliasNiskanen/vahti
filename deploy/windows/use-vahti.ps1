# Points this computer's DNS at a Vahti that is already running.
# start-vahti.ps1 is easier: it starts Vahti and does this in one go.
# Run as administrator:
#   powershell -ExecutionPolicy Bypass -File .\use-vahti.ps1
#Requires -RunAsAdministrator
$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'VahtiDns.psm1') -Force

$procs = @(Get-Process -Name 'vahti' -ErrorAction SilentlyContinue)
if ($procs.Count -ne 1 -or -not (Test-VahtiDns)) {
    Write-Host 'Vahti is not running. Start it first:  .\vahti.exe local' -ForegroundColor Red
    exit 1
}

# The guard points DNS at Vahti and restores it when Vahti is closed.
Start-VahtiGuard -VahtiPid $procs[0].Id
Start-Sleep -Seconds 3

$cur = @(Get-VahtiAdapters | ForEach-Object { (Get-DnsClientServerAddress -InterfaceIndex $_.ifIndex).ServerAddresses })
if (Test-PointsToVahti $cur) {
    Write-Host "Done. This computer's DNS queries now go through Vahti." -ForegroundColor Green
    Write-Host 'DNS settings are restored automatically when Vahti is closed.'
} else {
    Write-Host 'Changing the DNS settings failed. See the log: %ProgramData%\Vahti\dns.log' -ForegroundColor Red
    exit 1
}
