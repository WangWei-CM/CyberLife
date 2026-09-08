[CmdletBinding()]
param(
  [Parameter(Mandatory, Position = 0)]
  [ValidateNotNullOrEmpty()]
  [string]$Message,

  [Parameter(Mandatory, Position = 1)]
  [ValidateNotNullOrEmpty()]
  [string[]]$Paths
)

$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
$taskWeb = Join-Path $taskRoot 'web'
$taskDist = Join-Path $taskWeb 'dist'
$taskProductWeb = Join-Path $taskRoot 'product\web'

function Fail([object]$Output) {
  if ($null -ne $Output) { $Output | Out-String | Write-Error }
  exit 1
}

try {
  foreach ($taskPath in $Paths) {
    $taskFullPath = [IO.Path]::GetFullPath((Join-Path $taskRoot $taskPath))
    if (-not $taskFullPath.StartsWith($taskRoot, [StringComparison]::OrdinalIgnoreCase)) {
      throw "Refusing to stage a path outside the repository: $taskPath"
    }
  }

  Push-Location $taskWeb
  $taskBuildOutput = & pnpm build 2>&1
  if ($LASTEXITCODE -ne 0) { Fail $taskBuildOutput }
  Pop-Location

  $taskDeployOutput = & {
    Copy-Item -Path (Join-Path $taskDist '*') -Destination $taskProductWeb -Recurse -Force
    $taskIndex = Join-Path $taskProductWeb 'index.html'
    $taskHtml = Get-Content -LiteralPath $taskIndex -Raw
    $taskAssets = [regex]::Matches($taskHtml, '(?:href|src)="(/assets/[^"]+)"') |
      ForEach-Object { Join-Path $taskProductWeb $_.Groups[1].Value.TrimStart('/') }
    if (@($taskAssets | Where-Object { -not (Test-Path -LiteralPath $_) }).Count) {
      throw 'Deployed frontend asset validation failed.'
    }
  } 2>&1
  if ($LASTEXITCODE -ne 0) { Fail $taskDeployOutput }

  Push-Location $taskRoot
  $taskCommitOutput = & git add -- @Paths 2>&1
  if ($LASTEXITCODE -ne 0) { Fail $taskCommitOutput }
  $taskCommitOutput = & git commit -m $Message 2>&1
  if ($LASTEXITCODE -ne 0) { Fail $taskCommitOutput }
  Pop-Location
}
catch {
  Write-Error $_
  exit 1
}
finally {
  while ((Get-Location).Path -ne $taskRoot -and (Get-Location).Path -like "$taskWeb*") { Pop-Location }
}
