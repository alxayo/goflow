Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Resolve-Path (Join-Path $ScriptDir '..')

$OutputName = if ($args.Count -ge 1 -and $args[0]) { $args[0] } else { 'goflow.exe' }
$OutputDir = if ($args.Count -ge 2 -and $args[1]) { $args[1] } else { Join-Path $RepoRoot 'bin' }

New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
$OutputPath = Join-Path $OutputDir $OutputName

Push-Location $RepoRoot
try {
    Write-Host "Building goflow -> $OutputPath"
    go build -o $OutputPath ./cmd/workflow-runner/main.go
    Write-Host "Build complete: $OutputPath"
}
finally {
    Pop-Location
}
