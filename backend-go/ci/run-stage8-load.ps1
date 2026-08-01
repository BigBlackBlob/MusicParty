param(
    [int]$Connections = 10,
    [int]$DurationMs = 30000,
    [int]$RecoveryMs = 5000,
    [int]$QueueSize = 0,
    [int]$Rooms = 0,
    [string]$ReportName = '',
    [switch]$KeepTemporaryFiles
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$backendRoot = Join-Path $repositoryRoot 'backend-go'
$acceptanceRoot = Join-Path $backendRoot 'acceptance\stage8'
$temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) ('musicparty-go-stage8-' + [Guid]::NewGuid().ToString('N'))
$process = $null
$neteaseStubProcess = $null
$savedEnvironment = @{}

function Get-FreeTcpPort {
    $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
    $listener.Start()
    try { return ([Net.IPEndPoint]$listener.LocalEndpoint).Port } finally { $listener.Stop() }
}

function Set-IsolatedEnvironment([int]$Port, [int]$NeteaseStubPort) {
    $origin = "http://127.0.0.1:$Port"
    $values = @{
        SERVER_PORT = [string]$Port
        APP_MODE = 'server'
        BASE_URL = $origin
        ALLOWED_ORIGINS = $origin
        DB_ENABLED = 'true'
        DB_INIT_SCHEMA = 'true'
        DB_PATH = (Join-Path $temporaryRoot 'musicparty.db')
        LOCAL_LIBRARY_ENABLED = 'true'
        LOCAL_LIBRARY_PATH = (Join-Path $temporaryRoot 'local-library')
        CACHE_PATH = (Join-Path $temporaryRoot 'cache')
        CACHE_MAX_SIZE = '64MB'
        BOOTSTRAP_ADMIN_USERNAME = 'stage8-admin'
        BOOTSTRAP_ADMIN_PASSWORD = 'Stage8-Password-2026!'
        AUTH_SECURE_COOKIES = 'false'
        AUTH_RATE_LIMIT_ENABLED = 'false'
        ROOM_ACCESS_TOKEN_SECRET = 'stage8-room-access-secret-32-bytes'
        NETEASE_API_URL = "http://127.0.0.1:$NeteaseStubPort"
        YOUTUBE_ENABLED = 'false'
        NAVIDROME_ENABLED = 'false'
        SQUIDIFY_ENABLED = 'false'
        LOG_LEVEL = 'WARN'
    }
    foreach ($entry in $values.GetEnumerator()) {
        $previous = [Environment]::GetEnvironmentVariable($entry.Key, [EnvironmentVariableTarget]::Process)
        $savedEnvironment[$entry.Key] = [pscustomobject]@{ Exists = $null -ne $previous; Value = $previous }
        [Environment]::SetEnvironmentVariable($entry.Key, $entry.Value, 'Process')
    }
}

function Restore-Environment {
    foreach ($entry in $savedEnvironment.GetEnumerator()) {
        if ($entry.Value.Exists) {
            [Environment]::SetEnvironmentVariable($entry.Key, $entry.Value.Value, [EnvironmentVariableTarget]::Process)
        } else {
            Remove-Item -LiteralPath ("Env:" + $entry.Key) -ErrorAction SilentlyContinue
        }
    }
}

