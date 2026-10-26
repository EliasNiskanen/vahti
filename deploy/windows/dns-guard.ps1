# Vahti's DNS guard. Runs hidden as administrator (started by start-vahti.ps1
# or use-vahti.ps1). Points DNS at Vahti and restores it as soon as Vahti is
# closed, crashes or stops answering. Events: %ProgramData%\Vahti\dns.log
#Requires -RunAsAdministrator
param([Parameter(Mandatory = $true)][int]$VahtiPid)

Import-Module (Join-Path $PSScriptRoot 'VahtiDns.psm1') -Force

$proc = Get-Process -Id $VahtiPid -ErrorAction SilentlyContinue
if (-not $proc) { Write-VahtiLog "Guard: Vahti process $VahtiPid not found."; exit 1 }

try {
    Set-VahtiDns | Out-Null
    Write-VahtiLog "Guard running (Vahti PID $VahtiPid)."
    $fails = 0
    while ($true) {
        if ($proc.WaitForExit(5000)) { Write-VahtiLog 'Guard: Vahti was closed.'; break }
        if (Test-VahtiDns) { $fails = 0 }
        else {
            $fails++
            if ($fails -ge 3) { Write-VahtiLog 'Guard: Vahti did not answer for 15 seconds.'; break }
        }
    }
} catch {
    Write-VahtiLog "Guard: error: $($_.Exception.Message)"
} finally {
    Restore-VahtiDns -Reason 'Vahti closed'
}
