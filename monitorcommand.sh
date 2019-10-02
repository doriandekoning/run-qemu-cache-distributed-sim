#!/bin/bash
echo $1 | socat - UNIX-CONNECT:/tmp/qemu-monitor
