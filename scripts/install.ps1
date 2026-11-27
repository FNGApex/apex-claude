<#
.SYNOPSIS
  install.ps1 -- install apex and its workflow skills for one or more harnesses.

.DESCRIPTION
  Windows counterpart to install.sh; same flags, prompts, manifest and exit codes. Runs on
  Windows PowerShell 5.1 and PowerShell 7+. Undo with scripts/uninstall.ps1.

  Without -Workflow and -Harness an interactive terminal is asked; otherwise the previous
  install's choices are kept, else orchestration on agents.
  Exit: 0 ok, 1 ownership or prerequisite conflict, 2 missing go or claude, 64 usage.

.PARAMETER Workflow
  orchestration (default; "Orchestration-based Workflow"), multi-agent ("Multi-Agent
  Collaboration"), or both.

.PARAMETER Harness
  agents (default; ~/.agents/skills), claude (<ConfigDir>/skills), codex ($env:CODEX_HOME or
  ~/.codex, then skills), gemini ($env:GEMINI_CONFIG_DIR or ~/.gemini/config, then skills),
  t3 (T3 Code: the agents, claude, codex and gemini skill roots), custom (-SkillsDir).
  A comma list installs into several.

.PARAMETER SkillsDir
  Skills directory for the custom harness; implies custom.

.PARAMETER Prefix
  Where apex.exe goes. Default: ~/.local/bin. Must be on PATH.

.PARAMETER ConfigDir
  Claude config root. Default: $env:CLAUDE_CONFIG_DIR or ~/.claude.

.PARAMETER WithOutputStyle
  Install the Apex output style; -WithOutputStyle:$false removes an Apex-owned one.

.PARAMETER WithClaudeMods
  Install the token-spend and usage-footer mods; -WithClaudeMods:$false uninstalls the @apex ones.

.PARAMETER CheckPonytail
  Require the ponytail plugin.

.PARAMETER Custom
  Ask for each choice (alias -Interactive).

.PARAMETER Yes
  Never ask (alias -NonInteractive).

.PARAMETER Check
  Run the preflight checks and exit, writing nothing.

.PARAMETER Bin
  Install this apex binary instead of building one with go.
#>
[CmdletBinding()]
param(
  [string]$Workflow, [string]$Harness, [string]$SkillsDir, [string]$Prefix, [string]$ConfigDir,
  [switch]$WithOutputStyle, [switch]$WithClaudeMods, [switch]$CheckPonytail,
  [Alias('Interactive')][switch]$Custom,
  [Alias('NonInteractive')][switch]$Yes,
  [switch]$Check, [string]$Bin
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'apex-install-common.ps1')
$repo = Split-Path $PSScriptRoot -Parent
$conf = Join-Path $PSScriptRoot 'workflows.conf'

$ctx = Resolve-ApexContext -Prefix $Prefix -ConfigDir $ConfigDir
if ($Workflow -and -not (Test-ApexWorkflow $Workflow)) { Fail 64 "unknown workflow '$Workflow' (orchestration, multi-agent, both)" }
if ((@(Get-ApexConfSkills 'orchestration' $conf).Count -eq 0) -or (@(Get-ApexConfSkills 'multi-agent' $conf).Count -eq 0)) {
  Fail 1 "$conf does not list both workflows"
}
# A skills directory without an explicit harness means custom; beside one, it adds custom.
if ($SkillsDir) {
  if (-not $Harness) { $Harness = 'custom' }
  elseif ($Harness.Split(',') -cnotcontains 'custom') { $Harness = "$Harness,custom" }
}
if ($Harness) { Assert-ApexHarnessList $Harness $SkillsDir }

# Claude extras: '' leaves them alone, 'yes' installs, 'no' removes.
$style = ''
if ($PSBoundParameters.ContainsKey('WithOutputStyle')) { $style = if ($WithOutputStyle.IsPresent) { 'yes' } else { 'no' } }
$mods = ''
if ($PSBoundParameters.ContainsKey('WithClaudeMods')) { $mods = if ($WithClaudeMods.IsPresent) { 'yes' } else { 'no' } }
$ponytail = $CheckPonytail.IsPresent

# ---- decide what to install: flag, then prompt, then previous install, then default ----

function Read-ApexLine {
  param([string]$Prompt)
  Write-Host $Prompt -NoNewline
  $l = [Console]::In.ReadLine()
  if ($null -eq $l) { '' } else { $l.Trim() }
}
function Read-ApexYesNo {
  param([string]$Question)
  (Read-ApexLine "$Question [y/N] ") -match '^(y|yes)$'
}

$prev = Get-ApexPrev $ctx
$prompt = $false
if ($Yes.IsPresent) { $prompt = $false }
elseif ($Custom.IsPresent) { $prompt = $true }
elseif ((-not $Workflow -or -not $Harness) -and -not [Console]::IsInputRedirected) { $prompt = $true }

$kept = $false
if (-not $Workflow) {
  if ($prompt) {
    $def = '1'
    if ($prev.Workflow -ceq 'multi-agent') { $def = '2' } elseif ($prev.Workflow -ceq 'both') { $def = '3' }
    Write-Host 'Workflow:'
    Write-Host "  1) $(Get-ApexWorkflowName 'orchestration')"
    Write-Host "  2) $(Get-ApexWorkflowName 'multi-agent')"
    Write-Host '  3) Both'
    $r = Read-ApexLine "Choose [$def]: "
    if (-not $r) { $r = $def }
    if (($r -ceq '1') -or ($r -ceq 'orchestration')) { $Workflow = 'orchestration' }
    elseif (($r -ceq '2') -or ($r -ceq 'multi-agent')) { $Workflow = 'multi-agent' }
    elseif (($r -ceq '3') -or ($r -ceq 'both')) { $Workflow = 'both' }
    else { Fail 64 "invalid workflow choice '$r'" }
  } elseif ($prev.Workflow) { $Workflow = $prev.Workflow; $kept = $true }
  else { $Workflow = 'orchestration' }
}
if (-not $Harness) {
  if ($prompt) {
    $def = if ($prev.Harness) { $prev.Harness } else { 'agents' }
    $codexHome = if ($env:CODEX_HOME) { $env:CODEX_HOME } else { Join-Path $ctx.Home '.codex' }
    Write-Host 'Harness (comma separated for several):'
    Write-Host '  1) agents   ~/.agents/skills'
    Write-Host "  2) claude   $(Join-Path $ctx.ConfigDir 'skills')"
    Write-Host "  3) codex    $(Join-Path $codexHome 'skills')"
    Write-Host '  4) custom   a directory you name'
    $geminiDir = if ($env:GEMINI_CONFIG_DIR) { $env:GEMINI_CONFIG_DIR } else { Join-Path (Join-Path $ctx.Home '.gemini') 'config' }
    Write-Host "  5) gemini   $(Join-Path $geminiDir 'skills')"
    Write-Host '  6) t3       T3 Code (agents, claude, codex, gemini)'
    $r = Read-ApexLine "Choose [$def]: "
    if (-not $r) { $r = $def }
    $picked = New-Object System.Collections.ArrayList
    foreach ($tok in $r.Split(@(',', ' '), [StringSplitOptions]::RemoveEmptyEntries)) {
      $t = $tok
      if ($t -ceq '1') { $t = 'agents' } elseif ($t -ceq '2') { $t = 'claude' } elseif ($t -ceq '3') { $t = 'codex' } elseif ($t -ceq '4') { $t = 'custom' } elseif ($t -ceq '5') { $t = 'gemini' } elseif ($t -ceq '6') { $t = 't3' }
      if (-not (Test-ApexHarness $t)) { Fail 64 "invalid harness choice '$tok'" }
      [void]$picked.Add($t)
    }
    $Harness = $picked -join ','
    if (($picked -ccontains 'custom') -and -not $SkillsDir) { $SkillsDir = Read-ApexLine 'Skills directory: ' }
    Assert-ApexHarnessList $Harness $SkillsDir
  } elseif ($prev.Harness) {
    $Harness = $prev.Harness; $kept = $true
    # A custom directory is not derivable from the name; take it from the previous install.
    if (-not $SkillsDir) {
      $row = @($prev.Rows | Where-Object { $_.Harness -ceq 'custom' }) | Select-Object -First 1
      if ($row) { $SkillsDir = $row.Dir }
    }
    Assert-ApexHarnessList $Harness $SkillsDir
  } else { $Harness = 'agents' }
}
if ($kept) { Write-Host "keeping workflow $Workflow, harness $Harness from previous install; pass -Workflow/-Harness to change" }

if ($prompt -and (($Harness.Split(',') -ccontains 'claude') -or ($Harness.Split(',') -ccontains 't3'))) {
  if (-not $style) { if (Read-ApexYesNo 'Install the Apex output style?') { $style = 'yes' } }
  if (-not $mods) { if (Read-ApexYesNo 'Install the Claude mods (token-spend, usage-footer)?') { $mods = 'yes' } }
  if (-not $ponytail) { if (Read-ApexYesNo 'Require the ponytail plugin?') { $ponytail = $true } }
}

$targets = @(Get-ApexTargets $ctx $Harness $SkillsDir)
$skills = @(Get-ApexWorkflowSkills $Workflow $conf)
$stylePath = Get-ApexStylePath $ctx
$binDest = Join-Path $ctx.Prefix $ctx.BinName

if ($prompt) {
  Write-Host 'Will install:'
  Write-Host "  workflow: $(Get-ApexWorkflowName $Workflow) ($Workflow)"
  Write-Host "  skills:   $($skills -join ' ')"
  foreach ($t in $targets) { Write-Host "  harness:  $($t.Harness) -> $($t.Dir)" }
  Write-Host "  apex:     $binDest"
  if ($style) { Write-Host "  output style: $style" }
  if ($mods) { Write-Host "  claude mods:  $mods" }
  if ((Read-ApexLine 'Proceed? [Y/n] ') -match '^(n|no)$') { Write-Host 'aborted; nothing written'; exit 1 }
}

# ---- preflight: every check, zero writes ----

if (-not $Bin -and -not (Get-Command go -ErrorAction SilentlyContinue)) { Fail 2 'go is not on PATH, so apex cannot be built; install Go or pass -Bin FILE' }
if (($mods -eq 'yes') -and -not (Get-Command claude -ErrorAction SilentlyContinue)) { Fail 2 'claude is not on PATH; -WithClaudeMods needs the claude CLI' }
if ($Bin -and -not (Test-Path -LiteralPath $Bin -PathType Leaf)) { Fail 64 "-Bin $Bin is not a file" }
if ($ponytail -and -not (Test-ApexPlugin $ctx 'ponytail@ponytail')) {
  Fail 1 'ponytail plugin is not installed: /plugin marketplace add DietrichGebert/ponytail, then /plugin install ponytail@ponytail'
}
foreach ($n in 'apex', 'apex.exe') {
  $l = Join-Path (Join-Path $ctx.ConfigDir 'bin') $n
  if (Test-Path -LiteralPath $l) { Fail 1 "$l is a legacy Apex v0.3 install and shadows the new apex on PATH; remove it first: pwsh example/scripts/uninstall.ps1" }
}
foreach ($s in $skills) {
  if (-not (Test-Path -LiteralPath (Join-Path (Join-Path (Join-Path $repo 'skills') $s) 'SKILL.md'))) { Fail 1 "skills/$s/SKILL.md is missing from $repo" }
}
foreach ($t in $targets) {
  foreach ($s in $skills) {
    $d = Join-Path $t.Dir $s
    if ((Test-Path -LiteralPath $d) -and -not (Test-Path -LiteralPath (Join-Path $d $ApexMark))) { Fail 1 "$d exists and was not installed by apex; move it first" }
  }
}
if ($style -eq 'yes') {
  if (-not (Test-Path -LiteralPath (Join-Path (Join-Path $repo 'output-styles') 'apex.md'))) { Fail 1 "output-styles/apex.md is missing from $repo" }
  if ((Test-Path -LiteralPath $stylePath) -and -not (Test-ApexStyle $stylePath)) { Fail 1 "$stylePath exists and was not installed by apex; move it first" }
}
if ($mods -eq 'yes') {
  foreach ($m in $ApexMods) {
    if (Test-ApexForeignMod $ctx $m) { Fail 1 "another $m plugin is installed; uninstall it first: claude plugin uninstall <$m@marketplace>" }
  }
}
if ($Check.IsPresent) { Write-Host '[ok] preflight passed; nothing written'; exit 0 }

# ---- install ----

New-Item -ItemType Directory -Force -Path $ctx.Prefix | Out-Null
if ($Bin) { $src = $Bin }
else {
  Push-Location $repo
  $ErrorActionPreference = 'Continue'  # native stderr must not abort us
  try {
    go build -trimpath -o "bin/$($ctx.BinName)" ./cmd/apex
    $rc = $LASTEXITCODE
  } finally { $ErrorActionPreference = 'Stop'; Pop-Location }
  if ($rc -ne 0) { Fail 1 "go build failed ($rc)" }
  $src = Join-Path (Join-Path $repo 'bin') $ctx.BinName
}
Copy-Item -LiteralPath $src -Destination $binDest -Force
if ($prev.Bin -and ($prev.Bin -ine $binDest) -and (Test-Path -LiteralPath $prev.Bin)) { Remove-Item -LiteralPath $prev.Bin -Force }

# Sweep where the previous install put skills as well as where this one does, so switching
# workflow or harness leaves nothing stale. Unmarked skills are never touched.
foreach ($r in $prev.Rows) { Remove-ApexMarked $r.Dir }
foreach ($t in $targets) { Remove-ApexMarked $t.Dir }
$enc = New-Object System.Text.UTF8Encoding($false)
foreach ($t in $targets) {
  New-Item -ItemType Directory -Force -Path $t.Dir | Out-Null
  foreach ($s in $skills) {
    $d = Join-Path $t.Dir $s
    Copy-Item -LiteralPath (Join-Path (Join-Path $repo 'skills') $s) -Destination $d -Recurse
    [IO.File]::WriteAllBytes((Join-Path $d $ApexMark), (New-Object byte[] 0))
  }
}

# Claude extras: touched only when asked for.
if ($style -eq 'yes') {
  New-Item -ItemType Directory -Force -Path (Split-Path $stylePath -Parent) | Out-Null
  Copy-Item -LiteralPath (Join-Path (Join-Path $repo 'output-styles') 'apex.md') -Destination $stylePath -Force
} elseif ($style -eq 'no') { Remove-ApexStyle $ctx }
if ($mods -eq 'yes') {
  $ErrorActionPreference = 'Continue'  # native stderr must not abort us
  claude plugin marketplace add (Join-Path $repo 'mods')
  if ($LASTEXITCODE -ne 0) { Fail 1 "claude plugin marketplace add failed ($LASTEXITCODE)" }
  foreach ($m in $ApexMods) {
    claude plugin install "$m@apex"
    if ($LASTEXITCODE -ne 0) { Fail 1 "claude plugin install $m@apex failed ($LASTEXITCODE)" }
  }
  $ErrorActionPreference = 'Stop'
} elseif ($mods -eq 'no') { Remove-ApexMods $ctx }

# Manifest: LF lines, UTF-8 without BOM, written to a temp file and renamed so it is never half written.
$lines = @("version`t1", "workflow`t$Workflow", "bin`t$binDest", "config`t$($ctx.ConfigDir)")
foreach ($t in $targets) { $lines += "skills`t$($t.Harness)`t$($t.Dir)" }
New-Item -ItemType Directory -Force -Path (Split-Path $ctx.Manifest -Parent) | Out-Null
$tmp = "$($ctx.Manifest).tmp.$PID"
[IO.File]::WriteAllText($tmp, (($lines -join "`n") + "`n"), $enc)
Move-Item -LiteralPath $tmp -Destination $ctx.Manifest -Force

$norm = $ctx.Prefix.TrimEnd('\', '/')
$onPath = $env:PATH.Split([IO.Path]::PathSeparator) | Where-Object { $_ -and ($_.TrimEnd('\', '/') -ieq $norm) }
if (-not $onPath) { Warn "$($ctx.Prefix) is not on PATH; every skill calls a bare apex" }
Write-Host "[ok] apex -> $($ctx.Prefix); workflow $Workflow ($($skills.Count) skills) -> $(($targets | ForEach-Object { $_.Dir }) -join ' '). Restart your harness." -ForegroundColor Green
exit 0
