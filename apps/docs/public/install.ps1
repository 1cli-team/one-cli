# install.ps1 - One CLI installer for Windows 10/11 x64.
#
#   irm https://1cli.dev/install.ps1 | iex
#
# Environment variables:
#   ONE_VERSION          Pin a release tag such as v0.2.0.
#   ONE_INSTALL_DIR      Install directory.
#   ONE_FORCE            Set to 1 to reinstall, overwrite, or downgrade.
#   ONE_REPO_URL         GitHub repository URL.
#   ONE_RELEASE_BASE_URL Release asset base URL.
#   ONE_LATEST_URL       Latest-release redirect URL.
#   ONE_SKIP_VERIFY      Set to 1 to skip SHA256 verification.
#   ONE_NO_PATH_UPDATE   Set to 1 to leave the user PATH unchanged.

[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version 2.0

function Get-OneEnv {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Default
    )

    $value = [Environment]::GetEnvironmentVariable($Name)
    if ([string]::IsNullOrWhiteSpace($value)) {
        return $Default
    }
    return $value.Trim()
}

function Resolve-OneVersion {
    param(
        [Parameter(Mandatory = $true)][string]$LatestUrl,
        [Parameter(Mandatory = $true)][hashtable]$Headers
    )

    $response = Invoke-WebRequest -Uri $LatestUrl -MaximumRedirection 5 -UseBasicParsing -Headers $Headers
    $effectiveUri = $null
    if ($response.BaseResponse -and $response.BaseResponse.PSObject.Properties["ResponseUri"]) {
        $effectiveUri = $response.BaseResponse.ResponseUri.AbsoluteUri
    } elseif ($response.BaseResponse -and $response.BaseResponse.PSObject.Properties["RequestMessage"]) {
        $effectiveUri = $response.BaseResponse.RequestMessage.RequestUri.AbsoluteUri
    }
    if ([string]::IsNullOrWhiteSpace($effectiveUri)) {
        throw "Could not resolve the latest release tag from $LatestUrl. Set ONE_VERSION=vX.Y.Z and retry."
    }
    return ([Uri]$effectiveUri).Segments[-1].Trim("/")
}

function ConvertTo-OneVersion {
    param([Parameter(Mandatory = $true)][string]$Value)

    $normalized = $Value.Trim().TrimStart("v").Split("-")[0].Split("+")[0]
    try {
        return [Version]$normalized
    } catch {
        throw "Invalid version '$Value'; expected vX.Y.Z."
    }
}

function Get-InstalledOneVersion {
    param([Parameter(Mandatory = $true)][string]$Path)

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return $null
    }
    try {
        $output = @(& $Path --version 2>$null)
        if ($output.Count -eq 0 -or [string]::IsNullOrWhiteSpace([string]$output[0])) {
            return $null
        }
        $value = ([string]$output[0]).Trim()
        if (-not $value.StartsWith("v")) {
            $value = "v$value"
        }
        return $value
    } catch {
        return $null
    }
}

