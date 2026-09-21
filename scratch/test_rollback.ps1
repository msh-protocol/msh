$headers = @{ Authorization = "Bearer msh-admin-secret" }
$history = Invoke-RestMethod -Uri "http://127.0.0.1:9000/api/history" -Headers $headers
$found = $null
foreach ($h in $history) {
    if ($h.resp.files_changed -contains "msh.exe" -or $h.resp.files_changed -contains ".\msh.exe") {
        $found = $h
        break
    }
}

if ($found) {
    Write-Host "Found execution $($found.id) with msh.exe"
    try {
        $body = @{ id = $found.id; file = "msh.exe" } | ConvertTo-Json
        $res = Invoke-RestMethod -Uri "http://127.0.0.1:9000/api/rollback" -Method Post -Headers $headers -Body $body -ContentType "application/json"
        Write-Host "Rollback result: $res"
    } catch {
        Write-Host "Caught expected error: $($_.Exception.Message)"
        try {
            $stream = $_.Exception.Response.GetResponseStream()
            $reader = New-Object System.IO.StreamReader($stream)
            Write-Host "Backend body: $($reader.ReadToEnd())"
        } catch {}
    }
} else {
    Write-Host "No execution found with msh.exe in files_changed"
}
