# Shared by install.ps1 and uninstall.ps1; apex-install-common.sh is the POSIX twin: keep flags,
# marks, the manifest and exit codes in step. Runs on Windows PowerShell 5.1 and PowerShell 7+.
# Exit codes: 0 ok, 1 ownership or prerequisite conflict, 2 missing go or claude, 64 usage error.

$ApexMark = '.apex-installed'
# Plugins read live from mods/ (the apex marketplace); the @apex id is the ownership mark.
$ApexMods = @('token-spend', 'usage-footer')

function Fail {
  param([int]$Code, [string]$Message)
  [Console]::Error.WriteLine("error: $Message")
  exit $Code
}
function Warn { param([string]$Message) [Console]::Error.WriteLine("warning: $Message") }

# Where things live: flag value, else environment, else default. Also points claude at the config dir.
function Resolve-ApexContext {
  param([string]$Prefix, [string]$ConfigDir)
  $h = if ($env:USERPROFILE) { $env:USERPROFILE } else { $env:HOME }
  if (-not $h) { Fail 64 'USERPROFILE and HOME are not set' }
  if (-not $Prefix) { $Prefix = Join-Path (Join-Path $h '.local') 'bin' }
  if (-not $ConfigDir) { $ConfigDir = if ($env:CLAUDE_CONFIG_DIR) { $env:CLAUDE_CONFIG_DIR } else { Join-Path $h '.claude' } }
  $xdg = if ($env:XDG_CONFIG_HOME) { $env:XDG_CONFIG_HOME } else { Join-Path $h '.config' }
  $env:CLAUDE_CONFIG_DIR = $ConfigDir  # claude plugin reads this, not -ConfigDir
  $win = [IO.Path]::DirectorySeparatorChar -eq '\'
  [pscustomobject]@{
    Home      = $h
    Prefix    = $Prefix
    ConfigDir = $ConfigDir
    Manifest  = Join-Path (Join-Path $xdg 'apex') 'manifest'
    BinName   = if ($win) { 'apex.exe' } else { 'apex' }
  }
}

# ---- workflows and harnesses ----

function Get-ApexWorkflowName {
  param([string]$Workflow)
  if ($Workflow -ceq 'orchestration') { 'Orchestration-based Workflow' }
  elseif ($Workflow -ceq 'multi-agent') { 'Multi-Agent Collaboration' }
  else { 'Both workflows' }
}
function Test-ApexWorkflow { param([string]$W) @('orchestration', 'multi-agent', 'both') -ccontains $W }
function Test-ApexHarness { param([string]$H) @('agents', 'claude', 'codex', 'custom') -ccontains $H }

function Get-ApexConfSkills {
  param([string]$Name, [string]$Conf)
  foreach ($line in [IO.File]::ReadAllLines($Conf)) {
    if ($line.StartsWith("${Name}:")) { return $line.Substring($Name.Length + 1).Split(' ', [StringSplitOptions]::RemoveEmptyEntries) }
  }
}

# "both" is the union with duplicates dropped, order kept.
function Get-ApexWorkflowSkills {
  param([string]$Workflow, [string]$Conf)
  $names = if ($Workflow -ceq 'both') { @('orchestration', 'multi-agent') } else { @($Workflow) }
  $out = New-Object System.Collections.ArrayList
  foreach ($n in $names) {
    foreach ($s in @(Get-ApexConfSkills $n $Conf)) { if (-not $out.Contains($s)) { [void]$out.Add($s) } }
  }
  $out.ToArray()
}

function Get-ApexHarnessDir {
  param($Ctx, [string]$Harness, [string]$SkillsDir)
  if ($Harness -ceq 'agents') { Join-Path (Join-Path $Ctx.Home '.agents') 'skills' }
  elseif ($Harness -ceq 'claude') { Join-Path $Ctx.ConfigDir 'skills' }
  elseif ($Harness -ceq 'codex') {
    $c = if ($env:CODEX_HOME) { $env:CODEX_HOME } else { Join-Path $Ctx.Home '.codex' }
    Join-Path $c 'skills'
  }
  else { $SkillsDir }
}

# Validates a comma list of harnesses; custom needs a skills directory.
function Assert-ApexHarnessList {
  param([string]$Harness, [string]$SkillsDir)
  if (-not $Harness) { Fail 64 '-Harness needs at least one of agents, claude, codex, custom' }
  foreach ($h in $Harness.Split(',')) {
    if (-not (Test-ApexHarness $h)) { Fail 64 "unknown harness '$h' (agents, claude, codex, custom)" }
    if (($h -ceq 'custom') -and -not $SkillsDir) { Fail 64 '-Harness custom needs -SkillsDir DIR' }
  }
}

