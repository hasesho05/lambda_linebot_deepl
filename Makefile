.PHONY: build

build:
	sam build

local:
	sam local start-api --env-vars env.json

