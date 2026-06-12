<#
.SYNOPSIS
    Install aks-helper on Windows: the binary on your PATH and the agent skill
    into the skills directories that coding agents read. Safe to re-run.

.PARAMETER Scope
    'global' (default, into ~/) or 'local' (into the current project).

.PARAMETER Agent
    'all' (default), 'claude', 'copilot' or 'agents'.

.PARAMETER SkillOnly
    Install only the agent skill.

.PARAMETER BinaryOnly
    Build and install only the binary.

.EXAMPLE
    pwsh scripts/install.ps1
    pwsh scripts/install.ps1 -SkillOnly -Agent copilot

.NOTES
    Skill locations (Agent Skills spec):
      global  claude=~/.claude/skills  copilot=~/.copilot/skills  agents=~/.agents/skills
      local   claude=.claude/skills    copilot=.github/skills     agents=.agents/skills
    Override the binary dir with $env:BINDIR.
#>
[CmdletBinding()]
param(
    [ValidateSet('global', 'local')] [string]$Scope = 'global',
    [ValidateSet('all', 'claude', 'copilot', 'agents')] [string]$Agent = 'all',
    [switch]$SkillOnly,
    [switch]$BinaryOnly
)

$ErrorActionPreference = 'Stop'
$RepoRoot  = Split-Path -Parent $PSScriptRoot
$BinDir    = if ($env:BINDIR) { $env:BINDIR } else { Join-Path $env:USERPROFILE '.local\bin' }
$SkillName = 'aks-access'
$SkillSrc  = Join-Path $RepoRoot ".claude\skills\$SkillName"

function Get-SkillBase([string]$a) {
    if ($Scope -eq 'global') {
        switch ($a) {
            'claude'  { Join-Path $env:USERPROFILE '.claude\skills' }
            'copilot' { Join-Path $env:USERPROFILE '.copilot\skills' }
            'agents'  { Join-Path $env:USERPROFILE '.agents\skills' }
        }
    }
    else {
        switch ($a) {
            'claude'  { Join-Path (Get-Location) '.claude\skills' }
            'copilot' { Join-Path (Get-Location) '.github\skills' }
            'agents'  { Join-Path (Get-Location) '.agents\skills' }
        }
    }
}

function Install-Binary {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "'go' is required to build aks-helper (or download a release binary)"
    }
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    $out = Join-Path $BinDir 'aks-helper.exe'
    Write-Host "Building aks-helper -> $out"
    Push-Location $RepoRoot
    try { & go build -trimpath -ldflags '-s -w' -o $out . }
    finally { Pop-Location }
    if (($env:PATH -split ';') -notcontains $BinDir) {
        Write-Warning "$BinDir is not on your PATH — add it so 'aks-helper' resolves."
    }
}

function Install-Skill {
    if (-not (Test-Path (Join-Path $SkillSrc 'SKILL.md'))) {
        throw "skill not found at $SkillSrc"
    }
    $agents = if ($Agent -eq 'all') { @('claude', 'copilot', 'agents') } else { @($Agent) }
    foreach ($a in $agents) {
        $dest = Join-Path (Get-SkillBase $a) $SkillName
        if ((Resolve-Path $SkillSrc).Path -eq $dest) {
            Write-Host "skipping ${a}: source and destination are the same ($dest)"
            continue
        }
        New-Item -ItemType Directory -Force -Path $dest | Out-Null
        Copy-Item -Recurse -Force -Path (Join-Path $SkillSrc '*') -Destination $dest
        Write-Host "Installed skill '$SkillName' ($a, $Scope) -> $dest"
    }
}

if (-not $SkillOnly)  { Install-Binary }
if (-not $BinaryOnly) { Install-Skill }

Write-Host ""
Write-Host "Done."
if (-not $SkillOnly) {
    Write-Host "  - Enable shell integration (once): aks-helper shell-init powershell | Out-String | Invoke-Expression"
}
if (-not $BinaryOnly) {
    Write-Host "  - The '$SkillName' skill is now available to your coding agent(s)."
}