New-Item -ItemType Directory -Path $temporaryRoot | Out-Null
New-Item -ItemType Directory -Path $acceptanceRoot -Force | Out-Null
try {
    $port = Get-FreeTcpPort
    $neteaseStubPort = Get-FreeTcpPort
    Set-IsolatedEnvironment -Port $port -NeteaseStubPort $neteaseStubPort

    $neteaseStubStdout = Join-Path $temporaryRoot 'netease-stub.stdout.log'
    $neteaseStubStderr = Join-Path $temporaryRoot 'netease-stub.stderr.log'
    $neteaseStubScript = Join-Path $PSScriptRoot 'stage8-netease-stub.mjs'
    $neteaseStubProcess = Start-Process -FilePath 'node' -ArgumentList @($neteaseStubScript, "--port=$neteaseStubPort") -WorkingDirectory $temporaryRoot -PassThru -WindowStyle Hidden -RedirectStandardOutput $neteaseStubStdout -RedirectStandardError $neteaseStubStderr
    $neteaseStubHealth = "http://127.0.0.1:$neteaseStubPort/health"
    $neteaseStubDeadline = [DateTime]::UtcNow.AddSeconds(15)
    do {
        if ($neteaseStubProcess.HasExited) { throw "Netease test stub exited early. stdout=$neteaseStubStdout stderr=$neteaseStubStderr" }
        try {
            $stubHealth = Invoke-RestMethod -Uri $neteaseStubHealth -TimeoutSec 2 -NoProxy
            if ($stubHealth.status -eq 'UP') { break }
        } catch { Start-Sleep -Milliseconds 100 }
    } while ([DateTime]::UtcNow -lt $neteaseStubDeadline)
    if ([DateTime]::UtcNow -ge $neteaseStubDeadline) { throw "Netease test stub did not become healthy. stdout=$neteaseStubStdout stderr=$neteaseStubStderr" }

    $executable = Join-Path $temporaryRoot 'musicparty.exe'
    Push-Location $backendRoot
    try {
        & go build -o $executable ./cmd/musicparty
        if ($LASTEXITCODE -ne 0) { throw "Go build failed with exit code $LASTEXITCODE" }
    } finally { Pop-Location }

    $stdout = Join-Path $temporaryRoot 'go.stdout.log'
    $stderr = Join-Path $temporaryRoot 'go.stderr.log'
    $process = Start-Process -FilePath $executable -WorkingDirectory $temporaryRoot -PassThru -WindowStyle Hidden -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $baseUrl = "http://127.0.0.1:$port"
    $deadline = [DateTime]::UtcNow.AddSeconds(30)
    do {
        if ($process.HasExited) { throw "Go backend exited early. stdout=$stdout stderr=$stderr" }
        try {
            $health = Invoke-RestMethod -Uri "$baseUrl/actuator/health" -TimeoutSec 2 -NoProxy
            if ($health.status -eq 'UP') { break }
        } catch { Start-Sleep -Milliseconds 200 }
    } while ([DateTime]::UtcNow -lt $deadline)
    if ([DateTime]::UtcNow -ge $deadline) { throw "Go backend did not become healthy. stdout=$stdout stderr=$stderr" }

    if ([string]::IsNullOrWhiteSpace($ReportName)) {
        $ReportName = "load-c$Connections-r$Rooms-q$QueueSize-$DurationMs.json"
    }
    $reportPath = Join-Path $acceptanceRoot $ReportName
    $arguments = @(
        'ci/stage8-load.mjs',
        "--baseUrl=$baseUrl",
        "--connections=$Connections",
        "--durationMs=$DurationMs",
        "--recoveryMs=$RecoveryMs",
        "--queueSize=$QueueSize",
        "--rooms=$Rooms",
        '--username=stage8-admin',
        '--password=Stage8-Password-2026!'
    )
    Push-Location $backendRoot
    try {
        & node @arguments | Tee-Object -FilePath $reportPath
        if ($LASTEXITCODE -ne 0) { throw "stage8-load failed with exit code $LASTEXITCODE" }
    } finally { Pop-Location }
    Copy-Item -LiteralPath $stdout -Destination (Join-Path $acceptanceRoot ($ReportName + '.stdout.log')) -Force
    Copy-Item -LiteralPath $stderr -Destination (Join-Path $acceptanceRoot ($ReportName + '.stderr.log')) -Force
    Write-Host "Stage 8 report saved to $reportPath"
} finally {
    if ($process -and -not $process.HasExited) {
        Stop-Process -Id $process.Id -Force
        $process.WaitForExit()
    }
    if ($neteaseStubProcess -and -not $neteaseStubProcess.HasExited) {
        Stop-Process -Id $neteaseStubProcess.Id -Force
        $neteaseStubProcess.WaitForExit()
    }
    Restore-Environment
    if ($KeepTemporaryFiles) {
        Write-Host "Temporary Stage 8 environment retained at $temporaryRoot"
    } elseif (Test-Path -LiteralPath $temporaryRoot) {
        $resolvedTemp = (Resolve-Path -LiteralPath $temporaryRoot).Path
        $systemTemp = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
        if (-not $resolvedTemp.StartsWith($systemTemp, [StringComparison]::OrdinalIgnoreCase) -or (Split-Path $resolvedTemp -Leaf) -notlike 'musicparty-go-stage8-*') {
            throw "Refusing to remove unexpected path $resolvedTemp"
        }
        Remove-Item -LiteralPath $resolvedTemp -Recurse -Force
    }
}
