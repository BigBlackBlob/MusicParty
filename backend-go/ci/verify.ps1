$ErrorActionPreference = 'Stop'

gofmt -w .
git diff --exit-code -- .
if ($LASTEXITCODE -ne 0) {
    throw 'gofmt produced changes'
}
Write-Host 'gofmt check passed'

go mod tidy
git diff --exit-code -- go.mod go.sum
if ($LASTEXITCODE -ne 0) {
    throw 'go mod tidy produced changes'
}
Write-Host 'go.mod/go.sum check passed'

go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'go test failed' }
go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
if ($LASTEXITCODE -ne 0) { throw 'staticcheck failed' }
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
if ($LASTEXITCODE -ne 0) { throw 'govulncheck failed' }
go build ./...
if ($LASTEXITCODE -ne 0) { throw 'go build failed' }
