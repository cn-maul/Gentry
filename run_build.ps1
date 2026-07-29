param([string]$Arg)
$batPath = Join-Path $PSScriptRoot "build-windows.bat"
$proc = Start-Process -FilePath "cmd.exe" -ArgumentList "/c", "call", "`"$batPath`"", $Arg -Wait -NoNewWindow -RedirectStandardOutput "build_stdout.txt" -RedirectStandardError "build_stderr.txt" -PassThru
$proc.ExitCode
