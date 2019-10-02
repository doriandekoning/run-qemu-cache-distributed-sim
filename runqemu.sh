#!/bin/bash
$1\
  -drive file=$2,format=qcow2\
  -m 12G\
  -nographic\
  -net nic -net user,hostfwd=tcp::2222-:22\
  -append "root=/dev/sda3 console=ttyS0 zswap.enabled=1 cgroup_enable=memory swapaccount=1 storage-driver=overlay2 transparent_hugepage=never"\
  -no-reboot\
  -kernel $3\
  -monitor unix:/tmp/qemu-monitor,server,nowait\
  -smp 1\
  -trace events=$4,file=$5
