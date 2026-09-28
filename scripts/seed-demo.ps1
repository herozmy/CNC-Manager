<#
  seed-demo.ps1 —— 灌入一套演示数据

  第一次打开界面时不是一片空白，方便直观看到整套结构：
    两个图纸，各自带工序，工序下带程序、刀具刀补表和 NC 版本。

  前提：后端已在运行（先执行 run-backend.cmd）。

  用法：
    scripts\seed-demo.cmd
#>
param(
    [string]$BaseUrl = 'http://127.0.0.1:8080'
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '_common.ps1')
Set-ApiBase $BaseUrl

Write-Host 'CNC 加工程序管理系统 —— 灌入演示数据' -ForegroundColor Yellow

try {
    $meta = Invoke-Api -Method Get -Path '/api/meta'
    Info "后端版本 $($meta.version)，数据目录 $($meta.dataDir)"
} catch {
    Write-Host "连不上后端：$($_.Exception.Message)" -ForegroundColor Red
    Write-Host '请先启动后端：scripts\run-backend.cmd' -ForegroundColor Yellow
    exit 1
}

# --- 先建两份机台字典（工序要引用它）---------------------------------------
Step '建立机台字典'
$machineIds = @{}
$machines = @(
    @{ code = 'CK6150'; name = '数控车床 CK6150'; controller = 'FANUC 0i-TF' },
    @{ code = 'VMC850'; name = '加工中心 VMC850'; controller = 'FANUC 0i-MF' },
    @{ code = 'XK714'; name = '数控铣床 XK714'; controller = 'SIEMENS 828D' }
)
foreach ($m in $machines) {
    try {
        $created = Invoke-Api -Method Post -Path '/api/machines' -Body @{
            code = $m.code; name = $m.name; controller = $m.controller; remark = ''
        }
        $machineIds[$m.code] = $created.id
        Ok "机台 $($m.code) 已建立 id=$($created.id)"
    } catch {
        # 已存在就查出来复用，保证脚本可以重复执行
        $existing = @(Invoke-Api -Method Get -Path '/api/machines') | Where-Object { $_.code -eq $m.code }
        if ($existing) {
            $machineIds[$m.code] = $existing[0].id
            Info "机台 $($m.code) 已存在，复用 id=$($existing[0].id)"
        } else {
            Bad "建立机台 $($m.code) 失败：$($_.Exception.Message)"
        }
    }
}

# --- 刀具字典 --------------------------------------------------------------
Step '建立刀具字典'
$toolDict = @(
    @{ toolNo = 'T01'; name = '外圆粗车刀'; spec = 'CNMG120408'; toolType = '车刀' },
    @{ toolNo = 'T02'; name = '外圆精车刀'; spec = 'DNMG150404'; toolType = '车刀' },
    @{ toolNo = 'T03'; name = '切槽刀'; spec = '3mm'; toolType = '车刀' },
    @{ toolNo = 'T04'; name = 'Ø12 立铣刀'; spec = 'Ø12 4刃'; toolType = '铣刀' },
    @{ toolNo = 'T05'; name = 'Ø8 立铣刀'; spec = 'Ø8 4刃'; toolType = '铣刀' },
    @{ toolNo = 'T06'; name = 'Ø10 钻头'; spec = 'Ø10 HSS'; toolType = '钻头' },
    @{ toolNo = 'T07'; name = 'M8 丝锥'; spec = 'M8x1.25'; toolType = '丝锥' }
)
foreach ($t in $toolDict) {
    try {
        Invoke-Api -Method Post -Path '/api/tools' -Body @{
            toolNo = $t.toolNo; name = $t.name; spec = $t.spec
            toolType = $t.toolType; remark = ''
        } | Out-Null
    } catch {
        # 重复就跳过
    }
}
$allTools = @(Invoke-Api -Method Get -Path '/api/tools')
Ok "刀具字典共 $($allTools.Count) 条"

function ToolIdOf([string]$toolNo) {
    $hit = @($allTools | Where-Object { $_.toolNo -eq $toolNo })
    if ($hit.Count -gt 0) { return $hit[0].id }
    return $null
}

