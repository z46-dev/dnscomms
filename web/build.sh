#!/bin/bash

GOOS=js GOARCH=wasm go build -o ./web/public/main.wasm ./web/src