# Ohjaa tämän koneen nimikyselyt jo käynnissä olevalle Vahdille.
# Helpompi tapa on kaynnista-vahti.ps1, joka käynnistää Vahdin ja tämän yhdellä kertaa.
# Aja järjestelmänvalvojana:
#   powershell -ExecutionPolicy Bypass -File .\kayta-vahtia.ps1
#Requires -RunAsAdministrator
$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'VahtiDns.psm1') -Force

$procs = @(Get-Process -Name 'vahti' -ErrorAction SilentlyContinue)
if ($procs.Count -ne 1 -or -not (Test-VahtiDns)) {
    Write-Host 'Vahti ei ole käynnissä. Käynnistä se ensin:  .\vahti.exe local' -ForegroundColor Red
    exit 1
}

# Vartija ohjaa DNS:n ja palauttaa sen, kun Vahti suljetaan.
Start-VahtiGuard -VahtiPid $procs[0].Id
Start-Sleep -Seconds 3

$cur = @(Get-VahtiAdapters | ForEach-Object { (Get-DnsClientServerAddress -InterfaceIndex $_.ifIndex).ServerAddresses })
if (Test-PointsToVahti $cur) {
    Write-Host 'Valmis. Tämän koneen nimikyselyt kulkevat nyt Vahdin kautta.' -ForegroundColor Green
    Write-Host 'DNS-asetukset palautetaan automaattisesti, kun Vahti suljetaan.'
} else {
    Write-Host 'DNS-asetusten muuttaminen epäonnistui. Katso loki: %ProgramData%\Vahti\dns.log' -ForegroundColor Red
    exit 1
}
