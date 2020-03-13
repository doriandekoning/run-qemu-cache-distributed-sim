#!/bin/bash
set +e

echo "Drive:       "$2
echo "Kernel:      "$3
echo "Cores:       "$4
echo "Trace events:"$5
echo "Trace file:  "$6
echo "Memory:      "$7


$1 -drive file=$2,format=qcow2\
  -accel tcg\
  -m $7,maxmem=$7\
  -nographic\
  -net nic -net user,hostfwd=tcp::2222-:22\
  -append "root=/dev/sda1 console=ttyS0 zswap.enabled=1 cgroup_enable=memory swapaccount=1 storage-driver=overlay2 norandmaps"\
  -no-reboot\
  -kernel $3\
  -monitor unix:/tmp/qemu-monitor,server,nowait\
  -smp $4 \
  -qmp unix:/tmp/qemu-qmp,server,nowait\
  -trace events=$5,file=$6
#  -drive "file=/media/ssd/disks/ubuntu-cloud-user-data.img,format=raw"
#root=/dev/sda3

  #-plugin  ~/qemuplugin/plugin.so
  #-qmp tcp:localhost:4444,server,nowait
  #-icount shift=4\

#  -accel tcg,thread=multi\

