# Vahdin Windows-skriptien yhteiset toiminnot: DNS-asetusten varmuuskopio,
# ohjaus Vahdille ja palautus. Tiedot tallennetaan kansioon %ProgramData%\Vahti,
# jota vain järjestelmä ja järjestelmänvalvojat voivat muuttaa.

$script:Dir        = Join-Path $env:ProgramData 'Vahti'
$script:BackupFile = Join-Path $script:Dir 'dns-varmuuskopio.json'
$script:LogFile    = Join-Path $script:Dir 'dns.log'
$script:TaskName   = 'Vahti DNS-palautus'
$script:LegacyBackup = Join-Path $env:LOCALAPPDATA 'Vahti\dns-varmuuskopio.json'

function Write-VahtiLog([string]$Message) {
    try {
        if (Test-Path $script:Dir) {
            Add-Content -Path $script:LogFile -Encoding UTF8 -Value ('{0:yyyy-MM-dd HH:mm:ss} {1}' -f (Get-Date), $Message)
        }
    } catch { }
}

# Vastaako Vahti osoitteessa? Kysely ei lähde internetiin: Vahti vastaa siihen itse.
function Test-VahtiDns([string]$Server = '127.0.0.1') {
    try {
        Resolve-DnsName -Name 'selfcheck.vahti.invalid' -Type TXT -Server $Server -DnsOnly -QuickTimeout -ErrorAction Stop | Out-Null
        return $true
    } catch { return $false }
}

function Get-VahtiAdapters {
    return @(Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and -not $_.Virtual })
}

function Test-PointsToVahti($Addresses) {
    return [bool](@($Addresses) | Where-Object { $_ -eq '127.0.0.1' -or $_ -eq '::1' })
}

# Kansio, jonka sisältöä käynnistyksessä ajettava palautus käyttää: vain SYSTEM ja
# järjestelmänvalvojat voivat kirjoittaa, tavalliset käyttäjät voivat lukea.
function Protect-VahtiDir {
    New-Item -ItemType Directory -Force -Path $script:Dir | Out-Null
    & icacls.exe $script:Dir /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' '*S-1-5-32-545:(OI)(CI)RX' | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Kansion $($script:Dir) oikeuksia ei voitu asettaa." }
}

function Save-DnsBackup {
    if (Test-Path $script:BackupFile) { return }   # alkuperäinen varmuuskopio säilyy
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

function Register-RestoreTask {
    Copy-Item -Path (Join-Path $PSScriptRoot 'VahtiDns.psm1') -Destination $script:Dir -Force
    $bootScript = Join-Path $script:Dir 'palauta-kaynnistyksessa.ps1'
    $module = Join-Path $script:Dir 'VahtiDns.psm1'
    Set-Content -Path $bootScript -Encoding UTF8 -Value "Import-Module '$module' -Force`r`nRestore-VahtiDns -Reason 'koneen käynnistys'"
    $action    = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument "-NoProfile -NonInteractive -ExecutionPolicy Bypass -File `"$bootScript`""
    $trigger   = New-ScheduledTaskTrigger -AtStartup
    $principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
    Register-ScheduledTask -TaskName $script:TaskName -Action $action -Trigger $trigger -Principal $principal -Description 'Palauttaa DNS-asetukset, jos kone sammui Vahdin ollessa käynnissä.' -Force | Out-Null
}

function Unregister-RestoreTask {
    Unregister-ScheduledTask -TaskName $script:TaskName -Confirm:$false -ErrorAction SilentlyContinue
}

# Ohjaa koneen nimikyselyt Vahdille. Palauttaa $true, jos myös IPv6 (::1) on käytössä.
function Set-VahtiDns {
    if (-not (Test-VahtiDns '127.0.0.1')) { throw 'Vahti ei vastaa osoitteessa 127.0.0.1. Käynnistä Vahti ensin.' }
    $adapters = Get-VahtiAdapters
    if ($adapters.Count -eq 0) { throw 'Käytössä olevaa verkkosovitinta ei löytynyt.' }
    $v6 = Test-VahtiDns '::1'
    Protect-VahtiDir
    Save-DnsBackup
    Register-RestoreTask
    $servers = @('127.0.0.1')
    if ($v6) { $servers += '::1' }
    foreach ($a in $adapters) {
        Set-DnsClientServerAddress -InterfaceIndex $a.ifIndex -ServerAddresses $servers
        Write-VahtiLog ("DNS ohjattu Vahdille: {0} -> {1}" -f $a.Name, ($servers -join ', '))
    }
    Clear-DnsClientCache
    return $v6
}

# Palauttaa alkuperäiset asetukset varmuuskopiosta. Ilman varmuuskopiota
# vain Vahtiin osoittavat sovittimet asetetaan automaattisiksi.
function Restore-VahtiDns([string]$Reason = 'käyttäjä') {
    $file = $script:BackupFile
    if (-not (Test-Path $file) -and (Test-Path $script:LegacyBackup)) { $file = $script:LegacyBackup }
    if (Test-Path $file) {
        $backup = @(Get-Content -Raw -Path $file | ConvertFrom-Json)
        foreach ($b in $backup) {
            $a = Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue | Where-Object { $_.InterfaceGuid -eq $b.Guid } | Select-Object -First 1
            $idx = if ($a) { $a.ifIndex } else { $b.Index }
            Set-DnsClientServerAddress -InterfaceIndex $idx -ResetServerAddresses -ErrorAction Continue
            $list = @(@($b.Static4, $b.Static6) | ForEach-Object { if ($_) { $_ -split '[ ,;]+' } } |
                Where-Object { $_ -and $_ -ne '127.0.0.1' -and $_ -ne '::1' })
            if ($list.Count -gt 0) {
                Set-DnsClientServerAddress -InterfaceIndex $idx -ServerAddresses $list -ErrorAction Continue
                Write-VahtiLog ("DNS palautettu ({0}): {1} -> {2}" -f $Reason, $b.Name, ($list -join ', '))
            } else {
                Write-VahtiLog ("DNS palautettu ({0}): {1} -> automaattinen" -f $Reason, $b.Name)
            }
        }
        Remove-Item -Path $file -ErrorAction SilentlyContinue
    }
    foreach ($a in @(Get-NetAdapter -ErrorAction SilentlyContinue | Where-Object { $_.Status -eq 'Up' })) {
        $cur = @(Get-DnsClientServerAddress -InterfaceIndex $a.ifIndex -ErrorAction SilentlyContinue | ForEach-Object { $_.ServerAddresses })
        if (Test-PointsToVahti $cur) {
            Set-DnsClientServerAddress -InterfaceIndex $a.ifIndex -ResetServerAddresses -ErrorAction Continue
            Write-VahtiLog ("DNS palautettu automaattiseksi ({0}): {1}" -f $Reason, $a.Name)
        }
    }
    Clear-DnsClientCache
    Unregister-RestoreTask
}

# Käynnistää piilotetun vartijan, joka palauttaa asetukset, kun Vahti suljetaan.
function Start-VahtiGuard([int]$VahtiPid) {
    $guard = Join-Path $PSScriptRoot 'dns-vartija.ps1'
    $argList = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-WindowStyle', 'Hidden', '-File', "`"$guard`"", '-VahtiPid', $VahtiPid)
    Start-Process -FilePath 'powershell.exe' -Verb RunAs -WindowStyle Hidden -ArgumentList $argList
}

Export-ModuleMember -Function Test-VahtiDns, Get-VahtiAdapters, Test-PointsToVahti, Set-VahtiDns, Restore-VahtiDns, Start-VahtiGuard, Write-VahtiLog