function Add-OneToUserPath {
    param([Parameter(Mandatory = $true)][string]$InstallDir)

    if ([Environment]::GetEnvironmentVariable("ONE_NO_PATH_UPDATE") -eq "1") {
        return
    }
    $normalized = $InstallDir.TrimEnd("\", "/")
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $userEntries = @($userPath -split ";" | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    $alreadyPresent = $false
    foreach ($entry in $userEntries) {
        if ([string]::Equals($entry.Trim().TrimEnd("\", "/"), $normalized, [StringComparison]::OrdinalIgnoreCase)) {
            $alreadyPresent = $true
            break
        }
    }
    if (-not $alreadyPresent) {
        $newUserPath = if ([string]::IsNullOrWhiteSpace($userPath)) {
            $InstallDir
        } else {
            $userPath.TrimEnd(";") + ";" + $InstallDir
        }
        [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
        Write-Host "one-cli: added $InstallDir to your user PATH."
    }

    $processEntries = @($env:Path -split ";")
    $processHasEntry = $false
    foreach ($entry in $processEntries) {
        if ([string]::Equals($entry.Trim().TrimEnd("\", "/"), $normalized, [StringComparison]::OrdinalIgnoreCase)) {
            $processHasEntry = $true
            break
        }
    }
    if (-not $processHasEntry) {
        $env:Path = $InstallDir + ";" + $env:Path
    }
}

function Install-One {
    if ($env:OS -ne "Windows_NT") {
        throw "install.ps1 supports Windows only. On macOS/Linux use: curl -fsSL https://1cli.dev/install.sh | bash"
    }

    $nativeArch = if ($env:PROCESSOR_ARCHITEW6432) {
        $env:PROCESSOR_ARCHITEW6432
    } else {
        $env:PROCESSOR_ARCHITECTURE
    }
    if ($nativeArch -ne "AMD64") {
        throw "Unsupported Windows architecture '$nativeArch'. One CLI currently publishes Windows x64 builds."
    }

    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    $headers = @{ "User-Agent" = "one-cli-installer" }
    $repoUrl = (Get-OneEnv "ONE_REPO_URL" "https://github.com/1cli-team/one-cli").TrimEnd("/")
    $releaseBaseUrl = (Get-OneEnv "ONE_RELEASE_BASE_URL" "$repoUrl/releases/download").TrimEnd("/")
    $latestUrl = Get-OneEnv "ONE_LATEST_URL" "$repoUrl/releases/latest"
    $localAppData = [Environment]::GetFolderPath("LocalApplicationData")
    $installDir = Get-OneEnv "ONE_INSTALL_DIR" (Join-Path $localAppData "Programs\one\bin")
    $force = (Get-OneEnv "ONE_FORCE" "0") -eq "1"
    $skipVerify = (Get-OneEnv "ONE_SKIP_VERIFY" "0") -eq "1"

    $version = [Environment]::GetEnvironmentVariable("ONE_VERSION")
    if ([string]::IsNullOrWhiteSpace($version)) {
        Write-Host "one-cli: resolving latest release..."
        $version = Resolve-OneVersion $latestUrl $headers
    }
    $version = $version.Trim()
    if ($version -notmatch "^v[0-9]+\.[0-9]+\.[0-9]+(?:[-+].+)?$") {
        throw "Invalid ONE_VERSION '$version'; expected vX.Y.Z."
    }

    $target = Join-Path $installDir "one.exe"
    $installed = Get-InstalledOneVersion $target
    if ($installed) {
        $comparison = (ConvertTo-OneVersion $version).CompareTo((ConvertTo-OneVersion $installed))
        if ($comparison -eq 0 -and -not $force) {
            Write-Host "one-cli: $target is already at $installed; skipping."
            Add-OneToUserPath $installDir
            return
        }
        if ($comparison -lt 0 -and -not $force) {
            throw "Downgrade blocked: installed $installed is newer than target $version. Set ONE_FORCE=1 to continue."
        }
        if ($comparison -gt 0) {
            Write-Host "one-cli: upgrading $installed to $version."
        }
    } elseif ((Test-Path -LiteralPath $target) -and -not $force) {
        throw "$target exists but its version cannot be read. Set ONE_FORCE=1 to overwrite it."
    }

    $archive = "one-cli_windows_amd64.zip"
    $archiveUrl = "$releaseBaseUrl/$version/$archive"
    $checksumUrl = "$releaseBaseUrl/$version/checksums.txt"
    $tempDir = Join-Path ([IO.Path]::GetTempPath()) ("one-cli-installer-" + [Guid]::NewGuid().ToString("N"))
    [IO.Directory]::CreateDirectory($tempDir) | Out-Null

    try {
        $archivePath = Join-Path $tempDir $archive
        $checksumsPath = Join-Path $tempDir "checksums.txt"
        Write-Host "one-cli: downloading $archive ($version)..."
        Invoke-WebRequest -Uri $archiveUrl -OutFile $archivePath -UseBasicParsing -Headers $headers

        if ($skipVerify) {
            Write-Warning "one-cli: SHA256 verification skipped because ONE_SKIP_VERIFY=1."
        } else {
            Invoke-WebRequest -Uri $checksumUrl -OutFile $checksumsPath -UseBasicParsing -Headers $headers
            $checksumLine = Get-Content -LiteralPath $checksumsPath | Where-Object {
                $parts = $_.Trim() -split "\s+", 2
                $parts.Count -eq 2 -and $parts[1].TrimStart("*") -eq $archive
            } | Select-Object -First 1
            if (-not $checksumLine) {
                throw "checksums.txt has no entry for $archive."
            }
            $expected = ($checksumLine.Trim() -split "\s+", 2)[0].ToLowerInvariant()
            $actual = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
            if ($expected -ne $actual) {
                throw "SHA256 mismatch for $archive. Expected $expected, got $actual."
            }
        }

        $extractDir = Join-Path $tempDir "extracted"
        Expand-Archive -LiteralPath $archivePath -DestinationPath $extractDir -Force
        $candidate = Join-Path $extractDir "one.exe"
        if (-not (Test-Path -LiteralPath $candidate -PathType Leaf)) {
            throw "$archive does not contain one.exe at its top level."
        }

        [IO.Directory]::CreateDirectory($installDir) | Out-Null
        $staged = Join-Path $installDir ("one.exe." + [Guid]::NewGuid().ToString("N") + ".tmp")
        try {
            [IO.File]::Copy($candidate, $staged, $true)
            if (Test-Path -LiteralPath $target) {
                [IO.File]::Replace($staged, $target, $null)
            } else {
                [IO.File]::Move($staged, $target)
            }
        } finally {
            if (Test-Path -LiteralPath $staged) {
                Remove-Item -LiteralPath $staged -Force
            }
        }
    } finally {
        if (Test-Path -LiteralPath $tempDir) {
            Remove-Item -LiteralPath $tempDir -Recurse -Force
        }
    }

    Add-OneToUserPath $installDir
    $reported = (& $target --version | Select-Object -First 1)
    Write-Host "one-cli: installed $target ($reported)."
    Write-Host "Open a new terminal if another shell cannot find 'one', then run: one skills install"
}

Install-One
