@echo off
REM Windows: CIFT TIKLA calistir. TUN modunu baslatir = TUM uygulamalar
REM (Discord, Spotify, oyunlar dahil) DPI'yi asar. Yonetici (UAC) gerekir.
REM
REM ONEMLI: Wintun surucusu gerekir. wintun.dll dosyasi serbestdpi.exe ile AYNI
REM klasorde olmali. https://www.wintun.net adresinden indirip mimarinize uygun
REM (cogu PC icin bin\amd64\wintun.dll) dosyayi bin\ icine "wintun.dll" koyun.

REM Yonetici degilsek kendimizi UAC ile yeniden baslat.
net session >nul 2>&1
if %errorLevel% neq 0 (
  powershell -NoProfile -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
  exit /b
)

cd /d "%~dp0\.."

if not exist bin\serbestdpi.exe (
  where go >nul 2>nul
  if errorlevel 1 (
    echo Go kurulu degil ve bin\serbestdpi.exe yok. https://go.dev/dl
    pause & exit /b 1
  )
  echo Derleniyor...
  go build -o bin\serbestdpi.exe .\cmd\serbestdpi || (echo Derleme basarisiz & pause & exit /b 1)
)

if not exist bin\wintun.dll (
  echo.
  echo UYARI: bin\wintun.dll bulunamadi - TUN acilmaz.
  echo https://www.wintun.net indirip wintun.dll dosyasini bin\ icine koyun.
  echo.
  pause
)

echo.
echo TUN modu baslatiliyor - TUM uygulamalar. Durdurmak icin bu pencereyi kapatin.
echo.
bin\serbestdpi.exe tun --profile generic
pause
