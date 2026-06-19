<#
.SYNOPSIS
  install.ps1 — install Apex Claude on native Windows (no bash/python/go).

.DESCRIPTION
  The Windows-native counterpart to scripts/install.sh. Downloads the prebuilt
  release bundle (apex.exe + commands/agents/skills/output-style) published by
  scripts/publish.ps1, copies the loose artifacts into ~/.claude, installs the
  binary into ~/.claude/bin, and wires the PreToolUse + SessionStart hooks into
  ~/.claude/settings.json — preserving every other setting.

  Designed for one-line install:

      irm https://github.com/FNGApex/apex-claude/releases/latest/download/install.ps1 | iex

  Because `iex` cannot pass parameters, overrides are read from env vars:
      $env:APEX_VERSION      pin a release tag (default: latest)
      $env:CLAUDE_CONFIG_DIR install root      (default: $env:USERPROFILE\.claude)

  Runs on Windows PowerShell 5.1 and PowerShell 7+. It does NOT touch
  ~/.claude/CLAUDE.md — the Apex spine is opt-in. Remove with uninstall.ps1.

.PARAMETER Version
  Release tag to install (e.g. v0.2.0). Default: latest.

.PARAMETER ConfigDir
  Install root. Default: $env:CLAUDE_CONFIG_DIR or $env:USERPROFILE\.claude.

.EXAMPLE
  irm .../install.ps1 | iex
  pwsh scripts/install.ps1 -Version v0.2.0
#>
[CmdletBinding()]
param(
  [string]$Version,
  [string]$ConfigDir
)

$ErrorActionPreference = 'Stop'

$Repo = 'FNGApex/apex-claude'

if (-not $Version)   { $Version   = if ($env:APEX_VERSION) { $env:APEX_VERSION } else { 'latest' } }
if (-not $ConfigDir) { $ConfigDir = if ($env:CLAUDE_CONFIG_DIR) { $env:CLAUDE_CONFIG_DIR } else { Join-Path $env:USERPROFILE '.claude' } }

function Say { param($m) Write-Host "==> $m" -ForegroundColor Cyan }
function Die { param($m) Write-Host "error: $m" -ForegroundColor Red; exit 1 }

