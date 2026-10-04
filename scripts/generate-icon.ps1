$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Add-Type -AssemblyName System.Drawing

$root = Split-Path -Parent $PSScriptRoot
$output = Join-Path $root 'assets\cnccool.ico'
$embeddedOutput = Join-Path $root 'backend\internal\tray\cnccool.ico'
$preview = Join-Path $root 'assets\cnccool-icon.png'
$sizes = @(16, 20, 24, 32, 48, 64, 128, 256)
$frames = New-Object System.Collections.Generic.List[byte[]]

foreach ($size in $sizes) {
    $bitmap = New-Object System.Drawing.Bitmap($size, $size, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
    $graphics.Clear([System.Drawing.Color]::Transparent)

    $scale = $size / 256.0
    $background = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::FromArgb(255, 12, 39, 59))
    $metal = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::FromArgb(255, 224, 237, 245))
    $shadow = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::FromArgb(255, 126, 157, 177))
    $accent = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::FromArgb(255, 46, 211, 198))
    $cut = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::FromArgb(255, 19, 61, 88))

    $graphics.FillRectangle($background, [single](8*$scale), [single](8*$scale), [single](240*$scale), [single](240*$scale))
    $graphics.FillRectangle($accent, [single](38*$scale), [single](190*$scale), [single](180*$scale), [single](30*$scale))
    $graphics.FillRectangle($shadow, [single](60*$scale), [single](161*$scale), [single](136*$scale), [single](29*$scale))
    $graphics.FillRectangle($metal, [single](78*$scale), [single](40*$scale), [single](100*$scale), [single](44*$scale))

    $tool = New-Object System.Drawing.Drawing2D.GraphicsPath
    $tool.AddPolygon([System.Drawing.PointF[]]@(
        (New-Object System.Drawing.PointF([single](94*$scale), [single](84*$scale))),
        (New-Object System.Drawing.PointF([single](162*$scale), [single](84*$scale))),
        (New-Object System.Drawing.PointF([single](152*$scale), [single](148*$scale))),
        (New-Object System.Drawing.PointF([single](128*$scale), [single](176*$scale))),
        (New-Object System.Drawing.PointF([single](104*$scale), [single](148*$scale)))
    ))
    $graphics.FillPath($metal, $tool)
    $graphics.FillRectangle($cut, [single](121*$scale), [single](104*$scale), [single](14*$scale), [single](62*$scale))

    $stream = New-Object System.IO.MemoryStream
    $bitmap.Save($stream, [System.Drawing.Imaging.ImageFormat]::Png)
    $frames.Add($stream.ToArray())
    if ($size -eq 256) {
        [System.IO.File]::WriteAllBytes($preview, $stream.ToArray())
    }

    $stream.Dispose()
    $tool.Dispose()
    $background.Dispose(); $metal.Dispose(); $shadow.Dispose(); $accent.Dispose(); $cut.Dispose()
    $graphics.Dispose()
    $bitmap.Dispose()
}

$file = [System.IO.File]::Create($output)
$writer = New-Object System.IO.BinaryWriter($file)
try {
    $writer.Write([uint16]0)
    $writer.Write([uint16]1)
    $writer.Write([uint16]$frames.Count)
    $offset = 6 + 16 * $frames.Count
    for ($index = 0; $index -lt $frames.Count; $index++) {
        $size = $sizes[$index]
        $writer.Write([byte]$(if ($size -eq 256) { 0 } else { $size }))
        $writer.Write([byte]$(if ($size -eq 256) { 0 } else { $size }))
        $writer.Write([byte]0)
        $writer.Write([byte]0)
        $writer.Write([uint16]1)
        $writer.Write([uint16]32)
        $writer.Write([uint32]$frames[$index].Length)
        $writer.Write([uint32]$offset)
        $offset += $frames[$index].Length
    }
    foreach ($frame in $frames) {
        $writer.Write($frame)
    }
}
finally {
    $writer.Dispose()
    $file.Dispose()
}

Write-Host "Generated $output"
Copy-Item -LiteralPath $output -Destination $embeddedOutput -Force
Write-Host "Generated $embeddedOutput"
