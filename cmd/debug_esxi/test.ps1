$jwt = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImFkbWluQGV4YW1wbGUuY29tIiwiZXhwIjoxNzc5NTc1OTIyLCJpYXQiOjE3Nzk1NDcxMjIsImlzcyI6InZtT3JiaXQiLCJyb2xlcyI6W10sInN1YiI6ImI2YjJmZWQzLWFkNmYtNDFkZS1iMDg0LTg5MzczZmMwN2ZmYiIsInVzZXJuYW1lIjoiYWRtaW4ifQ.tohgcLDdK3jkkps7fM2_aZj50qyKD0JmEL7tkasZtpM"
$r2 = Invoke-WebRequest -Uri "http://localhost:8080/api/v1/vms/d01a0d01-1c8f-44c1-99b9-8fdcd0f4d335/console" -Method POST -Headers @{"Authorization"="Bearer $jwt"} -ContentType "application/json" -Body "{}" -UseBasicParsing
$sess = ($r2.Content | ConvertFrom-Json).data
$rawUrl = $sess.url
$ticket = ($rawUrl -split "sessionTicket=")[1] -split "&" | Select-Object -First 1
$wssUrl = "wss://192.168.20.100/ticket/$ticket"
Write-Host "WSS URL: $wssUrl"
& go run ./cmd/debug_esxi/main.go $wssUrl