# --- 图纸 1：支架（车 + 铣，两个程序）--------------------------------------
Step '建立图纸 A-1001 支架（一序车 / 二序铣）'
try {
    $d1 = Invoke-Api -Method Post -Path '/api/drawings' -Body @{
        drawingNo = 'A-1001'; name = '主轴支架'; customer = '常州精工'
        material = '45#'; drawingVersion = 'B'; remark = '2024 年改版，法兰厚度 +2mm'
    }
    Ok "图纸 A-1001 id=$($d1.id)"

    $d1op10 = Invoke-Api -Method Post -Path "/api/drawings/$($d1.id)/operations" -Body @{
        opNo = 10; opName = '车'; machineId = $machineIds['CK6150']
        fixture = '三爪卡盘 + 中心架'; zHeight = 25.5; remark = '车外圆 Ø60、端面及内孔 Ø30H7'
    }
    Ok "  工序 10 车 id=$($d1op10.id)"

    $d1op20 = Invoke-Api -Method Post -Path "/api/drawings/$($d1.id)/operations" -Body @{
        opNo = 20; opName = '铣'; machineId = $machineIds['VMC850']
        fixture = '平口钳 + 等高垫块'; zHeight = 35.0; remark = '铣两侧扁位、钻攻 M8 螺纹孔'
    }
    Ok "  工序 20 铣 id=$($d1op20.id)"

    # 工序 10 的程序
    $p1 = Invoke-Api -Method Post -Path "/api/operations/$($d1op10.id)/programs" -Body @{
        programNo = 'O1001'; programName = '外圆粗车'; controller = 'FANUC 0i-TF'; remark = 'G71 粗车循环'
    }
    $null = Send-NcVersion -ProgramId $p1.id -FilePath (Get-SampleFile 'O1234_v1.nc') -ChangeNote '初版'
    $null = Send-NcVersion -ProgramId $p1.id -FilePath (Get-SampleFile 'O1234_v2.nc') -ChangeNote '增加精车刀 T02，转速与进给优化'
    Ok "  程序 O1001 id=$($p1.id)（2 个版本）"

    Invoke-Api -Method Put -Path "/api/programs/$($p1.id)/tools" -Body @{
        items = @(
            @{ seq = 1; toolId = (ToolIdOf 'T01'); toolNo = 'T01'; offsetNo = 'D01'; toolName = '外圆粗车刀'
                toolDia = 12.5; cornerRadius = 0.8; compAmount = 0.8; spindleSpeed = 1400; speedMode = 0
                feed = 0.25; feedMode = 1; cutDepth = 2.0; coolant = 1
                machiningContent = '粗车外圆与端面'; remark = 'CNMG120408' },
            @{ seq = 2; toolId = (ToolIdOf 'T02'); toolNo = 'T02'; offsetNo = 'D02'; toolName = '外圆精车刀'
                toolDia = 6.0; cornerRadius = 0.4; compAmount = 0.4; spindleSpeed = 180; speedMode = 1
                feed = 0.08; feedMode = 1; cutDepth = 0.25; coolant = 1
                machiningContent = '精车外圆 Ø60h6'; remark = 'G96 恒线速 180 m/min' },
            @{ seq = 3; toolId = (ToolIdOf 'T03'); toolNo = 'T03'; offsetNo = 'D03'; toolName = '切槽刀'
                toolDia = 3.0; cornerRadius = 0.2; compAmount = 0.2; spindleSpeed = 800; speedMode = 0
                feed = 0.06; feedMode = 1; cutDepth = 1.5; coolant = 1
                machiningContent = '退刀槽 Ø52×3'; remark = '刀宽 3mm' }
        )
    } | Out-Null
    Ok '  O1001 刀具刀补表已写入（3 把刀）'

    $p2 = Invoke-Api -Method Post -Path "/api/operations/$($d1op10.id)/programs" -Body @{
        programNo = 'O1002'; programName = '内孔精车'; controller = 'FANUC 0i-TF'; remark = 'Ø30H7 铰孔前精车'
    }
    $null = Send-NcVersion -ProgramId $p2.id -FilePath (Get-SampleFile 'O1234_v1.nc') -ChangeNote '初版'
    Ok "  程序 O1002 id=$($p2.id)（1 个版本）"

    # 工序 20 的程序
    $p3 = Invoke-Api -Method Post -Path "/api/operations/$($d1op20.id)/programs" -Body @{
        programNo = 'O2001'; programName = '铣扁位'; controller = 'FANUC 0i-MF'; remark = '两侧对称扁位'
    }
    $null = Send-NcVersion -ProgramId $p3.id -FilePath (Get-SampleFile 'O1234_v1.nc') -ChangeNote '初版'
    Ok "  程序 O2001 id=$($p3.id)（1 个版本）"

    Invoke-Api -Method Put -Path "/api/programs/$($p3.id)/tools" -Body @{
        items = @(
            @{ seq = 1; toolId = (ToolIdOf 'T04'); toolNo = 'T04'; offsetNo = 'D01'; toolName = 'Ø12 立铣刀'
                toolDia = 12.0; cornerRadius = 0.0; compAmount = 6.0; spindleSpeed = 3000; speedMode = 0
                feed = 800; feedMode = 0; cutDepth = 3.0; coolant = 1
                machiningContent = '粗铣两侧扁位'; remark = '' },
            @{ seq = 2; toolId = (ToolIdOf 'T05'); toolNo = 'T05'; offsetNo = 'D02'; toolName = 'Ø8 立铣刀'
                toolDia = 8.0; cornerRadius = 0.0; compAmount = 4.0; spindleSpeed = 4200; speedMode = 0
                feed = 600; feedMode = 0; cutDepth = 0.3; coolant = 1
                machiningContent = '精铣扁位至尺寸'; remark = '' },
            @{ seq = 3; toolId = (ToolIdOf 'T06'); toolNo = 'T06'; offsetNo = 'D03'; toolName = 'Ø10 钻头'
                toolDia = 10.0; cornerRadius = 0.0; compAmount = 5.0; spindleSpeed = 1200; speedMode = 0
                feed = 150; feedMode = 0; cutDepth = 12.0; coolant = 1
                machiningContent = '钻 M8 底孔 Ø6.8'; remark = '实际用 Ø6.8，此处按图纸 Ø10 位置示' },
            @{ seq = 4; toolId = (ToolIdOf 'T07'); toolNo = 'T07'; offsetNo = 'D04'; toolName = 'M8 丝锥'
                toolDia = 8.0; cornerRadius = 0.0; compAmount = 4.0; spindleSpeed = 400; speedMode = 0
                feed = 1.25; feedMode = 1; cutDepth = 15.0; coolant = 1
                machiningContent = '攻 M8 螺纹'; remark = '刚性攻丝' }
        )
    } | Out-Null
    Ok '  O2001 刀具刀补表已写入（4 把刀）'
} catch {
    Bad "建立图纸 A-1001 失败：$($_.Exception.Message)"
}

