@echo off
echo Building Tailwind CSS for production...
cd frontend
npm run tailwind:build
cd ..

echo Building Wails application...
wails build -clean

echo Build complete! Check build/bin/blackbox-gui.exe
pause
