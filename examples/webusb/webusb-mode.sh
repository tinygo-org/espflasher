#!/bin/sh
# Switch Linux USB serial boards between WebUSB and normal serial ports.
# udev only reads files ending in .rules, so renaming toggles the rule.
set -e

rule=/etc/udev/rules.d/61-espflasher-webusb-unbind.rules

case "$1" in
on)
	if [ ! -e "$rule" ]; then
		if [ ! -e "$rule.off" ]; then
			echo "$rule not found, install it first (see README.md)" >&2
			exit 1
		fi
		sudo mv "$rule.off" "$rule"
	fi
	;;
off)
	if [ -e "$rule" ]; then
		sudo mv "$rule" "$rule.off"
	fi
	;;
status)
	if [ -e "$rule" ]; then echo on; else echo off; fi
	exit 0
	;;
*)
	echo "usage: $0 on|off|status" >&2
	exit 2
	;;
esac

sudo udevadm control --reload-rules
echo "WebUSB mode $1. Replug the board to apply."
