<#
.SYNOPSIS
  uninstall.ps1 -- remove what install.ps1 put on this machine.

.DESCRIPTION
  Windows counterpart to uninstall.sh. Only what apex owns goes: skill folders with the
  .apex-installed mark (in the manifest's directories, the agents, claude and codex presets,
  and -SkillsDir), an output style whose frontmatter is `name: Apex`, mods installed as
  <mod>@apex, and the binary and manifest. Safe to run twice. Runs on Windows PowerShell 5.1
  and PowerShell 7+.

.PARAMETER Prefix
  Where apex.exe was installed. Default: ~/.local/bin (the manifest's recorded binary goes too).

.PARAMETER ConfigDir
  Claude config root. Default: the manifest's, else $env:CLAUDE_CONFIG_DIR or ~/.claude.

.PARAMETER SkillsDir
  An extra skills directory to sweep.
#>
[CmdletBinding()]
param([string]$Prefix, [string]$ConfigDir, [string]$SkillsDir, [Alias('NonInteractive')][switch]$Yes)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'apex-install-common.ps1')

$ctx = Resolve-ApexContext -Prefix $Prefix -ConfigDir $ConfigDir
$prev = Get-ApexPrev $ctx
# Config dir: flag, else the one the install recorded, else the environment.
if (-not $ConfigDir -and $prev.Config) {
  $ctx = Resolve-ApexContext -Prefix $Prefix -ConfigDir $prev.Config
  $prev = Get-ApexPrev $ctx
}

# Every directory a skill could be in: the manifest's, the preset harnesses', and -SkillsDir.
$dirs = @($prev.Rows | ForEach-Object { $_.Dir })
foreach ($h in 'agents', 'claude', 'codex') { $dirs += Get-ApexHarnessDir $ctx $h '' }
if ($SkillsDir) { $dirs += $SkillsDir }
foreach ($d in $dirs) { Remove-ApexMarked $d }

if ($prev.Bin) { Remove-Item -LiteralPath $prev.Bin -Force -ErrorAction SilentlyContinue }
foreach ($n in 'apex.exe', 'apex') { Remove-Item -LiteralPath (Join-Path $ctx.Prefix $n) -Force -ErrorAction SilentlyContinue }
Remove-ApexStyle $ctx
Remove-ApexMods $ctx
Remove-Item -LiteralPath $ctx.Manifest -Force -ErrorAction SilentlyContinue
$mdir = Split-Path $ctx.Manifest -Parent
if ((Test-Path -LiteralPath $mdir) -and (@(Get-ChildItem -LiteralPath $mdir -Force).Count -eq 0)) { Remove-Item -LiteralPath $mdir -Force }
Write-Host '[ok] apex removed. Restart your harness.' -ForegroundColor Green
exit 0
