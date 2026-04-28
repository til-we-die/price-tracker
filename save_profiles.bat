@echo off
echo Saving all profiles...

REM CPU profile (30 seconds)
go run main.go -save-cpu -cpu-duration=30s

REM Memory profile
go run main.go -save-mem

REM Block profile
go run main.go -save-block

REM Mutex profile
go run main.go -save-mutex

REM Goroutine profile
go run main.go -save-goroutine

echo All profiles saved to ./profiles/
echo.
echo To view profiles:
echo go tool pprof -http=:8080 profiles\cpu.prof
echo go tool pprof -http=:8080 profiles\mem.prof
