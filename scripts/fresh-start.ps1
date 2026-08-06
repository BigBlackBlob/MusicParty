param(
    [switch]$StartNeteaseApi,
    [switch]$SkipBrowser,
    [switch]$BackendOnly,
    [switch]$FrontendOnly,
    [string]$EnvFile
)

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$pidDir = Join-Path $root '.dev-logs\pids'

if ($BackendOnly -and $FrontendOnly) {
    throw '-BackendOnly and -FrontendOnly are mutually exclusive.'
}

if (Test-Path -LiteralPath $pidDir) {
    Get-ChildItem -LiteralPath $pidDir -Filter '*.pid' | ForEach-Object {
        $pidText = (Get-Content -LiteralPath $_.FullName -Raw).Trim()
        if ($pidText -notmatch '^\d+$') { return }

        $processId = [int]$pidText
        $process = Get-CimInstance Win32_Process -Filter "ProcessId = $processId" -ErrorAction SilentlyContinue
        if (-not $process) {
            Remove-Item -LiteralPath $_.FullName -Force
            return
        }

        $commandLine = [string]$process.CommandLine
        if ($commandLine.IndexOf($root, [StringComparison]::OrdinalIgnoreCase) -lt 0) {
            Write-Warning "Skipping PID $processId because its command line does not reference this repository."
            return
        }

        Write-Host "[fresh-start] stopping verified MusicParty process pid=$processId"
        Stop-Process -Id $processId -Force
        Remove-Item -LiteralPath $_.FullName -Force
    }
}

$arguments = @()
if ($StartNeteaseApi) { $arguments += '--start-netease-api' }
if ($SkipBrowser) { $arguments += '--skip-browser' }
if ($BackendOnly) { $arguments += '--backend-only' }
if ($FrontendOnly) { $arguments += '--frontend-only' }
if ($EnvFile) { $arguments += @('--env-file', $EnvFile) }

Write-Host "[fresh-start] starting Go-only development stack from $root"
& bash (Join-Path $root 'start-dev.sh') @arguments
