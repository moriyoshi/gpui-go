//go:build gpui && gpui_prebuilt

package nativeshim

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"
)

var (
	sableRuntime unsafe.Pointer
	doorbell     *os.File
	pumpDone     chan struct{}
	tokens       atomic.Uint64
	waiting      sync.Map // token -> chan uint64
)

var backend *Backend

const platform = runtime.GOOS + "/" + runtime.GOARCH

// Register installs the published module for target. The generated packages
// under prebuilt/ call it from init, so it runs before main.
func Register(target string, b *Backend) {
	if target != platform {
		panic("gpui: prebuilt module for " + target + " registered on " + platform)
	}
	if backend != nil {
		panic("gpui: prebuilt module for " + target + " registered twice")
	}
	backend = b
}

// Ready reports whether a module was registered for this platform.
func Ready() error {
	if backend == nil {
		return fmt.Errorf("gpui: no prebuilt native module for %s; import _ %q", platform, "github.com/moriyoshi/gpui-go/prebuilt/"+platform)
	}
	return nil
}

func registered() *Backend {
	if backend == nil {
		panic(Ready())
	}
	return backend
}

// Start creates the Sable runtime and begins delivering its completions.
func Start() {
	sableRuntime = SableRuntimeNew()
	if sableRuntime == nil {
		panic("gpui: sable_runtime_new failed")
	}
	// Rust closes the original descriptor on teardown; Go owns this copy.
	duplicate, err := dupDoorbell(int(SableCompletionFD(sableRuntime)))
	if err != nil {
		panic("gpui: duplicate completion doorbell: " + err.Error())
	}
	doorbell = os.NewFile(duplicate, "gpui-completion")
	if doorbell == nil {
		panic("gpui: invalid completion doorbell")
	}
	pumpDone = make(chan struct{})
	go pump(doorbell)
}

// pump drains to empty on each wakeup, since doorbell writes can coalesce.
func pump(file *os.File) {
	defer close(pumpDone)
	buffer := make([]byte, 8)
	for {
		if _, err := file.Read(buffer); err != nil && (errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed)) {
			drainCompletions()
			return
		}
		drainCompletions()
	}
}

func drainCompletions() {
	var token, handle uint64
	for SableNextCompletion(sableRuntime, unsafe.Pointer(&token), unsafe.Pointer(&handle)) != 0 {
		done, ok := waiting.LoadAndDelete(token)
		if !ok {
			panic("gpui: completion for an unknown Sable token")
		}
		done.(chan uint64) <- handle
	}
}

// Call awaits one Rust operation. Cancellation aborts the Rust task, and the
// completion is still awaited so its handle and request bytes stay accounted for.
func Call(ctx context.Context, op uint32, request []byte) ([]byte, error) {
	token := tokens.Add(1)
	done := make(chan uint64, 1)
	waiting.Store(token, done)
	// Rust reads the request until its completion arrives, after this call returns.
	var pinner runtime.Pinner
	defer pinner.Unpin()
	var data unsafe.Pointer
	if len(request) != 0 {
		data = unsafe.Pointer(unsafe.SliceData(request))
		pinner.Pin(data)
	}
	SableCallContext(sableRuntime, op, data, uintptr(len(request)), token)
	var handle uint64
	select {
	case handle = <-done:
	case <-ctx.Done():
		SableCallCancel(token)
		handle = <-done
	}
	if handle == 0 { // Sable's cancellation sentinel.
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("gpui: call aborted by runtime shutdown")
	}
	return callResult(handle)
}

// callResult holds the buffer address as a uintptr because an empty result can
// carry a sub-page sentinel that the garbage collector rejects as a pointer.
func callResult(handle uint64) ([]byte, error) {
	var ok int32
	var address, length uintptr
	SableCallResult(handle, unsafe.Pointer(&ok), unsafe.Pointer(&address), unsafe.Pointer(&length))
	var result []byte
	if length != 0 && address != 0 {
		result = append(result, unsafe.Slice(*(**byte)(unsafe.Pointer(&address)), int(length))...)
	}
	SableCallFree(handle)
	if ok == 0 {
		return nil, errors.New(string(result))
	}
	return result, nil
}

// Shutdown frees Sable after every Call has returned. The pump is stopped
// before the runtime is freed because it dereferences the runtime pointer.
func Shutdown() {
	if sableRuntime == nil {
		return
	}
	SableRuntimeShutdown(sableRuntime)
	doorbell.Close()
	<-pumpDone
	SableRuntimeFree(sableRuntime)
	sableRuntime = nil
}

// FinalizeThread releases native TLS on the calling thread, which Go's
// runtime does not do for code linked in through generated bindings.
func FinalizeThread() { TLSFinalizeCurrentThread() }
