#!/bin/bash
protoc --go_out=plugins=grpc:.  dicemagic.proto