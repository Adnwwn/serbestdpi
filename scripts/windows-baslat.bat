@echo off
REM Windows: CIFT TIKLA calistir. Derler ve proxy'yi turktelekom profiliyle baslatir.
cd /d "%~dp0\.."

where go >nul 2>nul
if errorlevel 1 (
  echo Go kurulu degil. https://go.dev/dl adresinden kurun.
  pause
  exit /b 1
)

echo SerbestDPI derleniyor...
go build -o bin\serbestdpi.exe .\cmd\serbestdpi
if errorlevel 1 (
  echo Derleme basarisiz.
  pause
  exit /b 1
)

echo.
echo Proxy baslatiliyor: 127.0.0.1:1080
echo Tarayicida SOCKS5 proxy olarak 127.0.0.1:1080 ayarlayin.
echo Durdurmak icin bu pencereyi kapatin (Ctrl+C).
echo.
bin\serbestdpi.exe run --profile turktelekom -v
pause
