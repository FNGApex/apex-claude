# Shared by install.ps1 and uninstall.ps1. Keep SKILLS and the marks in step with the Makefile.

function Fail { param($m) Write-Host "error: $m" -ForegroundColor Red; exit 1 }

function Get-ApexInstallConfig {
  param([string]$Prefix, [string]$ConfigDir)
  if (-not $Prefix) { $Prefix = Join-Path $env:USERPROFILE '.local\bin' }
  if (-not $ConfigDir) { $ConfigDir = if ($env:CLAUDE_CONFIG_DIR) { $env:CLAUDE_CONFIG_DIR } else { Join-Path $env:USERPROFILE '.claude' } }
  [pscustomobject]@{
    Prefix    = $Prefix
    ConfigDir = $ConfigDir
    SkillsDir = Join-Path $ConfigDir 'skills'
    Style     = Join-Path $ConfigDir 'output-styles\apex.md'
    LegacyBin = Join-Path $ConfigDir 'bin\apex.exe'
    Mark      = '.apex-installed'
    Skills    = @('agent-protocol', 'git-discipline', 'sign-off', 'apex-init', 'pr-review')
    # Plugins read live from mods/ (the apex marketplace); the @apex id is the ownership mark.
    Mods      = @('token-spend', 'usage-footer')
  }
}

# The output style is one file, so its own frontmatter name is the ownership mark.
function Test-ApexStyle { param($Path) [bool](Select-String -Path $Path -Pattern '^name: Apex$' -Quiet) }

# Every marked skill, listed or not, so a skill dropped from skills/ leaves no stale copy behind.
function Remove-ApexSkills {
  param($Cfg)
  if (-not (Test-Path $Cfg.SkillsDir)) { return }
  Get-ChildItem $Cfg.SkillsDir -Directory | Where-Object { Test-Path (Join-Path $_.FullName $Cfg.Mark) } |
    ForEach-Object { Remove-Item $_.FullName -Recurse -Force }
}
