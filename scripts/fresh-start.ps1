param(
    [switch]$StartNeteaseApi,
    [switch]$SkipBrowser
)

$ErrorActionPreference = 'Stop'

$root = Resolve-Path (Join-Path $PSScriptRoot '..')
$ports = @(8080, 5173, 3000)

foreach ($port in $ports) {
    $connections = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue
    foreach ($connection in $connections) {
        $processId = $connection.OwningProcess
        if (-not $processId) {
            continue
        }

        $process = Get-Process -Id $processId -ErrorAction SilentlyContinue
        if (-not $process) {
            continue
        }

        Write-Host "[fresh-start] stopping $($process.ProcessName) pid=$processId on port $port"
        Stop-Process -Id $processId -Force
    }
}

$argsList = @()
if ($StartNeteaseApi) {
    $argsList += '--start-netease-api'
}
if ($SkipBrowser) {
    $argsList += '--skip-browser'
}

Write-Host "[fresh-start] starting dev stack from $root"
bash (Join-Path $root 'start-dev.sh') @argsList
