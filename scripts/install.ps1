$ErrorActionPreference = "Stop"

$repository = "https://github.com/wnma3mz/yuxin"
$architecture = if ($env:YUXIN_ARCHITECTURE) {
    $env:YUXIN_ARCHITECTURE
} else {
    [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
}
$releaseArchitecture = switch ($architecture.ToUpperInvariant()) {
    "X64" { "x86_64" }
    "AMD64" { "x86_64" }
    "ARM64" { "arm64" }
    default { throw "暂不支持 Windows $architecture。" }
}

$asset = "yuxin-windows-$releaseArchitecture.zip"
$base = if ($env:YUXIN_RELEASE_BASE) {
    $env:YUXIN_RELEASE_BASE.TrimEnd("/", "\")
} else {
    "$repository/releases/latest/download"
}
$temporary = Join-Path ([System.IO.Path]::GetTempPath()) ("yuxin-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $temporary | Out-Null

function Receive-ReleaseFile {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Name,
        [Parameter(Mandatory = $true)]
        [string] $Destination
    )

    if (Test-Path -LiteralPath $base -PathType Container) {
        Copy-Item -LiteralPath (Join-Path $base $Name) -Destination $Destination
        return
    }

    $baseUri = $null
    if ([Uri]::TryCreate($base, [UriKind]::Absolute, [ref] $baseUri) -and $baseUri.IsFile) {
        Copy-Item -LiteralPath (Join-Path $baseUri.LocalPath $Name) -Destination $Destination
        return
    }

    Invoke-WebRequest -UseBasicParsing "$base/$Name" -OutFile $Destination
}

try {
    $archive = Join-Path $temporary $asset
    $checksumFile = "$archive.sha256"
    Write-Host "正在下载 Yuxin 最新正式版（windows/$releaseArchitecture）…"
    Receive-ReleaseFile -Name $asset -Destination $archive
    Receive-ReleaseFile -Name "$asset.sha256" -Destination $checksumFile

    $expected = ((Get-Content $checksumFile -First 1) -split "\s+")[0].ToLowerInvariant()
    $actual = (Get-FileHash -Algorithm SHA256 $archive).Hash.ToLowerInvariant()
    if ([string]::IsNullOrWhiteSpace($expected) -or $actual -ne $expected) {
        throw "SHA-256 校验失败。"
    }

    $expanded = Join-Path $temporary "release"
    Expand-Archive -Path $archive -DestinationPath $expanded
    $executable = Get-ChildItem -Path $expanded -Filter "yuxin.exe" -File -Recurse | Select-Object -First 1
    if ($null -eq $executable) {
        throw "发布包中缺少 yuxin.exe。"
    }

    $installDirectory = if ($env:YUXIN_INSTALL_DIR) {
        $env:YUXIN_INSTALL_DIR
    } else {
        Join-Path $env:LOCALAPPDATA "Yuxin\bin"
    }
    New-Item -ItemType Directory -Force -Path $installDirectory | Out-Null
    Copy-Item -Force $executable.FullName (Join-Path $installDirectory "yuxin.exe")

    if (-not $env:YUXIN_SKIP_PATH) {
        $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
        $entries = @($userPath -split ";" | Where-Object { $_ })
        if ($entries -notcontains $installDirectory) {
            $newPath = (@($entries) + $installDirectory) -join ";"
            [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
            Write-Host "已将 $installDirectory 加入用户 PATH，请重新打开终端。"
        }
    }
    Write-Host "已安装到 $installDirectory\yuxin.exe"
} finally {
    Remove-Item -Recurse -Force $temporary -ErrorAction SilentlyContinue
}
