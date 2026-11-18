<#
.SYNOPSIS
  uninstall.ps1 -- Windows counterpart to `make uninstall`.

.DESCRIPTION
  Removes apex.exe from -Prefix, and the Apex skills and output style from -ConfigDir -- only
  those apex installed (skill folders carrying the .apex-installed mark; an output style whose
  frontmatter is `name: Apex`), and the mods
  installed as <mod>@apex with the apex marketplace. Runs on Windows PowerShell 5.1 and PowerShell 7+.
#>
[CmdletBinding()]
param([string]$Prefix, [string]$ConfigDir)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'apex-install-common.ps1')

$cfg = Get-ApexInstallConfig -Prefix $Prefix -ConfigDir $ConfigDir

Remove-Item (Join-Path $cfg.Prefix 'apex.exe') -Force -ErrorAction SilentlyContinue
if ((Test-Path $cfg.Style) -and (Test-ApexStyle $cfg.Style)) { Remove-Item $cfg.Style -Force }
Remove-ApexSkills $cfg
foreach ($m in $cfg.Mods) { & claude plugin uninstall "$m@apex" }
& claude plugin marketplace remove apex
Write-Host '[ok] apex removed. Restart Claude Code.' -ForegroundColor Green
