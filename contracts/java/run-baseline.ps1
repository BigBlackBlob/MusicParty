param(
    [ValidateSet('capture', 'compare')]
    [string]$Mode = 'compare',
    [string]$JarPath = '',
    [switch]$KeepTemporaryFiles
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) ('musicparty-contract-' + [Guid]::NewGuid().ToString('N'))
$javaProcess = $null
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
        ALLOWED_ORIGINS = ''
        DB_ENABLED = 'true'
        DB_INIT_SCHEMA = 'true'
        DB_PATH = (Join-Path $temporaryRoot 'musicparty.db')
        LOCAL_LIBRARY_ENABLED = 'true'
        LOCAL_LIBRARY_PATH = (Join-Path $temporaryRoot 'local-library')
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
    if (-not $JarPath) {
        Push-Location $repositoryRoot
        try {
            & (Join-Path $repositoryRoot 'mvnw.cmd') -q -DskipTests package
            if ($LASTEXITCODE -ne 0) { throw "Maven package failed with exit code $LASTEXITCODE" }
        } finally {
            Pop-Location
        }
        $jar = Get-ChildItem (Join-Path $repositoryRoot 'target') -Filter '*.jar' -File |
            Where-Object { $_.Name -notmatch '\.original$' -and $_.Name -notmatch '^original-' } |
            Sort-Object LastWriteTime -Descending |
            Select-Object -First 1
        if (-not $jar) { throw 'No runnable Spring Boot JAR was produced' }
        $JarPath = $jar.FullName
    } else {
        $JarPath = (Resolve-Path $JarPath).Path
    }

    $port = Get-FreeTcpPort
    Set-IsolatedEnvironment -Port $port
    $stdout = Join-Path $temporaryRoot 'java.stdout.log'
    $stderr = Join-Path $temporaryRoot 'java.stderr.log'
    $javaProcess = Start-Process -FilePath 'java' -ArgumentList @('-jar', $JarPath) -WorkingDirectory $temporaryRoot -PassThru -WindowStyle Hidden -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $baseUrl = "http://127.0.0.1:$port"
    $deadline = [DateTime]::UtcNow.AddSeconds(90)
    do {
        if ($javaProcess.HasExited) {
            throw "Java baseline exited early. stdout=$stdout stderr=$stderr"
        }
        try {
            $health = Invoke-RestMethod -Uri "$baseUrl/actuator/health" -TimeoutSec 2 -NoProxy
            if ($health.status -eq 'UP') { break }
        } catch {
            Start-Sleep -Milliseconds 250
        }
    } while ([DateTime]::UtcNow -lt $deadline)
    if ([DateTime]::UtcNow -ge $deadline) { throw "Java baseline did not become healthy. stdout=$stdout stderr=$stderr" }

    Push-Location (Join-Path $repositoryRoot 'backend-go')
    try {
        & go run ./cmd/contractprobe -mode $Mode -repo .. -base-url $baseUrl -admin-user contract-admin -admin-password 'Contract-Password-123!'
        if ($LASTEXITCODE -ne 0) { throw "contractprobe failed with exit code $LASTEXITCODE" }
    } finally {
        Pop-Location
    }
} finally {
    if ($javaProcess -and -not $javaProcess.HasExited) {
        Stop-Process -Id $javaProcess.Id -Force
        $javaProcess.WaitForExit()
    }
    Restore-Environment
    if ($KeepTemporaryFiles) {
        Write-Host "Temporary baseline retained at $temporaryRoot"
    } elseif (Test-Path -LiteralPath $temporaryRoot) {
        $resolvedTemp = (Resolve-Path -LiteralPath $temporaryRoot).Path
        $systemTemp = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
        if (-not $resolvedTemp.StartsWith($systemTemp, [StringComparison]::OrdinalIgnoreCase) -or (Split-Path $resolvedTemp -Leaf) -notlike 'musicparty-contract-*') {
            throw "Refusing to remove unexpected path $resolvedTemp"
        }
        Remove-Item -LiteralPath $resolvedTemp -Recurse -Force
    }
}
