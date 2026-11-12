@echo off
setlocal
cd /d "%~dp0\.."

rem sem "go mod tidy": as dependências ficam travadas no go.sum (o CI
rem confere com "go mod tidy -diff"); se precisar mudar, faça à mão e confira
echo Conferindo dependencias...
go mod verify || goto erro

echo Rodando testes...
go test ./... || goto erro

echo Gerando icone e versao do .exe...
go run github.com/tc-hib/go-winres@v0.3.3 make --in build/windows/winres.json --arch amd64 || goto erro

echo Compilando...
go build -trimpath -tags desktop,production -ldflags "-w -s -H windowsgui" -o RadLog.exe . || goto erro

echo.
echo Pronto: RadLog.exe
goto fim

:erro
echo.
echo Deu erro. Veja a mensagem acima.
exit /b 1

:fim
endlocal
