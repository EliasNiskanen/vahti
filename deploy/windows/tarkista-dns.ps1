# Näyttää tämän koneen DNS-asetukset ja kertoo, ovatko ne normaalit.
# Ei vaadi järjestelmänvalvojan oikeuksia eikä muuta mitään.
#   powershell -ExecutionPolicy Bypass -File .\tarkista-dns.ps1
Import-Module (Join-Path $PSScriptRoot 'VahtiDns.psm1') -Force

$pointsToVahti = $false
Write-Host 'Verkkosovittimien nimipalvelimet:'
foreach ($a in Get-VahtiAdapters) {
    $addrs = @(Get-DnsClientServerAddress -InterfaceIndex $a.ifIndex -ErrorAction SilentlyContinue | ForEach-Object { $_.ServerAddresses })
    if (Test-PointsToVahti $addrs) { $pointsToVahti = $true }
    $shown = if ($addrs.Count) { $addrs -join ', ' } else { '(ei nimipalvelinta)' }
    Write-Host ("  {0}: {1}" -f $a.Name, $shown)
}

$vahtiRunning = Test-VahtiDns
$backup = Test-Path (Join-Path $env:ProgramData 'Vahti\dns-varmuuskopio.json')
$task = [bool](Get-ScheduledTask -TaskName 'Vahti DNS-palautus' -ErrorAction SilentlyContinue)
$internet = $true
try { Resolve-DnsName -Name 'example.com' -Type A -DnsOnly -QuickTimeout -ErrorAction Stop | Out-Null } catch { $internet = $false }

Write-Host ''
Write-Host ("  Vahti käynnissä:          {0}" -f $(if ($vahtiRunning) { 'kyllä' } else { 'ei' }))
Write-Host ("  DNS ohjattu Vahdille:     {0}" -f $(if ($pointsToVahti) { 'kyllä' } else { 'ei' }))
Write-Host ("  Nimipalvelu toimii:       {0}" -f $(if ($internet) { 'kyllä' } else { 'ei' }))
Write-Host ("  Varmuuskopio tallessa:    {0}" -f $(if ($backup) { 'kyllä' } else { 'ei' }))
Write-Host ("  Palautus käynnistyksessä: {0}" -f $(if ($task) { 'ajastettu' } else { 'ei' }))
Write-Host ''

if (-not $pointsToVahti -and $internet) {
    Write-Host 'DNS-asetukset ovat normaalit.' -ForegroundColor Green
} elseif ($pointsToVahti -and $vahtiRunning) {
    Write-Host 'Kone käyttää Vahtia. Asetukset palautuvat, kun Vahti suljetaan.' -ForegroundColor Green
} elseif ($pointsToVahti -and -not $vahtiRunning) {
    Write-Host 'DNS osoittaa Vahtiin, mutta Vahti ei ole käynnissä, joten netti ei toimi.' -ForegroundColor Red
    Write-Host 'Korjaa järjestelmänvalvojana:  powershell -ExecutionPolicy Bypass -File .\palauta-dns.ps1'
} else {
    Write-Host 'Asetukset eivät osoita Vahtiin, mutta nimipalvelu ei vastaa. Vika ei liity Vahtiin: tarkista verkkoyhteys.' -ForegroundColor Yellow
}
