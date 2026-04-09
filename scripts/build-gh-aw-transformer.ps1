Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Resolve-Path (Join-Path $ScriptDir '..')

$TargetProjectDir = if ($args.Count -ge 1 -and $args[0]) { $args[0] } else { Join-Path $RepoRoot '..\gh-aw-transformer' }
$OutputName = if ($args.Count -ge 2 -and $args[1]) { $args[1] } else { 'gh-aw-transformer.exe' }
$OutputDir = if ($args.Count -ge 3 -and $args[2]) { $args[2] } else { Join-Path $TargetProjectDir 'bin' }

if (-not (Test-Path $TargetProjectDir -PathType Container)) {
    throw "Project directory not found: $TargetProjectDir"
}

$GoModPath = Join-Path $TargetProjectDir 'go.mod'
if (-not (Test-Path $GoModPath -PathType Leaf)) {
    throw "go.mod not found in: $TargetProjectDir"
}

New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
$OutputPath = Join-Path $OutputDir $OutputName

Push-Location $TargetProjectDir
try {
    Write-Host "Building gh-aw-transformer -> $OutputPath"

    if (Test-Path '.\cmd\gh-aw-transformer\main.go' -PathType Leaf) {
        go build -o $OutputPath ./cmd/gh-aw-transformer/main.go
    }
    elseif (Test-Path '.\cmd\main.go' -PathType Leaf) {
        go build -o $OutputPath ./cmd/main.go
    }
    else {
        go build -o $OutputPath .
    }

    Write-Host "Build complete: $OutputPath"
}
finally {
    Pop-Location
}
