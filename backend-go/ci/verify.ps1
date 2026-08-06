$ErrorActionPreference = 'Stop'

$unformatted = @(gofmt -l .)
if ($LASTEXITCODE -ne 0) { throw 'gofmt failed' }
if ($unformatted.Count -gt 0) {
    throw "gofmt is required for:`n$($unformatted -join "`n")"
}

go run ./cmd/contractgen -check -repo ..
if ($LASTEXITCODE -ne 0) { throw 'contract drift check failed' }
go test ./internal/contractspec
if ($LASTEXITCODE -ne 0) { throw 'contract tests failed' }
go mod tidy
if ($LASTEXITCODE -ne 0) { throw 'go mod tidy failed' }
git diff --exit-code -- go.mod go.sum
if ($LASTEXITCODE -ne 0) { throw 'go mod tidy produced changes' }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }
go test -count=1 ./...
if ($LASTEXITCODE -ne 0) { throw 'go test failed' }
go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
if ($LASTEXITCODE -ne 0) { throw 'staticcheck failed' }
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
if ($LASTEXITCODE -ne 0) { throw 'govulncheck failed' }
$env:CGO_ENABLED = '1'
go test -race -count=1 ./...
if ($LASTEXITCODE -ne 0) { throw 'race tests failed' }
$env:CGO_ENABLED = '0'
go build -trimpath ./...
if ($LASTEXITCODE -ne 0) { throw 'go build failed' }
