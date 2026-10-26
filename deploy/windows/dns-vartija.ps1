# Vahdin DNS-vartija. Käynnistyy piilotettuna järjestelmänvalvojana
# (kaynnista-vahti.ps1 tai kayta-vahtia.ps1 käynnistää sen).
# Ohjaa DNS:n Vahdille ja palauttaa sen heti, kun Vahti suljetaan, kaatuu
# tai lakkaa vastaamasta. Tapahtumat: %ProgramData%\Vahti\dns.log
#Requires -RunAsAdministrator
param([Parameter(Mandatory = $true)][int]$VahtiPid)

Import-Module (Join-Path $PSScriptRoot 'VahtiDns.psm1') -Force

$proc = Get-Process -Id $VahtiPid -ErrorAction SilentlyContinue
if (-not $proc) { Write-VahtiLog "Vartija: Vahti-prosessia $VahtiPid ei löytynyt."; exit 1 }

try {
    Set-VahtiDns | Out-Null
    Write-VahtiLog "Vartija käynnissä (Vahti PID $VahtiPid)."
    $fails = 0
    while ($true) {
        if ($proc.WaitForExit(5000)) { Write-VahtiLog 'Vartija: Vahti suljettiin.'; break }
        if (Test-VahtiDns) { $fails = 0 }
        else {
            $fails++
            if ($fails -ge 3) { Write-VahtiLog 'Vartija: Vahti ei vastannut 15 sekuntiin.'; break }
        }
    }
} catch {
    Write-VahtiLog "Vartija: virhe: $($_.Exception.Message)"
} finally {
    Restore-VahtiDns -Reason 'Vahti suljettiin'
}
