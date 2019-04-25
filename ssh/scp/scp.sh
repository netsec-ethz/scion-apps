#!/bin/bash

scp -S "${BASH_SOURCE%/*}/../client/client" $@
