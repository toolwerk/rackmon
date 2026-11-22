# Installiert den Rackmon-Agenten unter Windows als Dienst. In einer PowerShell als Administrator ausfuehren, wiederholbar.
#
#   .\windows.ps1 -Binary .\rackmon.exe -Server http://server:8470 -TokenFile .\token
#   Mit Temperatur aus LibreHardwareMonitor: zusaetzlich -TempUrl http://127.0.0.1:8085/data.json
#
# Der Dienst laeuft als LocalService. Das Token liegt in einer Datei, die nur Administratoren, System und dieser Dienst lesen.
param(
  [Parameter(Mandatory = $true)][string]$Binary,
  [Parameter(Mandatory = $true)][string]$Server,
  [Parameter(Mandatory = $true)][string]$TokenFile,
  [string]$TempUrl = ''   # optional: data.json von LibreHardwareMonitor, nur localhost, fuer die Prozessortemperatur
)
$ErrorActionPreference = 'Stop'
$name = 'RackmonAgent'
$dir = 'C:\Program Files\Rackmon'
$data = 'C:\ProgramData\Rackmon'
New-Item -ItemType Directory -Force -Path $dir, $data | Out-Null

# Feste SIDs statt Namen, die je nach Sprache verschieden heissen: Administratoren, System, LocalService.
$admin = '*S-1-5-32-544'; $system = '*S-1-5-18'; $svc = '*S-1-5-19'
$null = icacls.exe $dir /inheritance:r /grant "${admin}:(OI)(CI)F" /grant "${system}:(OI)(CI)F" /grant "${svc}:(OI)(CI)RX"
$null = icacls.exe $data /inheritance:r /grant "${admin}:(OI)(CI)F" /grant "${system}:(OI)(CI)F" /grant "${svc}:(OI)(CI)R"

if (Get-Service $name -ErrorAction SilentlyContinue) { Stop-Service $name -Force; Start-Sleep -Seconds 2 }
Copy-Item -Path $Binary -Destination "$dir\rackmon.exe" -Force
$tok = (Get-Content -Raw $TokenFile).Trim()
Set-Content -Path "$data\token" -Value $tok -NoNewline -Encoding ascii

$cmd = "`"$dir\rackmon.exe`" agent -server $Server -token-file `"$data\token`""
if ($TempUrl) { $cmd += " -temp-url $TempUrl" }
if (Get-Service $name -ErrorAction SilentlyContinue) {
  # sc.exe config mit Leerzeichen und Anfuehrungszeichen in PowerShell scheitert still, deshalb direkt in die Registrierung
  Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\$name" -Name ImagePath -Value $cmd
} else {
  $null = New-Service -Name $name -BinaryPathName $cmd -DisplayName 'Rackmon agent' -StartupType Automatic
}
$null = sc.exe config $name obj= 'NT AUTHORITY\LocalService'
$null = sc.exe failure $name reset= 86400 actions= restart/5000/restart/5000/restart/5000
Start-Service $name
Start-Sleep -Seconds 2
Get-Service $name
