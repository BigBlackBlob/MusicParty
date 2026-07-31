param(
    [ValidateSet('capture', 'compare')]
    [string]$Mode = 'compare',
    [switch]$KeepTemporaryFiles
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$backendRoot = Join-Path $repositoryRoot 'backend-go'
$temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) ('musicparty-go-contract-' + [Guid]::NewGuid().ToString('N'))
$process = $null
$savedEnvironment = @{}

function Get-FreeTcpPort {
    $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
    $listener.Start()
    try { return ([Net.IPEndPoint]$listener.LocalEndpoint).Port } finally { $listener.Stop() }
}

function Set-IsolatedEnvironment([int]$Port) {
    $values = @{
        SERVER_PORT = [string]$Port
        APP_MODE = 'server'
        BASE_URL = 'http://contract.invalid'
        ALLOWED_ORIGINS = 'http://contract.invalid'
        DB_ENABLED = 'true'
        DB_INIT_SCHEMA = 'true'
        DB_PATH = (Join-Path $temporaryRoot 'musicparty.db')
        LOCAL_LIBRARY_ENABLED = 'true'
        LOCAL_LIBRARY_PATH = (Join-Path $temporaryRoot 'local-library')
        CACHE_PATH = (Join-Path $temporaryRoot 'cache')
        CACHE_MAX_SIZE = '32MB'
        BOOTSTRAP_ADMIN_USERNAME = 'contract-admin'
        BOOTSTRAP_ADMIN_PASSWORD = 'Contract-Password-123!'
        AUTH_SECURE_COOKIES = 'false'
        AUTH_RATE_LIMIT_ENABLED = 'false'
        ROOM_ACCESS_TOKEN_SECRET = 'contract-room-access-secret-32-bytes'
        NETEASE_API_URL = 'http://127.0.0.1:9'
        YOUTUBE_ENABLED = 'false'
        NAVIDROME_ENABLED = 'false'
        SQUIDIFY_ENABLED = 'false'
        LOG_LEVEL = 'WARN'
    }
    foreach ($entry in $values.GetEnumerator()) {
        $savedEnvironment[$entry.Key] = [Environment]::GetEnvironmentVariable($entry.Key, 'Process')
        [Environment]::SetEnvironmentVariable($entry.Key, $entry.Value, 'Process')
    }
}

function Restore-Environment {
    foreach ($entry in $savedEnvironment.GetEnumerator()) {
        [Environment]::SetEnvironmentVariable($entry.Key, $entry.Value, 'Process')
    }
}

New-Item -ItemType Directory -Path $temporaryRoot | Out-Null
try {
    $port = Get-FreeTcpPort
    Set-IsolatedEnvironment -Port $port
    $executable = Join-Path $temporaryRoot 'musicparty.exe'
    Push-Location $backendRoot
    try {
        & go build -o $executable ./cmd/musicparty
        if ($LASTEXITCODE -ne 0) { throw "Go build failed with exit code $LASTEXITCODE" }
    } finally {
        Pop-Location
    }

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
        } catch {
            Start-Sleep -Milliseconds 200
        }
    } while ([DateTime]::UtcNow -lt $deadline)
    if ([DateTime]::UtcNow -ge $deadline) { throw "Go backend did not become healthy. stdout=$stdout stderr=$stderr" }

    Push-Location $backendRoot
    try {
        & go run ./cmd/contractprobe -mode $Mode -repo .. -base-url $baseUrl -admin-user contract-admin -admin-password 'Contract-Password-123!'
        if ($LASTEXITCODE -ne 0) { throw "contractprobe failed with exit code $LASTEXITCODE" }
    } finally {
        Pop-Location
    }
} finally {
    if ($process -and -not $process.HasExited) {
        Stop-Process -Id $process.Id -Force
        $process.WaitForExit()
    }
    Restore-Environment
    if ($KeepTemporaryFiles) {
        Write-Host "Temporary Go contract environment retained at $temporaryRoot"
    } elseif (Test-Path -LiteralPath $temporaryRoot) {
        $resolvedTemp = (Resolve-Path -LiteralPath $temporaryRoot).Path
        $systemTemp = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
        if (-not $resolvedTemp.StartsWith($systemTemp, [StringComparison]::OrdinalIgnoreCase) -or (Split-Path $resolvedTemp -Leaf) -notlike 'musicparty-go-contract-*') {
            throw "Refusing to remove unexpected path $resolvedTemp"
        }
        Remove-Item -LiteralPath $resolvedTemp -Recurse -Force
    }
}
