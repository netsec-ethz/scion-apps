#!/bin/bash
set -x

printf "\n### Getting vendor libraries\n"
govendor sync -v
