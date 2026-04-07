# PostgreSQL Self-Permission Manager - Build Script
# Windows PowerShell Build Script

Write-Host "=== PostgreSQL Self-Permission Manager - Build ===" -ForegroundColor Cyan
Write-Host ""

# Set build parameters
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "1"

# Build configuration
$OUTPUT_NAME = "lock-ipos.exe"
$LD_FLAGS = "-s -w"  # Strip debug info for smaller binary

Write-Host "Build Configuration:" -ForegroundColor Yellow
Write-Host "  GOOS:     $env:GOOS" -ForegroundColor Gray
Write-Host "  GOARCH:   $env:GOARCH" -ForegroundColor Gray
Write-Host "  CGO:      $env:CGO_ENABLED" -ForegroundColor Gray
Write-Host "  Output:   $OUTPUT_NAME" -ForegroundColor Gray
Write-Host ""

# Build
Write-Host "Building..." -ForegroundColor Green
go build -ldflags "$LD_FLAGS" -o $OUTPUT_NAME .

if ($LASTEXITCODE -eq 0) {
	Write-Host ""
	Write-Host "=== Build Successful! ===" -ForegroundColor Green
	Write-Host ""

	# Get file size
	$fileSize = (Get-Item $OUTPUT_NAME).Length
	$fileSizeMB = [math]::Round($fileSize / 1MB, 2)
	Write-Host "Binary Size: $fileSizeMB MB" -ForegroundColor Cyan

	Write-Host ""
	Write-Host "Run with: .\$OUTPUT_NAME" -ForegroundColor Yellow
} else {
	Write-Host ""
	Write-Host "=== Build Failed! ===" -ForegroundColor Red
	exit 1
}
