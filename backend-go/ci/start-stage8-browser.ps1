param([int]$Port = 18081)

$ErrorActionPreference = 'Stop'
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$backendRoot = Join-Path $repositoryRoot 'backend-go'
$temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) ('musicparty-stage8-browser-' + [Guid]::NewGuid().ToString('N'))
$acceptanceRoot = Join-Path $backendRoot 'acceptance\stage8'
New-Item -ItemType Directory $temporaryRoot | Out-Null
New-Item -ItemType Directory $acceptanceRoot -Force | Out-Null

$origin = "http://127.0.0.1:$Port"
$env:SERVER_PORT = [string]$Port
$env:APP_MODE = 'server'
$env:BASE_URL = $origin
$env:ALLOWED_ORIGINS = $origin
$env:DB_ENABLED = 'true'
$env:DB_INIT_SCHEMA = 'true'
$env:DB_PATH = Join-Path $temporaryRoot 'musicparty.db'
$env:LOCAL_LIBRARY_PATH = Join-Path $temporaryRoot 'local'
$env:CACHE_PATH = Join-Path $temporaryRoot 'cache'
$env:STATIC_PATH = Join-Path $repositoryRoot 'music-party-web\dist'
$env:BOOTSTRAP_ADMIN_USERNAME = 'stage8-admin'
$env:BOOTSTRAP_ADMIN_PASSWORD = 'Stage8-Password-2026!'
$env:AUTH_SECURE_COOKIES = 'false'
$env:AUTH_RATE_LIMIT_ENABLED = 'false'
$env:ROOM_ACCESS_TOKEN_SECRET = 'stage8-browser-secret-32-bytes'
$env:NETEASE_API_URL = 'http://127.0.0.1:9'
$env:YOUTUBE_ENABLED = 'false'
$env:NAVIDROME_ENABLED = 'false'
$env:SQUIDIFY_ENABLED = 'false'

$executable = Join-Path $temporaryRoot 'musicparty.exe'
Push-Location $backendRoot
try {
    & go build -o $executable ./cmd/musicparty
    if ($LASTEXITCODE -ne 0) { throw "Go build failed with exit code $LASTEXITCODE" }
} finally { Pop-Location }

$stdout = Join-Path $temporaryRoot 'stdout.log'
$stderr = Join-Path $temporaryRoot 'stderr.log'
$process = Start-Process -FilePath $executable -WorkingDirectory $temporaryRoot -PassThru -WindowStyle Hidden -RedirectStandardOutput $stdout -RedirectStandardError $stderr
Set-Content -Path (Join-Path $acceptanceRoot 'browser-process.txt') -Value @("PID=$($process.Id)", "TEMP=$temporaryRoot", "PORT=$Port")
Start-Sleep -Seconds 2
$health = Invoke-RestMethod "$origin/actuator/health" -NoProxy
if ($health.status -ne 'UP') { throw 'Browser acceptance backend is not healthy' }
Write-Host "Browser acceptance backend started at $origin with PID $($process.Id)"
