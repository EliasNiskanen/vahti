# Palauttaa tämän koneen alkuperäiset DNS-asetukset käsin.
# Yleensä tätä ei tarvita: asetukset palautuvat itsestään, kun Vahti suljetaan.
# Aja järjestelmänvalvojana:
#   powershell -ExecutionPolicy Bypass -File .\palauta-dns.ps1
#Requires -RunAsAdministrator
Import-Module (Join-Path $PSScriptRoot 'VahtiDns.psm1') -Force

Restore-VahtiDns -Reason 'palauta-dns.ps1'
foreach ($a in Get-VahtiAdapters) {
    $s = @((Get-DnsClientServerAddress -InterfaceIndex $a.ifIndex).ServerAddresses) -join ', '
    if (-not $s) { $s = '(ei nimipalvelinta)' }
    Write-Host ("  {0}: {1}" -f $a.Name, $s)
}
Write-Host ''
Write-Host 'DNS-asetukset palautettu.' -ForegroundColor Green