# One {Harness, Dir} per distinct directory, order kept.
function Get-ApexTargets {
  param($Ctx, [string]$Harness, [string]$SkillsDir)
  $seen = @{}
  $out = New-Object System.Collections.ArrayList
  foreach ($h in $Harness.Split(',')) {
    $d = Get-ApexHarnessDir $Ctx $h $SkillsDir
    $k = $d.TrimEnd('\', '/')
    if (-not $seen.ContainsKey($k)) {
      $seen[$k] = $true
      [void]$out.Add([pscustomobject]@{ Harness = $h; Dir = $d })
    }
  }
  $out.ToArray()
}

# ---- manifest and previous install ----

# An install made before the manifest existed left only marked skills under the config dir.
function Test-ApexLegacy {
  param($Ctx)
  $sd = Join-Path $Ctx.ConfigDir 'skills'
  if (-not (Test-Path -LiteralPath $sd -PathType Container)) { return $false }
  foreach ($d in @(Get-ChildItem -LiteralPath $sd -Directory)) {
    if (Test-Path -LiteralPath (Join-Path $d.FullName $ApexMark)) { return $true }
  }
  $false
}

# The previous install: from the manifest, else from legacy marked skills (multi-agent on claude).
function Get-ApexPrev {
  param($Ctx)
  $p = [pscustomobject]@{ Workflow = ''; Harness = ''; Bin = ''; Config = ''; Rows = @() }
  $rows = New-Object System.Collections.ArrayList
  if (Test-Path -LiteralPath $Ctx.Manifest) {
    foreach ($line in [IO.File]::ReadAllLines($Ctx.Manifest)) {
      $f = $line.Split("`t")
      if ($f[0] -ceq 'workflow') { $p.Workflow = $f[1] }
      elseif ($f[0] -ceq 'bin') { $p.Bin = $f[1] }
      elseif ($f[0] -ceq 'config') { $p.Config = $f[1] }
      elseif (($f[0] -ceq 'skills') -and ($f.Count -ge 3)) { [void]$rows.Add([pscustomobject]@{ Harness = $f[1]; Dir = $f[2] }) }
    }
  } elseif (Test-ApexLegacy $Ctx) {
    $p.Workflow = 'multi-agent'
    [void]$rows.Add([pscustomobject]@{ Harness = 'claude'; Dir = (Join-Path $Ctx.ConfigDir 'skills') })
  }
  $p.Rows = $rows.ToArray()
  $p.Harness = (@($p.Rows | ForEach-Object { $_.Harness } | Select-Object -Unique) -join ',')
  $p
}

# ---- ownership marks ----

# Every marked skill, listed or not, so a skill dropped from skills/ leaves no stale copy behind.
function Remove-ApexMarked {
  param([string]$Dir)
  if (-not $Dir -or -not (Test-Path -LiteralPath $Dir -PathType Container)) { return }
  foreach ($d in @(Get-ChildItem -LiteralPath $Dir -Directory)) {
    if (Test-Path -LiteralPath (Join-Path $d.FullName $ApexMark)) { Remove-Item -LiteralPath $d.FullName -Recurse -Force }
  }
}

# The output style is one file, so its own frontmatter name is the ownership mark.
function Get-ApexStylePath { param($Ctx) Join-Path (Join-Path $Ctx.ConfigDir 'output-styles') 'apex.md' }
function Test-ApexStyle { param([string]$Path) [bool](Select-String -LiteralPath $Path -Pattern '^name: Apex\s*$' -Quiet) }
function Remove-ApexStyle {
  param($Ctx)
  $f = Get-ApexStylePath $Ctx
  if ((Test-Path -LiteralPath $f) -and (Test-ApexStyle $f)) { Remove-Item -LiteralPath $f -Force }
}

function Get-ApexPluginsJson { param($Ctx) Join-Path (Join-Path $Ctx.ConfigDir 'plugins') 'installed_plugins.json' }
function Test-ApexPlugin {
  param($Ctx, [string]$Id)
  $j = Get-ApexPluginsJson $Ctx
  (Test-Path -LiteralPath $j) -and [bool](Select-String -LiteralPath $j -Pattern "`"$Id`"" -SimpleMatch -Quiet)
}
# Another marketplace's copy of the mod.
function Test-ApexForeignMod {
  param($Ctx, [string]$Mod)
  $j = Get-ApexPluginsJson $Ctx
  (Test-Path -LiteralPath $j) -and [bool](Select-String -LiteralPath $j -Pattern "`"$Mod@(?!apex`")" -Quiet)
}

# Uninstalls the @apex mods present in installed_plugins.json, then the apex marketplace.
# Native stderr must not abort us, so errors are non-terminating around the claude calls.
function Remove-ApexMods {
  param($Ctx)
  $found = @($ApexMods | Where-Object { Test-ApexPlugin $Ctx "$_@apex" })
  if ($found.Count -eq 0) { return }
  if (-not (Get-Command claude -ErrorAction SilentlyContinue)) {
    Warn "claude is not on PATH; mods still installed: $($found -join ' ') (claude plugin uninstall <mod>@apex)"
    return
  }
  $ErrorActionPreference = 'Continue'
  foreach ($m in $found) {
    claude plugin uninstall "$m@apex"
    if ($LASTEXITCODE -ne 0) { Warn "claude plugin uninstall $m@apex failed" }
  }
  claude plugin marketplace remove apex
  if ($LASTEXITCODE -ne 0) { Warn 'claude plugin marketplace remove apex failed' }
  $ErrorActionPreference = 'Stop'
}
