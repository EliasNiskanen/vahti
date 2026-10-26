# Ohjaa taman tietokoneen nimikyselyt Vahdille (vahti.exe local).
# Aja jarjestelmanvalvojana:
#   powershell -ExecutionPolicy Bypass -File .\kayta-vahtia.ps1
# Palautus: palauta-dns.ps1
#Requires -RunAsAdministrator
$ErrorActionPreference = 'Stop'

$backupFile = Join-Path $env:LOCALAPPDATA 'Vahti\dns-varmuuskopio.json'

function Test-Vahti([string]$server) {
    try {
        Resolve-DnsName -Name 'selfcheck.vahti.invalid' -Type TXT -Server $server -DnsOnly -QuickTimeout -ErrorAction Stop | Out-Null
        return $true
    } catch { return $false }
}

# 1. Vahdin pitää vastata ennen kuin asetuksiin kosketaan, muuten netti katkeaa.
if (-not (Test-Vahti '127.0.0.1')) {
    Write-Host 'Vahti ei vastaa osoitteessa 127.0.0.1.' -ForegroundColor Red
    Write-Host 'Käynnistä se ensin toisessa PowerShell-ikkunassa:  .\vahti.exe local'
    exit 1
}
$ipv6 = Test-Vahti '::1'

# 2. Fyysiset, käytössä olevat verkkosovittimet (ei VPN- tai virtuaalisovittimia).
$adapters = @(Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and -not $_.Virtual })
if ($adapters.Count -eq 0) {
    Write-Host 'Käytössä olevaa verkkosovitinta ei löytynyt.' -ForegroundColor Red
    exit 1
}

# 3. Talleta alkuperäiset asetukset vain kerran, jotta uusi ajo ei korvaa niitä.
if (-not (Test-Path $backupFile)) {
    $backup = foreach ($a in $adapters) {
        $g = $a.InterfaceGuid
        $s4 = (Get-ItemProperty "HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\$g" -Name NameServer -ErrorAction SilentlyContinue).NameServer
        $s6 = (Get-ItemProperty "HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip6\Parameters\Interfaces\$g" -Name NameServer -ErrorAction SilentlyContinue).NameServer
        [pscustomobject]@{ Index = $a.ifIndex; Name = $a.Name; Static4 = $s4; Static6 = $s6 }
    }
    New-Item -ItemType Directory -Force -Path (Split-Path $backupFile) | Out-Null
    ConvertTo-Json -InputObject @($backup) | Set-Content -Encoding UTF8 -Path $backupFile
}

# 4. Vahti ainoaksi nimipalvelimeksi. Varapalvelin ohittaisi suodatuksen.
$servers = @('127.0.0.1')
if ($ipv6) { $servers += '::1' }
foreach ($a in $adapters) {
    Set-DnsClientServerAddress -InterfaceIndex $a.ifIndex -ServerAddresses $servers
    Write-Host ("  {0}: DNS -> {1}" -f $a.Name, ($servers -join ', '))
}
Clear-DnsClientCache

Write-Host ''
Write-Host 'Valmis. Tämän koneen nimikyselyt kulkevat nyt Vahdin kautta.' -ForegroundColor Green
if (-not $ipv6) {
    Write-Host 'Huom: Vahti ei vastaa IPv6-osoitteessa ::1, joten IPv6-nimipalvelin voi ohittaa sen.' -ForegroundColor Yellow
}
Write-Host 'Pidä Vahti käynnissä. Ennen kuin suljet sen, aja:  .\palauta-dns.ps1'
Write-Host 'Selaimet: Chromen ja Edgen "Käytä suojattua DNS:ää" kannattaa kytkeä pois, jotta kyselyt eivät ohita Vahtia.'
