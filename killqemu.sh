#!/bin/bash

kill -KILL $(pgrep -u dorian qemu-system-x86) || echo "Qemu not running"
kill -KILL $(pgrep -u dorian simple) || echo "Sim not running"
