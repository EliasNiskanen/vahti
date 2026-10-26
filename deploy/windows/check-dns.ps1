# Shows this computer's DNS settings and whether they are normal.
# Needs no administrator rights and changes nothing.
#   powershell -ExecutionPolicy Bypass -File .\check-dns.ps1
Import-Module (Join-Path $PSScriptRoot 'VahtiDns.psm1') -Force

$pointsToVahti = $false
Write-Host 'DNS servers per network adapter:'
foreach ($a in Get-VahtiAdapters) {
    $addrs = @(Get-DnsClientServerAddress -InterfaceIndex $a.ifIndex -ErrorAction SilentlyContinue | ForEach-Object { $_.ServerAddresses })
    if (Test-PointsToVahti $addrs) { $pointsToVahti = $true }
    $shown = if ($addrs.Count) { $addrs -join ', ' } else { '(no DNS server)' }
    Write-Host ("  {0}: {1}" -f $a.Name, $shown)
}

$vahtiRunning = Test-VahtiDns
$backup = (Test-Path (Join-Path $env:ProgramData 'Vahti\dns-backup.json')) -or (Test-Path (Join-Path $env:ProgramData 'Vahti\dns-varmuuskopio.json'))
$task = [bool](Get-ScheduledTask -TaskName 'Vahti DNS restore' -ErrorAction SilentlyContinue)
$internet = $true
try { Resolve-DnsName -Name 'example.com' -Type A -DnsOnly -QuickTimeout -ErrorAction Stop | Out-Null } catch { $internet = $false }

function YesNo([bool]$v) { if ($v) { 'yes' } else { 'no' } }
Write-Host ''
Write-Host ("  Vahti running:            {0}" -f (YesNo $vahtiRunning))
Write-Host ("  DNS points to Vahti:      {0}" -f (YesNo $pointsToVahti))
Write-Host ("  DNS resolution works:     {0}" -f (YesNo $internet))
Write-Host ("  Backup saved:             {0}" -f (YesNo $backup))
Write-Host ("  Restore at startup:       {0}" -f $(if ($task) { 'scheduled' } else { 'no' }))
Write-Host ''

if (-not $pointsToVahti -and $internet) {
    Write-Host 'DNS settings are normal.' -ForegroundColor Green
} elseif ($pointsToVahti -and $vahtiRunning) {
    Write-Host 'This computer uses Vahti. The settings are restored when Vahti is closed.' -ForegroundColor Green
} elseif ($pointsToVahti -and -not $vahtiRunning) {
    Write-Host 'DNS points to Vahti, but Vahti is not running, so the internet does not work.' -ForegroundColor Red
    Write-Host 'Fix it as administrator:  powershell -ExecutionPolicy Bypass -File .\restore-dns.ps1'
} else {
    Write-Host 'The settings do not point to Vahti, but DNS is not answering. This is not caused by Vahti: check your network connection.' -ForegroundColor Yellow
}
