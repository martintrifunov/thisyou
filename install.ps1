[CmdletBinding()]
param(
    [string]$InstallDir = $env:THISYOU_INSTALL_DIR
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

if ([System.Environment]::OSVersion.Platform -ne [System.PlatformID]::Win32NT) {
    throw "This installer supports Windows only."
}

if ([string]::IsNullOrWhiteSpace($InstallDir)) {
    if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
        $InstallDir = Join-Path $env:USERPROFILE "AppData\Local\Programs\thisyou"
    }
    else {
        $InstallDir = Join-Path $env:LOCALAPPDATA "Programs\thisyou"
    }
}

$Architecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
switch ($Architecture) {
    "x64" { $ArchiveName = "thisyou-windows-amd64.zip" }
    "arm64" { $ArchiveName = "thisyou-windows-arm64.zip" }
    default { throw "Unsupported Windows architecture: $Architecture" }
}

$ReleaseBase = "https://github.com/martintrifunov/thisyou/releases/latest/download"
$TemporaryDirectory = Join-Path ([System.IO.Path]::GetTempPath()) ("thisyou-install-" + [System.Guid]::NewGuid().ToString("N"))
$ArchivePath = Join-Path $TemporaryDirectory $ArchiveName
$ChecksumPath = Join-Path $TemporaryDirectory "SHA256SUMS"
$ExtractPath = Join-Path $TemporaryDirectory "extracted"

New-Item -ItemType Directory -Force -Path $TemporaryDirectory | Out-Null
try {
    Write-Host "Downloading the latest thisyou release for Windows $Architecture..."
    Invoke-WebRequest -UseBasicParsing -Uri "$ReleaseBase/$ArchiveName" -OutFile $ArchivePath
    Invoke-WebRequest -UseBasicParsing -Uri "$ReleaseBase/SHA256SUMS" -OutFile $ChecksumPath

    $ChecksumLine = Get-Content -LiteralPath $ChecksumPath | Where-Object { $_ -match "\s+$([regex]::Escape($ArchiveName))$" } | Select-Object -First 1
    if ([string]::IsNullOrWhiteSpace($ChecksumLine)) {
        throw "The release checksum for $ArchiveName is missing."
    }

    $ExpectedHash = ($ChecksumLine -split "\s+")[0]
    $ActualHash = (Get-FileHash -LiteralPath $ArchivePath -Algorithm SHA256).Hash
    if (-not [string]::Equals($ExpectedHash, $ActualHash, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "The downloaded archive failed SHA-256 verification."
    }

    Expand-Archive -LiteralPath $ArchivePath -DestinationPath $ExtractPath -Force
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $TargetPath = Join-Path $InstallDir "thisyou.exe"
    Copy-Item -LiteralPath (Join-Path $ExtractPath "thisyou.exe") -Destination $TargetPath -Force

    $NormalizedInstallDir = [System.IO.Path]::GetFullPath($InstallDir).TrimEnd([System.IO.Path]::DirectorySeparatorChar)
    $UserPath = [System.Environment]::GetEnvironmentVariable("Path", "User")
    $UserEntries = @($UserPath -split ";" | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    $AlreadyOnUserPath = $UserEntries | Where-Object {
        [string]::Equals(
            $_.Trim().TrimEnd([System.IO.Path]::DirectorySeparatorChar),
            $NormalizedInstallDir,
            [System.StringComparison]::OrdinalIgnoreCase
        )
    }
    if (-not $AlreadyOnUserPath) {
        [System.Environment]::SetEnvironmentVariable("Path", (($UserEntries + $NormalizedInstallDir) -join ";"), "User")
    }

    $ProcessEntries = @($env:Path -split ";")
    $AlreadyOnProcessPath = $ProcessEntries | Where-Object {
        [string]::Equals(
            $_.Trim().TrimEnd([System.IO.Path]::DirectorySeparatorChar),
            $NormalizedInstallDir,
            [System.StringComparison]::OrdinalIgnoreCase
        )
    }
    if (-not $AlreadyOnProcessPath) {
        $env:Path = "$env:Path;$NormalizedInstallDir"
    }

    Write-Host "Installed thisyou to $TargetPath"
    & $TargetPath --version
}
finally {
    if (Test-Path -LiteralPath $TemporaryDirectory) {
        Remove-Item -LiteralPath $TemporaryDirectory -Recurse -Force
    }
}
