# Smoke-test the cavimg-mcp stdio protocol without a container.
# Uses a .NET Process so we can hold stdin open until responses flush (a stdio MCP
# server tears the connection down on read-EOF, which can race ahead of flushing
# in-flight responses — real clients keep stdin open for the whole session).
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")

# BOM-less UTF-8 for the child's stdin: the implicit StreamWriter inherits the
# parent console's input encoding, and a UTF-8-with-BOM encoding would prepend a
# preamble the JSON decoder rejects ("invalid character 'ï'").
try { [Console]::InputEncoding = New-Object System.Text.UTF8Encoding($false) } catch {}

& go build -o cavimg-mcp.exe .
if ($LASTEXITCODE -ne 0) { throw "go build failed" }

$requests = @(
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}'
  '{"jsonrpc":"2.0","method":"notifications/initialized"}'
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
)

$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = (Resolve-Path ".\cavimg-mcp.exe").Path
$psi.RedirectStandardInput = $true
$psi.RedirectStandardOutput = $true
$psi.UseShellExecute = $false
$p = [System.Diagnostics.Process]::Start($psi)

foreach ($line in $requests) { $p.StandardInput.WriteLine($line) }
$p.StandardInput.Flush()
Start-Sleep -Milliseconds 1000   # let in-flight responses flush before EOF
$p.StandardInput.Close()

$out = $p.StandardOutput.ReadToEnd()
$p.WaitForExit()

$fail = $false
foreach ($tool in "detect_project","install_cavimg","list_image_usages","apply_cavimg") {
  if ($out -notmatch [regex]::Escape("`"$tool`"")) {
    Write-Host "MISSING tool: $tool"
    $fail = $true
  }
}
if ($fail) {
  Write-Host "SMOKE FAILED"
  Write-Host $out
  exit 1
}
Write-Host "SMOKE OK: all four tools listed"
