# scripts/load_env.ps1
Get-Content .env | Where-Object { $_ -notmatch "^#" -and $_ -match "=" } | ForEach-Object {
    $name, $value = $_ -split "=", 2
    $value = $value.Trim('"')
    [System.Environment]::SetEnvironmentVariable($name.Trim(), $value, "Process")
}
Write-Host "Environment loaded!" -ForegroundColor Green