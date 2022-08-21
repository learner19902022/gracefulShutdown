//go:build darwin || windows || linux

package service

import (
	"os"
	"syscall"
)

func macosSignal() []os.Signal {
	signal := []os.Signal{
		os.Interrupt,
		os.Kill,
		syscall.SIGKILL,
		syscall.SIGSTOP,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGILL,
		syscall.SIGABRT,
		syscall.SIGSYS,
		syscall.SIGTERM,
	}
	return signal
}

func linuxSignal() []os.Signal {
	signal := []os.Signal{
		os.Interrupt,
		os.Kill,
		syscall.SIGKILL,
		syscall.SIGSTOP,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGILL,
		syscall.SIGABRT,
		syscall.SIGSYS,
		syscall.SIGTERM,
	}
	return signal
}

func windowsSignal() []os.Signal {
	signal := []os.Signal{
		os.Interrupt,
		os.Kill,
		syscall.SIGKILL,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGILL,
		syscall.SIGABRT,
		syscall.SIGTERM,
	}
	return signal
}
