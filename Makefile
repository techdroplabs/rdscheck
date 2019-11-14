CMD ?=

unit:
	./scripts/tests/unit
.PHONY: unit

build:
	./scripts/build $(CMD)
.PHONY: build
