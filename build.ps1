Write-Host "Building Tailwind CSS for production..." -ForegroundColor Green
Set-Location frontend
npm run tailwind:build
Set-Location ..

Write-Host "Building Wails application..." -ForegroundColor Green
wails build -clean

Write-Host "Build complete! Check build/bin/blackbox-gui.exe" -ForegroundColor Green
