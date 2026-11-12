#Requires -RunAsAdministrator
param(
  [string]$Listen = '127.0.0.1:8080',
  [string]$Hosts = 'localhost'
)
$ErrorActionPreference = 'Stop'
$erpSource = Join-Path $PSScriptRoot 'erp.exe'
$erpInstallDir = Join-Path $env:ProgramFiles 'RaizERP'
$erpStateRoot = Join-Path $env:ProgramData 'RaizERP'
$erpDataDir = Join-Path $erpStateRoot 'data'
if (-not (Test-Path $erpSource)) { throw 'No se encontró erp.exe junto al instalador.' }
$erpService = Get-Service -Name 'RaizERP' -ErrorAction SilentlyContinue
if ($erpService -and $erpService.Status -ne 'Stopped') { throw 'Detén RaizERP antes de actualizar el ejecutable.' }
New-Item -ItemType Directory -Force -Path $erpInstallDir, $erpStateRoot | Out-Null
Copy-Item $erpSource (Join-Path $erpInstallDir 'erp.exe') -Force
if (-not (Test-Path (Join-Path $erpDataDir 'config.json'))) {
  & (Join-Path $erpInstallDir 'erp.exe') init --data-dir $erpDataDir --listen $Listen --hosts $Hosts
  if ($LASTEXITCODE -ne 0) { throw 'No fue posible inicializar el ERP.' }
}
# Replace inherited broad permissions; retain Administrators, SYSTEM and LocalService.
& icacls $erpStateRoot /inheritance:r /grant:r '*S-1-5-32-544:(OI)(CI)F' '*S-1-5-18:(OI)(CI)F' '*S-1-5-19:(OI)(CI)M' /T | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'No fue posible restringir los permisos de los datos.' }
$erpCommand = '"' + (Join-Path $erpInstallDir 'erp.exe') + '" service --data-dir "' + $erpDataDir + '"'
if (-not $erpService) {
  & sc.exe create RaizERP binPath= $erpCommand start= auto obj= 'NT AUTHORITY\LocalService' DisplayName= 'Raíz ERP local'
  if ($LASTEXITCODE -ne 0) { throw 'No fue posible registrar el servicio.' }
}
& sc.exe failure RaizERP reset= 86400 actions= restart/5000/restart/10000/restart/30000 | Out-Null
Start-Service RaizERP
Write-Host "Servicio instalado. Obtén el código inicial desde $erpDataDir\setup.token y abre la dirección local configurada."
