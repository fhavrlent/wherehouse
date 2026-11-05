.PHONY: binary dev web build test fmt image clean

# `make` with no target must only compile and exit: tools such as GitHub code
# scanning run it to build the Go code. Uses whatever frontend is in web/dist;
# `make build` rebuilds the frontend first.
binary:
	@mkdir -p web/dist && touch web/dist/.gitkeep
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o wherehouse ./cmd/wherehouse

# Backend with the local dev login on :8095; run `make web` in a second terminal
# for the Vite dev server with hot reload on :5175.
dev:
	@mkdir -p web/dist && touch web/dist/.gitkeep
	WHEREHOUSE_DEV_LOGIN=true WHEREHOUSE_LISTEN=:8095 WHEREHOUSE_DATA_DIR=./data WHEREHOUSE_APP_URL=http://localhost:5175 go run ./cmd/wherehouse

web:
	cd web && npm install && npm run dev

build:
	cd web && npm ci && npm run build
	@touch web/dist/.gitkeep
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o wherehouse ./cmd/wherehouse

test:
	@mkdir -p web/dist && touch web/dist/.gitkeep
	go vet ./...
	go test ./...
	cd web && npm run check

# Format and auto-fix everything.
fmt:
	gofmt -w .
	cd web && npm run lint:fix && npm run format

image:
	docker build -t wherehouse:local .

clean:
	rm -rf wherehouse web/dist/assets web/dist/index.html
