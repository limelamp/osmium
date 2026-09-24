# Osmium installer for Windows.
#
#   irm https://osmiummc.com/install.ps1 | iex
#
# Environment:
#   OSMIUM_VERSION      a release tag to install, e.g. v0.1.1 (default: latest)
#   OSMIUM_INSTALL_DIR  where to put osmium.exe (default: %LOCALAPPDATA%\Programs\osmium)

& {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue'
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

    $repo = 'limelamp/osmium'

    # A 32-bit PowerShell on 64-bit Windows reports the real CPU in PROCESSOR_ARCHITEW6432.
    $cpu = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
    $arch = switch ($cpu) {
        'AMD64' { 'amd64' }
        'ARM64' { 'arm64' }
        default { throw "Unsupported architecture: $cpu. Try: go install github.com/$repo@latest" }
    }

    $tag = $env:OSMIUM_VERSION
    if (-not $tag) {
        $tag = (Invoke-RestMethod -UseBasicParsing "https://api.github.com/repos/$repo/releases/latest").tag_name
    }
    if (-not $tag.StartsWith('v')) { $tag = "v$tag" }
    $version = $tag.Substring(1)

    $dir = if ($env:OSMIUM_INSTALL_DIR) { $env:OSMIUM_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\osmium' }
    $archive = "osmium_${version}_windows_${arch}.zip"
    $base = "https://github.com/$repo/releases/download/$tag"

    Write-Host "Installing osmium $version (windows/$arch)"

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ("osmium-" + [Guid]::NewGuid())
    New-Item -ItemType Directory -Path $tmp | Out-Null

    try {
        foreach ($name in $archive, 'checksums.txt') {
            try {
                Invoke-WebRequest -UseBasicParsing "$base/$name" -OutFile (Join-Path $tmp $name)
            } catch {
                throw "Could not download $base/$name"
            }
        }

        $line = Get-Content (Join-Path $tmp 'checksums.txt') | Where-Object { ($_ -split '\s+')[1] -eq $archive }
        if (-not $line) { throw "$archive is not listed in checksums.txt" }
        $want = ($line -split '\s+')[0]
        $got = (Get-FileHash -Algorithm SHA256 (Join-Path $tmp $archive)).Hash
        if ($got -ne $want) { throw "Checksum mismatch for $archive" }

        Expand-Archive -Path (Join-Path $tmp $archive) -DestinationPath (Join-Path $tmp 'x') -Force

        New-Item -ItemType Directory -Path $dir -Force | Out-Null
        try {
            Copy-Item (Join-Path $tmp 'x\osmium.exe') (Join-Path $dir 'osmium.exe') -Force
        } catch {
            throw "Could not replace $dir\osmium.exe. Close any running osmium and try again."
        }
    } finally {
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    }

    Write-Host "Installed to $dir\osmium.exe"
    Write-Host ''

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $entries = if ($userPath) { $userPath -split ';' } else { @() }
    if ($entries -notcontains $dir) {
        $newPath = (@($entries | Where-Object { $_ }) + $dir) -join ';'
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
        Write-Host "Added $dir to your user PATH. Open a new terminal to use it."
        Write-Host ''
    }
    if (($env:Path -split ';') -notcontains $dir) { $env:Path = "$env:Path;$dir" }

    if (-not (Get-Command java -ErrorAction SilentlyContinue)) {
        Write-Host 'No java found. A Minecraft server needs Java 21 for Minecraft 1.20.5 and newer:' -ForegroundColor DarkGray
        Write-Host 'https://osmiummc.com/docs/getting-started/installation' -ForegroundColor DarkGray
        Write-Host ''
    }

    Write-Host 'Next:'
    Write-Host '  osmium completion install   # how to set up tab completion'
    Write-Host '  osmium                      # open the interface'
}
