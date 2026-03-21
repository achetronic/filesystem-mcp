:: build.cmd - Simple Windows build script for filesystem-mcp
:: Run with: .\build.cmd@echo off
setlocal EnableDelayedExpansion

echo.
echo =============================================
echo    filesystem-mcp Windows Build Script
echo =============================================
echo.

echo [1/3] Formatting code...
go fmt ./...
if %ERRORLEVEL% neq 0 (
    echo Formatting failed!
    exit /b 1
)

echo [2/3] Vetting code...
go vet ./...
if %ERRORLEVEL% neq 0 (
    echo Vetting failed!
    exit /b 1
)

echo [3/3] Building Windows binary...

:: Delete old binaries, if any
if exist bin\filesystem-mcp.exe (
    del /Q bin\filesystem-mcp.exe >nul 2>&1
)

go build -o bin\filesystem-mcp.exe -ldflags "-s -w" cmd\main.go
if %ERRORLEVEL% neq 0 (
    echo Build failed!
    exit /b 1
)

echo.
echo Build completed successfully:
echo   -> bin\filesystem-mcp.exe
echo.
echo You can now start the server, for example:
echo   filesystem-mcp.exe --config config.yaml
echo.
endlocal