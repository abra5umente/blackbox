# Blackbox Developer Quick Reference

## 🚀 **Quick Start**

```bash
# Clone and setup
git clone <repository>
cd blackbox
npm install

# Development
wails dev                    # Start dev server + Tailwind watcher

# Production build
npm run build:gui           # Build CSS + Wails executable
```

## 🛠️ **Essential Commands**

### **Development**
```bash
wails dev                    # Development server with Tailwind watching
npm run build:css           # Build Tailwind CSS only
```

### **Production**
```bash
npm run build:gui           # Complete production build
npm run build:css && wails build -clean  # Manual build
```

### **Cross-Platform Builds**
```bash
# Windows
build.bat

# PowerShell
.\build.ps1

# Any platform
npm run build:gui
```

## 📁 **Key Files & Directories**

### **Source Code**
- `main.go` - Wails GUI entrypoint
- `internal/ui/app.go` - GUI backend API
- `internal/audio/` - Audio capture system
- `frontend/src/index.html` - Source HTML for Tailwind

### **Configuration**
- `wails.json` - Wails project config
- `frontend/tailwind.config.js` - Tailwind CSS config
- `frontend/package.json` - Frontend dependencies
- `package.json` - Project build scripts

### **Build Outputs**
- `build/bin/blackbox-gui.exe` - Production executable
- `frontend/dist/output.css` - Generated Tailwind CSS
- `frontend/dist/index.html` - Production HTML

## 🎨 **Tailwind CSS Workflow**

### **Adding New Styles**
1. Edit `frontend/src/index.html` (source file)
2. Add Tailwind utility classes
3. CSS automatically rebuilds in development
4. For production: `npm run build:css`

### **CSS Build Process**
```bash
# Development (automatic)
wails dev                    # Watches and rebuilds CSS

# Production (manual)
cd frontend
npm run tailwind:build      # Generates output.css
```

### **Content Scanning**
- Tailwind scans `frontend/src/index.html`
- Generated CSS goes to `frontend/dist/output.css`
- Production HTML links to `/output.css`

## 🔧 **Development Patterns**

### **Adding New UI Elements**
1. **HTML**: Add to `frontend/src/index.html`
2. **Styling**: Use Tailwind utility classes
3. **JavaScript**: Add event handlers and logic
4. **Testing**: Verify in development and production

### **Backend API Changes**
1. **Method**: Add to `internal/ui/app.go`
2. **Binding**: Wails automatically generates bindings
3. **Frontend**: Call via `window.go.ui.App.MethodName()`

### **Audio Processing**
1. **Recorder**: Use `internal/audio/loopback.go` or `mic.go`
2. **WAV**: Use `internal/wav/writer.go` for file output
3. **Mixing**: Sample-wise averaging in S16LE format

## 🐛 **Troubleshooting**

### **Tailwind CSS Not Working**
```bash
# Check CSS generation
cd frontend
npm run tailwind:build

# Verify file sizes
ls -la dist/output.css

# Check content scanning
npx tailwindcss --content ./src/index.html
```

### **Build Issues**
```bash
# Clean build
rm -rf build/
npm run build:gui

# Check dependencies
npm install
cd frontend && npm install
```

### **Common Problems**
- **CSS not updating**: Restart `wails dev`
- **Build fails**: Ensure CSS is built before Wails build
- **Styling missing**: Check `frontend/src/index.html` exists
- **Executable large**: Normal - includes all frontend assets

## 📚 **Documentation Files**

- `README.md` - User and developer guide
- `agents.md` - AI agent reference
- `PROJECT_STATUS.md` - Comprehensive project status
- `DEVELOPER_QUICK_REFERENCE.md` - This file

## 🔗 **Useful Links**

- **Wails**: https://wails.io/
- **Tailwind CSS**: https://tailwindcss.com/
- **Go**: https://golang.org/
- **Whisper.cpp**: https://github.com/ggerganov/whisper.cpp

## 💡 **Pro Tips**

1. **Always use `frontend/src/index.html`** for Tailwind scanning
2. **Run `npm run build:css`** before `wails build` for production
3. **Use `wails dev`** for development - it handles CSS watching
4. **Check file sizes** - `output.css` should be ~2-3MB when working
5. **Source HTML first** - edit source, then verify in dist

---

**Need Help?** Check the documentation files or run `npm run build:gui` to test the build process.
