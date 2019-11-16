CMD ?=

unit:
	./scripts/tests/unit
.PHONY: unit

build:
	./scripts/build $(CMD)
.PHONY: build

lint:
	./scripts/tests/lint
.PHONY: lint

terraform:
	./scripts/tests/terraform $(CMD)
.PHONY: terraform
