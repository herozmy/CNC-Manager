param(
    [switch]$SkipBuild,
    [switch]$CheckOnly
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = Split-Path -Parent $PSScriptRoot
$version = (Get-Content (Join-Path $root 'VERSION') -Raw).Trim()
$remote = (& git -C $root remote get-url origin).Trim()
if ($LASTEXITCODE -ne 0) { throw 'Unable to read the origin remote.' }
if ($remote -notmatch 'github\.com[/:]([^/]+)/([^/]+?)(?:\.git)?$') {
    throw "Origin is not a supported GitHub URL: $remote"
}
$owner = $Matches[1]
$repository = $Matches[2]

function Get-GitHubCredential {
    $start = New-Object System.Diagnostics.ProcessStartInfo
    $start.FileName = 'git.exe'
    $start.Arguments = 'credential fill'
    $start.UseShellExecute = $false
    $start.RedirectStandardInput = $true
    $start.RedirectStandardOutput = $true
    $start.RedirectStandardError = $true
    $start.CreateNoWindow = $true

    $process = New-Object System.Diagnostics.Process
    $process.StartInfo = $start
    if (-not $process.Start()) { throw 'Unable to start Git Credential Manager.' }
    $process.StandardInput.Write("protocol=https`nhost=github.com`n`n")
    $process.StandardInput.Close()
    $output = $process.StandardOutput.ReadToEnd()
    $null = $process.StandardError.ReadToEnd()
    $process.WaitForExit()
    if ($process.ExitCode -ne 0) { throw 'GitHub authorization is unavailable in Git Credential Manager.' }

    $credential = @{}
    foreach ($line in ($output -split "`r?`n")) {
        $separator = $line.IndexOf('=')
        if ($separator -gt 0) {
            $credential[$line.Substring(0, $separator)] = $line.Substring($separator + 1)
        }
    }
    if (-not $credential.ContainsKey('password') -or [string]::IsNullOrWhiteSpace($credential.password)) {
        throw 'Git Credential Manager did not return GitHub authorization.'
    }
    return $credential
}

$credential = Get-GitHubCredential
$headers = @{
    Accept                 = 'application/vnd.github+json'
    Authorization          = "Bearer $($credential.password)"
    'X-GitHub-Api-Version' = '2022-11-28'
    'User-Agent'           = 'CNC-Manager-Release-Script'
}

try {
    $account = Invoke-RestMethod -Uri 'https://api.github.com/user' -Headers $headers -Method Get
    $repoInfo = Invoke-RestMethod -Uri "https://api.github.com/repos/$owner/$repository" -Headers $headers -Method Get
    if (-not $repoInfo.permissions.push) {
        throw "GitHub account $($account.login) does not have push permission for $owner/$repository."
    }
    Write-Host "GitHub authorization OK: $($account.login) -> $owner/$repository" -ForegroundColor Green

    if ($CheckOnly) { exit 0 }

    $null = & git -C $root ls-remote --exit-code --tags origin "refs/tags/$version"
    if ($LASTEXITCODE -ne 0) { throw "Remote tag $version does not exist. Push the tag before publishing." }

    if (-not $SkipBuild) {
        & (Join-Path $PSScriptRoot 'build-release.cmd')
        if ($LASTEXITCODE -ne 0) { throw 'Release build failed.' }
    }

    $assetName = "cnccool-$version-windows-amd64.zip"
    $assetPath = Join-Path $root ".local\release\$assetName"
    if (-not (Test-Path $assetPath -PathType Leaf)) { throw "Release asset not found: $assetPath" }

    $release = $null
    try {
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$owner/$repository/releases/tags/$version" -Headers $headers -Method Get
    }
    catch {
        if ($_.Exception.Response.StatusCode.value__ -ne 404) { throw }
    }

    if ($null -eq $release) {
        $body = @{
            tag_name   = $version
            name       = "CNC Manager $version"
            body       = "Windows portable release. Extract the archive and run cnccool-server.exe."
            draft      = $false
            prerelease = $false
        } | ConvertTo-Json
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$owner/$repository/releases" -Headers $headers -Method Post -ContentType 'application/json; charset=utf-8' -Body $body
        Write-Host "Created GitHub Release $version"
    }

    if ($release.assets | Where-Object { $_.name -eq $assetName }) {
        throw "Release asset already exists: $assetName. It was not replaced."
    }

    $uploadURL = $release.upload_url -replace '\{\?name,label\}$', ''
    $encodedName = [System.Uri]::EscapeDataString($assetName)
    $bytes = [System.IO.File]::ReadAllBytes($assetPath)
    $asset = Invoke-RestMethod -Uri "$uploadURL?name=$encodedName" -Headers $headers -Method Post -ContentType 'application/zip' -Body $bytes
    Write-Host "Uploaded: $($asset.browser_download_url)" -ForegroundColor Green
}
finally {
    $credential.password = $null
    $headers.Authorization = $null
}
