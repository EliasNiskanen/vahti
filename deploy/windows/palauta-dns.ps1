# Palauttaa taman tietokoneen alkuperaiset DNS-asetukset.
# Aja jarjestelmanvalvojana:
#   powershell -ExecutionPolicy Bypass -File .\palauta-dns.ps1
#Requires -RunAsAdministrator
$ErrorActionPreference = 'Continue'

$backupFile = Join-Path $env:LOCALAPPDATA 'Vahti\dns-varmuuskopio.json'

if (Test-Path $backupFile) {
    $backup = @(Get-Content -Raw -Path $backupFile | ConvertFrom-Json)
    foreach ($b in $backup) {
        # Ensin automaattiseksi (molemmat osoiteperheet), sitten mahdolliset kiinteät osoitteet takaisin.
        Set-DnsClientServerAddress -InterfaceIndex $b.Index -ResetServerAddresses
        $list = @()
        if ($b.Static4) { $list += @($b.Static4 -split '[ ,]+' | Where-Object { $_ }) }
        if ($b.Static6) { $list += @($b.Static6 -split '[ ,]+' | Where-Object { $_ }) }
        if ($list.Count -gt 0) {
            Set-DnsClientServerAddress -InterfaceIndex $b.Index -ServerAddresses $list
            Write-Host ("  {0}: DNS -> {1}" -f $b.Name, ($list -join ', '))
        } else {
            Write-Host ("  {0}: DNS -> automaattinen" -f $b.Name)
        }
    }
    Remove-Item -Path $backupFile
} else {
    # Varmuuskopiota ei ole: palautetaan kaikki käytössä olevat sovittimet automaattiseksi.
    foreach ($a in @(Get-NetAdapter | Where-Object { $_.Status -eq 'Up' })) {
        Set-DnsClientServerAddress -InterfaceIndex $a.ifIndex -ResetServerAddresses
        Write-Host ("  {0}: DNS -> automaattinen" -f $a.Name)
    }
}
Clear-DnsClientCache
Write-Host ''
Write-Host 'DNS-asetukset palautettu. Voit nyt sulkea Vahdin.' -ForegroundColor Green
