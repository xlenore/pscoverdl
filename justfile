set dotenv-load := false

export GOCACHE := env('GOCACHE', justfile_directory() / '.cache/go-build')
export GOMODCACHE := env('GOMODCACHE', justfile_directory() / '.cache/go-mod')

_default:
    @just --list

test:
    go test ./... -count=1

lint:
    go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./...

build:
    ./build.sh

build-cli:
    mkdir -p dist/host
    go build -trimpath -o dist/host/pscoverdl ./cmd/pscoverdl

build-gui:
    mkdir -p dist/host
    go build -tags production -trimpath -o dist/host/pscoverdl-gui ./cmd/pscoverdl-gui

run-gui cli='dist/host/pscoverdl':
    just build-cli
    assetdir=cmd/pscoverdl-gui/frontend/dist go run -tags dev ./cmd/pscoverdl-gui -cli {{ quote(cli) }}

run-cli *args:
    go run ./cmd/pscoverdl {{ args }}

clean:
    rm -rf dist .cache
