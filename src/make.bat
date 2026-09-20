:: Copyright 2012 The Go Authors. All rights reserved.
:: Use of this source code is governed by a BSD-style
:: license that can be found in the LICENSE file.

:: Environment variables that control make.bat:
::
:: GKHOSTARCH: The architecture for host tools (compilers and
:: binaries).  Binaries of this type must be executable on the current
:: system, so the only common reason to set this is to set
:: GKHOSTARCH=386 on an amd64 machine.
::
:: GKARCH: The target architecture for installed packages and tools.
::
:: GKOS: The target operating system for installed packages and tools.
::
:: GK_GCFLAGS: Additional go tool compile arguments to use when
:: building the packages and commands.
::
:: GK_LDFLAGS: Additional go tool link arguments to use when
:: building the commands.
::
:: CGO_ENABLED: Controls cgo usage during the build. Set it to 1
:: to include all cgo related files, .c and .go file with "cgo"
:: build directive, in the build. Set it to 0 to ignore them.
::
:: CC: Command line to run to compile C code for GKHOSTARCH.
:: Default is "gcc".
::
:: CC_FOR_TARGET: Command line to run compile C code for GKARCH.
:: This is used by cgo. Default is CC.
::
:: FC: Command line to run to compile Fortran code.
:: This is used by cgo. Default is "gfortran".

@echo off

setlocal

if not exist make.bat (
	echo Must run make.bat from Go src directory.
	exit /b 1
)

:: Clean old generated file that will cause problems in the build.
del /F ".\pkg\runtime\runtime_defs.go" 2>NUL

:: Set GKROOT for build.
cd ..
set GOROOT_TEMP=%CD%
set GKROOT=
cd src
set vflag=
if x%1==x-v set vflag=-v
if x%2==x-v set vflag=-v
if x%3==x-v set vflag=-v
if x%4==x-v set vflag=-v

if not exist ..\bin\tool mkdir ..\bin\tool

:: Calculating GKROOT_BOOTSTRAP
if not "x%GKROOT_BOOTSTRAP%"=="x" goto bootstrapset
for /f "tokens=*" %%g in ('where go 2^>nul') do (
	setlocal
	call :nogoenv
	for /f "tokens=*" %%i in ('"%%g" env GOROOT 2^>nul') do (
		endlocal
		if /I not "%%i"=="%GOROOT_TEMP%" (
			set GKROOT_BOOTSTRAP=%%i
			goto bootstrapset
		)
	)
)

set bootgo=1.26.0
if "x%GKROOT_BOOTSTRAP%"=="x" if exist "%HOMEDRIVE%%HOMEPATH%\go%bootgo%" set GKROOT_BOOTSTRAP=%HOMEDRIVE%%HOMEPATH%\go%bootgo%
if "x%GKROOT_BOOTSTRAP%"=="x" if exist "%HOMEDRIVE%%HOMEPATH%\sdk\go%bootgo%" set GKROOT_BOOTSTRAP=%HOMEDRIVE%%HOMEPATH%\sdk\go%bootgo%
if "x%GKROOT_BOOTSTRAP%"=="x" set GKROOT_BOOTSTRAP=%HOMEDRIVE%%HOMEPATH%\Go1.4

:bootstrapset
if not exist "%GKROOT_BOOTSTRAP%\bin\go.exe" (
	echo ERROR: Cannot find %GKROOT_BOOTSTRAP%\bin\go.exe
	echo Set GKROOT_BOOTSTRAP to a working Go tree ^>= Go %bootgo%.
	exit /b 1
)
set GKROOT=%GOROOT_TEMP%
set GOROOT_TEMP=

setlocal
call :nogoenv
for /f "tokens=*" %%g IN ('"%GKROOT_BOOTSTRAP%\bin\go" version') do (set GOROOT_BOOTSTRAP_VERSION=%%g)
set GOROOT_BOOTSTRAP_VERSION=%GOROOT_BOOTSTRAP_VERSION:go version =%
echo Building Go cmd/dist using %GKROOT_BOOTSTRAP%. (%GOROOT_BOOTSTRAP_VERSION%)
if x%vflag==x-v echo cmd/dist
set GKROOT=%GKROOT_BOOTSTRAP%
set GKBIN=
"%GKROOT_BOOTSTRAP%\bin\go.exe" build -o cmd\dist\dist.exe .\cmd\dist || exit /b 1
endlocal
.\cmd\dist\dist.exe env -w -p >env.bat || exit /b 1
call .\env.bat
del env.bat
if x%vflag==x-v echo.

if x%1==x--dist-tool (
	mkdir "%GKTOOLDIR%" 2>NUL
	if not x%2==x (
		copy cmd\dist\dist.exe "%2"
	)
	move cmd\dist\dist.exe "%GKTOOLDIR%\dist.exe"
	goto :eof
)

:: Run dist bootstrap to complete make.bash.
:: Bootstrap installs a proper cmd/dist, built with the new toolchain.
:: Throw ours, built with the bootstrap toolchain, away after bootstrap.
.\cmd\dist\dist.exe bootstrap -a %* || exit /b 1
del .\cmd\dist\dist.exe
goto :eof

:: DO NOT ADD ANY NEW CODE HERE.
:: The bootstrap+del above are the final step of make.bat.
:: If something must be added, add it to cmd/dist's cmdbootstrap,
:: to avoid needing three copies in three different shell languages
:: (make.bash, make.bat, make.rc).

:nogoenv
set GK111MODULE=off
set GKENV=off
set GKOS=
set GKARCH=
set GKEXPERIMENT=
set GKFLAGS=
set GKTOOLCHAIN=local
set GO111MODULE=off
set GOENV=off
set GOOS=
set GOARCH=
set GOEXPERIMENT=
set GOFLAGS=
set GOTOOLCHAIN=local

exit /b 0