# --- 1. resolve download URL -------------------------------------------------
$asset = 'apex-claude-windows-amd64.zip'
$url = if ($Version -eq 'latest') {
  "https://github.com/$Repo/releases/latest/download/$asset"
} else {
  "https://github.com/$Repo/releases/download/$Version/$asset"
}

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("apex-install-" + [System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmp -Force | Out-Null
$zip = Join-Path $tmp $asset

try {
  Say "Downloading $Version bundle"
  $oldProgress = $ProgressPreference
  $ProgressPreference = 'SilentlyContinue'
  try {
    Invoke-WebRequest -Uri $url -OutFile $zip -UseBasicParsing
  } catch {
    Die "download failed from $url — check the version tag and that a release exists ($($_.Exception.Message))"
  } finally {
    $ProgressPreference = $oldProgress
  }

  Say "Extracting"
  $src = Join-Path $tmp 'bundle'
  Expand-Archive -Path $zip -DestinationPath $src -Force

  $exe = Join-Path $src 'apex.exe'
  if (-not (Test-Path $exe)) { Die "bundle is missing apex.exe — corrupt or wrong asset" }

  # --- 2. drop any prior plugin install (migration) --------------------------
  if (Get-Command claude -ErrorAction SilentlyContinue) {
    $list = & claude plugin list 2>$null
    if ($LASTEXITCODE -eq 0 -and ($list -match 'apex-claude@apex-claude')) {
      Say "Removing prior plugin install of Apex (switching to loose artifacts)"
      & claude plugin uninstall 'apex-claude@apex-claude' 2>$null
      & claude plugin marketplace remove 'apex-claude' 2>$null
    }
  }

  # --- 3. copy artifacts -----------------------------------------------------
  Say "Installing artifacts into $ConfigDir"
  foreach ($sub in 'commands','agents','skills','output-styles','bin') {
    New-Item -ItemType Directory -Path (Join-Path $ConfigDir $sub) -Force | Out-Null
  }
  Copy-Item (Join-Path $src 'commands/ax-*.md') (Join-Path $ConfigDir 'commands') -Force
  Copy-Item (Join-Path $src 'agents/ax-*.md')   (Join-Path $ConfigDir 'agents')   -Force
  Copy-Item (Join-Path $src 'output-styles/apex.md') (Join-Path $ConfigDir 'output-styles/apex.md') -Force

  # Skills are dir/SKILL.md — replace each ax-* skill dir wholesale.
  Get-ChildItem (Join-Path $src 'skills') -Directory | ForEach-Object {
    $dest = Join-Path $ConfigDir "skills/$($_.Name)"
    if (Test-Path $dest) { Remove-Item $dest -Recurse -Force }
    Copy-Item $_.FullName $dest -Recurse -Force
  }

  # --- 4. binary -------------------------------------------------------------
  $apexBin = Join-Path $ConfigDir 'bin/apex.exe'
  Say "Installing binary into $apexBin"
  Copy-Item $exe $apexBin -Force

  # --- 5. wire hooks (preserve all other settings) ---------------------------
  $settingsPath = Join-Path $ConfigDir 'settings.json'
  Say "Wiring hooks into $settingsPath"

  # Hook command: forward-slash full path to the .exe (Claude Code on Windows
  # invokes the executable directly; forward slashes are the documented form).
  # Quote only if the path contains a space.
  $binPath = ($apexBin -replace '\\','/')
  $cmdBin  = if ($binPath -match ' ') { '"' + $binPath + '"' } else { $binPath }

  $data = [pscustomobject]@{}
  if (Test-Path $settingsPath) {
    $raw = Get-Content $settingsPath -Raw
    if ($raw.Trim()) {
      try { $data = $raw | ConvertFrom-Json } catch {
        Die "settings.json is not valid JSON — fix it by hand and re-run ($($_.Exception.Message))"
      }
    }
  }

  function Has-Prop($obj, $name) { $obj.PSObject.Properties.Name -contains $name }
  function Test-IsApex($group) {
    foreach ($h in @($group.hooks)) {
      if ([string]$h.command -match 'apex(\.exe)? hooks') { return $true }
    }
    return $false
  }

  if (-not (Has-Prop $data 'hooks')) {
    $data | Add-Member -NotePropertyName 'hooks' -NotePropertyValue ([pscustomobject]@{})
  }
  $hooks = $data.hooks

  $newGroups = @{
    PreToolUse  = [pscustomobject]@{ matcher = 'Bash'; hooks = @([pscustomobject]@{ type = 'command'; command = "$cmdBin hooks pre-bash" }) }
    SessionStart = [pscustomobject]@{ hooks = @([pscustomobject]@{ type = 'command'; command = "$cmdBin hooks session-start" }) }
  }

  foreach ($event in 'PreToolUse','SessionStart') {
    # Strip any prior apex group so re-runs don't stack duplicates.
    $kept = @()
    if (Has-Prop $hooks $event) {
      $kept = @($hooks.$event | Where-Object { -not (Test-IsApex $_) })
    }
    $merged = @($kept) + @($newGroups[$event])
    if (Has-Prop $hooks $event) { $hooks.$event = $merged }
    else { $hooks | Add-Member -NotePropertyName $event -NotePropertyValue $merged }
  }

  ($data | ConvertTo-Json -Depth 20) | Set-Content -Path $settingsPath -Encoding UTF8
  Write-Host "  hooks wired -> $binPath"

  # --- done ------------------------------------------------------------------
  $cmdCount   = (Get-ChildItem (Join-Path $ConfigDir 'commands/ax-*.md')).Count
  $agentCount = (Get-ChildItem (Join-Path $ConfigDir 'agents/ax-*.md')).Count
  $skillCount = (Get-ChildItem (Join-Path $ConfigDir 'skills') -Directory -Filter 'ax-*').Count

  Write-Host ""
  Write-Host "✔ Apex Claude installed (loose artifacts)." -ForegroundColor Green
  Write-Host "  commands : $cmdCount  -> $ConfigDir\commands"
  Write-Host "  agents   : $agentCount  -> $ConfigDir\agents"
  Write-Host "  skills   : $skillCount   -> $ConfigDir\skills"
  Write-Host "  style    : Apex -> $ConfigDir\output-styles\apex.md"
  Write-Host ""
  Write-Host "Next steps:"
  Write-Host "  - Restart Claude Code so /ax-* commands, agents, skills, and hooks load."
  Write-Host "  - Activate the output style: /output-style Apex"

  $binDir = Join-Path $ConfigDir 'bin'
  $onPath = ($env:PATH -split ';') -contains $binDir
  if (-not $onPath) {
    Write-Host ""
    Write-Host "! $binDir is not on your PATH" -ForegroundColor Yellow
    Write-Host "  Claude Code's hooks call apex by full path, so they work regardless."
    Write-Host "  To run 'apex' yourself, add it to PATH (persists for new shells):"
    Write-Host "    [Environment]::SetEnvironmentVariable('Path', `"$binDir;`$env:Path`", 'User')"
  }
} finally {
  Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}
