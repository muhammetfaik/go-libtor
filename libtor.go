// go-libtor - Self-contained Tor from Go
// Copyright (c) 2018 Péter Szilágyi. All rights reserved.

// Package libtor is a self-contained static tor library.
package libtor

// This file is a simplified clone from github.com/cretz/bine/process/embedded.

/*
#include <stdlib.h>
#include <tor_api.h>

static char** makeCharArray(int size) {
	return calloc(sizeof(char*), size);
}
static void setArrayString(char **a, char *s, int n) {
	a[n] = s;
}
static void freeCharArray(char **a, int size) {
	int i;
	for (i = 0; i < size; i++)
		free(a[i]);
	free(a);
}
*/
import "C"
import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/cretz/bine/process"
)

// start creates a new tor process, returning a termination channel.
func start(args ...string) (chan int, error) {
	// Create the char array for the args
	args = append([]string{"tor"}, args...)

	charArray := C.makeCharArray(C.int(len(args)))
	for i, a := range args {
		C.setArrayString(charArray, C.CString(a), C.int(i))
	}
	// Build the tor configuration
	config := C.tor_main_configuration_new()
	if code := C.tor_main_configuration_set_command_line(config, C.int(len(args)), charArray); code != 0 {
		C.tor_main_configuration_free(config)
		C.freeCharArray(charArray, C.int(len(args)))
		return nil, fmt.Errorf("failed to set arguments: %v", int(code))
	}
	// Start tor and return
	done := make(chan int, 1)
	go func() {
		defer C.freeCharArray(charArray, C.int(len(args)))
		defer C.tor_main_configuration_free(config)
		done <- int(C.tor_run_main(config))
	}()
	return done, nil
}

// Creator implements the bine.process.Creator, permitting libtor to act as an API
// backend for the bine/tor Go interface.
var Creator process.Creator = new(embeddedCreator)

// embeddedCreator implements process.Creator, permitting libtor to act as an API
// backend for the bine/tor Go interface.
type embeddedCreator struct{}

// New implements process.Creator, creating a new embedded tor process.
func (embeddedCreator) New(ctx context.Context, args ...string) (process.Process, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return &embeddedProcess{ctx: ctx, args: args}, nil
}

// embeddedProcess implements process.Process, permitting libtor to act as an API
// backend for the bine/tor Go interface.
type embeddedProcess struct {
	ctx  context.Context
	args []string
	done chan int
}

// Start implements process.Process, starting up the libtor embedded process.
func (e *embeddedProcess) Start() error {
	if e.done != nil {
		return errors.New("already started")
	}
	done, err := start(e.args...)
	if err != nil {
		return err
	}
	e.done = done
	return nil
}

// Wait implements process.Process, blocking until the embedded process terminates.
func (e *embeddedProcess) Wait() error {
	if e.done == nil {
		return errors.New("not started")
	}
	select {
	case <-e.ctx.Done():
		return e.ctx.Err()

	case code := <-e.done:
		if code == 0 {
			return nil
		}
		return fmt.Errorf("embedded tor failed: %v", code)
	}
}

// EmbeddedControlConn implements process.Process, but is a noop in the current version.
func (e *embeddedProcess) EmbeddedControlConn() (net.Conn, error) {
	return nil, process.ErrControlConnUnsupported
}
