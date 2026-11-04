@echo off
setlocal
cd /d "%~dp0\.."

echo Baixando dependencias...
go mod tidy || goto erro

echo Rodando testes...
go test ./... || goto erro

echo Compilando...
go build -trimpath -tags desktop,production -ldflags "-w -s -H windowsgui" -o GMC-Monitor.exe . || goto erro

echo.
echo Pronto: GMC-Monitor.exe
goto fim

:erro
echo.
echo Deu erro. Veja a mensagem acima.
exit /b 1

:fim
endlocal
