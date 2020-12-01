#!/bin/bash

#max:4000000
freq=2200000

print=$1

for i in {1..11}
do
    echo "${i} startup, setting ON"
    echo 1 > /sys/devices/system/cpu/cpu${i}/online
    sleep 0.5
done

while [ true ]
do
    sleep 0.5
    [ -z "$print" ] || printf "\033c"
    for i in {1..11}
    do
        [ -z "$print" ] || echo "${i}: $(cat /sys/devices/system/cpu/cpu${i}/online)"
    done
    break=no

    [ -z "$print" ] || cat /proc/cpuinfo | grep "^[c]pu MHz"

    [ -z "$print" ] || echo "Current:   $(cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq)"
    [ -z "$print" ] || echo "Threshold: ${freq}"

    if [ "$(cat /sys/devices/system/cpu/cpu1/online)" -eq "1" ]; then
        for count in {1..30}
        do
            if [ $(cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq) -gt ${freq} ]; then
                [ -z "$print" ] || echo "Went too high!"
                eval "break=yes"
                break
            else
                sleep "0.1"
            fi
        done
        if [ "$break" == "yes" ]; then
            continue
        fi
        for i in {1..11}
        do
            [ -z "$print" ] || echo "${i} Off"
            echo 0 > /sys/devices/system/cpu/cpu${i}/online
            sleep 0.1
        done
        sleep 1
    else
        for count in {1..30}
        do
            if [ $(cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq) -lt ${freq} ]; then
                [ -z "$print" ] || echo "Went too low!"
                eval "break=yes"
                break
            else
                sleep "0.1"
            fi
        done
        if [ "$break" == "yes" ]; then
            continue
        fi
        for i in {1..11}
        do
            [ -z "$print" ] || echo "${i} On"
            echo 1 > /sys/devices/system/cpu/cpu${i}/online
            sleep 0.1
        done
        sleep 1
    fi
done
