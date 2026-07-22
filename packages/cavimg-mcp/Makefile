IMAGE ?= cavimg-mcp
WORKSPACE ?= $(CURDIR)

.PHONY: build run test smoke

build:
	podman build -t $(IMAGE) -f Containerfile .

run:
	podman run -i --rm --userns=keep-id \
		-v "$(WORKSPACE)":/workspace:Z \
		-e CAVIMG_WORKSPACE_ROOT=/workspace \
		$(IMAGE)

test:
	go test ./...

smoke:
	bash scripts/smoke.sh
