# goST-Link library

## About

Go library that provides support for interacting with ST-Link (> V2) to read out ram or SWO messages. It also includes a simple SeggerRTT interface which provides support for real time transfer.

The usb interface is based on the OpenOCD project
https://sourceforge.net/p/openocd/code

The rtt implemenation ist based on phryniszak reversed engineered strtt library
https://github.com/phryniszak/strtt

## Usage

For usage see in stRttLogger/

## Installation under Windows

Gostlink uses the gousb package. For installation see:

https://github.com/google/gousb

