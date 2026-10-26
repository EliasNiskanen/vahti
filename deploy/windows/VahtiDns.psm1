# Shared functions for Vahti's Windows scripts: backing up the DNS settings,
# pointing them at Vahti and restoring them. Files are kept in %ProgramData%\Vahti,
# which only SYSTEM and administrators can change.

$script:Dir        = Join-Path $env:ProgramData 'Vahti'
$script:BackupFile = Join-Path $script:Dir 'dns-backup.json'
$script:LogFile    = Join-Path $script:Dir 'dns.log'
$script:TaskName   = 'Vahti DNS restore'
# Names used by earlier (Finnish) versions, still recognised when restoring.
$script:LegacyBackups = @(
    (Join-Path $script:Dir 'dns-varmuuskopio.json'),
    (Join-Path $env:LOCALAPPDATA 'Vahti\dns-varmuuskopio.json')
)
$script:LegacyTaskName = 'Vahti DNS-palautus'

# Appends a timestamped line to Vahti's DNS log file. Logging errors are ignored.
function Write-VahtiLog([string]$Message) {
    try {
        if (Test-Path $script:Dir) {
            Add-Content -Path $script:LogFile -Encoding UTF8 -Value ('{0:yyyy-MM-dd HH:mm:ss} {1}' -f (Get-Date), $Message)
        }
    } catch { }
}

# Does Vahti answer at this address? The query never leaves the computer: Vahti answers it itself.
function Test-VahtiDns([string]$Server = '127.0.0.1') {
    try {
        Resolve-DnsName -Name 'selfcheck.vahti.invalid' -Type TXT -Server $Server -DnsOnly -QuickTimeout -ErrorAction Stop | Out-Null
        return $true
    } catch { return $false }
}

# Returns the active, physical network adapters (Wi-Fi, Ethernet). Virtual adapters such as
# Hyper-V and VPN adapters are left alone.
function Get-VahtiAdapters {
    return @(Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and -not $_.Virtual })
}

# Returns $true if the list of DNS servers includes Vahti (127.0.0.1 or ::1).
function Test-PointsToVahti($Addresses) {
    return [bool](@($Addresses) | Where-Object { $_ -eq '127.0.0.1' -or $_ -eq '::1' })
}

# The restore that runs at boot trusts this folder, so only SYSTEM and
# administrators may write to it; normal users may read it.
function Protect-VahtiDir {
    New-Item -ItemType Directory -Force -Path $script:Dir | Out-Null
    & icacls.exe $script:Dir /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' '*S-1-5-32-545:(OI)(CI)RX' | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Could not set permissions on $($script:Dir)." }
}

# Saves each adapter's current DNS settings (automatic or static servers) before Vahti
# changes them. An existing backup is kept, so the original settings are never overwritten.
function Save-DnsBackup {
    if (Test-Path $script:BackupFile) { return }   # keep the original backup
    $backup = foreach ($a in Get-VahtiAdapters) {
        $g = $a.InterfaceGuid
        $p4 = Get-ItemProperty "HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\$g" -Name NameServer -ErrorAction SilentlyContinue
        $p6 = Get-ItemProperty "HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip6\Parameters\Interfaces\$g" -Name NameServer -ErrorAction SilentlyContinue
        [pscustomobject]@{
            Index   = $a.ifIndex
            Guid    = $g
            Name    = $a.Name
            Static4 = if ($p4) { $p4.NameServer } else { '' }
            Static6 = if ($p6) { $p6.NameServer } else { '' }
        }
    }
    ConvertTo-Json -InputObject @($backup) | Set-Content -Path $script:BackupFile -Encoding UTF8
}

