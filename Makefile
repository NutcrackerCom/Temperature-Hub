APP_NAME := temperature-hub
CMD_DIR  := cmd/server
MAIN     := $(CMD_DIR)/main.go
BIN_DIR  := bin
BIN      := $(BIN_DIR)/$(APP_NAME)

GO       := go
GOFLAGS  :=
LDFLAGS  := -s -w

.DEFAULT_GOAL := help

help:
	@echo "Targets:"
	@echo "  make build       - собрать бинарник"
	@echo "  make run         - собрать и запустить"
	@echo "  make test        - запустить тесты"
	@echo "  make clean       - удалить бинарники/артефакты"


$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

build: $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BIN) $(MAIN)
	@echo "Built: $(BIN)"

run: build
	@$(BIN)

clean:
	@rm -rf $(BIN_DIR)
	@# иногда полезно чистить кеш
	@$(GO) clean -cache -testcache -modcache
	@echo "Cleaned"