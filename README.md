# 0E7

## A Toolbox for Security

## Quick start
Please make sure you have golang and node.js environments
```shell
# build the frontend first
cd frontend
npm install
npm run build-only
# build the project
go mod tidy
go build
# rename format of 0e7_<platform>_<arch>(.type)
# 0e7_windows_amd64.exe
# 0e7_darwin_arm64
# 0e7_linux_386
```