# Creates a scheduled task that runs at startup as SYSTEM and restores the DNS settings,
# in case the computer was shut down or crashed while Vahti was running.
function Register-RestoreTask {
    Copy-Item -Path (Join-Path $PSScriptRoot 'VahtiDns.psm1') -Destination $script:Dir -Force
    $bootScript = Join-Path $script:Dir 'restore-at-boot.ps1'
    $module = Join-Path $script:Dir 'VahtiDns.psm1'
    Set-Content -Path $bootScript -Encoding UTF8 -Value "Import-Module '$module' -Force`r`nRestore-VahtiDns -Reason 'computer start'"
    $action    = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument "-NoProfile -NonInteractive -ExecutionPolicy Bypass -File `"$bootScript`""
    $trigger   = New-ScheduledTaskTrigger -AtStartup
    $principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
    Register-ScheduledTask -TaskName $script:TaskName -Action $action -Trigger $trigger -Principal $principal -Description 'Restores DNS settings if the computer shut down while Vahti was running.' -Force | Out-Null
}

# Removes the startup restore task (also the one created by older versions).
function Unregister-RestoreTask {
    foreach ($name in @($script:TaskName, $script:LegacyTaskName)) {
        Unregister-ScheduledTask -TaskName $name -Confirm:$false -ErrorAction SilentlyContinue
    }
}

# Points this computer's DNS at Vahti. Returns $true if IPv6 (::1) is used too.
function Set-VahtiDns {
    if (-not (Test-VahtiDns '127.0.0.1')) { throw 'Vahti is not answering at 127.0.0.1. Start Vahti first.' }
    $adapters = Get-VahtiAdapters
    if ($adapters.Count -eq 0) { throw 'No active network adapter was found.' }
    $v6 = Test-VahtiDns '::1'
    Protect-VahtiDir
    Save-DnsBackup
    Register-RestoreTask
    $servers = @('127.0.0.1')
    if ($v6) { $servers += '::1' }
    foreach ($a in $adapters) {
        Set-DnsClientServerAddress -InterfaceIndex $a.ifIndex -ServerAddresses $servers
        Write-VahtiLog ("DNS pointed at Vahti: {0} -> {1}" -f $a.Name, ($servers -join ', '))
    }
    Clear-DnsClientCache
    return $v6
}

# Restores the original settings from the backup. Without a backup, only
# adapters that point at Vahti are set back to automatic.
function Restore-VahtiDns([string]$Reason = 'user') {
    $file = $script:BackupFile
    if (-not (Test-Path $file)) {
        $file = $script:LegacyBackups | Where-Object { Test-Path $_ } | Select-Object -First 1
    }
    if ($file -and (Test-Path $file)) {
        $backup = @(Get-Content -Raw -Path $file | ConvertFrom-Json)
        foreach ($b in $backup) {
            $a = Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue | Where-Object { $_.InterfaceGuid -eq $b.Guid } | Select-Object -First 1
            $idx = if ($a) { $a.ifIndex } else { $b.Index }
            Set-DnsClientServerAddress -InterfaceIndex $idx -ResetServerAddresses -ErrorAction Continue
            $list = @(@($b.Static4, $b.Static6) | ForEach-Object { if ($_) { $_ -split '[ ,;]+' } } |
                Where-Object { $_ -and $_ -ne '127.0.0.1' -and $_ -ne '::1' })
            if ($list.Count -gt 0) {
                Set-DnsClientServerAddress -InterfaceIndex $idx -ServerAddresses $list -ErrorAction Continue
                Write-VahtiLog ("DNS restored ({0}): {1} -> {2}" -f $Reason, $b.Name, ($list -join ', '))
            } else {
                Write-VahtiLog ("DNS restored ({0}): {1} -> automatic" -f $Reason, $b.Name)
            }
        }
        Remove-Item -Path $file -ErrorAction SilentlyContinue
    }
    foreach ($a in @(Get-NetAdapter -ErrorAction SilentlyContinue | Where-Object { $_.Status -eq 'Up' })) {
        $cur = @(Get-DnsClientServerAddress -InterfaceIndex $a.ifIndex -ErrorAction SilentlyContinue | ForEach-Object { $_.ServerAddresses })
        if (Test-PointsToVahti $cur) {
            Set-DnsClientServerAddress -InterfaceIndex $a.ifIndex -ResetServerAddresses -ErrorAction Continue
            Write-VahtiLog ("DNS set to automatic ({0}): {1}" -f $Reason, $a.Name)
        }
    }
    Clear-DnsClientCache
    Unregister-RestoreTask
}

# Starts the hidden guard that restores the settings when Vahti is closed.
function Start-VahtiGuard([int]$VahtiPid) {
    $guard = Join-Path $PSScriptRoot 'dns-guard.ps1'
    $argList = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-WindowStyle', 'Hidden', '-File', "`"$guard`"", '-VahtiPid', $VahtiPid)
    Start-Process -FilePath 'powershell.exe' -Verb RunAs -WindowStyle Hidden -ArgumentList $argList
}

Export-ModuleMember -Function Test-VahtiDns, Get-VahtiAdapters, Test-PointsToVahti, Set-VahtiDns, Restore-VahtiDns, Start-VahtiGuard, Write-VahtiLog
