#!/bin/bash
echo "Tracing enabled for guest mem before exec"
echo "trace-event guest_mem_store_before_exec on" | socat - UNIX-CONNECT:/tmp/qemu-monitor
echo "trace-event guest_mem_load_before_exec on" | socat - UNIX-CONNECT:/tmp/qemu-monitor
