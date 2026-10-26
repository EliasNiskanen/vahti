# Käynnistää Vahdin ja ohjaa tämän koneen nimikyselyt sille.
# DNS-asetukset palautetaan automaattisesti, kun Vahdin ikkuna suljetaan.
#
#   powershell -ExecutionPolicy Bypass -File .\kaynnista-vahti.ps1
#   powershell -ExecutionPolicy Bypass -File .\kaynnista-vahti.ps1 -Router 192.168.1.1
param([string]$Router = '')
$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'VahtiDns.psm1') -Force

$exe = Join-Path $PSScriptRoot 'vahti.exe'
if (-not (Test-Path $exe)) { Write-Host "vahti.exe puuttuu kansiosta $PSScriptRoot" -ForegroundColor Red; exit 1 }
if (Test-VahtiDns) { Write-Host 'Vahti on jo käynnissä.' -ForegroundColor Yellow; exit 0 }

$vahtiArgs = @('local')
if ($Router) { $vahtiArgs += @('-router', $Router) }
$p = Start-Process -FilePath $exe -ArgumentList $vahtiArgs -WorkingDirectory $PSScriptRoot -PassThru

Write-Host 'Käynnistetään Vahtia...'
$ok = $false
for ($i = 0; $i -lt 30; $i++) {
    if ($p.HasExited) { break }
    if (Test-VahtiDns) { $ok = $true; break }
    Start-Sleep -Milliseconds 500
}
if (-not $ok) {
    Write-Host 'Vahti ei käynnistynyt. Katso virhe Vahdin ikkunasta.' -ForegroundColor Red
    exit 1
}

Write-Host 'Windows kysyy seuraavaksi lupaa DNS-asetusten muuttamiseen.'
try {
    Start-VahtiGuard -VahtiPid $p.Id
} catch {
    Write-Host 'Lupaa ei annettu, joten DNS-asetuksia ei muutettu.' -ForegroundColor Yellow
    Write-Host 'Vahti on käynnissä, mutta se ei suodata tämän koneen liikennettä.'
    exit 1
}

Start-Sleep -Seconds 2
Write-Host ''
Write-Host 'Vahti on käynnissä ja suodattaa tämän koneen liikennettä.' -ForegroundColor Green
Write-Host '  Hallinta:  http://127.0.0.1:8080'
Write-Host '  Lopetus:   sulje Vahdin ikkuna tai paina siinä Ctrl+C.'
Write-Host '             DNS-asetukset palautetaan automaattisesti.'
Write-Host '  Tarkistus: powershell -ExecutionPolicy Bypass -File .\tarkista-dns.ps1'
