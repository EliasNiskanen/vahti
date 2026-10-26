$ErrorActionPreference = 'Stop'
# Simuloi Windowsin verkkosovittimet ja testaa DNS-asetusten ohjauksen ja palautuksen.
# Aja:  pwsh -File deploy/windows/tests/VahtiDns.Tests.ps1
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ('vahti-test-' + [guid]::NewGuid())
$env:ProgramData = Join-Path $tmp 'pd'; $env:LOCALAPPDATA = Join-Path $tmp 'la'
Remove-Item -Recurse -Force $env:ProgramData -ErrorAction SilentlyContinue
# --- simuloitu Windows ---
$global:dns = @{ 7 = @('192.168.1.1'); 9 = @('1.1.1.1','8.8.8.8') }   # 7 = DHCP (wifi), 9 = kiinteä (ethernet)
$global:static = @{ 7 = ''; 9 = '1.1.1.1,8.8.8.8' }
$global:vahti = $true; $global:tasks = @{}
function global:Get-NetAdapter { param([switch]$IncludeHidden)
  @([pscustomobject]@{Name='Wi-Fi';ifIndex=7;InterfaceGuid='{G7}';Status='Up';Virtual=$false},
    [pscustomobject]@{Name='Ethernet';ifIndex=9;InterfaceGuid='{G9}';Status='Up';Virtual=$false},
    [pscustomobject]@{Name='vEthernet';ifIndex=11;InterfaceGuid='{G11}';Status='Up';Virtual=$true}) }
function global:Get-ItemProperty { param($Path,$Name,$ErrorAction)
  $i = if ($Path -match 'G7') {7} else {9}
  if ($Path -match 'Tcpip6') { return $null }
  if ($global:static[$i]) { [pscustomobject]@{NameServer=$global:static[$i]} } else { [pscustomobject]@{NameServer=''} } }
function global:Set-DnsClientServerAddress { param($InterfaceIndex,$ServerAddresses,[switch]$ResetServerAddresses,$ErrorAction)
  if ($ResetServerAddresses) { $global:dns[$InterfaceIndex] = @('192.168.1.1'); $global:static[$InterfaceIndex]='' }
  else { $global:dns[$InterfaceIndex] = @($ServerAddresses); $global:static[$InterfaceIndex] = ($ServerAddresses -join ',') } }
function global:Get-DnsClientServerAddress { param($InterfaceIndex,$ErrorAction) [pscustomobject]@{ServerAddresses=$global:dns[$InterfaceIndex]} }
function global:Clear-DnsClientCache {}
function global:Resolve-DnsName { param($Name,$Type,$Server,[switch]$DnsOnly,[switch]$QuickTimeout,$ErrorAction)
  if ($Name -eq 'selfcheck.vahti.invalid' -and -not ($global:vahti -and $Server -eq '127.0.0.1')) { throw 'no' } }
function global:icacls.exe { $global:LASTEXITCODE = 0 }
function global:New-ScheduledTaskAction { param($Execute,$Argument) $Argument }
function global:New-ScheduledTaskTrigger { param([switch]$AtStartup) }
function global:New-ScheduledTaskPrincipal { param($UserId,$LogonType,$RunLevel) }
function global:Register-ScheduledTask { param($TaskName,$Action,$Trigger,$Principal,$Description,[switch]$Force) $global:tasks[$TaskName]=$Action }
function global:Unregister-ScheduledTask { param($TaskName,$Confirm,$ErrorAction) $global:tasks.Remove($TaskName) }
Import-Module (Join-Path $PSScriptRoot '../VahtiDns.psm1') -Force
function Check($c,$m){ if(-not $c){ throw "FAIL: $m" } else { "ok: $m" } }

$v6 = Set-VahtiDns
Check (($global:dns[7] -join ',') -eq '127.0.0.1') 'wifi ohjattu Vahdille'
Check (($global:dns[9] -join ',') -eq '127.0.0.1') 'ethernet ohjattu Vahdille'
Check ($global:tasks.Count -eq 1) 'käynnistyspalautus ajastettu'
Check (Test-Path (Join-Path $env:ProgramData 'Vahti/palauta-kaynnistyksessa.ps1')) 'palautusskripti suojattuun kansioon'
Set-VahtiDns | Out-Null   # toinen ajo ei saa korvata alkuperäistä varmuuskopiota
Restore-VahtiDns
Check (($global:dns[7] -join ',') -eq '192.168.1.1' -and -not $global:static[7]) 'wifi takaisin automaattiseksi'
Check (($global:dns[9] -join ',') -eq '1.1.1.1,8.8.8.8') 'ethernetin kiinteät osoitteet palautettu'
Check ($global:tasks.Count -eq 0) 'ajastus poistettu palautuksen jälkeen'
Check (-not (Test-Path (Join-Path $env:ProgramData 'Vahti/dns-varmuuskopio.json'))) 'varmuuskopio poistettu'

# Varmuuskopio kadonnut, mutta DNS osoittaa Vahtiin -> korjataan silti
$global:dns[7] = @('127.0.0.1'); Restore-VahtiDns
Check (($global:dns[7] -join ',') -eq '192.168.1.1') 'ilman varmuuskopiota Vahtiin osoittava sovitin korjataan'
Check (($global:dns[9] -join ',') -eq '1.1.1.1,8.8.8.8') 'muihin sovittimiin ei kosketa'

# Vahti ei käynnissä -> ei muuteta mitään
$global:vahti = $false
try { Set-VahtiDns; 'FAIL' } catch { Check ($_.Exception.Message -match 'ei vastaa') 'asetuksia ei muuteta, jos Vahti ei vastaa' }
Check (($global:dns[7] -join ',') -eq '192.168.1.1') 'wifi koskematon'
Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
