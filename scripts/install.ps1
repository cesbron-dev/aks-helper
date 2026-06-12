<#
.SYNOPSIS
    Install aks-helper globally on Windows: the binary on your PATH and the agent
    skill into Claude Code's personal skills directory. Safe to re-run.

.PARAMETER SkillOnly
    Install only the agent skill.

.PARAMETER BinaryOnly
    Build and install only the binary.

.EXAMPLE
    pwsh scripts/install.ps1
    pwsh scripts/install.ps1 -SkillOnly

.NOTES
    Override locations with $env:BINDIR and $env:SKILLS_DIR.
#>
[CmdletBinding()]
param(
    [switch]$SkillOnly,
    [switch]$BinaryOnly
)

$ErrorActionPreference = 'Stop'
$RepoRoot  = Split-Path -Parent $PSScriptRoot
$BinDir    = if ($env:BINDIR)     { $env:BINDIR }     else { Join-Path $env:USERPROFILE '.local\bin' }
$SkillsDir = if ($env:SKILLS_DIR) { $env:SKILLS_DIR } else { Join-Path $env:USERPROFILE '.claude\skills' }
$SkillName = 'aks-access'

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
    $src  = Join-Path $RepoRoot ".claude\skills\$SkillName"
    $dest = Join-Path $SkillsDir $SkillName
    if (-not (Test-Path (Join-Path $src 'SKILL.md'))) {
        throw "skill not found at $src"
    }
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    Copy-Item -Recurse -Force -Path (Join-Path $src '*') -Destination $dest
    Write-Host "Installed skill '$SkillName' -> $dest"
}

if (-not $SkillOnly)  { Install-Binary }
if (-not $BinaryOnly) { Install-Skill }

Write-Host ""
Write-Host "Done."
if (-not $SkillOnly) {
    Write-Host "  - Enable shell integration (once): aks-helper shell-init powershell | Out-String | Invoke-Expression"
}
if (-not $BinaryOnly) {
    Write-Host "  - The '$SkillName' skill is now available to Claude Code in every session."
}
