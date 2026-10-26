$ErrorActionPreference = 'Stop'
# Simulates Windows network adapters and tests pointing DNS at Vahti and restoring it.
# Run:  pwsh -File deploy/windows/tests/VahtiDns.Tests.ps1
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ('vahti-test-' + [guid]::NewGuid())
$env:ProgramData = Join-Path $tmp 'pd'; $env:LOCALAPPDATA = Join-Path $tmp 'la'
Remove-Item -Recurse -Force $env:ProgramData -ErrorAction SilentlyContinue
# --- simulated Windows ---
$global:dns = @{ 7 = @('192.168.1.1'); 9 = @('1.1.1.1','8.8.8.8') }   # 7 = DHCP (wifi), 9 = static (ethernet)
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
Check (($global:dns[7] -join ',') -eq '127.0.0.1') 'wifi points to Vahti'
Check (($global:dns[9] -join ',') -eq '127.0.0.1') 'ethernet points to Vahti'
Check ($global:tasks.Count -eq 1) 'restore at startup scheduled'
Check (Test-Path (Join-Path $env:ProgramData 'Vahti/restore-at-boot.ps1')) 'restore script in protected folder'
Set-VahtiDns | Out-Null   # a second run must not overwrite the original backup
Restore-VahtiDns
Check (($global:dns[7] -join ',') -eq '192.168.1.1' -and -not $global:static[7]) 'wifi back to automatic'
Check (($global:dns[9] -join ',') -eq '1.1.1.1,8.8.8.8') 'ethernet static servers restored'
Check ($global:tasks.Count -eq 0) 'scheduled task removed after restore'
Check (-not (Test-Path (Join-Path $env:ProgramData 'Vahti/dns-backup.json'))) 'backup removed'

# Backup missing but DNS points to Vahti -> still fixed
$global:dns[7] = @('127.0.0.1'); Restore-VahtiDns
Check (($global:dns[7] -join ',') -eq '192.168.1.1') 'adapter pointing to Vahti is fixed without a backup'
Check (($global:dns[9] -join ',') -eq '1.1.1.1,8.8.8.8') 'other adapters are untouched'

# Vahti not running -> nothing is changed
$global:vahti = $false
try { Set-VahtiDns; 'FAIL' } catch { Check ($_.Exception.Message -match 'not answering') 'settings unchanged if Vahti does not answer' }
Check (($global:dns[7] -join ',') -eq '192.168.1.1') 'wifi untouched'

# Backup from an earlier (Finnish) version is still restored
$global:dns[7] = @('127.0.0.1')
New-Item -ItemType Directory -Force (Join-Path $env:ProgramData 'Vahti') | Out-Null
ConvertTo-Json -InputObject @([pscustomobject]@{Index=7;Guid='{G7}';Name='Wi-Fi';Static4='9.9.9.9';Static6=''}) | Set-Content (Join-Path $env:ProgramData 'Vahti/dns-varmuuskopio.json')
Restore-VahtiDns
Check (($global:dns[7] -join ',') -eq '9.9.9.9') 'legacy backup restored'
Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
