Get-ChildItem -Recurse -Include *.svg,*.png,*.ico -File | Where-Object { $_.FullName -notmatch 'node_modules|dist' } | ForEach-Object { Write-Host $_.FullName }
