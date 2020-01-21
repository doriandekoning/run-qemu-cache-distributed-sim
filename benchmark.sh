#!/bin/bash


go build -o bin/main

for i in `seq 0 3`;
do
    ./bin/main --config=config_qemu_only.yaml
    sleep 10
    ls -lah /media/ssd/testtrace
    ./killqemu.sh 
    rm /media/ssd/pgtable_dump/* 
    rm /media/ssd/testtrace
    echo "Sleeping"
    sleep 60
done
