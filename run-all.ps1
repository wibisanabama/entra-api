$services = @("auth-service", "event-service", "ticket-service", "payment-service", "cashless-service", "gate-service", "storage-service")

foreach ($svc in $services) {
    Write-Host "Starting $svc..."
    Start-Process -NoNewWindow -FilePath "go" -ArgumentList "run ./$svc/cmd/api" -WorkingDirectory "c:\Users\wibis\Documents\Code\Project\entra-api"
}

Write-Host "All services started."