# --- 图纸 2：法兰 ----------------------------------------------------------
Step '建立图纸 A-1002 法兰（一序车）'
try {
    $d2 = Invoke-Api -Method Post -Path '/api/drawings' -Body @{
        drawingNo = 'A-1002'; name = '连接法兰'; customer = '无锡重工'
        material = '304 不锈钢'; drawingVersion = 'A'; remark = '不锈钢件，注意切削液浓度'
    }
    Ok "图纸 A-1002 id=$($d2.id)"

    $d2op10 = Invoke-Api -Method Post -Path "/api/drawings/$($d2.id)/operations" -Body @{
        opNo = 10; opName = '车'; machineId = $machineIds['CK6150']
        fixture = '三爪卡盘'; zHeight = 12.0; remark = '车内外圆、端面及螺栓孔倒角'
    }
    Ok "  工序 10 车 id=$($d2op10.id)"

    $p4 = Invoke-Api -Method Post -Path "/api/operations/$($d2op10.id)/programs" -Body @{
        programNo = 'O3010'; programName = '法兰外圆车削'; controller = 'FANUC 0i-TF'; remark = '不锈钢，转速需降低'
    }
    $null = Send-NcVersion -ProgramId $p4.id -FilePath (Get-SampleFile 'O1234_v1.nc') -ChangeNote '初版'
    Ok "  程序 O3010 id=$($p4.id)（1 个版本）"

    Invoke-Api -Method Put -Path "/api/programs/$($p4.id)/tools" -Body @{
        items = @(
            @{ seq = 1; toolId = (ToolIdOf 'T01'); toolNo = 'T01'; offsetNo = 'D01'; toolName = '外圆粗车刀'
                toolDia = 12.5; cornerRadius = 0.8; compAmount = 0.8; spindleSpeed = 800; speedMode = 0
                feed = 0.18; feedMode = 1; cutDepth = 1.5; coolant = 1
                machiningContent = '粗车外圆'; remark = '不锈钢转速已降低' },
            @{ seq = 2; toolId = (ToolIdOf 'T02'); toolNo = 'T02'; offsetNo = 'D02'; toolName = '外圆精车刀'
                toolDia = 6.0; cornerRadius = 0.4; compAmount = 0.4; spindleSpeed = 140; speedMode = 1
                feed = 0.1; feedMode = 1; cutDepth = 0.3; coolant = 1
                machiningContent = '精车外圆'; remark = '' }
        )
    } | Out-Null
    Ok '  O3010 刀具刀补表已写入（2 把刀）'
} catch {
    Bad "建立图纸 A-1002 失败：$($_.Exception.Message)"
}

# --- 汇总 ------------------------------------------------------------------
Step '完成'
$tree = @(Invoke-Api -Method Get -Path '/api/tree')
$opCount = 0
$progCount = 0
foreach ($d in $tree) {
    $opCount += @($d.operations).Count
    foreach ($o in $d.operations) { $progCount += @($o.programs).Count }
}
Ok "当前库中：$($tree.Count) 个图纸 / $opCount 道工序 / $progCount 个程序"

if ($global:FailCount -eq 0) {
    Write-Host ''
    Write-Host '==========================================' -ForegroundColor Green
    Write-Host '  演示数据灌入完成' -ForegroundColor Green
    Write-Host '==========================================' -ForegroundColor Green
    Write-Host ' 现在启动前端（frontend\dev.cmd），在左侧图纸列表里就能看到这些数据。' -ForegroundColor DarkGray
    exit 0
} else {
    Write-Host "  有 $($global:FailCount) 项失败" -ForegroundColor Red
    exit 1
}
