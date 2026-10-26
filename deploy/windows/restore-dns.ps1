# Restores this computer's original DNS settings by hand.
# Usually not needed: the settings are restored when Vahti is closed.
# Run as administrator:
#   powershell -ExecutionPolicy Bypass -File .\restore-dns.ps1
#Requires -RunAsAdministrator
Import-Module (Join-Path $PSScriptRoot 'VahtiDns.psm1') -Force

Restore-VahtiDns -Reason 'restore-dns.ps1'
foreach ($a in Get-VahtiAdapters) {
    $s = @((Get-DnsClientServerAddress -InterfaceIndex $a.ifIndex).ServerAddresses) -join ', '
    if (-not $s) { $s = '(no DNS server)' }
    Write-Host ("  {0}: {1}" -f $a.Name, $s)
}
Write-Host ''
Write-Host 'DNS settings restored.' -ForegroundColor Green
